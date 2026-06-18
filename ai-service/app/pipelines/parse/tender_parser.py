from __future__ import annotations

import json
import re
from pathlib import Path

from app.schemas.knowledge import KnowledgeProcessResult
from app.schemas.tender import (
    TenderParseFieldEvidence,
    TenderParseModule,
    TenderParseModuleResult,
    TenderParseRequest,
    TenderParseStructuredResult,
    TenderRequirementItem,
)


MERGEABLE_TOP_LEVEL_FIELDS = {
    "project_name",
    "bid_type",
    "deadline",
    "qualification_requirements",
    "invalid_clause_risks",
    "scoring_points",
    "outline",
    "material_suggestions",
}

MODULE_ORDER: tuple[TenderParseModule, ...] = (
    "basic",
    "qualification",
    "evaluation",
    "submission",
    "invalid_risk",
    "annex",
)

MODULE_TITLES: dict[TenderParseModule, str] = {
    "basic": "基础信息",
    "qualification": "资格要求",
    "evaluation": "评审办法",
    "submission": "递交要求",
    "invalid_risk": "否决风险",
    "annex": "附件格式",
}

MODULE_CONTEXT_KEYWORDS: dict[TenderParseModule, tuple[str, ...]] = {
    "basic": ("项目名称", "采购人", "招标人", "预算", "最高限价", "项目编号", "投标截止", "开标时间"),
    "qualification": ("资格", "资质", "业绩", "证书", "项目负责人", "联合体", "保证金", "信用"),
    "evaluation": ("评分", "分值", "评审", "技术方案", "商务", "价格", "服务方案", "实施方案"),
    "submission": ("递交", "提交", "投标文件", "密封", "签章", "份数", "电子", "开标"),
    "invalid_risk": ("无效", "废标", "否决", "重大偏差", "实质性响应", "投标有效期", "签章"),
    "annex": ("附件", "格式", "投标函", "报价表", "承诺函", "清单", "响应文件格式"),
}

MODULE_CHECKLIST_VERSION = "xparse-six-module-v1"
MODULE_CONTEXT_ROUTER_VERSION = "xparse-context-router-v1"
MODULE_CONTEXT_RECORD_LIMIT = 24
MODULE_CHECKLISTS: dict[TenderParseModule, dict[str, object]] = {
    "basic": {
        "required_fields": ("project_name", "purchaser", "project_code", "budget", "deadline", "opening_time"),
        "requirement_types": (),
        "source_policy": "每个关键事实必须对应招标文件连续原文、页码或 chunk。",
        "acceptance_checks": ("项目名称可定位", "截止/开标时间不凭空生成", "预算和编号缺失时进入人工复核"),
    },
    "qualification": {
        "required_fields": ("qualification_requirements",),
        "requirement_types": ("qualification", "certificate", "performance", "personnel", "credit", "deposit"),
        "source_policy": "每条资格要求必须保留原文摘录和 citation_id。",
        "acceptance_checks": ("强制资格项 mandatory=true", "证书/业绩/人员要求拆成独立 requirement_items"),
    },
    "evaluation": {
        "required_fields": ("scoring_points",),
        "requirement_types": ("scoring", "technical", "business", "price", "service", "delivery"),
        "source_policy": "评分项必须保留分值、原文摘录和引用位置。",
        "acceptance_checks": ("分值能解析则写入 score", "高分评分项 priority=high", "表格评分不得重排原文"),
    },
    "submission": {
        "required_fields": ("submission_requirements",),
        "requirement_types": ("submission", "signature", "seal", "copies", "deadline", "electronic_bid"),
        "source_policy": "递交、签章、密封、份数要求必须带来源引用。",
        "acceptance_checks": ("签章/密封/份数要求拆项", "电子标要求保留原文", "缺来源时 needs_review=true"),
    },
    "invalid_risk": {
        "required_fields": ("invalid_clause_risks",),
        "requirement_types": ("invalid_risk", "rejection", "material_deviation", "validity", "signature"),
        "source_policy": "否决/废标条款只能从原文抽取，不允许推断扩写。",
        "acceptance_checks": ("默认 mandatory=true", "高风险条款 priority=high", "缺页码或原文时必须人工复核"),
    },
    "annex": {
        "required_fields": ("annex_items",),
        "requirement_types": ("annex", "form", "quote_sheet", "commitment", "bill_of_quantities"),
        "source_policy": "附件和响应格式应带文件名、页码/块号和原文摘录。",
        "acceptance_checks": ("报价表/投标函/承诺函独立成项", "工程量清单保持表格块引用"),
    },
}


def module_parse_manifest() -> dict[str, object]:
    return {
        "version": MODULE_CHECKLIST_VERSION,
        "modules": {
            module: {
                **MODULE_CHECKLISTS[module],
                "title": MODULE_TITLES[module],
                "required_fields": list(MODULE_CHECKLISTS[module]["required_fields"]),
                "requirement_types": list(MODULE_CHECKLISTS[module]["requirement_types"]),
                "acceptance_checks": list(MODULE_CHECKLISTS[module]["acceptance_checks"]),
                "keywords": list(MODULE_CONTEXT_KEYWORDS[module]),
            }
            for module in MODULE_ORDER
        },
    }


def module_context_manifest(parsed: KnowledgeProcessResult) -> dict[str, object]:
    modules: dict[str, object] = {}
    for module in MODULE_ORDER:
        records = _module_context_records(parsed, module)
        modules[module] = _module_context_summary(records)
    return {"version": MODULE_CONTEXT_ROUTER_VERSION, "modules": modules}


