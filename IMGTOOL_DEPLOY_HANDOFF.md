# Imgtool Deployment Handoff

Last updated: 2026-09-02 (Asia/Shanghai)

This handoff intentionally contains no SecretId, SecretKey, SSO secret, admin password, private key contents, or other runtime secrets.

## Server Access Status

Production traffic uses the Tokyo origin:

```text
Public IPv4: 43.165.190.2
SSH alias: togoapi-tokyo
SSH target: ubuntu@43.165.190.2
Hostname: togoapi-tokyo
```

Direct SSH was verified with:

```text
ssh togoapi-tokyo
```

The proxied Cloudflare A record for `img.togoapi.com` points to `43.165.190.2`. The Singapore origin `43.160.240.196` is retained only for the short rollback window with Imgtool stopped. The former US and isolated origins are retired and must not be used for active access or deployment.

## Production Deployment

Imgtool was migrated from Singapore to Tokyo and publicly verified on 2026-08-23:

```text
Deployment: /opt/imgtool
Service: imgtool.service
Binding: 127.0.0.1:18082
Nginx site: /etc/nginx/sites-available/imgtool
Public URL: https://img.togoapi.com
```

The service is enabled and active. The local and public `/api/health` endpoints return HTTP `200`; the frontend loads and shows the expected "请从主站进入" state without an SSO session.

The separate Let's Encrypt certificate is stored at:

```text
/etc/letsencrypt/live/img.togoapi.com/fullchain.pem
/etc/letsencrypt/live/img.togoapi.com/privkey.pem
```

It expires on 2026-10-23. Renewal uses manual DNS-01, so `certbot.timer` is disabled. Use the scripts in `D:\code\myapi\imgtool\deploy` and renew manually before expiry.

## Completed

### Grok image workspace release (2026-09-02)

Deployed the Imgtool Grok image workspace update to the Tokyo origin. The release adds
the `grok-imagine-image-2.0` text-to-image/image-edit preset, the `grok-4.6` image-to-text
preset, Grok aspect-ratio/resolution/quality controls, provider-prefixed model handling,
and `response_format=b64_json` for Grok image responses.

Release verification:

- Public frontend: `https://img.togoapi.com/` returned HTTP 200 and references the new frontend bundle.
- Local API: `http://127.0.0.1:18082/api/health` returned `{"data":{"status":"ok"},"ok":true}`.
- Public API health: `https://img.togoapi.com/api/health` returned HTTP 200.
- Service: `imgtool.service` active after restart; Nginx configuration test successful.
- Binary SHA-256: `046816a80656d487f95321de42a3d192ae0bf7a06a6145c163acb439ee21273b`.
- Rollback backup: `/opt/imgtool/.deploy-backups/20260902-160048/`.

### Grok upstream timeout hotfix (2026-09-02)

The public `api.togoapi.com` Cloudflare route could return HTTP 524 while a Grok
image request was still processing upstream. Imgtool now routes configured Core
requests directly to the local Core listener when the channel host is
`api.togoapi.com`:

```env
IMGTOOL_INTERNAL_CORE_BASE_URL=http://127.0.0.1:8080
```

Release verification:

- Binary SHA-256: `8cce4071a3f9c7bb7eeda8b7ef6275a9126821e7fb8f28bef9b4f30392546e3f`.
- Local and public `/api/health` returned `{"data":{"status":"ok"},"ok":true}`.
- `imgtool.service` is active after restart with exit status `0`.
- Rollback backup: `/opt/imgtool/.deploy-backups/20260902-162353/`.

### COS large-image delivery hotfix (2026-09-02)

Grok Base64 results were parsed correctly, but large result files could stall
while uploading from the Tokyo origin to the Guangzhou COS bucket. COS storage
now uses 1 MB multipart uploads with four workers and a two-minute request
timeout, so completed images are delivered through the same existing history
and preview pipeline without an unbounded `running` task.

Release verification:

- Binary SHA-256: `4039c993281a0b33a3ad4517cf80445cc90fc6676b8af04faf5fe7eb87f37940`.
- Local and public `/api/health` returned `{"data":{"status":"ok"},"ok":true}`.
- `imgtool.service` is active with zero restarts after deployment.
- Rollback backup: `/opt/imgtool/.deploy-backups/20260902-172600/`.

### Cloudflare domain

`img.togoapi.com` is proxied through Cloudflare:

```text
Hostname: img.togoapi.com
Record: A 43.165.190.2
Proxy status: Proxied
TTL: Auto
Origin protocol: HTTPS
```

The former EdgeOne CNAME is historical and is not the active production path.

### Tencent COS

COS bucket has been created:

```text
Bucket: togoapi-img-1385779910
Region: ap-guangzhou
Endpoint: https://togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com
Access: private read/write
Redundancy: single AZ
Server-side encryption: SSE-COS
```

CAM sub-user was created for imgtool COS access:

```text
User: imgtool-cos-writer
Access mode: programmatic access only
Policy currently attached: QcloudCOSDataFullControl
```

The downloaded CSV containing SecretId/SecretKey was placed locally under:

```text
D:\code\myapi\imgtool\新建子用户信息.csv
```

Do not commit, upload, paste, or screenshot this CSV. It should be removed or moved to a password manager after deployment.

### Local imgtool copy

The original image-generation project was copied into the main project:

```text
D:\code\myapi\imgtool
```

The original source directory was restored and no longer contains the Tencent COS changes:

```text
D:\code\aicode\ai\imgtool
```

The project-local copy `D:\code\myapi\imgtool` includes Tencent COS support. The original source remains on the Aliyun OSS implementation.

### Local imgtool environment

Local runtime env was created:

```text
D:\code\myapi\imgtool\.env
```

This file is ignored by `D:\code\myapi\imgtool\.gitignore`. It contains generated runtime secrets and the COS SecretId/SecretKey from the CSV. Do not print or commit it.

Configured non-secret values:

```env
IMGTOOL_ENV=production
IMGTOOL_HTTP_ADDR=127.0.0.1:18082
IMGTOOL_DATABASE_PATH=./data/imgtool.sqlite
IMGTOOL_STORAGE_PROVIDER=tencent-cos
TENCENT_COS_BUCKET=togoapi-img-1385779910
TENCENT_COS_REGION=ap-guangzhou
TENCENT_COS_ENDPOINT=https://togoapi-img-1385779910.cos.ap-guangzhou.myqcloud.com
TENCENT_COS_PREFIX=aiImg
TENCENT_COS_SIGNED_URL_EXPIRES=600
```

The same `IMGTOOL_SSO_SECRET` from this `.env` must later be set in the Portal runtime environment.

### Local verification

The copied project passes backend tests:

```powershell
cd D:\code\myapi\imgtool\server
$env:GOPROXY='https://goproxy.cn,direct'
go test ./...
```

Result:

```text
all packages passed
```

## Server State Already Checked

On the current Tokyo origin:

```bash
hostname
hostname -I
curl -fsS http://127.0.0.1:8080/health
sudo nginx -t
```

Observed:

```text
hostname: togoapi-tokyo
Public IP: 43.165.190.2
Core health: {"status":"ok"}
nginx -t: syntax is ok; test is successful
```

Core Nginx site:

```text
/etc/nginx/sites-available/sub2api
```

Core source certificate:

```text
/etc/nginx/ssl/togoapi/fullchain.pem
/etc/nginx/ssl/togoapi/privkey.pem
```

Current certificate SANs:

```text
DNS:api.togoapi.com
DNS:togoapi.com
```

Imgtool intentionally uses the separate certificate documented above rather than modifying the Core certificate.

## Operational Verification

Before maintenance or redeployment, verify SSH and the hostname:

```powershell
ssh togoapi-tokyo "echo ok && hostname"
```

Expected:

```text
ok
togoapi-tokyo
```

## Redeployment Procedure

### 1. Upload imgtool

