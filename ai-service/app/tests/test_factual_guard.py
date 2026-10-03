import pytest

from app.gateway.factual_guard import REVIEW_MARKER, guard_chapter_content
from app.gateway.openai_compatible_provider import _chapter_response_from_json
from app.schemas.generation import ChapterActionRequest, ChapterGenerateRequest, RetrievedKnowledgeRef, TenderRequirementRef


def request(**kwargs):
    return ChapterGenerateRequest(tenant_id="tenant", bid_document_id="bid", bid_part_id="part",
                                  chapter_id="chapter", chapter_title="服务方案", **kwargs)


@pytest.mark.parametrize("text", [
    "交付后提供12个月质量保修期。", "我方承诺质保期为两年。", "故障响应时间为2小时。",
    "我方投标报价为128万元。", "我方拥有丰富的施工经验。", "我司已完成排水工程项目A。",
    "我方具备市政施工总承包三级资质。", "我方将提供近三年内承接的类似工程业绩。",
    "我方承诺满足该资质要求，并提供有效的市政公用工程施工总承包三级证书。",
    "我方承诺在中标后7天内提交详细的施工组织设计。",
    "我方将在合同生效后30天交付。",
])
def test_unsupported_commitment_or_enterprise_fact_is_not_saved_as_body_fact(text):
    result, notes, issues = guard_chapter_content({"plain_text": text}, request())
    assert result["plain_text"].startswith(REVIEW_MARKER)
    assert text not in result["plain_text"]
    assert notes and issues


def test_same_number_in_delivery_or_different_semantics_does_not_support_warranty():
    payload = request(project_context={"delivery_period": "合同生效后12个月"},
        requirement_refs=[TenderRequirementRef(id="r", requirement="响应时限", source_text="响应时间12小时")])
    result, _, issues = guard_chapter_content({"plain_text": "质保期12个月，响应时间2小时。"}, payload)
    assert result["plain_text"].startswith(REVIEW_MARKER)
    assert {issue["kind"] for issue in issues} == {"质保承诺", "响应时限"}


def test_budget_is_not_supplier_bid_price_and_tender_qualification_not_company_ownership():
    payload = request(project_context={"project_budget": "128万元"}, requirement_refs=[
        TenderRequirementRef(id="r", requirement="资格", source_text="须具备三级资质")])
    _, _, issues = guard_chapter_content({"plain_text": "投标报价128万元。我方具备三级资质。"}, payload)
    assert {issue["kind"] for issue in issues} == {"投标报价", "企业事实"}


def test_grounded_commitments_and_proposed_plan_survive_without_mutating_input():
    payload = request(project_context={"delivery_period": "合同生效后30天"}, requirement_refs=[TenderRequirementRef(id="r", requirement="保修", source_text="质量保修期12个月；故障响应时间2小时。")])
    source = {"plain_text": "质量保修期12个月；故障响应时间2小时。拟在第5天完成拆除，合同生效后30天交付。"}
    result, notes, issues = guard_chapter_content(source, payload)
    assert result == source and not notes and not issues


def test_numbers_cannot_be_swapped_between_semantic_categories():
    payload = request(requirement_refs=[TenderRequirementRef(id="r", requirement="售后", source_text="质保期12个月，响应时间2小时。")])
    _, _, issues = guard_chapter_content({"plain_text": "质保期2小时，响应时间12个月。"}, payload)
    assert {issue["kind"] for issue in issues} == {"质保承诺", "响应时限"}


def test_delivery_period_is_grounded_in_context_not_a_model_default():
    payload = request(project_context={"delivery_period": "合同生效后30天"})
    assert not guard_chapter_content({"plain_text": "交付期限为合同生效后30天。"}, payload)[2]
    assert guard_chapter_content({"plain_text": "施工工期为60天。"}, payload)[2]


def test_submission_promise_cannot_reuse_unrelated_warranty_duration():
    text = "我方承诺在中标后7天内提交详细施工组织设计。"
    wrong = request(requirement_refs=[TenderRequirementRef(id="r", requirement="售后", source_text="质保期7天。")])
    assert guard_chapter_content({"plain_text": text}, wrong)[2]
    grounded = request(requirement_refs=[TenderRequirementRef(id="r", requirement="施工方案", source_text="中标后7天内提交施工组织设计。")])
    assert not guard_chapter_content({"plain_text": text}, grounded)[2]