def build_tender_structured_result(
    payload: TenderParseRequest,
    parsed: KnowledgeProcessResult,
) -> dict[str, object]:
    text = "\n".join(chunk.content for chunk in parsed.chunks)
    project_name = payload.bid_title or _project_name_from_text(text) or Path(payload.filename).stem
    has_tech_part = any(marker in text for marker in ("技术标", "技术方案", "实施方案"))
    has_business_part = any(marker in text for marker in ("商务标", "商务响应", "报价文件", "报价说明"))
    bid_type = "separated" if has_tech_part and has_business_part else "combined"
    outline_parts = _tender_outline_parts(text, bid_type)
    source_records = _source_records(parsed, payload)
    qualification_requirements, qualification_evidence = _keyword_values_with_evidence(
        source_records,
        "qualification_requirements",
        ("资格", "资质", "业绩", "证书", "项目负责人", "联合体"),
        fallback=("营业执照、授权及签章材料齐备", "按招标文件提交资格证明材料"),
    )
    invalid_clause_risks, invalid_evidence = _keyword_values_with_evidence(
        source_records,
        "invalid_clause_risks",
        ("无效", "废标", "否决", "投标保证金", "投标有效期", "签章"),
        fallback=("签章、报价、投标有效期等关键条款需人工复核",),
    )
    scoring_points, scoring_evidence = _keyword_values_with_evidence(
        source_records,
        "scoring_points",
        ("评分", "分值", "评审", "技术方案", "服务方案", "实施方案"),
        fallback=("实施方案完整性", "项目团队与业绩能力", "服务承诺与响应程度"),
    )
    submission_requirements, submission_evidence = _keyword_values_with_evidence(
        source_records,
        "submission_requirements",
        ("递交", "提交", "投标文件", "密封", "签章", "份数", "电子", "开标"),
        fallback=("按招标文件要求提交、签章、密封和递交投标文件",),
    )
    annex_items, annex_evidence = _keyword_values_with_evidence(
        source_records,
        "annex_items",
        ("附件", "格式", "投标函", "报价表", "承诺函", "清单", "响应文件格式"),
        fallback=("按招标文件附件和响应文件格式准备投标函、报价表和承诺函",),
    )
    deadline = _first_date(text)
    base_fields = {
        "project_name": project_name,
        "bid_type": bid_type,
        "deadline": deadline,
        "purchaser": _first_label_value(text, ("采购人", "招标人", "采购单位")),
        "project_code": _first_label_value(text, ("项目编号", "采购编号", "招标编号")),
        "budget": _first_budget(text),
        "location": _first_label_value(text, ("项目地点", "建设地点", "服务地点", "履约地点")),
        "opening_time": _first_line_with_keywords(text, ("开标时间", "开启时间")),
    }
    modules = _build_module_results(
        base_fields=base_fields,
        qualification_requirements=qualification_requirements,
        qualification_evidence=qualification_evidence,
        invalid_clause_risks=invalid_clause_risks,
        invalid_evidence=invalid_evidence,
        scoring_points=scoring_points,
        scoring_evidence=scoring_evidence,
        submission_requirements=submission_requirements,
        submission_evidence=submission_evidence,
        annex_items=annex_items,
        annex_evidence=annex_evidence,
        source_records=source_records,
    )
    field_evidence = [evidence for module in modules.values() for evidence in module.evidence]
    requirement_items = [
        item for module in modules.values() for item in module.requirement_items
    ]
    quality_gates = _quality_gates(parsed, modules, field_evidence, requirement_items)
    parse_metadata = {
        "parser": parsed.metadata.get("parser"),
        "page_count": parsed.metadata.get("page_count"),
        "chunk_count": len(parsed.chunks),
        "table_count": parsed.metadata.get("table_count", 0),
        "ocr": parsed.metadata.get("ocr"),
        "ocr_required": parsed.metadata.get("ocr_required", False),
        "ocr_page_count": parsed.metadata.get("ocr_page_count", 0),
        "module_count": len(modules),
        "requirement_count": len(requirement_items),
        "low_confidence_count": int(quality_gates["interpret"]["low_confidence_count"]),
        "missing_source_count": int(quality_gates["interpret"]["missing_source_count"]),
        "module_checklist_version": MODULE_CHECKLIST_VERSION,
        "module_checklist": module_parse_manifest(),
        "module_context_router_version": MODULE_CONTEXT_ROUTER_VERSION,
        "module_context": module_context_manifest(parsed),
    }
    structured = TenderParseStructuredResult(
        project_name=project_name,
        bid_type=bid_type,
        source_file={
            "file_asset_id": payload.file_id,
            "filename": payload.filename,
            "content_type": payload.content_type,
        },
        deadline=deadline,
        qualification_requirements=qualification_requirements,
        invalid_clause_risks=invalid_clause_risks,
        scoring_points=scoring_points,
        outline={"parts": outline_parts},
        material_suggestions=[
            {
                "title": "企业资质证书",
                "ref_type": "qualification",
                "reason": "响应资格审查要求",
                "selected": True,
            },
            {
                "title": "同类项目案例",
                "ref_type": "case",
                "reason": "支撑评分项中的业绩能力",
                "selected": True,
            },
            {
                "title": "实施方案素材",
                "ref_type": "solution",
                "reason": "覆盖技术、服务和交付响应",
                "selected": True,
            },
        ],
        modules=modules,
        field_evidence=field_evidence,
        requirement_items=requirement_items,
        quality_gates=quality_gates,
        parse_metadata=parse_metadata,
    )
    return structured.model_dump(mode="json")


def build_tender_parse_prompt(
    payload: TenderParseRequest,
    parsed: KnowledgeProcessResult,
    base_result: dict[str, object],
) -> str:
    source_text = "\n\n".join(chunk.content for chunk in parsed.chunks)[:24000]
    return json.dumps(
        {
            "task": "Extract a bid tender parse result from the source document.",
            "bid_title": payload.bid_title,
            "filename": payload.filename,
            "deterministic_result": base_result,
            "source_excerpt": source_text,
            "output_contract": {
                "project_name": "string",
                "bid_type": "combined | separated | custom",
                "deadline": "YYYY-MM-DD or null",
                "qualification_requirements": ["string"],
                "invalid_clause_risks": ["string"],
                "scoring_points": ["string"],
                "outline": {
                    "parts": [
                        {
                            "code": "string",
                            "title": "string",
                            "sort_order": 10,
                            "chapters": [
                                {
                                    "title": "string",
                                    "plain_text": "string",
                                    "sort_order": 10,
                                }
                            ],
                        }
                    ]
                },
                "material_suggestions": [
                    {
                        "title": "string",
                        "ref_type": "qualification | case | solution | other",
                        "reason": "string",
                        "selected": True,
                    }
                ],
                "modules": {
                    module: {
                        "fields": "module-specific facts with evidence-backed values",
                        "requirement_items": "requirements extracted from this module",
                    }
                    for module in MODULE_ORDER
                },
                "field_evidence": [
                    {
                        "field": "string",
                        "value": "any",
                        "confidence": "0.0-1.0",
                        "source_text": "short original excerpt",
                        "citation_id": "stable source citation id",
                        "reference_id": "AutoRFP-style source reference id",
                        "page_start": "number or null",
                        "chunk_id": "source chunk id or null",
                        "needs_review": "boolean",
                    }
                ],
                "requirement_items": [
                    {
                        "id": "stable requirement id",
                        "module": "qualification | evaluation | submission | invalid_risk | annex",
                        "type": "requirement category",
                        "requirement": "short requirement text",
                        "mandatory": "boolean",
                        "priority": "high | medium | low",
                        "source_ref": "evidence object",
                    }
                ],
            },
            "rules": [
                "Return only JSON.",
                "Use source facts first and keep uncertain values conservative.",
                "Do not invent dates, certificates, prices, or names that are not in the source.",
                "Keep source evidence and requirement items if the source supports them.",
                "Every source_ref must carry citation_id/reference_id when the source is traceable.",
                "Keep chapter titles concise and business-facing.",
            ],
        },
        ensure_ascii=False,
    )


def build_tender_module_prompt(
    payload: TenderParseRequest,
    parsed: KnowledgeProcessResult,
    base_result: dict[str, object],
    module: TenderParseModule,
) -> str:
    module_result = _module_record(base_result, module)
    context_records = _module_context_records(parsed, module)
    source_excerpt = "\n".join(_module_context_lines_from_records(context_records))[:10000]
    return json.dumps(
        {
            "task": "Improve one tender parse module using only source-backed facts.",
            "module": module,
            "module_title": MODULE_TITLES[module],
            "module_checklist": MODULE_CHECKLISTS[module],
            "module_checklist_version": MODULE_CHECKLIST_VERSION,
            "module_context_router_version": MODULE_CONTEXT_ROUTER_VERSION,
            "bid_title": payload.bid_title,
            "filename": payload.filename,
            "deterministic_module": module_result,
            "source_context": _compact_module_context_records(context_records),
            "source_excerpt": source_excerpt,
            "output_contract": {
                "module": module,
                "title": MODULE_TITLES[module],
                "status": "done | needs_review | empty",
                "fields": "object with module-specific fields",
                "evidence": [
                    {
                        "field": "string",
                        "value": "any",
                        "confidence": "0.0-1.0",
                        "source_text": "short original excerpt",
                        "citation_id": "stable source citation id",
                        "reference_id": "AutoRFP-style source reference id",
                        "page_start": "number or null",
                        "page_end": "number or null",
                        "chunk_id": "source chunk id or null",
                        "needs_review": "boolean",
                    }
                ],
                "requirement_items": [
                    {
                        "id": f"{module}-001",
                        "module": module,
                        "type": "requirement category",
                        "requirement": "short requirement text",
                        "priority": "high | medium | low",
                        "mandatory": "boolean",
                        "score": "number or null",
                        "expected_response": "what the bid should answer",
                        "source_ref": "evidence object",
                        "needs_review": "boolean",
                    }
                ],
                "warnings": ["string"],
            },
            "rules": [
                "Return only JSON.",
                "Only improve the requested module.",
                "Do not invent dates, certificates, prices, page numbers, names, or scores.",
                "Every changed field or requirement must include source_text or needs_review=true.",
                "Every traceable changed field or requirement must keep citation_id/reference_id.",
                "Prefer source_context chunk_id/page/table_block_id when emitting source evidence.",
                "Follow the module_checklist required_fields and requirement_types.",
                "Keep business wording concise and suitable for human review.",
            ],
        },
        ensure_ascii=False,
    )


