"""Conservative, source-bound checks on newly generated content (not user edits).

This is a deterministic guard for specific high-risk claims, not a general
factuality classifier. Unknown commitments become visible review placeholders;
they never become defaults merely because a model emitted them.
"""
from __future__ import annotations

from copy import deepcopy
import re

from app.schemas.generation import ChapterGenerateRequest

REVIEW_MARKER = "【事实待核实："
_NUMBER = r"(?:\d+(?:\.\d+)?|[一二三四五六七八九十百零两]+)"
_DURATION = re.compile(_NUMBER + r"\s*(?:个\s*)?(?:工作日|小时|分钟|个月|月|年|天|日)")
_AMOUNT = re.compile(_NUMBER + r"\s*(?:亿|万)?\s*(?:元|万元|亿元)")
_SUBJECT = r"(?:我方|我司|我公司|本公司|本企业|我们|我单位|我团队|本单位|本团队|本投标人)"
_CATEGORIES = {
    "质保承诺": r"质保|保修",
    "响应时限": r"响应|到场|修复|故障处理|应急处理|恢复",
    "投标报价": r"投标报价|我方报价|报价金额|投标总价|报价总价",
    "交付工期": r"交付周期|交付期限|交付|施工工期|总工期|工期|履约期限",
}
_OWNERSHIP = re.compile(_SUBJECT + r".{0,12}?(?:拥有|具备|持有|取得|已获|曾|已完成|已承接|已承担|积累|承诺(?:已)?(?:满足|符合))")
_HISTORY_PROMISE = re.compile(_SUBJECT + r".{0,12}(?:将|拟)?提供.{0,24}(?:近.{0,4}年|承接|完成|承担).{0,24}(?:业绩|项目|工程)")
_CERT_ASSERTION = re.compile(_SUBJECT + r"(?:.{0,8}(?:承诺|保证).{0,15}(?:许可证|证书|资质).{0,15}(?:有效|符合|满足)|.{0,6}(?:安全生产许可证|资格证书|资质证书|资质).{0,16}(?:在有效期内|有效期为|有效至|符合|满足))")
_CERT_PROVISION = re.compile(_SUBJECT + r".{0,12}(?:将|拟|承诺|保证).{0,8}提供.{0,45}(?:资质证书|资格证书|安全生产许可证|人员证书|建造师证|资质.{0,8}证书)")
_UNCONFIRMED_PREFIX = re.compile(r"(?:不|未|不能|无法|尚未|尚不能)(?:承诺|保证|确认|认定|断言|声称)\s*$")
_NEGATIVE_OWNERSHIP = re.compile(_SUBJECT + r"(?:尚未|并未|没有|不|未|未曾)(?:拥有|具备|持有|取得|曾|完成|承接|承担|积累)")
_SUBMISSION_PROMISE = re.compile(_SUBJECT + r".{0,12}(?:承诺|保证|将).{0,24}(" + _NUMBER + r"\s*(?:工作日|天|日|小时|个月|月))\s*(?:内|后)?.{0,6}(?:提交|报送)")
_DATE = re.compile(r"(\d{4})[-年/](\d{1,2})[-月/](\d{1,2})(?:日)?")
_TIME = re.compile(r"(\d{1,2})[:：时](\d{1,2})(?:分)?")
_CALENDAR_LABELS = {
    "submission_deadline": r"(?:投标|递交|提交)(?:文件)?截止(?:时间|日期)?",
    "bid_opening_time": r"开标(?:时间|日期)?",
}
_COPY_DETAIL = re.compile(r"(?:正本|副本|电子版|书面版)\s*" + _NUMBER + r"\s*份|" + _NUMBER + r"正" + _NUMBER + r"副|U\s*盘", re.IGNORECASE)
_BOQ_ASSERTION = re.compile(r"招标文件(?:中|的).{0,6}工程量清单")
_UNRESOLVED = re.compile(r"[\[【（(](?:待澄清|待填写|待补充|待确认|待核实)[\]】）)]")
_UNRESOLVED_NUMBER = re.compile(r"x{2,}\s*(?:日历天|工作日|小时|分钟|个月|万元|亿元|天|日|月|年|元)", re.IGNORECASE)
_OPENING_AS_SUBMISSION_LIMIT = re.compile(
    r"开标.{0,70}(?:前|之前).{0,35}(?:递交|提交|送达)|(?:递交|提交|送达).{0,35}开标.{0,70}(?:前|之前)"
)
_CLAUSE_SPLIT = re.compile(r"(?<=[。！？；;\n])")


