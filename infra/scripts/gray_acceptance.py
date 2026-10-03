#!/usr/bin/env python3
"""Fresh-enterprise gray regression, followed by real generation and exports.

Creates dedicated test data only; existing users/passwords/bids are untouched.
Credentials and evidence stay in /opt/zbt-private. Never print authentication
responses or signed file URLs. This is NOT completion of the 171-case test plan.
"""
import argparse
import json
import os
from pathlib import Path
import secrets
import urllib.error
import urllib.request
import uuid

from chapter_generation_smoke import api_call
from full_bid_smoke import run as full_run, expect_rejection


def expect_error(base, path, body, status, code):
    request = urllib.request.Request(base + path, data=json.dumps(body).encode(),
                                     headers={'Content-Type': 'application/json'}, method='POST')
    try:
        with urllib.request.urlopen(request, timeout=20):
            raise RuntimeError('expected request rejection: ' + path)
    except urllib.error.HTTPError as error:
        result = json.loads(error.read())
        if error.code != status or result.get('code') != code:
            raise RuntimeError('incorrect error contract: ' + path) from None


def run(origin):
    os.umask(0o077)
    base = origin.rstrip('/') + '/api/v1'
    with urllib.request.urlopen(origin.rstrip('/') + '/', timeout=20) as response:
        if 'no-cache' not in response.headers.get('Cache-Control', ''):
            raise RuntimeError('HTML entry must revalidate after a release')
    try:
        with urllib.request.urlopen(origin.rstrip('/') + '/assets/gray-nonexistent-chunk.js', timeout=20):
            raise RuntimeError('missing JS chunk must not return the SPA HTML page')
    except urllib.error.HTTPError as error:
        if error.code != 404:
            raise RuntimeError('incorrect missing chunk response') from None
    marker = uuid.uuid4().hex[:12]
    password = secrets.token_urlsafe(24)
    email = 'gray-regression-' + marker + '@example.com'
    registration = {'tenant_name': '灰度回归企业-' + marker, 'admin_name': '灰度回归管理员',
                    'email': email, 'password': password}
    registered = api_call(base, '/auth/register', method='POST', body=registration)
    tenant = registered['session']['tenant']['id']
    Path('/opt/zbt-private/gray-acceptance-account.json').write_text(json.dumps(
        {'email': email, 'password': password, 'tenant_id': tenant}, ensure_ascii=False))
    api_call(base, '/auth/logout', method='POST', token=registered['access_token'], body={})
    # No tenant_id and no tenant UUID in the URL: this was the public login P0.
    login = api_call(base, '/auth/login', method='POST', body={'email': email, 'password': password})
    if login['session']['tenant']['id'] != tenant:
        raise RuntimeError('public login entered the wrong enterprise')
    expect_error(base, '/auth/login', {'email': email, 'password': 'incorrect-password'}, 401, 'invalid_credentials')
    expect_error(base, '/auth/register', registration, 409, 'email_already_registered')
    expect_error(base, '/auth/login', {'email': email, 'password': password,
                 'tenant_id': '00000000-0000-4000-8000-000000000001'}, 401, 'invalid_credentials')
    for bid_type in ('combined', 'separated', 'custom'):
        title = '灰度类型回归-' + bid_type + '-' + marker
        bid = api_call(base, '/bids', method='POST', token=login['access_token'],
                       body={'title': title, 'project_name': title, 'bid_type': bid_type})
        persisted = api_call(base, '/bids/' + bid['id'], token=login['access_token'])
        if persisted.get('bid_type') != bid_type or persisted.get('project_name') != title or not persisted.get('project_id'):
            raise RuntimeError('bid type or project name did not survive reload')
        expect_rejection(base, '/bids/' + bid['id'] + '/submit-for-approval', login['access_token'],
                         {}, 409, 'approval_not_ready')
        api_call(base, '/bids/' + bid['id'], method='PATCH', token=login['access_token'], body={'status': 'archived'})
    pool = api_call(base, '/platform/tenders?limit=1', token=login['access_token'])
    if not pool.get('items'):
        raise RuntimeError('expected imported historical public announcement')
    listed = pool['items'][0]
    detail = api_call(base, '/platform/tenders/' + listed['id'], token=login['access_token'])
    if detail.get('title') != listed['title'] or not detail.get('raw_content_preview'):
        raise RuntimeError('public announcement detail did not return saved content')
    os.environ.update(ZBT_SMOKE_EMAIL=email, ZBT_SMOKE_PASSWORD=password)
    result = full_run(base, origin.rstrip('/'), verify_approval=True)
    result.update(origin=origin, tenant_id=tenant, public_login='passed', duplicate_email='passed',
                  wrong_password='passed', unauthorized_tenant_login='rejected',
                  public_announcement_detail='saved content returned',
                  interpretation='original deadline, qualifications, scores and end clause checked',
                  scope='targeted gray regression, not all 171 cases')
    Path('/opt/zbt-private/gray-acceptance.json').write_text(json.dumps(result, ensure_ascii=False, indent=2))
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--origin', default='https://47.114.51.41')
    args = parser.parse_args()
    try:
        print(json.dumps(run(args.origin), ensure_ascii=False))
    except Exception as error:
        # Exception text may contain a signed URL or provider response.
        raise SystemExit('Gray acceptance failed: ' + type(error).__name__) from None