def merge_tender_structured_result(
    base_result: dict[str, object],
    model_result: dict[str, object],
) -> dict[str, object]:
    merged = dict(base_result)
    overridden_fields: set[str] = set()
    for key in MERGEABLE_TOP_LEVEL_FIELDS:
        value = model_result.get(key)
        if _usable_model_value(value):
            merged[key] = value
            overridden_fields.add(key)
    if overridden_fields:
        _sync_model_overrides_to_modules(merged, overridden_fields)
    return merged


def merge_tender_module_result(
    base_result: dict[str, object],
    module: TenderParseModule,
    model_result: dict[str, object],
) -> dict[str, object]:
    merged = dict(base_result)
    modules = merged.get("modules")
    if not isinstance(modules, dict):
        return merged
    current = _module_record(merged, module)
    module_result = _normalized_model_module_result(module, current, model_result)
    modules = dict(modules)
    modules[module] = module_result
    merged["modules"] = modules
    _apply_module_to_compatible_fields(merged, module_result)
    _refresh_structured_indexes(merged)
    return merged


def mark_tender_module_enhancement_failed(
    base_result: dict[str, object],
    module: TenderParseModule,
    error_type: str,
) -> dict[str, object]:
    merged = dict(base_result)
    modules = merged.get("modules")
    if not isinstance(modules, dict):
        return merged
    module_result = _module_record(merged, module)
    module_result["status"] = "needs_review"
    warnings = module_result.get("warnings")
    if not isinstance(warnings, list):
        warnings = []
    warnings.append(f"{MODULE_TITLES[module]}增强未完成，已保留基础解析结果并等待人工确认。")
    module_result["warnings"] = warnings
    module_result["enhancement_error"] = {"type": error_type, "needs_review": True}
    modules = dict(modules)
    modules[module] = module_result
    merged["modules"] = modules
    _refresh_structured_indexes(merged)
    return merged


def _usable_model_value(value: object) -> bool:
    if value is None:
        return False
    if isinstance(value, str):
        return bool(value.strip())
    if isinstance(value, list | dict):
        return bool(value)
    return True


MODEL_FIELD_MODULES: dict[str, tuple[TenderParseModule, str]] = {
    "project_name": ("basic", "project_name"),
    "bid_type": ("basic", "bid_type"),
    "deadline": ("basic", "deadline"),
    "qualification_requirements": ("qualification", "qualification_requirements"),
    "invalid_clause_risks": ("invalid_risk", "invalid_clause_risks"),
    "scoring_points": ("evaluation", "scoring_points"),
}

MODEL_REQUIREMENT_CONFIGS: dict[str, tuple[TenderParseModule, str, bool, str, str]] = {
    "qualification_requirements": (
        "qualification",
        "qualification",
        True,
        "high",
        "提供资质、业绩、人员、证书等资格响应材料。",
    ),
    "invalid_clause_risks": (
        "invalid_risk",
        "invalid_risk",
        True,
        "high",
        "逐条规避否决投标和无效标条款。",
    ),
    "scoring_points": (
        "evaluation",
        "scoring",
        False,
        "high",
        "在章节方案中覆盖评分点并提供可验证支撑。",
    ),
}


def _module_record(result: dict[str, object], module: TenderParseModule) -> dict[str, object]:
    modules = result.get("modules")
    if not isinstance(modules, dict):
        return {"module": module, "title": MODULE_TITLES[module], "fields": {}, "evidence": [], "requirement_items": []}
    current = modules.get(module)
    if isinstance(current, dict):
        return dict(current)
    return {"module": module, "title": MODULE_TITLES[module], "fields": {}, "evidence": [], "requirement_items": []}


def _module_context_lines(parsed: KnowledgeProcessResult, module: TenderParseModule) -> list[str]:
    return _module_context_lines_from_records(_module_context_records(parsed, module))


def _module_context_records(parsed: KnowledgeProcessResult, module: TenderParseModule) -> list[dict[str, object]]:
    keywords = MODULE_CONTEXT_KEYWORDS[module]
    records: list[dict[str, object]] = []
    for chunk_index, chunk in enumerate(parsed.chunks, start=1):
        title_path = _chunk_title_path(chunk)
        title_text = f"{chunk.title} {title_path}".strip()
        keyword_lines = _keyword_context_lines(chunk.content, keywords, limit=24)
        reasons: list[str] = []
        score = 0
        if _contains_keyword(title_text, keywords):
            reasons.append("title_path")
            score += 5
        if keyword_lines:
            reasons.append("keyword_line")
            score += min(len(keyword_lines), 10)
        if not reasons:
            continue
        lines = keyword_lines
        if "title_path" in reasons:
            lines = _dedupe_context_lines([*keyword_lines, *_leading_context_lines(chunk.content, limit=8)])
        records.append(
            {
                "kind": "chunk",
                "chunk_id": _chunk_id(chunk, chunk_index),
                "title": chunk.title,
                "title_path": title_path,
                "page_start": chunk.page_start,
                "page_end": chunk.page_end,
                "reasons": reasons,
                "score": score,
                "lines": lines[:18],
            }
        )
    records.extend(_module_table_context_records(parsed, module))
    if not records:
        records = _fallback_module_context_records(parsed)
    return sorted(records, key=_module_context_sort_key)[:MODULE_CONTEXT_RECORD_LIMIT]


def _module_table_context_records(
    parsed: KnowledgeProcessResult,
    module: TenderParseModule,
) -> list[dict[str, object]]:
    table_blocks = parsed.metadata.get("table_blocks")
    if not isinstance(table_blocks, list):
        return []
    keywords = MODULE_CONTEXT_KEYWORDS[module]
    records: list[dict[str, object]] = []
    for table_index, table in enumerate(table_blocks[:50], start=1):
        if not isinstance(table, dict):
            continue
        table_text = _table_context_text(table)
        if not table_text or not _contains_keyword(table_text, keywords):
            continue
        matched_keywords = [keyword for keyword in keywords if keyword in table_text][:6]
        score = 4 + min(len(matched_keywords), 6)
        records.append(
            {
                "kind": "table",
                "table_block_id": _table_block_id(table, table_index),
                "table_source": str(table.get("source") or ""),
                "page_start": _object_int(table.get("page_start")),
                "page_end": _object_int(table.get("page_end")),
                "sheet": str(table.get("sheet") or ""),
                "slide": _object_int(table.get("slide")),
                "row_count": _object_int(table.get("row_count")) or len(_table_rows(table)),
                "reasons": ["table_block", "table_keyword"],
                "matched_keywords": matched_keywords,
                "score": score,
                "lines": _leading_context_lines(table_text, limit=16),
            }
        )
    return records