def _normalized(text: str) -> str:
    return re.sub(r"\s+", "", text)


def _text(node: object) -> str:
    if isinstance(node, dict):
        if node.get("type") == "hardBreak":
            return "\n"
        value = node.get("text")
        return (value if isinstance(value, str) else "") + _text(node.get("content"))
    if isinstance(node, list):
        return "".join(_text(child) for child in node)
    return ""


def _numeric_pairs(text: str, category: str) -> set[str]:
    """Pair a number with its semantic category inside one clause.

    A 12-month duration elsewhere or a 2-hour response does not substantiate a
    12-month warranty. Nor does a budget substantiate the supplier's bid price.
    """
    pairs: set[str] = set()
    numbers = _AMOUNT if category == "投标报价" else _DURATION
    labels = [(kind, match) for kind, pattern in _CATEGORIES.items() for match in re.finditer(pattern, text)]
    for number in numbers.finditer(text):
        preceding = [(kind, label) for kind, label in labels if 0 <= number.start() - label.end() <= 22]
        following = [(kind, label) for kind, label in labels if 0 <= label.start() - number.end() <= 10]
        selected = max(preceding, key=lambda item: item[1].end()) if preceding else (
            min(following, key=lambda item: item[1].start()) if following else None)
        if selected and selected[0] == category:
            pairs.add(_normalized(number.group()))
    return pairs


def _calendar_mismatch(text: str, context: dict[str, str]) -> bool:
    text = _normalized(text)
    if any(_OPENING_AS_SUBMISSION_LIMIT.search(part) for part in re.split(r"[，,]", text)):
        # A correctly quoted opening time still cannot serve as the delivery
        # deadline. Permit this phrasing only when both sourced timestamps match.
        for pattern in (_DATE, _TIME):
            deadline = pattern.search(_normalized(context.get("submission_deadline", "")))
            opening = pattern.search(_normalized(context.get("bid_opening_time", "")))
            if not deadline or not opening or tuple(map(int, deadline.groups())) != tuple(map(int, opening.groups())):
                return True
    labels = sorted((match.start(), match.end(), key)
                    for key, pattern in _CALENDAR_LABELS.items() for match in re.finditer(pattern, text))
    for pattern in (_DATE, _TIME):
        for value in pattern.finditer(text):
            before = [label for label in labels if 0 <= value.start() - label[1] <= 45]
            if not before:
                continue
            targets = [before[-1]]
            # "投标截止时间及开标时间为10:00" claims the same time for both.
            if len(before) > 1 and re.fullmatch(r"(?:及|和|与|、|/|暨)", text[before[-2][1]:before[-1][0]]):
                targets.append(before[-2])
            for _, _, key in targets:
                expected = pattern.search(_normalized(context.get(key, "")))
                if not expected or tuple(map(int, value.groups())) != tuple(map(int, expected.groups())):
                    return True
    return False


