#!/usr/bin/env python3
"""Real-provider full generation/export acceptance on a NEW dedicated fixture bid.

Automates review actions on the test fixture only. This is workflow acceptance,
not evidence that the compliance engine evaluates real-world legal correctness.
Never changes user bids, tenant rules, or seed chapters.
"""
from __future__ import annotations
import argparse
import base64
import io
import json
import os
import re
import subprocess
import time
import secrets
import uuid
import urllib.error
import urllib.request
import zipfile
from xml.etree import ElementTree
from chapter_generation_smoke import api_call
from file_upload_smoke import run_smoke, require_origin


def wait_for(base, path, token, field, timeout=1200):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = api_call(base, path, token=token)
        record = result[field] if field else result
        if record["status"] == "done":
            return result
        if record["status"] in {"failed", "cancelled"}:
            raise RuntimeError("workflow failed: " + str(record.get("error_message")))
        time.sleep(2)
    raise RuntimeError("workflow deadline exceeded: " + path)


def expect_rejection(base, path, token, body, status, code):
    request = urllib.request.Request(base + path, data=json.dumps(body).encode(),
        headers={'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token}, method='POST')
    try:
        with urllib.request.urlopen(request, timeout=20):
            raise RuntimeError('expected workflow gate rejection: ' + path)
    except urllib.error.HTTPError as error:
        response = json.loads(error.read())
        if error.code != status or response.get('code') != code:
            raise RuntimeError('unexpected workflow gate error contract: ' + path) from None


def review_fixture_text(text):
    # ONLY our synthetic fixture: remove unsupported promises, do not assert
    # new company facts or invent replacement numbers. Real bids need a human.
    return re.sub(r'【事实待核实：[^】]*】', '相关承诺未作出；应由企业提供真实依据，经人工确认后另行编制。', text)


def verify_second_reviewer(base, token, bid):
    print('Acceptance: invite dedicated project-manager reviewer', flush=True)
    marker = uuid.uuid4().hex[:12]
    password = secrets.token_urlsafe(24)
    email = 'gray-reviewer-' + marker + '@example.com'
    member = api_call(base, '/tenant/members/invite', method='POST', token=token, body={
        'email': email, 'name': '灰度专用第二审批人', 'role_code': 'project_manager', 'initial_password': password})
    api_call(base, '/approval-chains', method='POST', token=token, body={
        'name': '灰度专用指定第二人-' + marker, 'resource_type': 'bid', 'enabled': True,
        'steps': [{'order': 1, 'name': '第二审批人复核', 'user_id': member['user']['id'],
                   'role_code': 'project_manager', 'required': True}]})
    reviewer = api_call(base, '/auth/login', method='POST', body={'email': email, 'password': password})
    from pathlib import Path
    Path('/opt/zbt-private/reviewer-' + bid + '.json').write_text(json.dumps({'email':email, 'password':password}))
    started = api_call(base, f'/bids/{bid}/submit-for-approval', method='POST', token=token, body={})
    instance = started['instance']['id']
    print('Acceptance: reject decision by non-assigned submitter', flush=True)
    expect_rejection(base, f'/approvals/{instance}/approve', token,
        {'comment': '提交人不得代替指定审批人'}, 403, 'permission_denied')
    if reviewer['session']['permissions'].get('team') != 'read':
        raise RuntimeError('reviewer was improperly granted team administration')
    # Pending approvals must not use stale pre-edit checks or exports.
    print('Acceptance: block approval after a post-submission content change', flush=True)
    chapter = api_call(base, f'/bids/{bid}/chapters', token=token)['items'][0]
    api_call(base, '/chapters/' + chapter['id'] + '/content', method='PUT', token=token,
             body={'plain_text': chapter['plain_text'] + '\n【事实待核实：送审后变更夹具。】'})
    expect_rejection(base, f'/approvals/{instance}/approve', reviewer['access_token'], {}, 409, 'factual_review_required')
    api_call(base, '/chapters/' + chapter['id'] + '/content', method='PUT', token=token,
             body={'plain_text': chapter['plain_text'], 'content': chapter['content']})
    api_call(base, '/chapters/' + chapter['id'] + '/accept', method='POST', token=token, body={})
    expect_rejection(base, f'/approvals/{instance}/approve', reviewer['access_token'], {}, 409, 'approval_not_ready')
    check = api_call(base, '/compliance/checks', method='POST', token=token,
                     body={'name': '专用夹具送审后变更复核', 'bid_document_id': bid, 'levels': ['L1','L2','L3']})
    for issue in check.get('issues', []):
        api_call(base, '/compliance/issues/' + issue['id'] + '/ignore', method='POST', token=token,
                 body={'reason': '仅合成工作流夹具，正文已恢复原样；非真实投标合规认证。'})
    exported = api_call(base, f'/bids/{bid}/exports', method='POST', token=token,
        body={'export_type': 'docx', 'part_code': 'combined_body'})
    wait_for(base, '/bid-exports/' + exported['export']['id'], token, 'export')
    print('Acceptance: assigned project manager approves without team administration', flush=True)
    approved = api_call(base, f'/approvals/{instance}/approve', method='POST', token=reviewer['access_token'],
        body={'comment': '仅专用夹具：验证第二角色审批流；非真实项目合规认证。'})
    persisted = api_call(base, f'/bids/{bid}', token=token)
    if approved['instance']['status'] != 'approved' or persisted['status'] != 'approved':
        raise RuntimeError('second reviewer approval did not persist')
    return {'instance_id': instance, 'reviewer_user_id': member['user']['id'],
            'submitter_approve': '403 rejected', 'second_reviewer_approve': 'passed',
            'reviewer_team_permission': 'read', 'post_submission_fact_gap': '409 rejected',
            'post_submission_stale_review': '409 rejected; rechecked and reexported before approval'}


def run(base, origin, bid_type='combined', verify_approval=False):
    if bid_type not in ('combined', 'separated'):
        raise ValueError('full workflow fixture supports combined or separated layouts')
    password = os.environ["ZBT_SMOKE_PASSWORD"]
    print("Acceptance: real tender upload and six-module interpretation", flush=True)
    upload = run_smoke(base, origin, password, 1200, archive=False, bid_type=bid_type)
    bid = upload["bid_id"]
    login = api_call(base, "/auth/login", method="POST", body={"email": os.environ["ZBT_SMOKE_EMAIL"], "password": password})
    token = login["access_token"]
    parsed_task = api_call(base, '/ai-tasks/' + upload['task_id'], token=token)
    external_task = parsed_task['external_task_id']
    # A persisted terminal result is not enough: its durable callback must also
    # be acknowledged, otherwise oversized responses create endless retries.
    callback_deadline = time.monotonic() + 90
    while True:
        state = subprocess.check_output(['docker', 'exec', 'zbt-dev-ai', 'python', '-c',
            "import sqlite3,os,sys; d=sqlite3.connect(os.environ['AI_DURABLE_QUEUE_PATH']); r=d.execute('select state from jobs where id=?',(sys.argv[1],)).fetchone(); print(r[0] if r else 'absent')",
            external_task], text=True).strip()
        if state == 'done':
            break
        if time.monotonic() >= callback_deadline:
            raise RuntimeError('persisted interpretation callback was not durably acknowledged')
        time.sleep(2)
    parsed = api_call(base, f"/bids/{bid}/parse-result", token=token)
    structured = parsed["structured_result"]
    # Bound model expense while still generating EVERY chapter of the fixture.
    fixture_chapters = [
        {"title": "项目理解与交付方案", "plain_text": "根据真实上传的测试招标文件编写项目理解、实施计划、原文工期和技术评分响应；未明确的期限不得猜测。"},
        {"title": "商务响应与人工核对事项", "plain_text": "根据文件编写预算、报价原则和原文资格核对事项；测试项目不提供真实企业证明，不得虚构资质。"}]
    structured["outline"] = {"parts": (
        [{"code": "combined_body", "title": "验收测试综合标书", "chapters": fixture_chapters}]
        if bid_type == 'combined' else [
            {"code": "tech", "title": "验收测试技术标", "chapters": fixture_chapters[:1]},
            {"code": "business", "title": "验收测试商务标", "chapters": fixture_chapters[1:]},
        ])}
    api_call(base, f"/bids/{bid}/parse-result", method="PUT", token=token, body={"structured_result": structured})
    api_call(base, f"/bids/{bid}/material-selection", method="PUT", token=token,
             body={"selected_refs": [], "notes": "专用工作流测试：仅引用招标文件，企业证明待人工提供，不用于实际投标。"})
    api_call(base, f"/bids/{bid}/outline/generate", method="POST", token=token, body={})
    chapters = api_call(base, f"/bids/{bid}/chapters", token=token)["items"]
    if len(chapters) != 2:
        raise RuntimeError("fixture outline did not persist exactly two chapters")
    generated = api_call(base, f"/bids/{bid}/generate", method="POST", token=token, body={"scope": "full"})
    print("Acceptance: full generation for dedicated bid " + bid, flush=True)
    completed = wait_for(base, "/generation-jobs/" + generated["job"]["id"], token, "job")
    if completed["job"]["completed_steps"] != 2 or any(step["status"] != "done" for step in completed["steps"]):
        raise RuntimeError("not every fixture chapter completed")
    chapters = api_call(base, f"/bids/{bid}/chapters", token=token)["items"]
    # Exercise all three backend fact gates even if the current model emits no
    # risky claim. This canary is on a new fixture, never on an existing bid.
    first = chapters[0]
    canary = first['plain_text'] + '\n【事实待核实：灰度闸门专用夹具。】'
    api_call(base, '/chapters/' + first['id'] + '/content', method='PUT', token=token,
             body={'plain_text': canary})
    for path, body in [('/chapters/' + first['id'] + '/accept', {}),
                       (f'/bids/{bid}/exports', {'export_type': 'docx'}),
                       (f'/bids/{bid}/submit-for-approval', {})]:
        expect_rejection(base, path, token, body, 409, 'factual_review_required')
    api_call(base, '/chapters/' + first['id'] + '/content', method='PUT', token=token,
             body={'plain_text': first['plain_text'], 'content': first['content']})
    factual_review_count = 0
    for chapter in chapters:
        if not chapter.get("plain_text") or not (chapter.get("content") or {}).get("content"):
            raise RuntimeError("generated chapter is missing persisted text or editor content")
        if not chapter.get("source_refs") and not chapter.get("needs_human_input"):
            raise RuntimeError("chapter without enterprise evidence must explicitly require human input")
        versions = api_call(base, "/chapters/" + chapter["id"] + "/versions", token=token)
        if not versions.get("items"):
            raise RuntimeError("chapter version was not persisted")
        if '【事实待核实：' in chapter['plain_text']:
            factual_review_count += 1
            text = review_fixture_text(chapter['plain_text'])
            api_call(base, '/chapters/' + chapter['id'] + '/content', method='PUT', token=token,
                     body={'plain_text': text})
        api_call(base, "/chapters/" + chapter["id"] + "/accept", method="POST", token=token, body={})
    chapters = api_call(base, f"/bids/{bid}/chapters", token=token)["items"]
    content_samples = [re.sub(r'\W+', '', chapter['plain_text'])[-60:] for chapter in chapters]
    technical = next(chapter for chapter in chapters if chapter['title'] == fixture_chapters[0]['title'])['plain_text']
    if not re.search(r'雨水|排水|管道', technical) or re.search(r'云平台|云计算|软件许可', technical):
        raise RuntimeError('civil-engineering fixture generated unrelated software content')
    if re.search(r'(交付|完工|竣工).{0,12}(2026[-年]11[-月]15|2026年11月15日)', technical):
        raise RuntimeError('bid submission deadline was misrepresented as project delivery date')
    if re.search(r'项目\s*[ABＡＢ]|500\s*万元|200\s*万元|我方拥有丰富|我司拥有丰富', ''.join(chapter['plain_text'] for chapter in chapters)):
        raise RuntimeError('fixture generated fictional enterprise achievements or example prices')
    export_part_code = 'combined_body' if bid_type == 'combined' else 'tech'
    parts = api_call(base, f'/bids/{bid}/parts', token=token)['items']
    export_part_id = next(part['id'] for part in parts if part['code'] == export_part_code)
    part_chapters = [chapter for chapter in chapters if chapter['bid_part_id'] == export_part_id]
    docx_samples = [re.sub(r'\W+', '', chapter['plain_text'])[-60:] for chapter in part_chapters]
    check = api_call(base, "/compliance/checks", method="POST", token=token,
                     body={"name": "发布验收-仅测试项目", "bid_document_id": bid, "levels": ["L1", "L2", "L3"]})
    # Seed rules generate review flags. Exercise reviewer acknowledgement on the
    # fixture without disabling rules globally or bypassing gates through SQL.
    for issue in check.get("issues", []):
        api_call(base, "/compliance/issues/" + issue["id"] + "/ignore", method="POST", token=token,
                 body={"reason": "自动化工作流夹具，不是真实投标；验证人工审核动作与导出闸门。"})
    exports = {}
    # The UI deliberately disables ZIP for a single-part combined bid. Test
    # packing with a real two-part separated fixture, not an invalid request.
    for kind in (('docx', 'pdf') if bid_type == 'combined' else ('docx', 'pdf', 'zip')):
        print("Acceptance: validate " + kind + " export", flush=True)
        started = api_call(base, f"/bids/{bid}/exports", method="POST", token=token,
                           body={"export_type": kind, "part_code": "all" if kind == "zip" else export_part_code})
        exported = wait_for(base, "/bid-exports/" + started["export"]["id"], token, "export")["export"]
        download = api_call(base, "/files/" + exported["file_asset_id"] + "/download-url", token=token)
        require_origin(download["url"], origin)
        with urllib.request.urlopen(download["url"], timeout=60) as response:
            content = response.read()
        if kind == "pdf":
            # Validate PDF with the deployed parser, not only the magic bytes.
            pdf_check = json.dumps({'content': base64.b64encode(content).decode(), 'samples': docx_samples,
                                    'titles': [chapter['title'] for chapter in part_chapters]}).encode()
            subprocess.run(["docker", "exec", "-i", "zbt-dev-ai", "python", "-c",
                "import sys,fitz,json,base64,re; p=json.load(sys.stdin); d=fitz.open(stream=base64.b64decode(p['content']),filetype='pdf'); assert len(d)>=3; t=re.sub(r'\\W+','',''.join(page.get_text() for page in d)); assert all(s and s in t for s in p['samples']), 'PDF chapter bodies missing'; toc=re.sub(r'\\W+','',d[1].get_text()); assert all(re.sub(r'\\W+','',title) in toc for title in p['titles']), 'PDF directory entries missing'"], input=pdf_check, check=True)
        else:
            with zipfile.ZipFile(io.BytesIO(content)) as archive:
                if archive.testzip() is not None:
                    raise RuntimeError("exported archive is corrupt")
                if kind == "docx":
                    document = ElementTree.fromstring(archive.read('word/document.xml'))
                    text = ''.join(node.text or '' for node in document.iter('{http://schemas.openxmlformats.org/wordprocessingml/2006/main}t'))
                    normalized = re.sub(r'\W+', '', text)
                    if not all(sample and sample in normalized for sample in docx_samples):
                        raise RuntimeError('DOCX does not contain the persisted generated chapter bodies')
                else:
                    documents = [name for name in archive.namelist() if name.endswith('.docx')]
                    texts = []
                    for name in documents:
                        with zipfile.ZipFile(io.BytesIO(archive.read(name))) as nested:
                            document = ElementTree.fromstring(nested.read('word/document.xml'))
                            texts.append(''.join(node.text or '' for node in document.iter('{http://schemas.openxmlformats.org/wordprocessingml/2006/main}t')))
                    normalized = re.sub(r'\W+', '', ''.join(texts))
                    if len(documents) < 2 or not all(sample and sample in normalized for sample in content_samples):
                        raise RuntimeError('ZIP does not contain both persisted generated part bodies')
        exports[kind] = {"export_id": exported["id"], "bytes": len(content)}
    approval = verify_second_reviewer(base, token, bid) if verify_approval else {'status': 'not_executed'}
    api_call(base, f"/bids/{bid}", method="PATCH", token=token, body={"status": "archived"})
    evidence = {"status": "passed", "bid_id": bid, "bid_type": bid_type, "chapters_generated": 2, "chapter_ids": [chapter["id"] for chapter in chapters], "exports": exports,
            "review": "fixture-only acknowledgement; not semantic compliance certification",
            "factual_gate_canary": "accept/export/approval rejected unresolved placeholder",
            "factual_reviewed_chapters": factual_review_count,
            "approval": approval,
            "source_sha": os.environ.get('GITHUB_SHA', '')}
    from pathlib import Path
    Path("/opt/zbt-private/full-acceptance.json").write_text(json.dumps(evidence, indent=2))
    # The following fresh-enterprise run must not overwrite evidence for the
    # earlier split-part DOCX/PDF/ZIP acceptance.
    Path(f"/opt/zbt-private/full-acceptance-{bid_type}.json").write_text(json.dumps(evidence, indent=2))
    return evidence


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/api/v1")
    parser.add_argument("--public-origin", default="http://47.114.51.41:8080")
    parser.add_argument('--bid-type', choices=('combined', 'separated'), default='combined')
    args = parser.parse_args()
    print(json.dumps(run(args.base_url, args.public_origin, args.bid_type), ensure_ascii=False))