def _fallback_module_context_records(parsed: KnowledgeProcessResult) -> list[dict[str, object]]:
    records: list[dict[str, object]] = []
    for chunk_index, chunk in enumerate(parsed.chunks[:4], start=1):
        lines = _leading_context_lines(chunk.content, limit=8)
        if not lines:
            continue
        records.append(
            {
                "kind": "chunk",
                "chunk_id": _chunk_id(chunk, chunk_index),
                "title": chunk.title,
                "title_path": _chunk_title_path(chunk),
                "page_start": chunk.page_start,
                "page_end": chunk.page_end,
                "reasons": ["fallback"],
                "score": 0,
                "lines": lines,
            }
        )
    return records


def _module_context_lines_from_records(records: list[dict[str, object]]) -> list[str]:
    lines: list[str] = []
    seen: set[str] = set()
    for record in records:
        header = _module_context_header(record)
        for line in record.get("lines", []):
            value = re.sub(r"\s+", " ", str(line)).strip()
            if len(value) < 2:
                continue
            formatted = f"{header} {value}"
            if formatted in seen:
                continue
            seen.add(formatted)
            lines.append(formatted)
            if len(lines) >= 100:
                return lines
    return lines


def _compact_module_context_records(records: list[dict[str, object]]) -> list[dict[str, object]]:
    compact: list[dict[str, object]] = []
    for record in records[:16]:
        item: dict[str, object] = {
            "kind": record.get("kind"),
            "reasons": record.get("reasons", []),
            "score": record.get("score", 0),
        }
        for key in (
            "chunk_id",
            "table_block_id",
            "title_path",
            "page_start",
            "page_end",
            "table_source",
            "sheet",
            "slide",
            "row_count",
        ):
            value = record.get(key)
            if value not in (None, "", []):
                item[key] = value
        record_lines = [str(line).strip() for line in record.get("lines", []) if str(line).strip()]
        if record_lines:
            item["excerpt"] = "\n".join(record_lines[:8])[:1200]
        compact.append(item)
    return compact


def _module_context_summary(records: list[dict[str, object]]) -> dict[str, object]:
    chunk_ids = [
        str(record.get("chunk_id"))
        for record in records
        if record.get("kind") == "chunk" and str(record.get("chunk_id") or "").strip()
    ]
    table_block_ids = [
        str(record.get("table_block_id"))
        for record in records
        if record.get("kind") == "table" and str(record.get("table_block_id") or "").strip()
    ]
    reasons = sorted(
        {
            str(reason)
            for record in records
            for reason in record.get("reasons", [])
            if str(reason).strip()
        }
    )
    return {
        "record_count": len(records),
        "chunk_count": len(chunk_ids),
        "table_block_count": len(table_block_ids),
        "chunk_ids": chunk_ids[:12],
        "table_block_ids": table_block_ids[:12],
        "reasons": reasons,
    }


def _module_context_header(record: dict[str, object]) -> str:
    reasons = ",".join(str(reason) for reason in record.get("reasons", []) if str(reason).strip())
    page_start = record.get("page_start")
    page = f" page={page_start}" if page_start not in (None, "") else ""
    if record.get("kind") == "table":
        table_id = str(record.get("table_block_id") or "table")
        source = str(record.get("table_source") or "").strip()
        source_part = f" source={source}" if source else ""
        return f"[table={table_id}{source_part}{page} reason={reasons or 'table'}]"
    chunk_id = str(record.get("chunk_id") or "chunk")
    title_path = str(record.get("title_path") or "").strip()
    title_part = f" section={title_path}" if title_path else ""
    return f"[chunk={chunk_id}{page}{title_part} reason={reasons or 'context'}]"


def _module_context_sort_key(record: dict[str, object]) -> tuple[int, int, str]:
    score = int(record.get("score") or 0)
    page = _object_int(record.get("page_start")) or 999999
    identifier = str(record.get("chunk_id") or record.get("table_block_id") or "")
    return (-score, page, identifier)


def _keyword_context_lines(content: str, keywords: tuple[str, ...], *, limit: int) -> list[str]:
    matched: list[str] = []
    seen: set[str] = set()
    for line in content.splitlines():
        value = re.sub(r"\s+", " ", line).strip()
        if len(value) < 2 or value in seen:
            continue
        seen.add(value)
        if _contains_keyword(value, keywords):
            matched.append(value[:360])
            if len(matched) >= limit:
                break
    return matched


def _leading_context_lines(content: str, *, limit: int) -> list[str]:
    lines: list[str] = []
    seen: set[str] = set()
    for line in content.splitlines():
        value = re.sub(r"\s+", " ", line).strip()
        if len(value) < 2 or value in seen:
            continue
        seen.add(value)
        lines.append(value[:360])
        if len(lines) >= limit:
            break
    return lines


def _dedupe_context_lines(lines: list[str]) -> list[str]:
    deduped: list[str] = []
    seen: set[str] = set()
    for line in lines:
        value = re.sub(r"\s+", " ", str(line)).strip()
        if not value or value in seen:
            continue
        seen.add(value)
        deduped.append(value)
    return deduped


def _contains_keyword(text: str, keywords: tuple[str, ...]) -> bool:
    return any(keyword in text for keyword in keywords)


def _chunk_id(chunk: object, chunk_index: int) -> str:
    metadata = getattr(chunk, "metadata", {})
    if isinstance(metadata, dict):
        value = metadata.get("chunk_id") or metadata.get("id")
        if str(value or "").strip():
            return str(value).strip()
    return f"parse-chunk-{chunk_index:04d}"


def _chunk_title_path(chunk: object) -> str:
    section_path = str(getattr(chunk, "section_path", "") or "").strip()
    title = str(getattr(chunk, "title", "") or "").strip()
    return section_path or title


def _table_block_id(table: dict[str, object], table_index: int) -> str:
    for key in ("table_block_id", "id", "block_id"):
        value = str(table.get(key) or "").strip()
        if value:
            return value
    page = _object_int(table.get("page_start") or table.get("page"))
    source = _safe_ref_part(table.get("source") or "table")
    page_part = f"p{page}" if page else "p0"
    return f"{source}-{page_part}-{table_index:03d}"


def _table_context_text(table: dict[str, object]) -> str:
    md_table = str(table.get("md_table") or "").strip()
    rows = _table_rows(table)
    row_text = "\n".join(" | ".join(row) for row in rows[:20])
    parts = [
        str(table.get("sheet") or ""),
        str(table.get("source") or ""),
        md_table,
        row_text,
    ]
    return "\n".join(part for part in parts if part.strip()).strip()


def _table_rows(table: dict[str, object]) -> list[list[str]]:
    raw_rows = table.get("rows")
    if not isinstance(raw_rows, list):
        return []
    rows: list[list[str]] = []
    for row in raw_rows:
        if not isinstance(row, list):
            continue
        values = [str(cell).strip() for cell in row if str(cell).strip()]
        if values:
            rows.append(values)
    return rows