def guard_chapter_content(result: dict[str, object], payload: ChapterGenerateRequest) -> tuple[dict[str, object], list[str], list[dict[str, str]]]:
    guarded = deepcopy(result)
    # Only actual source quotations and retrieved document content are evidence.
    # Model summaries, current draft text and arbitrary identifiers are not.
    enterprise_sources = [ref.content for ref in payload.retrieved_knowledge_refs if ref.content.strip()]
    tender_sources = [(("交付期限：" if key == "delivery_period" else "") + value)
                      for key, value in payload.project_context.items()] + [
        ref.source_text for ref in payload.requirement_refs if ref.source_text.strip()
    ]
    sources = tender_sources + enterprise_sources
    supported = {category: set().union(*(_numeric_pairs(clause, category)
                 for source in (enterprise_sources if category == "投标报价" else sources)
                 for clause in _CLAUSE_SPLIT.split(source))) for category in _CATEGORIES}
    issues: list[dict[str, str]] = []
    notes: list[str] = []

    def sanitize(text: str) -> str:
        clauses = []
        for clause in _CLAUSE_SPLIT.split(text):
            kinds = []
            if _UNRESOLVED.search(clause) or _UNRESOLVED_NUMBER.search(clause):
                kinds.append("生成占位")
            if _calendar_mismatch(clause, payload.project_context):
                kinds.append("截止或开标时间")
            details = _COPY_DETAIL.findall(clause)
            if any(not any(_normalized(detail).lower() in _normalized(source).lower() for source in tender_sources)
                   for detail in details):
                kinds.append("递交份数或介质")
            if _BOQ_ASSERTION.search(clause) and not any("工程量清单" in source for source in tender_sources):
                kinds.append("原文附件")
            for category in _CATEGORIES:
                pairs = _numeric_pairs(clause, category)
                if pairs - supported[category]:
                    kinds.append(category)
            submission = _SUBMISSION_PROMISE.search(clause)
            if submission:
                # A newly promised post-award submission period is not a plan
                # default. Require a source with the same duration and action.
                duration = _normalized(submission.group(1))
                if not any(duration in _normalized(source_clause) and re.search(r"提交|报送", source_clause)
                           for source in sources for source_clause in _CLAUSE_SPLIT.split(source)):
                    kinds.append("提交时限")
            # "不承诺我方已具备" explicitly withholds the claim. Inspect each
            # occurrence so this disclaimer cannot excuse a later positive one.
            ownership = any(not _UNCONFIRMED_PREFIX.search(clause[:match.start()])
                            and not _NEGATIVE_OWNERSHIP.fullmatch(match.group())
                            for pattern in (_OWNERSHIP, _HISTORY_PROMISE, _CERT_ASSERTION, _CERT_PROVISION)
                            for match in pattern.finditer(clause))
            if ownership:
                # Ownership must be evidenced by enterprise records, not by a
                # tender's qualification requirement. Conservative literal
                # support: semantic inference remains a human review action.
                claim = re.sub(_SUBJECT, "", clause, count=1).strip("。；;\n ")
                if not any(_normalized(claim) in _normalized(source) for source in enterprise_sources):
                    kinds.append("企业事实")
            if kinds:
                label = "、".join(kinds)
                replacement = REVIEW_MARKER + label + "缺少可核验原文或企业资料，请补充依据后编写。】"
                clauses.append(replacement)
                for kind in kinds:
                    # Bounded audit evidence; not injected into exported body.
                    issue = {"kind": kind, "original_text": clause.strip()[:200]}
                    if issue not in issues and len(issues) < 20:
                        issues.append(issue)
                note = "正文中的" + label + "缺少依据，已替换为待核实提示；请人工补充资料，不得直接定稿。"
                if note not in notes and len(notes) < 10:
                    notes.append(note)
            else:
                clauses.append(clause)
        return "".join(clauses)

    def visit(node: object) -> None:
        if not isinstance(node, dict):
            return
        children = node.get("content")
        if node.get("type") in {"paragraph", "heading", "codeBlock"}:
            original = _text(node)
            revised = sanitize(original)
            if revised != original:
                # Check the whole block so splitting numbers/claims across bold
                # text nodes cannot evade the guard. Preserve unchanged blocks.
                node["content"] = [{"type": "text", "text": revised}]
        elif isinstance(children, list):
            for child in children:
                visit(child)
        elif isinstance(node.get("text"), str):
            node["text"] = sanitize(node["text"])

    if isinstance(guarded.get("tiptap_json"), dict):
        visit(guarded["tiptap_json"])
    for field in ("plain_text", "content"):
        if isinstance(guarded.get(field), str):
            guarded[field] = sanitize(guarded[field])
    return guarded, notes, issues
