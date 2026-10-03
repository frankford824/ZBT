import pytest

from app.pipelines.parse.tender_parser import (
    _tender_deadline,
    build_tender_structured_result,
    merge_tender_module_result,
    tender_module_source_context_records,
)
from app.schemas.knowledge import KnowledgeChunk, KnowledgeProcessResult
from app.schemas.tender import TenderParseRequest


def fixture(text):
    request = TenderParseRequest(tenant_id='tenant', bid_id='bid', bid_title='用户输入的简称',
                                bid_type='combined', file_id='file', object_key='tenant/test',
                                filename='test.docx', content_type='application/octet-stream')
    parsed = KnowledgeProcessResult(processed_title='test', summary='test', metadata={},
                                    chunks=[KnowledgeChunk(title='招标文件', content=text,
                                        section_path='正文', metadata={'chunk_id': 'parse-chunk-0001'})])
    return parsed, build_tender_structured_result(request, parsed)


@pytest.mark.parametrize('text,expected', [
    ('发布日期：2026-10-03\n投标截止时间：2026-11-15 09:30', '2026-11-15 09:30'),
    ('投标截止时间：2026年11月15日09时30分', '2026-11-15 09:30'),
    ('发布日期：2026-10-03\n开标时间：2026-11-16 10:00', None),
    ('投标截止时间：2026-02-30', None),
])
def test_deadline_is_labelled_and_keeps_clock(text, expected):
    assert _tender_deadline(text) == expected


def test_missing_source_does_not_create_generic_requirements():
    _, result = fixture('项目名称：城南雨水管道项目')
    assert result['deadline'] is None
    assert result['qualification_requirements'] == []
    assert result['scoring_points'] == []
    assert result['invalid_clause_risks'] == []
    assert result['modules']['annex']['requirement_items'] == []


def test_scope_and_contract_duration_are_distinct_from_bid_submission_date():
    parsed, result = fixture('项目名称：城南雨水管道项目\n投标截止时间：2026-11-15 09:30\n采购范围：雨水管道改造\n交付期限：合同生效后30天')
    basic = result['modules']['basic']
    assert basic['fields']['delivery_period'] == '合同生效后30天'
    assert basic['fields']['project_scope'] == '雨水管道改造'
    assert basic['fields']['deadline'] == '2026-11-15 09:30'
    assert any(item['field'] == 'delivery_period' and item['source_text'] == '交付期限：合同生效后30天' for item in basic['evidence'])
    merged = merge_tender_module_result(result, 'basic', {'fields': {'delivery_period': '2026-11-15'}, 'evidence': []},
                                        source_context_records=tender_module_source_context_records(parsed, 'basic'))
    assert merged['modules']['basic']['fields']['delivery_period'] == '合同生效后30天'


def test_user_layout_is_not_changed_by_technical_and_business_text():
    _, result = fixture('项目名称：城南雨水管道项目\n技术标：技术方案\n商务标：商务响应')
    assert result['bid_type'] == 'combined'


def test_real_quote_cannot_launder_an_invented_mandatory_annex():
    parsed, base = fixture('项目名称：城南雨水管道项目\n预算金额：100万元\n附件：报价表')
    quote = {'field': 'annex_items', 'value': '工程量清单', 'confidence': 0.99,
             'source_text': '预算金额：100万元', 'chunk_id': 'parse-chunk-0001'}
    result = merge_tender_module_result(base, 'annex', {
        'fields': {'annex_items': ['工程量清单']}, 'evidence': [quote],
        'requirement_items': [{'id': 'annex-004', 'requirement': '工程量清单',
                              'mandatory': True, 'source_ref': quote}]},
        source_context_records=tender_module_source_context_records(parsed, 'annex'))
    assert '工程量清单' not in str(result['modules']['annex']['fields'])
    assert all('工程量清单' not in item['requirement'] for item in result['requirement_items'])
    assert any('报价表' in item['requirement'] for item in result['requirement_items'])
    assert result['modules']['annex']['status'] == 'needs_review'


def test_wrong_date_with_real_quote_retains_original_deadline():
    parsed, base = fixture('项目名称：城南雨水管道项目\n投标截止时间：2026-11-15 09:30')
    result = merge_tender_module_result(base, 'basic', {
        'fields': {'deadline': '2026-10-17', 'bid_type': 'separated'},
        'evidence': [{'field': 'deadline', 'value': '2026-10-17', 'confidence': 0.99,
                      'source_text': '投标截止时间：2026-11-15 09:30', 'chunk_id': 'parse-chunk-0001'}]},
        source_context_records=tender_module_source_context_records(parsed, 'basic'))
    assert result['deadline'] == '2026-11-15 09:30'
    assert result['bid_type'] == 'combined'