def _object_int(value: object) -> int | None:
    if value in (None, ""):
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _normalized_model_module_result(
    module: TenderParseModule,
    current: dict[str, object],
    model_result: dict[str, object],
) -> dict[str, object]:
    if str(model_result.get("module") or module) != module:
        model_result = {key: value for key, value in model_result.items() if key != "module"}
    fields = dict(current.get("fields") if isinstance(current.get("fields"), dict) else {})
    model_fields = model_result.get("fields")
    if isinstance(model_fields, dict):
        for key, value in model_fields.items():
            if _usable_model_value(value):
                fields[str(key)] = value
    for top_level_field, (target_module, module_field) in MODEL_FIELD_MODULES.items():
        if target_module == module and _usable_model_value(model_result.get(top_level_field)):
            fields[module_field] = model_result[top_level_field]
    evidence = _normalize_model_evidence(module, model_result.get("evidence"))
    if not evidence:
        evidence = [
            item
            for item in current.get("evidence", [])
            if isinstance(item, dict)
        ]
    evidence = _ensure_field_evidence_for_model_fields(fields, evidence)
    requirement_items = _normalize_model_requirement_items(
        module,
        model_result.get("requirement_items"),
        evidence,
    )
    if not requirement_items:
        requirement_items = [
            item
            for item in current.get("requirement_items", [])
            if isinstance(item, dict)
        ]
    warnings = [
        str(item)
        for item in model_result.get("warnings", [])
        if str(item).strip()
    ] if isinstance(model_result.get("warnings"), list) else []
    current_warnings = [
        str(item)
        for item in current.get("warnings", [])
        if str(item).strip()
    ] if isinstance(current.get("warnings"), list) else []
    status = str(model_result.get("status") or current.get("status") or "done")
    if status not in {"done", "needs_review", "empty"}:
        status = "needs_review"
    if any(bool(item.get("needs_review")) for item in evidence if isinstance(item, dict)):
        status = "needs_review"
    return {
        "module": module,
        "title": str(model_result.get("title") or current.get("title") or MODULE_TITLES[module]),
        "status": status,
        "fields": fields,
        "evidence": evidence,
        "requirement_items": requirement_items,
        "warnings": warnings or current_warnings,
    }


def _normalize_model_evidence(module: TenderParseModule, raw: object) -> list[dict[str, object]]:
    if not isinstance(raw, list):
        return []
    evidence: list[dict[str, object]] = []
    for item in raw[:40]:
        if not isinstance(item, dict):
            continue
        field = str(item.get("field") or "").strip()
        if not field:
            continue
        source_text = str(item.get("source_text") or "").strip()[:240]
        chunk_id = str(item.get("chunk_id") or "").strip() or None
        page_start = _int_or_none(item.get("page_start"))
        page_end = _int_or_none(item.get("page_end"))
        citation_id = (
            str(item.get("citation_id") or item.get("reference_id") or item.get("referenceId") or "").strip()
            or None
        )
        reference_id = (
            str(item.get("reference_id") or item.get("referenceId") or citation_id or "").strip()
            or None
        )
        document_id = str(item.get("document_id") or item.get("source_document_id") or "").strip() or None
        file_id = str(item.get("file_id") or "").strip() or None
        filename = str(item.get("filename") or "").strip() or None
        traceable = bool(source_text and (citation_id or reference_id or chunk_id or page_start or document_id or file_id))
        evidence.append(
            {
                "field": field,
                "value": item.get("value"),
                "confidence": _confidence(item.get("confidence"), 0.62 if source_text else 0.45),
                "source_text": source_text,
                "citation_id": citation_id,
                "reference_id": reference_id,
                "source_kind": str(item.get("source_kind") or "tender_document").strip() or "tender_document",
                "document_id": document_id,
                "file_id": file_id,
                "filename": filename,
                "page_start": page_start,
                "page_end": page_end,
                "bbox": item.get("bbox") if isinstance(item.get("bbox"), list) else None,
                "chunk_id": chunk_id,
                "traceable": traceable,
                "needs_review": bool(item.get("needs_review")) or not source_text or not traceable,
            }
        )
    return evidence


def _ensure_field_evidence_for_model_fields(
    fields: dict[str, object],
    evidence: list[dict[str, object]],
) -> list[dict[str, object]]:
    covered = {
        str(item.get("field"))
        for item in evidence
        if isinstance(item, dict) and str(item.get("field") or "").strip()
    }
    for field, value in fields.items():
        if field in covered or not _usable_model_value(value):
            continue
        evidence.append(_model_override_evidence(field, value))
    return evidence


def _normalize_model_requirement_items(
    module: TenderParseModule,
    raw: object,
    evidence: list[dict[str, object]],
) -> list[dict[str, object]]:
    if not isinstance(raw, list):
        return []
    evidence_by_field = {
        str(item.get("field")): item
        for item in evidence
        if isinstance(item, dict) and str(item.get("field") or "").strip()
    }
    items: list[dict[str, object]] = []
    for index, item in enumerate(raw[:20], start=1):
        if not isinstance(item, dict):
            continue
        requirement = str(item.get("requirement") or "").strip()
        if not requirement:
            continue
        raw_ref = item.get("source_ref")
        source_ref = _normalize_model_evidence(module, [raw_ref])[0] if isinstance(raw_ref, dict) else None
        if source_ref is None:
            source_ref = evidence_by_field.get(str(item.get("type") or "")) or _model_override_evidence("requirement", requirement)
        score = item.get("score")
        items.append(
            {
                "id": str(item.get("id") or f"{module}-{index:03d}"),
                "module": module,
                "type": str(item.get("type") or module),
                "requirement": requirement[:240],
                "priority": _priority(item.get("priority")),
                "mandatory": bool(item.get("mandatory")),
                "score": _float_or_none(score),
                "expected_response": str(item.get("expected_response") or "").strip()[:240],
                "status": str(item.get("status") or "unmapped")
                if str(item.get("status") or "unmapped") in {"unmapped", "planned", "covered", "needs_review"}
                else "needs_review",
                "source_ref": source_ref,
                "needs_review": bool(item.get("needs_review")) or bool(source_ref.get("needs_review")),
            }
        )
    return items


def _apply_module_to_compatible_fields(
    merged: dict[str, object],
    module_result: dict[str, object],
) -> None:
    module = str(module_result.get("module") or "")
    fields = module_result.get("fields")
    if not isinstance(fields, dict):
        return
    if module == "basic":
        for key in ("project_name", "bid_type", "deadline"):
            value = fields.get(key)
            if _usable_model_value(value):
                merged[key] = value
    elif module == "qualification" and _usable_model_value(fields.get("qualification_requirements")):
        merged["qualification_requirements"] = fields["qualification_requirements"]
    elif module == "evaluation" and _usable_model_value(fields.get("scoring_points")):
        merged["scoring_points"] = fields["scoring_points"]
    elif module == "invalid_risk" and _usable_model_value(fields.get("invalid_clause_risks")):
        merged["invalid_clause_risks"] = fields["invalid_clause_risks"]


def _confidence(value: object, default: float) -> float:
    parsed = _float_or_none(value)
    if parsed is None:
        return default
    return max(0.0, min(parsed, 1.0))


def _priority(value: object) -> str:
    priority = str(value or "medium").strip().lower()
    if priority in {"high", "medium", "low"}:
        return priority
    return "medium"


def _float_or_none(value: object) -> float | None:
    if value is None or value == "":
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _int_or_none(value: object) -> int | None:
    if value is None or value == "":
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _sync_model_overrides_to_modules(
    merged: dict[str, object],
    overridden_fields: set[str],
) -> None:
    modules = merged.get("modules")
    if not isinstance(modules, dict):
        return
    for field in overridden_fields:
        module_config = MODEL_FIELD_MODULES.get(field)
        if not module_config:
            continue
        module_key, module_field = module_config
        module_result = modules.get(module_key)
        if not isinstance(module_result, dict):
            continue
        module_fields = module_result.setdefault("fields", {})
        if isinstance(module_fields, dict):
            module_fields[module_field] = merged[field]
        module_evidence = module_result.setdefault("evidence", [])
        if isinstance(module_evidence, list):
            module_evidence[:] = [
                item
                for item in module_evidence
                if not (isinstance(item, dict) and item.get("field") == field)
            ]
            values = merged[field] if isinstance(merged[field], list) else [merged[field]]
            for value in values:
                module_evidence.append(_model_override_evidence(field, value))
        requirement_config = MODEL_REQUIREMENT_CONFIGS.get(field)
        if requirement_config:
            module, requirement_type, mandatory, priority, expected_response = requirement_config
            values = [str(item) for item in merged[field]] if isinstance(merged[field], list) else [str(merged[field])]
            module_result["requirement_items"] = [
                _model_override_requirement(
                    module,
                    requirement_type,
                    value,
                    index,
                    mandatory=mandatory,
                    priority=priority,
                    expected_response=expected_response,
                    evidence=_model_override_evidence(field, value),
                )
                for index, value in enumerate(values[:12], start=1)
                if value.strip()
            ]
        module_result["status"] = "needs_review"
    _refresh_structured_indexes(merged)