Use SSH/SCP or rsync equivalent to upload:

```text
Local:  D:\code\myapi\imgtool
Remote: /opt/imgtool
```

Do not upload the CSV file to the server. Upload `.env` only to `/opt/imgtool/.env` with restrictive permissions.

Recommended remote permissions:

```bash
sudo chown -R ubuntu:ubuntu /opt/imgtool
chmod 600 /opt/imgtool/.env
```

### 2. Build or use existing binary

Existing local Linux binary:

```text
D:\code\myapi\imgtool\dist\imgtool-linux-amd64
```

Remote target:

```text
/opt/imgtool/dist/imgtool-linux-amd64
```

Make executable:

```bash
chmod +x /opt/imgtool/dist/imgtool-linux-amd64
```

### 3. Serve frontend

The Go server only handles `/api/*` and `/sso`. It does not serve Vite static files. Nginx must serve:

```text
/opt/imgtool/web/dist
```

If `web/dist` is missing or stale, build it locally before upload:

```powershell
cd D:\code\myapi\imgtool\web
npm install
npm run build
```

### 4. Install the systemd service

Use the tracked unit at `D:\code\myapi\imgtool\deploy\imgtool.service`. Install it as `/etc/systemd/system/imgtool.service`, then run:

```bash
sudo tee /etc/systemd/system/imgtool.service >/dev/null
sudo systemctl daemon-reload
sudo systemctl enable --now imgtool
sudo systemctl status imgtool --no-pager
curl -fsS http://127.0.0.1:18082/api/health
```

### 5. Renew the source certificate

Do not modify the Core certificate. Imgtool has its own Certbot lineage named `img.togoapi.com`.

Use the tracked scripts under `D:\code\myapi\imgtool\deploy` to start a manual DNS-01 renewal. Update `_acme-challenge.img` in DNSPod to the newly generated validation value, wait until both authoritative DNSPod nameservers return it, then allow Certbot to continue. Remove the temporary TXT record after successful issuance.

After renewal:

```bash
sudo openssl x509 \
  -in /etc/letsencrypt/live/img.togoapi.com/fullchain.pem \
  -noout -dates -ext subjectAltName
sudo nginx -t
sudo systemctl reload nginx
```

### 6. Install the Nginx site

Use the tracked site at `D:\code\myapi\imgtool\deploy\nginx-imgtool.conf`. Install it as `/etc/nginx/sites-available/imgtool`, enable it with a symlink under `/etc/nginx/sites-enabled/`, then validate and reload:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

### 7. Portal SSO env

Portal must be configured with:

```env
IMGTOOL_BASE_URL=https://img.togoapi.com
IMGTOOL_SSO_SECRET=<same value as /opt/imgtool/.env IMGTOOL_SSO_SECRET>
```

Do not generate a second SSO secret for Portal. It must match imgtool exactly.

### 8. End-to-end checks

After services are live:

```bash
curl -fsS http://127.0.0.1:18082/api/health
curl -kI --resolve img.togoapi.com:443:127.0.0.1 https://img.togoapi.com/
```

From local/client:

```powershell
curl.exe -I https://img.togoapi.com/
```

Expected public behavior:

- Direct visit without session shows imgtool's "enter from main Portal" state.
- Portal sidebar "图像生成" calls `/api/v1/integrations/imgtool/sso-url`.
- Browser opens `https://img.togoapi.com/sso?ticket=...`.
- Imgtool validates ticket, sets its HttpOnly session cookie, redirects to `/`.
- Generated image files are stored under COS prefix `aiImg/users/...`.

## Cleanup Later

After deployment succeeds:

- Move `D:\code\myapi\imgtool\新建子用户信息.csv` out of the repo tree or delete it after storing the credentials safely.
- Consider replacing `QcloudCOSDataFullControl` with a custom single-bucket policy limited to `togoapi-img-1385779910` and prefix `aiImg/*`.
- Update `SERVER.md` with the final imgtool deployment paths, systemd unit, certificate status, and verification results.
