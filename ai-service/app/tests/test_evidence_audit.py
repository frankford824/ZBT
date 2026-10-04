import copy
import json

import pytest

from app.gateway.evidence_audit import apply_targeted_repair, content_hash, validate_audit
from app.schemas.generation import ChapterGenerateRequest, TenderRequirementRef


def fixture():
    payload = ChapterGenerateRequest(
        tenant_id="t",
        bid_document_id="b",
        bid_part_id="p",
        chapter_id="c",
        chapter_title="项目理解",
        source_revision="2026-10-05T00:00:00Z",
        project_context={"submission_deadline": "2026-11-15 09:30"},
        requirement_refs=[
            TenderRequirementRef(id="r1", requirement="技术40分", source_text="技术方案40分。")
        ],
    )
    result = {"plain_text": "投标截止为2026-11-15 09:30。\n技术方案40分。"}
    raw = {
        "paragraphs": [
            {
                "index": 0,
                "kind": "tender_fact",
                "status": "supported",
                "evidence": [
                    {"source_id": "project:submission_deadline", "quote": "2026-11-15 09:30"}
                ],
            },
            {
                "index": 1,
                "kind": "tender_fact",
                "status": "supported",
                "evidence": [{"source_id": "requirement:r1", "quote": "技术方案40分。"}],
            },
        ],
        "requirements": [
            {"requirement_id": "r1", "status": "covered", "evidence": "技术方案40分。"}
        ],
    }
    return payload, result, raw


def test_exact_quote_and_entire_current_body_are_bound():
    payload, result, raw = fixture()
    audit = validate_audit(raw, result, payload)
    assert audit["status"] == "pass"
    assert audit['policy_version'] == 'source-bound-20261005-v2'
    assert audit["content_sha256"] == content_hash(result["plain_text"])
    assert audit["source_revision"] == payload.source_revision
    assert audit["paragraphs"][1]["evidence"][0]["quote"] == "技术方案40分。"


def test_legacy_drafting_hints_are_not_sent_to_writer_or_reviewer():
    from app.gateway.evidence_audit import audit_prompt
    from app.gateway.openai_compatible_provider import _chapter_prompt, _chapter_action_prompt
    from app.schemas.generation import ChapterActionRequest

    payload, result, _ = fixture()
    payload.requirement_refs[0].expected_response = 'UNVERIFIED-DEFAULT-投标函承诺函近三年业绩'
    payload.tender_requirements = ['技术方案40分；响应要点：UNVERIFIED-DEFAULT-投标函承诺函近三年业绩']
    action = ChapterActionRequest(**payload.model_dump(),action='expand',current_plain_text='草稿')
    for prompt in (_chapter_prompt(payload),_chapter_action_prompt(action),audit_prompt(result,payload)):
        assert 'UNVERIFIED-DEFAULT' not in prompt
        assert json.loads(prompt)['requirement_refs'][0]['source_text'] == '技术方案40分。'


def test_reviewer_order_cannot_make_current_evidence_look_stale():
    payload,result,raw=fixture()
    raw['paragraphs'].reverse()
    assert [row['index'] for row in validate_audit(raw,result,payload)['paragraphs']]==[0,1]


def test_requirement_paragraph_reference_uses_actual_body_not_model_transcription():
    payload, result, raw = fixture()
    raw['requirements'][0].update(paragraph_index=1, evidence='技术...40分')
    audit=validate_audit(raw,result,payload)
    assert audit['status']=='pass'
    assert audit['requirement_coverage'][0]['evidence']=='技术方案40分。'


@pytest.mark.parametrize('index', [-1, 2, True, None, '1'])
def test_invalid_requirement_paragraph_reference_cannot_pass(index):
    payload, result, raw = fixture()
    raw['requirements'][0]['paragraph_index']=index
    assert validate_audit(raw,result,payload)['status']=='needs_review'


def test_heading_reference_is_not_requirement_response_evidence():
    payload, result, raw = fixture()
    raw['paragraphs'][0]['kind']='heading'
    raw['requirements'][0]['paragraph_index']=0
    assert validate_audit(raw,result,payload)['status']=='needs_review'


def test_targeted_repair_preserves_approved_paragraph_and_formatting():
    payload, result, raw = fixture()
    result['tiptap_json']={'type':'doc','content':[
        {'type':'paragraph','content':[{'type':'text','text':line,'marks':[{'type':'bold'}]}]}
        for line in result['plain_text'].splitlines()]}
    raw['paragraphs'][0]['status']='unsupported'
    audit=validate_audit(raw,result,payload)
    saved=copy.deepcopy(result)
    repaired=apply_targeted_repair(result,audit,{'replacements':[{'index':0,'text':'投标截止为2026-11-15 09:30。'}]})
    assert result==saved
    assert repaired['tiptap_json']['content'][1]==saved['tiptap_json']['content'][1]
    assert repaired['tiptap_json']['content'][0]['content'][0]['text']=='投标截止为2026-11-15 09:30。'