def _model_override_evidence(field: str, value: object) -> dict[str, object]:
    return {
        "field": field,
        "value": value,
        "confidence": 0.56,
        "source_text": "",
        "citation_id": None,
        "reference_id": None,
        "source_kind": "model_override",
        "document_id": None,
        "file_id": None,
        "filename": None,
        "page_start": None,
        "page_end": None,
        "bbox": None,
        "chunk_id": None,
        "traceable": False,
        "needs_review": True,
    }


def _model_override_requirement(
    module: TenderParseModule,
    requirement_type: str,
    value: str,
    index: int,
    *,
    mandatory: bool,
    priority: str,
    expected_response: str,
    evidence: dict[str, object],
) -> dict[str, object]:
    score = _score_value(value) if module == "evaluation" else None
    return {
        "id": f"{module}-{index:03d}",
        "module": module,
        "type": requirement_type,
        "requirement": value,
        "priority": "high" if mandatory or (score is not None and score >= 5) else priority,
        "mandatory": mandatory,
        "score": score,
        "expected_response": expected_response,
        "status": "needs_review",
        "source_ref": evidence,
        "needs_review": True,
    }


def _refresh_structured_indexes(merged: dict[str, object]) -> None:
    modules = merged.get("modules")
    if not isinstance(modules, dict):
        return
    field_evidence: list[dict[str, object]] = []
    requirement_items: list[dict[str, object]] = []
    module_count = 0
    for module in MODULE_ORDER:
        module_result = modules.get(module)
        if not isinstance(module_result, dict):
            continue
        module_count += 1
        evidence_items = module_result.get("evidence", [])
        if isinstance(evidence_items, list):
            field_evidence.extend(item for item in evidence_items if isinstance(item, dict))
        requirement_values = module_result.get("requirement_items", [])
        if isinstance(requirement_values, list):
            requirement_items.extend(item for item in requirement_values if isinstance(item, dict))
    low_confidence_count = sum(
        1
        for item in field_evidence
        if float(item.get("confidence") or 0) < 0.65 or bool(item.get("needs_review"))
    )
    missing_source_count = sum(1 for item in field_evidence if not str(item.get("source_text") or "").strip())
    merged["field_evidence"] = field_evidence
    merged["requirement_items"] = requirement_items
    quality_gates = merged.get("quality_gates")
    if not isinstance(quality_gates, dict):
        quality_gates = {}
    interpret = quality_gates.get("interpret")
    if not isinstance(interpret, dict):
        interpret = {}
    ocr_required = bool(interpret.get("ocr_required"))
    missing_modules = [module for module in MODULE_ORDER if module not in modules]
    interpret.update(
        {
            "status": "needs_review"
            if low_confidence_count or missing_source_count or ocr_required or missing_modules
            else "pass",
            "module_count": module_count,
            "required_modules": list(MODULE_ORDER),
            "missing_modules": missing_modules,
            "module_checklist_version": MODULE_CHECKLIST_VERSION,
            "requirement_count": len(requirement_items),
            "low_confidence_count": low_confidence_count,
            "missing_source_count": missing_source_count,
        }
    )
    quality_gates["interpret"] = interpret
    merged["quality_gates"] = quality_gates
    parse_metadata = merged.get("parse_metadata")
    if isinstance(parse_metadata, dict):
        parse_metadata["module_count"] = module_count
        parse_metadata["requirement_count"] = len(requirement_items)
        parse_metadata["low_confidence_count"] = low_confidence_count
        parse_metadata["missing_source_count"] = missing_source_count
        parse_metadata["module_checklist_version"] = MODULE_CHECKLIST_VERSION


def _source_records(parsed: KnowledgeProcessResult, payload: TenderParseRequest) -> list[dict[str, object]]:
    records: list[dict[str, object]] = []
    for chunk_index, chunk in enumerate(parsed.chunks, start=1):
        chunk_id = str(
            chunk.metadata.get("chunk_id")
            or chunk.metadata.get("id")
            or f"parse-chunk-{chunk_index:04d}"
        )
        page_start = chunk.page_start
        page_end = chunk.page_end
        for raw_line in chunk.content.splitlines():
            value = re.sub(r"\s+", " ", raw_line).strip(" :：\t")
            if len(value) < 2:
                continue
            records.append(
                {
                    "text": value,
                    "page_start": page_start,
                    "page_end": page_end,
                    "chunk_id": chunk_id,
                    "citation_id": _citation_id(payload.file_id, chunk_id, page_start, len(records) + 1),
                    "reference_id": _citation_id(payload.file_id, chunk_id, page_start, len(records) + 1),
                    "source_kind": "tender_document",
                    "document_id": payload.bid_id or payload.file_id,
                    "file_id": payload.file_id,
                    "filename": payload.filename,
                }
            )
    return records


def _build_module_results(
    *,
    base_fields: dict[str, object],
    qualification_requirements: list[str],
    qualification_evidence: list[TenderParseFieldEvidence],
    invalid_clause_risks: list[str],
    invalid_evidence: list[TenderParseFieldEvidence],
    scoring_points: list[str],
    scoring_evidence: list[TenderParseFieldEvidence],
    submission_requirements: list[str],
    submission_evidence: list[TenderParseFieldEvidence],
    annex_items: list[str],
    annex_evidence: list[TenderParseFieldEvidence],
    source_records: list[dict[str, object]],
) -> dict[TenderParseModule, TenderParseModuleResult]:
    basic_evidence = [
        _value_evidence("project_name", base_fields.get("project_name"), source_records, ("项目名称",)),
        _value_evidence("deadline", base_fields.get("deadline"), source_records, ("截止", "递交")),
    ]
    for field, keywords in (
        ("purchaser", ("采购人", "招标人", "采购单位")),
        ("budget", ("预算", "最高限价", "控制价")),
        ("project_code", ("项目编号", "采购编号", "招标编号")),
        ("location", ("项目地点", "建设地点", "服务地点", "履约地点")),
        ("opening_time", ("开标时间", "开启时间")),
    ):
        if base_fields.get(field) not in (None, "", []):
            basic_evidence.append(_value_evidence(field, base_fields.get(field), source_records, keywords))
    modules: dict[TenderParseModule, TenderParseModuleResult] = {
        "basic": _module_result(
            "basic",
            {
                key: value
                for key, value in base_fields.items()
                if value not in (None, "", [])
            },
            basic_evidence,
            [],
        ),
        "qualification": _module_result(
            "qualification",
            {"qualification_requirements": qualification_requirements},
            qualification_evidence,
            _requirement_items(
                "qualification",
                "qualification",
                qualification_requirements,
                qualification_evidence,
                mandatory=True,
                priority="high",
                expected_response="提供资质、业绩、人员、证书等资格响应材料。",
            ),
        ),
        "evaluation": _module_result(
            "evaluation",
            {"scoring_points": scoring_points},
            scoring_evidence,
            _requirement_items(
                "evaluation",
                "scoring",
                scoring_points,
                scoring_evidence,
                mandatory=False,
                priority="high",
                expected_response="在章节方案中覆盖评分点并提供可验证支撑。",
            ),
        ),
        "submission": _module_result(
            "submission",
            {"submission_requirements": submission_requirements},
            submission_evidence,
            _requirement_items(
                "submission",
                "submission",
                submission_requirements,
                submission_evidence,
                mandatory=True,
                priority="high",
                expected_response="按文件格式、签章、份数、密封和递交要求准备响应文件。",
            ),
        ),
        "invalid_risk": _module_result(
            "invalid_risk",
            {"invalid_clause_risks": invalid_clause_risks},
            invalid_evidence,
            _requirement_items(
                "invalid_risk",
                "invalid_risk",
                invalid_clause_risks,
                invalid_evidence,
                mandatory=True,
                priority="high",
                expected_response="逐条规避否决投标和无效标条款。",
            ),
        ),
        "annex": _module_result(
            "annex",
            {"annex_items": annex_items},
            annex_evidence,
            _requirement_items(
                "annex",
                "annex",
                annex_items,
                annex_evidence,
                mandatory=False,
                priority="medium",
                expected_response="按附件格式准备投标函、报价表、承诺函和清单文件。",
            ),
        ),
    }
    return {module: modules[module] for module in MODULE_ORDER}


