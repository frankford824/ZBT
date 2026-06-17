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
    source_records = _source_records(parsed)
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
                        "page_start": "number or null",
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
    source_excerpt = "\n".join(_module_context_lines(parsed, module))[:10000]
    return json.dumps(
        {
            "task": "Improve one tender parse module using only source-backed facts.",
            "module": module,
            "module_title": MODULE_TITLES[module],
            "bid_title": payload.bid_title,
            "filename": payload.filename,
            "deterministic_module": module_result,
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
                        "page_start": "number or null",
                        "page_end": "number or null",
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
    keywords = MODULE_CONTEXT_KEYWORDS[module]
    matched: list[str] = []
    fallback: list[str] = []
    seen: set[str] = set()
    for chunk in parsed.chunks:
        for line in chunk.content.splitlines():
            value = re.sub(r"\s+", " ", line).strip()
            if len(value) < 4 or value in seen:
                continue
            seen.add(value)
            if any(keyword in value for keyword in keywords):
                matched.append(value)
            elif len(fallback) < 40:
                fallback.append(value)
            if len(matched) >= 80:
                break
        if len(matched) >= 80:
            break
    return matched or fallback[:40]


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
        evidence.append(
            {
                "field": field,
                "value": item.get("value"),
                "confidence": _confidence(item.get("confidence"), 0.62 if source_text else 0.45),
                "source_text": source_text,
                "page_start": _int_or_none(item.get("page_start")),
                "page_end": _int_or_none(item.get("page_end")),
                "bbox": item.get("bbox") if isinstance(item.get("bbox"), list) else None,
                "chunk_id": str(item.get("chunk_id") or "") or None,
                "needs_review": bool(item.get("needs_review")) or not source_text,
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
        "page_start": None,
        "page_end": None,
        "bbox": None,
        "chunk_id": None,
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
    interpret.update(
        {
            "status": "needs_review" if low_confidence_count or missing_source_count or ocr_required else "pass",
            "module_count": module_count,
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


def _source_records(parsed: KnowledgeProcessResult) -> list[dict[str, object]]:
    records: list[dict[str, object]] = []
    for chunk_index, chunk in enumerate(parsed.chunks, start=1):
        chunk_id = str(
            chunk.metadata.get("chunk_id")
            or chunk.metadata.get("id")
            or f"parse-chunk-{chunk_index:04d}"
        )
        for raw_line in chunk.content.splitlines():
            value = re.sub(r"\s+", " ", raw_line).strip(" :：\t")
            if len(value) < 2:
                continue
            records.append(
                {
                    "text": value,
                    "page_start": chunk.page_start,
                    "page_end": chunk.page_end,
                    "chunk_id": chunk_id,
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
    ocr_required = bool(parsed.metadata.get("ocr_required"))
    status = "needs_review" if low_confidence_count or missing_source_count or ocr_required else "pass"
    return {
        "interpret": {
            "status": status,
            "module_count": len(modules),
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
                page_start=_record_int(record, "page_start"),
                page_end=_record_int(record, "page_end"),
                chunk_id=str(record.get("chunk_id") or ""),
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
            page_start=_record_int(matched_record, "page_start"),
            page_end=_record_int(matched_record, "page_end"),
            chunk_id=str(matched_record.get("chunk_id") or ""),
            needs_review=not bool(value_text),
        )
    return TenderParseFieldEvidence(
        field=field,
        value=value,
        confidence=0.55 if value_text else 0.25,
        source_text="",
        needs_review=True,
    )


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
