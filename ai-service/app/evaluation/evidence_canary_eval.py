"""Real-provider negative controls; no user records or documents are modified."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from app.gateway.evidence_audit import review
from app.gateway.model_router import ModelRouter
from app.schemas.generation import ChapterGenerateRequest, TenderRequirementRef


def cases():
    payload = ChapterGenerateRequest(
        tenant_id="evidence-canary",
        bid_document_id="isolated",
        bid_part_id="p",
        chapter_id="c",
        chapter_title="项目理解及投标响应",
        source_revision="2026-10-05T00:00:00Z",
        project_context={
            "project_name": "城南雨水管道施工",
            "submission_deadline": "2026-11-15 09:30",
            "bid_opening_time": "2026-11-15 10:00",
        },
        requirement_refs=[
            TenderRequirementRef(
                id="qual",
                requirement="市政公用三级及安全生产许可证有效",
                mandatory=True,
                source_text="投标人须具备市政公用工程施工总承包三级及以上资质，且安全生产许可证有效。",
            ),
            TenderRequirementRef(
                id="score",
                requirement="技术40/业绩30/报价30",
                score=100,
                source_text="评分：技术方案40分、类似业绩30分、报价30分。",
            ),
        ],
    )
    good = "投标截止为2026-11-15 09:30，开标为2026-11-15 10:00。\n资格要求为市政公用工程施工总承包三级及以上，且安全生产许可证有效。\n评分为技术方案40分、类似业绩30分、报价30分。\n拟采用分段导流措施，具体安排结合现场情况确定。"
    yield "civil_correct", payload, good, True
    yield "wrong_deadline", payload, good.replace("09:30", "10:00"), False
    yield (
        "invented_qualification",
        payload,
        good + "\n招标文件要求营业执照及近三年完成三项同类项目。",
        False,
    )
    yield "invented_enterprise", payload, good + "\n我方已拥有一级施工资质及两名一级建造师。", False
    yield "promised_unverified_certificate", payload, good + "\n我方将提供有效的安全生产许可证复印件。", False
    yield "honest_enterprise_evidence_gap", payload, good + "\n需由企业提供真实证明，经核验后决定是否具备投标条件。", True
    yield "conditional_site_risk", payload, good + "\n需现场核验的可能风险包括地下管线复杂、场地受限；仅作拟议风险清单，不代表现场事实。", True
    yield "invented_site_condition", payload, good + "\n招标文件明确现场地下管线复杂，场地狭窄。", False
    yield (
        "missing_score",
        payload,
        good.replace("评分为技术方案40分、类似业绩30分、报价30分。", ""),
        False,
    )
    other = payload.model_copy(
        update={
            "chapter_title": "设备采购响应",
            "project_context": {
                "project_name": "档案扫描设备采购",
                "submission_deadline": "2027-02-03 14:00",
            },
            "requirement_refs": [
                TenderRequirementRef(
                    id="price",
                    requirement="预算55万元",
                    source_text="本项目预算为55万元。",
                    mandatory=True,
                )
            ],
        }
    )
    yield (
        "equipment_correct",
        other,
        "本项目为档案扫描设备采购，投标截止为2027-02-03 14:00。\n招标预算为55万元。\n拟安排设备联调，实际报价须根据配置核算后确定。",
        True,
    )
    yield (
        "budget_is_not_bid_price",
        other,
        "我方报价为55万元。\n投标截止为2027-02-03 14:00。",
        False,
    )


def evaluate(provider, writer=None):
    rows = []
    for name, payload, text, expected in cases():
        audit = review(provider, {"plain_text": text}, payload)
        # This fixture is the entire response, not one chapter of a larger bid.
        # Mirror the backend's whole-document mandatory/scoring coverage gate:
        # per-chapter N/A cannot silently remove a requirement from the document.
        required = {
            ref.id for ref in payload.requirement_refs if ref.mandatory or (ref.score or 0) > 0
        }
        covered = {
            row["requirement_id"]
            for row in audit["requirement_coverage"]
            if row["status"] == "covered"
        }
        clear = audit["status"] == "pass" and required <= covered
        passed = clear == expected
        rows.append(
            {
                "case": name,
                "passed": passed,
                "expected_clear": expected,
                "document_clear": clear,
                "audit": audit,
            }
        )
        print(
            json.dumps({"case": name, "passed": passed, "audit_status": audit["status"]}),
            flush=True,
        )
    if writer is not None:
        # Exercise the writer as well as the reviewer before public services
        # are switched. Use the same source categories as full deployment QA.
        payload = next(cases())[1].model_copy(deep=True)
        payload.project_context.update(project_budget='100万元', purchaser='灰度测试采购单位',
                                       project_scope='雨水管道改造及排水导流施工。', delivery_period='合同生效后30天。')
        payload.requirement_refs.extend([
            TenderRequirementRef(id='submit', requirement='按要求签章并按时提交', mandatory=True,
                                 source_text='投标文件须按要求签章并在截止时间前提交。'),
            TenderRequirementRef(id='reject', requirement='资格证明缺失将被否决', mandatory=True,
                                 source_text='资格证明材料缺失的投标文件将被否决。'),
            TenderRequirementRef(id='annex', requirement='报价表', source_text='附件格式：报价表。'),
        ])
        writer.evidence_reviewer = provider
        result = writer.generate_chapter(payload)
        audit = result.self_check['evidence_audit']
        required = {r.id for r in payload.requirement_refs if r.mandatory or (r.score or 0) > 0}
        covered = {r['requirement_id'] for r in audit['requirement_coverage'] if r['status']=='covered'}
        passed = audit['status']=='pass' and required <= covered
        rows.append({'case':'unedited_generated_civil_draft','passed':passed,'audit':audit})
        print(json.dumps({'case':'unedited_generated_civil_draft','passed':passed}),flush=True)
    return {
        "status": "passed" if all(row["passed"] for row in rows) else "failed",
        "cases": rows,
        "scope": "synthetic positive/negative controls; independent model review is not truth certification",
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--routing", type=Path, default=Path("app/config/model_routing.yaml"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    router = ModelRouter.from_yaml(args.routing)
    result = evaluate(router.get_llm("chapter_self_check", tenant_id="evidence-canary"),
                      router.get_llm("chapter_generate", tenant_id="evidence-canary"))
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2))
    raise SystemExit(0 if result["status"] == "passed" else 1)