def _module_result(
    module: TenderParseModule,
    fields: dict[str, object],
    evidence: list[TenderParseFieldEvidence],
    requirement_items: list[TenderRequirementItem],
) -> TenderParseModuleResult:
    usable_fields = {key: value for key, value in fields.items() if _usable_model_value(value)}
    needs_review = any(item.needs_review for item in evidence) or any(
        item.needs_review for item in requirement_items
    )
    status = "needs_review" if needs_review else "done"
    if not usable_fields and not requirement_items:
        status = "empty"
    warnings: list[str] = []
    if any(not item.source_text for item in evidence):
        warnings.append("部分字段缺少可定位来源，确认前需要人工复核。")
    return TenderParseModuleResult(
        module=module,
        title=MODULE_TITLES[module],
        status=status,
        fields=usable_fields,
        evidence=evidence,
        requirement_items=requirement_items,
        warnings=warnings,
    )


def _requirement_items(
    module: TenderParseModule,
    requirement_type: str,
    values: list[str],
    evidence: list[TenderParseFieldEvidence],
    *,
    mandatory: bool,
    priority: str,
    expected_response: str,
) -> list[TenderRequirementItem]:
    items: list[TenderRequirementItem] = []
    for index, value in enumerate(values[:12], start=1):
        source_ref = evidence[index - 1] if index - 1 < len(evidence) else None
        score = _score_value(value) if module == "evaluation" else None
        item_priority = "high" if mandatory or (score is not None and score >= 5) else priority
        items.append(
            TenderRequirementItem(
                id=f"{module}-{index:03d}",
                module=module,
                type=requirement_type,
                requirement=value,
                priority=item_priority,
                mandatory=mandatory,
                score=score,
                expected_response=expected_response,
                status="needs_review" if source_ref and source_ref.needs_review else "unmapped",
                source_ref=source_ref,
                needs_review=bool(source_ref.needs_review) if source_ref else True,
            )
        )
    return items


def _quality_gates(
    parsed: KnowledgeProcessResult,
    modules: dict[TenderParseModule, TenderParseModuleResult],
    field_evidence: list[TenderParseFieldEvidence],
    requirement_items: list[TenderRequirementItem],
) -> dict[str, object]:
    low_confidence_count = sum(1 for item in field_evidence if item.confidence < 0.65 or item.needs_review)
    missing_source_count = sum(1 for item in field_evidence if not item.source_text)
    missing_modules = [module for module in MODULE_ORDER if module not in modules]
    ocr_required = bool(parsed.metadata.get("ocr_required"))
    status = "needs_review" if low_confidence_count or missing_source_count or ocr_required or missing_modules else "pass"
    return {
        "interpret": {
            "status": status,
            "module_count": len(modules),
            "required_modules": list(MODULE_ORDER),
            "missing_modules": missing_modules,
            "module_checklist_version": MODULE_CHECKLIST_VERSION,
            "requirement_count": len(requirement_items),
            "low_confidence_count": low_confidence_count,
            "missing_source_count": missing_source_count,
            "ocr_required": ocr_required,
            "ocr_page_count": parsed.metadata.get("ocr_page_count", 0),
        }
    }


def _keyword_values_with_evidence(
    records: list[dict[str, object]],
    field: str,
    keywords: tuple[str, ...],
    *,
    fallback: tuple[str, ...],
    limit: int = 8,
) -> tuple[list[str], list[TenderParseFieldEvidence]]:
    values: list[str] = []
    evidence: list[TenderParseFieldEvidence] = []
    seen: set[str] = set()
    for record in records:
        value = str(record.get("text") or "").strip(" :：\t")
        if len(value) < 4 or not any(keyword in value for keyword in keywords):
            continue
        if value in seen:
            continue
        seen.add(value)
        clipped = value[:160]
        values.append(clipped)
        evidence.append(
            TenderParseFieldEvidence(
                field=field,
                value=clipped,
                confidence=0.78,
                source_text=clipped,
                citation_id=str(record.get("citation_id") or "") or None,
                reference_id=str(record.get("reference_id") or record.get("citation_id") or "") or None,
                source_kind=str(record.get("source_kind") or "tender_document"),
                document_id=str(record.get("document_id") or "") or None,
                file_id=str(record.get("file_id") or "") or None,
                filename=str(record.get("filename") or "") or None,
                page_start=_record_int(record, "page_start"),
                page_end=_record_int(record, "page_end"),
                chunk_id=str(record.get("chunk_id") or ""),
                traceable=True,
                needs_review=False,
            )
        )
        if len(values) >= limit:
            break
    if values:
        return values, evidence
    fallback_values = list(fallback)
    return fallback_values, [
        TenderParseFieldEvidence(
            field=field,
            value=value,
            confidence=0.35,
            source_text="",
            source_kind="fallback",
            traceable=False,
            needs_review=True,
        )
        for value in fallback_values
    ]


def _value_evidence(
    field: str,
    value: object,
    records: list[dict[str, object]],
    keywords: tuple[str, ...],
) -> TenderParseFieldEvidence:
    value_text = "" if value is None else str(value).strip()
    matched_record: dict[str, object] | None = None
    for record in records:
        source_text = str(record.get("text") or "")
        if value_text and value_text in source_text:
            matched_record = record
            break
        if any(keyword in source_text for keyword in keywords):
            matched_record = record
            break
    if matched_record:
        source_text = str(matched_record.get("text") or "")[:180]
        return TenderParseFieldEvidence(
            field=field,
            value=value,
            confidence=0.82 if value_text else 0.45,
            source_text=source_text,
            citation_id=str(matched_record.get("citation_id") or "") or None,
            reference_id=str(matched_record.get("reference_id") or matched_record.get("citation_id") or "") or None,
            source_kind=str(matched_record.get("source_kind") or "tender_document"),
            document_id=str(matched_record.get("document_id") or "") or None,
            file_id=str(matched_record.get("file_id") or "") or None,
            filename=str(matched_record.get("filename") or "") or None,
            page_start=_record_int(matched_record, "page_start"),
            page_end=_record_int(matched_record, "page_end"),
            chunk_id=str(matched_record.get("chunk_id") or ""),
            traceable=bool(source_text),
            needs_review=not bool(value_text),
        )
    return TenderParseFieldEvidence(
        field=field,
        value=value,
        confidence=0.55 if value_text else 0.25,
        source_text="",
        source_kind="fallback",
        traceable=False,
        needs_review=True,
    )


