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
    payload = request(requirement_refs=[TenderRequirementRef(id="r", requirement="保修", source_text="质量保修期12个月；故障响应时间2小时。")])
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
