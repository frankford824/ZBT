# ECS public IP HTTPS (no domain required)

The host TLS edge is separate from Compose: `https://47.114.51.41` on 443
and the existing `http://47.114.51.41:8080` coexist. Registration and user
accounts are unchanged. Port 80 serves HTTP-01 certificate challenges only;
there is no forced redirect and no HSTS.

## Host setup

Allow public TCP 80 and 443 in the ECS security group, without removing 8080.
Install Ubuntu Nginx and python3-venv. Install Certbot 5.4 or newer in an
isolated `/opt/zbt-certbot` environment (initial installation: 5.8.0).

Install `ecs-ip-http.conf` in `/etc/nginx/sites-available/`, enable its symlink
in `sites-enabled`, and disable the distribution's default site by moving its
symlink into `/opt/zbt-private/`. Create `/var/lib/zbt-acme` and reload Nginx
only after `nginx -t` passes. No Compose service restart is required.

First validate against staging using a separate certificate name. Then issue
the trusted certificate:

```sh
/opt/zbt-certbot/bin/certbot certonly --non-interactive --agree-tos \
  --register-unsafely-without-email --required-profile shortlived \
  --webroot --webroot-path /var/lib/zbt-acme \
  --ip-address 47.114.51.41 --cert-name 47.114.51.41
```

No contact email was supplied; none is invented. Renewal monitoring is
therefore important. Move the **staging-only** renewal configuration out of
`/etc/letsencrypt/renewal/` so routine renewal checks target production only.

Install and enable `ecs-ip-https.conf` after production issuance, check Nginx,
and reload it. Install `zbt-ip-cert-renew.service` and `.timer` in
`/etc/systemd/system/`, reload systemd, and enable/start the timer.

Certificates last about six days. The timer checks twice a day; a successful
renewal invokes `nginx -t` and reloads Nginx without stopping the application.
Verify renewal with `certbot renew --dry-run --run-deploy-hooks` and check
timer status. Never substitute a staging certificate into the live TLS site.

## Object signatures and compatibility

The backend currently signs object URLs using `http://47.114.51.41:8080`.
Only HTTPS API responses rewrite that public origin to `https://47.114.51.41`.
The HTTPS object proxy restores `Host: 47.114.51.41:8080` to MinIO, preserving
the signed authority, raw object path and query. Internal services remain
on loopback; the public object proxy exposes only the application bucket.

This compatibility edge does not change the HTTP API or re-sign URLs. If
`MINIO_PUBLIC_ENDPOINT`, IP, bucket or port changes, update the edge and rerun
the real signed PUT/GET test. Do not blindly remove the explicit MinIO Host.

## Acceptance

`infra/scripts/https_ip_smoke.py` verifies trusted TLS, public page routes,
the dedicated smoke-account login, real presigned upload, confirmation and
byte-identical download. It creates only an archived, clearly named test bid.
Run it from outside ECS as well as from the host. Verify HTTP :8080 remains
usable and registration still renders in a browser. It does not prove new
account onboarding, AI generation or semantic compliance correctness.

Rollback of this HTTPS-only change: disable the two new site symlinks and
renewal timer, check/reload host Nginx. Existing Compose services, :8080,
accounts and persistent volumes are unaffected.