def _citation_id(file_id: str, chunk_id: str, page_start: int | None, ordinal: int) -> str:
    page = page_start if page_start is not None else 0
    return f"tender:{_safe_ref_part(file_id)}:{_safe_ref_part(chunk_id)}:p{page}:l{ordinal}"


def _safe_ref_part(value: object) -> str:
    text = str(value or "").strip()
    text = re.sub(r"[^a-zA-Z0-9_.-]+", "-", text)
    return text.strip("-") or "unknown"


def _record_int(record: dict[str, object], key: str) -> int | None:
    value = record.get(key)
    if value is None:
        return None
    try:
        return int(value)
    except (TypeError, ValueError):
        return None


def _project_name_from_text(text: str) -> str:
    lines = _candidate_title_lines(text)
    label_value = _project_label_value(lines)
    if label_value:
        return label_value
    for index, line in enumerate(lines[:24]):
        if _project_title_noise(line):
            continue
        candidate_lines = [line]
        for next_line in lines[index + 1 : index + 4]:
            if _project_title_noise(next_line):
                break
            joined = "".join(candidate_lines)
            if "项目" in joined and len(_compact_project_title(joined)) >= 10:
                break
            candidate_lines.append(next_line)
            if "项目" in next_line:
                break
        candidate = _compact_project_title("".join(candidate_lines))
        if "项目" in candidate and 6 <= len(candidate) <= 120:
            return candidate
    if lines:
        return _compact_project_title(lines[0])[:120]
    return ""


def _candidate_title_lines(text: str) -> list[str]:
    lines: list[str] = []
    for raw_line in text.splitlines():
        value = re.sub(r"\s+", " ", raw_line).strip(" :：\t")
        if not value or value.startswith("["):
            continue
        lines.append(value)
        if len(lines) >= 80:
            break
    return lines


def _project_label_value(lines: list[str]) -> str:
    labels = ("项目名称", "采购项目名称", "招标项目名称", "项目名称及编号")
    for line in lines:
        for label in labels:
            match = re.search(rf"{re.escape(label)}\s*[:：]\s*(.+)", line)
            if not match:
                continue
            value = _compact_project_title(match.group(1))
            if value:
                return value[:120]
    return ""


def _project_title_noise(line: str) -> bool:
    compact = _compact_project_title(line)
    if not compact:
        return True
    noise_keywords = (
        "采购文件",
        "招标文件",
        "询比文件",
        "响应文件",
        "投标文件",
        "目录",
        "采购人",
        "招标人",
        "采购代理",
        "供应商",
        "盖单位章",
    )
    return any(keyword in compact for keyword in noise_keywords)


def _compact_project_title(value: str) -> str:
    return re.sub(r"\s+", "", value).strip(" :：\t")


def _keyword_lines(text: str, keywords: tuple[str, ...], *, fallback: tuple[str, ...]) -> list[str]:
    matches: list[str] = []
    seen: set[str] = set()
    for line in text.splitlines():
        value = re.sub(r"\s+", " ", line).strip(" :：\t")
        if len(value) < 4 or not any(keyword in value for keyword in keywords):
            continue
        if value in seen:
            continue
        seen.add(value)
        matches.append(value[:160])
        if len(matches) >= 8:
            break
    return matches or list(fallback)


def _first_date(text: str) -> str | None:
    patterns = (
        r"20\d{2}[-/.年]\d{1,2}[-/.月]\d{1,2}日?",
        r"20\d{2}\s*年\s*\d{1,2}\s*月\s*\d{1,2}\s*日",
    )
    for pattern in patterns:
        match = re.search(pattern, text)
        if not match:
            continue
        value = match.group(0).replace("年", "-").replace("月", "-").replace("日", "")
        value = value.replace("/", "-").replace(".", "-")
        parts = [part.strip() for part in value.split("-") if part.strip()]
        if len(parts) == 3:
            return f"{int(parts[0]):04d}-{int(parts[1]):02d}-{int(parts[2]):02d}"
    return None


def _first_label_value(text: str, labels: tuple[str, ...]) -> str | None:
    for line in text.splitlines():
        value = re.sub(r"\s+", " ", line).strip()
        if not any(label in value for label in labels):
            continue
        for label in labels:
            pattern = rf"{re.escape(label)}\s*[:：]\s*(.+)"
            match = re.search(pattern, value)
            if match:
                return match.group(1).strip(" :：\t")[:120]
        return value[:120]
    return None


def _first_line_with_keywords(text: str, keywords: tuple[str, ...]) -> str | None:
    for line in text.splitlines():
        value = re.sub(r"\s+", " ", line).strip()
        if value and any(keyword in value for keyword in keywords):
            return value[:160]
    return None


def _first_budget(text: str) -> str | None:
    pattern = r"(?:预算|最高限价|控制价)[^\n]{0,50}?([0-9]+(?:\.[0-9]+)?\s*(?:万元|元))"
    match = re.search(pattern, text)
    if match:
        return match.group(1).strip()
    return None


def _score_value(text: str) -> float | None:
    match = re.search(r"([0-9]+(?:\.[0-9]+)?)\s*分", text)
    if not match:
        return None
    try:
        return float(match.group(1))
    except ValueError:
        return None


def _tender_outline_parts(text: str, bid_type: str) -> list[dict[str, object]]:
    headings = _candidate_headings(text)
    if bid_type == "separated":
        return [
            {
                "code": "tech",
                "title": "技术标",
                "sort_order": 10,
                "chapters": _outline_chapters(headings, ("技术", "实施", "服务"), _default_tech_chapters()),
            },
            {
                "code": "business",
                "title": "商务标",
                "sort_order": 20,
                "chapters": _outline_chapters(headings, ("商务", "报价", "资格"), _default_business_chapters()),
            },
        ]
    return [
        {
            "code": "combined_body",
            "title": "综合标书",
            "sort_order": 10,
            "chapters": _outline_chapters(headings, (), _default_combined_chapters()),
        }
    ]


def _candidate_headings(text: str) -> list[str]:
    headings: list[str] = []
    seen: set[str] = set()
    for line in text.splitlines():
        value = re.sub(r"\s+", " ", line).strip()
        if not value or len(value) > 60:
            continue
        if re.match(r"^(第[一二三四五六七八九十\d]+[章节部分]|[一二三四五六七八九十\d]+[、.．])", value):
            title = re.sub(r"^(第[一二三四五六七八九十\d]+[章节部分]|[一二三四五六七八九十\d]+[、.．])\s*", "", value).strip()
            if title and title not in seen:
                seen.add(title)
                headings.append(title)
        if len(headings) >= 12:
            break
    return headings


def _outline_chapters(
    headings: list[str],
    keywords: tuple[str, ...],
    fallback: list[str],
) -> list[dict[str, object]]:
    selected = [
        heading
        for heading in headings
        if not keywords or any(keyword in heading for keyword in keywords)
    ]
    if len(selected) < 3:
        selected = fallback
    return [
        {
            "title": title,
            "plain_text": f"围绕“{title}”逐条响应招标文件要求。",
            "sort_order": (index + 1) * 10,
        }
        for index, title in enumerate(selected[:8])
    ]


def _default_combined_chapters() -> list[str]:
    return ["项目理解", "响应方案", "实施计划", "项目团队", "服务承诺", "风险控制"]


def _default_tech_chapters() -> list[str]:
    return ["项目理解", "总体技术方案", "实施计划", "质量保障", "运维服务"]


def _default_business_chapters() -> list[str]:
    return ["投标函", "资格证明", "商务响应", "报价说明", "服务承诺"]