@pytest.mark.parametrize('text', [
    '投标截止时间及开标时间为2026年11月15日10:00。',
    '投标截止时间：2026-11-15 10:00。',
    '投标截止时间为2026年11月16日09:30。',
    '开标时间为2026-11-15 09:30。',
    '投标文件正本一份、副本四份，电子版一份（U盘）。',
    '投标文件一正四副。',
    '我方承诺安全生产许可证在有效期内。',
    '我方安全生产许可证在有效期内（证书编号及有效期需人工核对后填写）。',
    '工程量需以招标文件中的工程量清单和图纸为准。',
    '我方提供[待澄清]年的质量保修期。',
    '计划合同签订后【待确认】天完成。',
    '确保在开标时间（2026-11-15 10:00）前完成所有递交手续。',
    '投标文件须递交至采购中心，确保在开标前送达。',
    '完成递交手续的时限为开标时间（2026-11-15 10:00）之前。',
    '建议施工工期为XX日历天（需人工确认）。',
    '业绩1：某市工程（合同金额约XX万元，完工时间XXXX年）。',
    '本投标人将提供近三年已完成的类似市政工程业绩。',
])
def test_actual_original_file_export_false_claims_require_review(text):
    payload = request(project_context={'submission_deadline':'2026-11-15 09:30','bid_opening_time':'2026-11-15 10:00'})
    guarded, _, issues = guard_chapter_content({'plain_text':text}, payload)
    assert guarded['plain_text'].startswith(REVIEW_MARKER) and issues


def test_submission_and_opening_times_can_share_date_but_not_time():
    text = '开标时间为2026年11月15日10:00，投标截止时间为2026年11月15日09:30。'
    payload = request(project_context={'submission_deadline':'2026-11-15 09:30','bid_opening_time':'开标时间：2026-11-15 10:00'})
    assert not guard_chapter_content({'plain_text':text},payload)[2]


def test_delivery_before_correct_deadline_not_confused_with_opening_in_next_clause():
    text = '开标时间2026-11-15 10:00，投标文件须在投标截止时间2026-11-15 09:30前递交。'
    payload = request(project_context={'submission_deadline':'2026-11-15 09:30','bid_opening_time':'2026-11-15 10:00'})
    assert not guard_chapter_content({'plain_text':text},payload)[2]
    same = request(project_context={'submission_deadline':'2026-11-15 09:30','bid_opening_time':'2026-11-15 09:30'})
    assert not guard_chapter_content({'plain_text':'在开标时间2026-11-15 09:30前完成递交手续。'},same)[2]


def test_numeric_placeholders_do_not_reject_legitimate_ascii_identifiers():
    assert not guard_chapter_content({'plain_text':'型号XX-300，编号GRAY-PS-2026-001。'},request())[2]


def test_literal_tender_copy_requirements_are_preserved():
    text = '投标文件正本一份、副本四份，电子版一份（U盘）。'
    payload = request(requirement_refs=[TenderRequirementRef(id='r',requirement='递交',source_text=text)])
    assert not guard_chapter_content({'plain_text':text},payload)[2]


def test_enterprise_quote_supports_literal_claim_not_other_claims():
    payload = request(retrieved_knowledge_refs=[RetrievedKnowledgeRef(chunk_id="k", document_id="d", title="企业资料", content="具备市政施工总承包三级资质")])
    result, _, issues = guard_chapter_content({"plain_text": "我方具备市政施工总承包三级资质。我方拥有丰富施工经验。"}, payload)
    assert "我方具备市政施工总承包三级资质。" in result["plain_text"]
    assert len(issues) == 1


def test_fragmented_tiptap_and_ai_action_cannot_bypass_guard():
    payload = ChapterActionRequest(**request().model_dump(), action="expand", current_plain_text="质保期12个月")
    doc = {"type": "doc", "content": [{"type": "paragraph", "content": [
        {"type": "text", "text": "质保期"}, {"type": "text", "text": "12", "marks": [{"type": "bold"}]},
        {"type": "text", "text": "个月。"}]}]}
    result, _, issues = guard_chapter_content({"tiptap_json": doc}, payload)
    assert result["tiptap_json"]["content"][0]["content"][0]["text"].startswith(REVIEW_MARKER)
    assert doc["content"][0]["content"][1]["text"] == "12"
    assert issues


def test_sanitized_body_cannot_keep_model_self_check_as_pass():
    payload = request(requirement_refs=[TenderRequirementRef(id="r", requirement="售后方案")])
    response = _chapter_response_from_json({"plain_text": "质保期12个月。", "self_check": {
        "status": "pass", "requirement_coverage": [{"requirement_id": "r", "satisfied": True}]}}, payload, "test", "model")
    assert response.self_check["status"] == "needs_review"
    assert response.self_check["requirement_coverage"][0]["satisfied"] is False
    assert response.needs_human_input