def test_grounded_model_requirement_is_preserved():
    parsed, base = fixture('项目名称：城南雨水管道项目\n附件要求：须提交报价表')
    quote = {'field': 'annex_items', 'value': '须提交报价表', 'confidence': 0.99,
             'source_text': '附件要求：须提交报价表', 'chunk_id': 'parse-chunk-0001'}
    result = merge_tender_module_result(base, 'annex', {
        'fields': {'annex_items': ['须提交报价表']}, 'evidence': [quote],
        'requirement_items': [{'id': 'annex-001', 'requirement': '须提交报价表', 'mandatory': True, 'source_ref': quote}]},
        source_context_records=tender_module_source_context_records(parsed, 'annex'))
    assert any('须提交报价表' in value for value in result['modules']['annex']['fields']['annex_items'])
    assert any(item['requirement'] == '须提交报价表' and item['mandatory'] for item in result['requirement_items'])


def test_exact_quote_cannot_hide_a_wrong_score_metadata():
    parsed, base = fixture('项目名称：城南雨水管道项目\n评分：技术方案40分；类似业绩30分；报价30分')
    quote = {'field': 'scoring_points', 'value': '类似业绩30分', 'confidence': 0.99,
             'source_text': '评分：技术方案40分；类似业绩30分；报价30分', 'chunk_id': 'parse-chunk-0001'}
    result = merge_tender_module_result(base, 'evaluation', {
        'requirement_items': [{'id': 'evaluation-wrong', 'requirement': '类似业绩30分', 'score': 40, 'source_ref': quote}]},
        source_context_records=tender_module_source_context_records(parsed, 'evaluation'))
    assert all(item['id'] != 'evaluation-wrong' for item in result['requirement_items'])


def test_mentioning_an_annex_does_not_make_it_mandatory():
    parsed, base = fixture('项目名称：城南雨水管道项目\n附件格式：报价表')
    quote = {'field': 'annex_items', 'value': '报价表', 'confidence': 0.99,
             'source_text': '附件格式：报价表', 'chunk_id': 'parse-chunk-0001'}
    result = merge_tender_module_result(base, 'annex', {
        'requirement_items': [{'id': 'annex-model', 'requirement': '报价表', 'mandatory': True, 'source_ref': quote}]},
        source_context_records=tender_module_source_context_records(parsed, 'annex'))
    assert not next(item for item in result['requirement_items'] if item['id'] == 'annex-model')['mandatory']


def test_partial_model_summary_does_not_delete_original_mandatory_qualification():
    parsed, base = fixture('项目名称：城南雨水管道项目\n投标人须具备市政公用工程施工总承包三级资质。\n安全生产许可证须在有效期内。')
    quote = {'field': 'qualification_requirements', 'value': '市政公用工程施工总承包三级资质', 'confidence': 0.99,
             'source_text': '投标人须具备市政公用工程施工总承包三级资质。', 'chunk_id': 'parse-chunk-0001'}
    result = merge_tender_module_result(base, 'qualification', {
        'fields': {'qualification_requirements': ['市政公用工程施工总承包三级资质']}, 'evidence': [quote]},
        source_context_records=tender_module_source_context_records(parsed, 'qualification'))
    assert any('安全生产许可证' in value for value in result['qualification_requirements'])
    assert any('市政公用工程施工总承包三级' in value for value in result['qualification_requirements'])


def test_date_only_model_output_does_not_erase_original_deadline_clock():
    parsed, base = fixture('项目名称：城南雨水管道项目\n投标截止时间：2026-11-15 09:30')
    result = merge_tender_module_result(base, 'basic', {
        'fields': {'deadline': '2026-11-15'},
        'evidence': [{'field': 'deadline', 'value': '2026-11-15', 'confidence': 0.99,
                      'source_text': '投标截止时间：2026-11-15 09:30', 'chunk_id': 'parse-chunk-0001'}]},
        source_context_records=tender_module_source_context_records(parsed, 'basic'))
    assert result['deadline'] == '2026-11-15 09:30'