def test_repair_cannot_change_good_paragraph_or_append_unrequested_content():
    payload,result,raw=fixture()
    raw['paragraphs'][0]['status']='unsupported'
    audit=validate_audit(raw,result,payload)
    with pytest.raises(ValueError):
        apply_targeted_repair(result,audit,{'replacements':[{'index':1,'text':'错误改写'}]})
    with pytest.raises(ValueError):
        apply_targeted_repair(result,audit,{'append_paragraphs':['未请求内容']})


def test_repair_can_append_a_missing_response_without_rewriting_body():
    payload,result,raw=fixture()
    raw['requirements'][0]['status']='missing'
    audit=validate_audit(raw,result,payload)
    repaired=apply_targeted_repair(result,audit,{'append_paragraphs':['拟按评分要求编制技术方案。']})
    assert repaired['plain_text'].startswith(result['plain_text'])
    assert repaired['plain_text'].endswith('拟按评分要求编制技术方案。')


@pytest.mark.parametrize('omit', [False, True])
def test_long_review_batches_preserve_global_indexes_and_fail_on_missing_rows(omit):
    from app.gateway.evidence_audit import review
    payload,_,_=fixture()
    payload.requirement_refs=[]
    result={'plain_text':'\n'.join(f'拟核验现场条件，计划步骤{index}。' for index in range(20))}
    class Reviewer:
        def generate_json(self,prompt,schema):
            batch=json.loads(prompt)
            assert len(batch['paragraphs'])<=8
            return {'paragraphs':[{'index':row['index'],'kind':'proposal','status':'supported','evidence':[]}
                                  for row in batch['paragraphs'] if not (omit and row['index']==12)],'requirements':[]}
    if omit:
        with pytest.raises(ValueError):
            review(Reviewer(),result,payload)
    else:
        audit=review(Reviewer(),result,payload)
        assert audit['status']=='pass'
        assert [p['index'] for p in audit['paragraphs']]==list(range(20))
        assert audit['review_request_count']==3


def test_long_review_keeps_sources_and_separately_checks_all_requirements():
    from app.gateway.evidence_audit import review
    payload,_,_=fixture()
    lines=['拟核验现场条件。']*20
    lines[15]='技术方案40分。'
    class Reviewer:
        def generate_json(self,prompt,schema):
            data=json.loads(prompt)
            assert 'requirement:r1' in data['sources']
            if data['requirement_refs']:
                return {'paragraphs':[],'requirements':[{'requirement_id':'r1','status':'covered','paragraph_index':15}]}
            return {'paragraphs':[{'index':r['index'],'kind':'proposal','status':'supported','evidence':[]}
                                  for r in data['paragraphs']],'requirements':[]}
    audit=review(Reviewer(),{'plain_text':'\n'.join(lines)},payload)
    assert audit['status']=='pass' and audit['review_request_count']==4
    assert audit['requirement_coverage'][0]['evidence']=='技术方案40分。'


@pytest.mark.parametrize('omit_required', [False, True])
def test_batch_empty_fields_may_be_absent_but_required_rows_cannot(omit_required):
    from app.gateway.evidence_audit import review
    payload,_,_=fixture()
    lines=['拟核验现场条件。']*20
    lines[15]='技术方案40分。'
    class Reviewer:
        def generate_json(self,prompt,schema):
            data=json.loads(prompt)
            if data['requirement_refs']:
                return {} if omit_required else {'requirements':[{'requirement_id':'r1','status':'covered','paragraph_index':15}]}
            return {'paragraphs':[{'index':r['index'],'kind':'proposal','status':'supported','evidence':[]}
                                  for r in data['paragraphs']]}
    if omit_required:
        with pytest.raises(ValueError):
            review(Reviewer(),{'plain_text':'\n'.join(lines)},payload)
    else:
        audit=review(Reviewer(),{'plain_text':'\n'.join(lines)},payload)
        assert audit['status']=='pass' and len(audit['paragraphs'])==20


@pytest.mark.parametrize('body,expected', [('拟进行现场条件核验。','pass'),('我方已拥有一级施工资质。','needs_review')])
def test_review_contract_retry_does_not_rewrite_or_override_factual_guards(body,expected):
    from app.gateway.evidence_audit import review
    payload,_,_=fixture()
    payload.requirement_refs=[]
    source={'plain_text':body}
    class Reviewer:
        calls=0
        def generate_json(self,prompt,schema):
            self.calls+=1
            correction='previous_review_validation_errors' in json.loads(prompt)
            return {'paragraphs':[{'index':0,'kind':'proposal' if correction else 'mixed','status':'supported','evidence':[]}], 'requirements':[]}
    provider=Reviewer()
    audit=review(provider,source,payload)
    assert audit['status']==expected and provider.calls==2
    assert source=={'plain_text':body}


