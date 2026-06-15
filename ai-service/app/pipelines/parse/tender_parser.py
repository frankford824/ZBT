from __future__ import annotations

import json
import re
from pathlib import Path

from app.schemas.knowledge import KnowledgeProcessResult
from app.schemas.tender import TenderParseRequest


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


def build_tender_structured_result(
    payload: TenderParseRequest,
    parsed: KnowledgeProcessResult,
) -> dict[str, object]:
    text = "\n".join(chunk.content for chunk in parsed.chunks)
    project_name = payload.bid_title or _first_content_line(text) or Path(payload.filename).stem
    has_tech_part = any(marker in text for marker in ("技术标", "技术方案", "实施方案"))
    has_business_part = any(marker in text for marker in ("商务标", "商务响应", "报价文件", "报价说明"))
    bid_type = "separated" if has_tech_part and has_business_part else "combined"
    outline_parts = _tender_outline_parts(text, bid_type)
    return {
        "project_name": project_name,
        "bid_type": bid_type,
        "source_file": {
            "file_asset_id": payload.file_id,
            "filename": payload.filename,
            "content_type": payload.content_type,
        },
        "deadline": _first_date(text),
        "qualification_requirements": _keyword_lines(
            text,
            ("资格", "资质", "业绩", "证书", "项目负责人", "联合体"),
            fallback=("营业执照、授权及签章材料齐备", "按招标文件提交资格证明材料"),
        ),
        "invalid_clause_risks": _keyword_lines(
            text,
            ("无效", "废标", "否决", "投标保证金", "投标有效期", "签章"),
            fallback=("签章、报价、投标有效期等关键条款需人工复核",),
        ),
        "scoring_points": _keyword_lines(
            text,
            ("评分", "分值", "评审", "技术方案", "服务方案", "实施方案"),
            fallback=("实施方案完整性", "项目团队与业绩能力", "服务承诺与响应程度"),
        ),
        "outline": {"parts": outline_parts},
        "material_suggestions": [
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
        "parse_metadata": {
            "parser": parsed.metadata.get("parser"),
            "page_count": parsed.metadata.get("page_count"),
            "chunk_count": len(parsed.chunks),
            "table_count": parsed.metadata.get("table_count", 0),
            "ocr": parsed.metadata.get("ocr"),
        },
    }


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
            },
            "rules": [
                "Return only JSON.",
                "Use source facts first and keep uncertain values conservative.",
                "Do not invent dates, certificates, prices, or names that are not in the source.",
                "Keep chapter titles concise and business-facing.",
            ],
        },
        ensure_ascii=False,
    )


def merge_tender_structured_result(
    base_result: dict[str, object],
    model_result: dict[str, object],
) -> dict[str, object]:
    merged = dict(base_result)
    for key in MERGEABLE_TOP_LEVEL_FIELDS:
        value = model_result.get(key)
        if _usable_model_value(value):
            merged[key] = value
    return merged


def _usable_model_value(value: object) -> bool:
    if value is None:
        return False
    if isinstance(value, str):
        return bool(value.strip())
    if isinstance(value, list | dict):
        return bool(value)
    return True


def _first_content_line(text: str) -> str:
    for line in text.splitlines():
        value = line.strip()
        if value and not value.startswith("["):
            return value[:120]
    return ""


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
