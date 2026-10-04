"""Bounded operational evidence checks. Never print credentials or private logs."""
from __future__ import annotations
import json
import socket
import ssl
import subprocess
import time
import urllib.request
from pathlib import Path


def inspect_health():
    checks=[]
    for label,url in [('https','https://47.114.51.41/'),('http8080','http://127.0.0.1:8080/')]:
        try:
            with urllib.request.urlopen(url,timeout=10) as response:
                checks.append({'check':label,'passed':response.status==200})
        except Exception as exc:
            checks.append({'check':label,'passed':False,'error_type':type(exc).__name__})
    try:
        with socket.create_connection(('47.114.51.41',443),timeout=10) as connection:
            with ssl.create_default_context().wrap_socket(connection,server_hostname='47.114.51.41') as secure:
                expires=ssl.cert_time_to_seconds(secure.getpeercert()['notAfter'])
        remaining=int(expires-time.time())
        checks.append({'check':'trusted_ip_certificate','passed':remaining>86400,'remaining_seconds':remaining,'expires_at':expires})
    except Exception as exc:
        checks.append({'check':'trusted_ip_certificate','passed':False,'error_type':type(exc).__name__})
    for unit in ('zbt-dev-backup.timer','zbt-ip-cert-renew.timer'):
        status=subprocess.run(['systemctl','is-active',unit],capture_output=True,text=True,timeout=10)
        checks.append({'check':unit,'passed':status.returncode==0})
    try:
        latest=Path(json.loads(Path('/opt/zbt-backups/latest.json').read_text())['path']).resolve()
        if latest.parent!=Path('/opt/zbt-backups'):raise ValueError('invalid backup path')
        age=int(time.time()-(latest/'manifest.json').stat().st_mtime)
        checks.append({'check':'recent_backup','passed':0<=age<36*3600,'age_seconds':age})
        evidence=json.loads((latest/'restore-evidence.json').read_text())
        checks.append({'check':'latest_backup_restore','passed':evidence.get('status')=='passed' and evidence.get('live_data_modified') is False})
    except Exception as exc:
        checks.append({'check':'backup_evidence','passed':False,'error_type':type(exc).__name__})
    return {'status':'passed' if all(c['passed'] for c in checks) else 'failed','checked_at':time.time(),'checks':checks,
            'scope':'single-host technical checks; no claim of off-host automation, end-to-end acceptance or 24-hour continuity'}


if __name__=='__main__':
    result=inspect_health()
    output=Path('/opt/zbt-private/operations-health.json')
    output.write_text(json.dumps(result,indent=2))
    output.chmod(0o600)
    print(json.dumps(result))
    raise SystemExit(0 if result['status']=='passed' else 1)