def test_bad_review_citations_stay_rejected_after_one_contract_retry():
    from app.gateway.evidence_audit import review
    payload,result,raw=fixture()
    raw['paragraphs'][0]['evidence'][0]['quote']='伪造的来源内容'
    class Reviewer:
        calls=0
        def generate_json(self,prompt,schema):
            self.calls+=1
            if 'previous_review_validation_errors' in json.loads(prompt):
                return {'paragraphs':[raw['paragraphs'][0]],'requirements':[]}
            return raw
    provider=Reviewer()
    assert review(provider,result,payload)['status']=='needs_review'
    assert provider.calls==2


@pytest.mark.parametrize(
    "mutation",
    ["missing_paragraph", "duplicate_index", "missing_requirement", "unknown_requirement"],
)
def test_incomplete_or_wrong_review_fails_closed(mutation):
    payload, result, raw = fixture()
    if mutation == "missing_paragraph":
        raw["paragraphs"].pop()
    if mutation == "duplicate_index":
        raw["paragraphs"][1]["index"] = 0
    if mutation == "missing_requirement":
        raw["requirements"] = []
    if mutation == "unknown_requirement":
        raw["requirements"][0]["requirement_id"] = "fake"
    with pytest.raises(ValueError):
        validate_audit(raw, result, payload)


@pytest.mark.parametrize(
    "mutation",
    [
        "fake_quote",
        "fake_source",
        "enterprise_from_tender",
        "fake_coverage",
        "missing_coverage",
        "reviewer_rejects",
    ],
)
def test_unsupported_claims_cannot_become_pass(mutation):
    payload, result, raw = fixture()
    if mutation == "fake_quote":
        raw["paragraphs"][0]["evidence"][0]["quote"] = "2026-11-16"
    if mutation == "fake_source":
        raw["paragraphs"][0]["evidence"][0]["source_id"] = "fake"
    if mutation == "enterprise_from_tender":
        raw["paragraphs"][0]["kind"] = "enterprise_fact"
    if mutation == "fake_coverage":
        raw["requirements"][0]["evidence"] = "没有写入正文的响应"
    if mutation == "missing_coverage":
        raw["requirements"][0]["status"] = "missing"
    if mutation == "reviewer_rejects":
        raw["paragraphs"][0]["status"] = "unsupported"
    assert validate_audit(raw, result, payload)["status"] == "needs_review"


def test_self_check_preserves_saved_body_and_does_not_call_writer(monkeypatch):
    from app.gateway.openai_compatible_provider import (
        OpenAICompatibleProvider,
        OpenAICompatibleTarget,
    )
    from app.schemas.generation import ChapterActionRequest

    payload, result, raw = fixture()
    provider = OpenAICompatibleProvider(
        "test",
        base_url_env="TEST_BASE",
        api_key_env="TEST_KEY",
        target=OpenAICompatibleTarget(model="m"),
    )
    calls = []

    def generate(prompt, schema):
        calls.append(schema)
        return copy.deepcopy(raw)

    monkeypatch.setattr(provider, "generate_json", generate)
    request = ChapterActionRequest(
        **payload.model_dump(),
        action="self_check",
        current_plain_text=result["plain_text"],
        current_tiptap_json={
            "type": "doc",
            "content": [
                {"type": "paragraph", "content": [{"type": "text", "text": line}]}
                for line in result["plain_text"].splitlines()
            ],
        },
    )
    response = provider.chapter_action(request)
    assert response.tiptap_json == request.current_tiptap_json
    assert calls == ["EvidenceAudit"]
    assert response.self_check["evidence_audit"]["status"] == "pass"


def test_reviewer_unavailable_does_not_fall_back_to_writer_self_score(monkeypatch):
    from app.gateway.openai_compatible_provider import (
        OpenAICompatibleProvider,
        OpenAICompatibleTarget,
    )

    payload, result, raw = fixture()
    provider = OpenAICompatibleProvider(
        "test",
        base_url_env="TEST_BASE",
        api_key_env="TEST_KEY",
        target=OpenAICompatibleTarget(model="m"),
    )

    def generate(prompt, schema):
        if schema == "EvidenceAudit":
            raise TimeoutError("reviewer unavailable")
        return result

    monkeypatch.setattr(provider, "generate_json", generate)
    with pytest.raises(TimeoutError):
        provider.generate_chapter(payload)


def test_reviewer_cannot_approve_budget_as_supplier_price():
    from app.gateway.evidence_audit import review

    payload, result, raw = fixture()
    payload.project_context = {"project_budget": "55万元"}
    payload.requirement_refs = []
    result = {"plain_text": "我方报价为55万元。"}

    class MisleadingReviewer:
        def generate_json(self, *args):
            return {
                "paragraphs": [
                    {"index": 0, "kind": "proposal", "status": "supported", "evidence": []}
                ],
                "requirements": [],
            }

    audit = review(MisleadingReviewer(), result, payload)
    assert audit["status"] == "needs_review"
    assert audit["deterministic_issues"][0]["kind"] == "投标报价"
