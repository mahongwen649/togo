# TogoAPI Server Handoff

Last updated: 2026-10-14 (Asia/Shanghai)

This is the deployment handoff for future sessions. It deliberately contains no passwords, private-key contents, Admin API Keys, database credentials, or application secrets.

## Architecture

| Public name | Responsibility | Current state |
| --- | --- | --- |
| `https://api.togoapi.com` | Sub2API Core, `/v1/*`, and internal account-pool administration | Deployed and healthy through Cloudflare |
| `https://newsub.43-165-190-2.sslip.io` | Standalone `ranxi2001/sub2api` `v2.7.8` instance with isolated data and dependencies | Deployed and healthy through direct HTTPS |
| `https://togoapi.com` | Public TogoAPI Portal and same-origin Core adapter | Deployed and healthy through Cloudflare |
| `https://img.togoapi.com` | Imgtool frontend, API, and Portal SSO endpoint | Deployed and healthy through Cloudflare |
| `https://togoapi.com/gift/` | Lucky-code campaign | Deployed and healthy through Cloudflare |
| `https://togoapi.com/lottery/` | Lottery campaign | Deployed and healthy through Cloudflare |

Core is the single source of truth for users, API keys, balances, billing, and account-pool execution. Portal must use Core's official APIs and must not access the Core database directly.

Production traffic uses the Tokyo origin `43.165.190.2`. The former Singapore application services are stopped; its two PostgreSQL containers remain running temporarily as the migration rollback point.

Expected public model API base URL:

```text
https://api.togoapi.com/v1
```

## SSH access

Primary production origin, Tokyo:

| Field | Value |
| --- | --- |
| Public IPv4 | `43.165.190.2` |
| SSH alias | `togoapi-tokyo` |
| SSH target | `ubuntu@43.165.190.2` |
| SSH port | `22` |
| Authentication | Ed25519 public-key authentication |
| Local private key | `C:\Users\Administrator\.ssh\id_ed25519_togoapi_sg` |
| Local known hosts | `C:\Users\Administrator\.ssh\known_hosts` |

PowerShell connection command:

```powershell
ssh togoapi-tokyo
```

Rollback origin, Singapore, retained temporarily:

| Field | Value |
| --- | --- |
| Public IPv4 | `43.160.240.196` |
| SSH alias | `togoapi-sg` |
| SSH target | `ubuntu@43.160.240.196` (port `22222`) |
| Purpose | Emergency rollback only; applications stopped, PostgreSQL retained |

The US server `170.106.190.46` is a legacy host and is not an active production or rollback origin. The matching public keys are in the `ubuntu` users' `~/.ssh/authorized_keys`. Never copy private keys into this repository or a deployment directory.

## Server profile

Primary production origin:

| Field | Value |
| --- | --- |
| Provider | Tencent Cloud Lighthouse |
| Region | Tokyo, zone 2 |
| Hostname | `togoapi-tokyo` |
| Operating system | Ubuntu 24.04.4 LTS |
| CPU / memory / disk | 4 cores / 8 GB / 120 GB |
| Privilege | `ubuntu` has passwordless `sudo` |
| Runtime | Docker Engine 29.6.1 and Docker Compose 5.3.1 |
| Public ingress | Nginx on ports `80` and `443` |

The Tokyo Tencent Cloud firewall currently permits SSH `22`, HTTP `80`, HTTPS `443`, and ICMP from all IPv4 addresses. Restrict the web ports to Cloudflare's official origin IP ranges and SSH to trusted administrator addresses after the rollback window.

## Core deployment

Operational record, 2026-10-14 (official `v0.2.16` deployment):

- Local `core/` fast-forwarded 80 commits from `3a6fd1c9d` to official `Wei-Shaw/sub2api` main `9c2cca9e627291bc719bada3d57de23b625a4180`, with a clean working tree. The latest stable release is `v0.2.16`, published 2026-10-13 15:20:24 Asia/Shanghai, commit `01842576b3d5c1e2545b5a99ad555d36e28ed306`. Main differs from the release only in VERSION and sponsor documentation/assets. Updates include Gemini explicit caching/Vertex support, group localization, account scheduling and Responses fallback fixes, and GPT-6.1 Sol subscription long-context billing corrections.
- Production runs official `weishaw/sub2api:0.2.16`, digest/image ID `sha256:799e9b4ee9e3164f13fe74f4df93022048cb902eda533b98fe5751c883eb1a7c`. Image source/revision and executable version were verified. Core started at 2026-10-14 17:15:06 Asia/Shanghai, healthy with zero restarts. Core PostgreSQL/Redis, Portal API/Web/database, Lottery, and standalone NewSub containers retained their IDs and start times.
- Verified PostgreSQL custom-format backup (289432872 bytes), readable restore listing (88027 bytes), protected runtime environment/Compose snapshots, migration/pricing snapshots, image metadata, and rollback tag are under `/opt/backups/sub2api-core-v0.2.16-20261014-171419/`. All backup SHA-256 checks passed. Full database restore was not rehearsed.
- All 292 existing migration checksums were preserved. Four additive migrations (`242_group_i18n.sql`, `243_gemini_cached_contents.sql`, `244_gemini_cached_contents_channel_usage.sql`, `245_gemini_cached_contents_mutation_lease.sql`) applied; all 296 records match official release source. Recharge multiplier `1`, subscription USD-to-CNY rate `0`, and recharge fee rate `0` were preserved.
- All 303 frontend focused tests across 19 files, Vue/TypeScript checking, and production build passed using upstream-compatible pnpm 9.15.9. Go 1.27.2 migration/domain/protocol/Claude/DTO/config/log-redaction package tests, focused gateway/Gemini/Vertex/long-context/billing/scheduling tests, handler/cache tests, and Portal Core-client tests passed. Resource-limited temporary test containers were removed automatically. Test source/logs are under `/opt/builds/sub2api-core-v0.2.16-20261014/`.
- Internal/public Core, admin UI, Portal, Imgtool, and NewSub checks returned `200`; unauthenticated models returned expected `401`; authenticated account/group APIs returned `200`, application code `0`. Nginx validation passed with existing protocol-option warnings. Recreation lasted 17:14:57–17:15:06 and included 8 model `502` responses. Initial post-start observation recorded 20 model `200` and no model `5xx`, with no ERROR/FATAL/PANIC logs. Application logs still include some upstream `503` statuses; this upgrade does not establish resolution of pre-existing upstream failures. See the final observation in the detailed report.
- Final observation through 2026-10-14 17:18:11 Asia/Shanghai: 54 post-start model `200`, zero model `5xx`, no ERROR/FATAL/PANIC logs, healthy, zero restarts. Four application-level upstream `503` statuses occurred without model-ingress `5xx` in the observation window. Backup hashes, migrations, pricing, and dependency snapshots reverified successfully.
- Scripts, migration manifest, validation logs, and rollback instructions: `artifacts/core-upgrade-20261014.md`, `artifacts/upgrade-core-20261014.sh`, `artifacts/verify-core-20261014.sh`. Only Core was deployed; unrelated local changes were preserved.

Operational record, 2026-10-09 (official `v0.2.15` deployment):

- Local `core/` fast-forwarded 146 commits from `3f1a2ea0a` to official `Wei-Shaw/sub2api` main `3a6fd1c9db07203ca308aaba69e502bc1f35b307`, with a clean working tree. Release `v0.2.15` points to `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`; main differs only in VERSION. Changes add Cline/Command Code platforms, centralize platform validation, fix protocol conversion and billing, and update Go to 1.27.2.
- Production runs official `weishaw/sub2api:0.2.15`, digest/image ID `sha256:da0fd94daebf890419e21e956821fd6c2008fc666f9617f2eb917f4492245da3`. Image source/revision and executable version were verified. Core started at 2026-10-09 14:38:31 Asia/Shanghai, healthy with zero restarts. Core PostgreSQL/Redis, Portal API/Web, and standalone NewSub retained their container IDs and start times.
- Verified PostgreSQL custom-format backup (288081075 bytes), restore listing (88027 bytes), protected runtime environment and Compose snapshots, migration/pricing snapshots, image metadata, and rollback tag are under `/opt/backups/sub2api-core-v0.2.15-20261009-143751/`. All backup SHA-256 checks passed. Full database restore was not rehearsed.
- All 291 existing migration checksums match the official release. Startup added only `242_drop_platform_check_constraints.sql`, checksum `a7725f31f637208d200e0f37d5270e9f263743d99fa9e4f1bb9a7387e8be5a61`. It removes two database platform CHECK constraints in favor of application validation; it does not remove business data. The resulting 292 migration records match official source. Recharge multiplier `1`, subscription USD-to-CNY rate `0`, and recharge fee rate `0` were preserved.
- Frontend focused tests passed 58/58; type checking and production build passed. Core migration/domain/protocol/xAI/WebSocket package tests, focused platform/routing/web-search tests, Claude billing and WebSocket group-pricing/image-usage tests, and Portal Core-client tests passed. The first service compile exceeded the isolated test container's 3 GiB limit; rerun with 4 GiB and reduced Go GC targets passed. Production was unaffected; temporary test containers were removed.
- Internal/public Core, admin UI, Portal, Imgtool, and NewSub checks returned `200`; client-side Core/Portal/Imgtool checks also returned `200`. Unauthenticated models returned expected `401`; authenticated account/group APIs returned `200`, application code `0`. Nginx validation passed with existing duplicate protocol-option warnings. During recreation one model request returned `502` at 14:38:29. The subsequent short observation counted at least 66 model `200`, zero model `5xx`, and no ERROR/FATAL/PANIC logs. Pre-upgrade observations included Grok video billing errors; this short check does not establish their permanent resolution.
- Scripts and rollback notes: `artifacts/core-upgrade-20261009.md`, `artifacts/upgrade-core-20261009.sh`, `artifacts/verify-core-20261009.sh`. Only Core was deployed; other local changes remain untouched.

Operational record, 2026-10-08 (official `v0.2.14` deployment):

- Local `core/` fast-forwarded 8 commits from `b8dece900` to official `Wei-Shaw/sub2api` main `3f1a2ea0a`, with a clean working tree. Release `v0.2.14` points to `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d`; main differs only in VERSION. This release fixes EasyPay callback signature replay, hardens administrator credentials on fresh installs, enables API-key discovery for remote Codex model catalogs, and updates frontend dependencies. Existing administrators are unaffected and there are no database migrations.
- Production runs official `weishaw/sub2api:0.2.14`, digest/image ID `sha256:7e6f05e3e37fbc42706704be9e2336338046f631b17e4c6d074e86194db8e393`. Image source/revision and executable version were verified. Core started at 2026-10-08 10:37:03 Asia/Shanghai, healthy with zero restarts. Core PostgreSQL/Redis, Portal API/Web, and standalone NewSub retained their original container IDs and start times.
- Verified PostgreSQL custom-format backup (292063950 bytes), restore listing (88027 bytes), protected runtime environment and Compose snapshots, migration/pricing snapshots, image metadata, traffic observations, and rollback image tag are under `/opt/backups/sub2api-core-v0.2.14-20261008-103619/`. All backup SHA-256 checks passed. The 291 migration filenames/checksums were identical before and after; recharge multiplier `1`, subscription USD-to-CNY rate `0`, and recharge fee rate `0` were preserved. Deployment and verification scripts are `artifacts/upgrade-core-20261008.sh` and `artifacts/verify-core-20261008.sh`; details and rollback commands are in `artifacts/core-upgrade-20261008.md`.
- Core payment-provider and setup unit tests, focused payment return-URL/resume-token unit tests, Portal Core-client tests, frontend locale completeness (3 tests), type checking, and production build passed. The official key-modal suite has three stale string assertions that do not include the newly added `api_key_model_discovery`; all 30 tests passed using a temporary expectation-only correction, then the temporary file was removed. Official source remains unchanged. The reproducible validation script and expectation patch are under `artifacts/`.
- Internal/public Core, admin UI, Portal, Imgtool, and NewSub checks returned `200`; unauthenticated models returned expected `401`; authenticated account/group APIs returned `200` with application code `0`. Nginx validation passed with existing duplicate protocol-option warnings. During Core recreation, six `/v1/responses` requests returned `502` between 10:36:56 and 10:37:03. The subsequent short observation recorded 38 model requests with `200`, no model `5xx`, no ERROR/FATAL/PANIC logs, and zero restarts. This observation does not establish that pre-existing upstream account-pool failures are resolved.

Operational record, 2026-10-05 (official `v0.2.13` deployment):

- Local `core/` fast-forwarded 39 commits from `42bc7f6cf` to official `Wei-Shaw/sub2api` main `b8dece900`, with a clean working tree. Release `v0.2.13` points to `3040209f205472038c1ba745a1bedd2edd9053b1`; main differs only in VERSION. Updates include usage settlement fixes, TypeSafe support, email/payment security fixes, and recharge promotions.
- Production runs official `weishaw/sub2api:0.2.13`, digest/image ID `sha256:73797a8b44894c7b65c0bfcefe6002c7f3cfba6223f01e9d909c1d31365fcdf8`. Image source/revision and executable version were verified. Core started at 2026-10-05 18:01:27 Asia/Shanghai, healthy with zero restarts. Core PostgreSQL/Redis, Portal API/Web, and standalone NewSub retained their original container IDs and start times.
- Verified PostgreSQL custom-format backup, restore listing, Compose snapshot, dependency/image metadata, and rollback image tag are under `/opt/backups/sub2api-core-v0.2.13-20261005-180030/`. SHA-256 checks passed; `rollback-image.txt` records the retained previous image. Scripts: `artifacts/upgrade-core-20261005.sh` and `artifacts/verify-core-20261005.sh` (uses `artifacts/check-core-20260930.py`). Deployment requires `sudo` for backup directory access.
- Applied `241_add_payment_order_bonus_amount.sql` (checksum `18b6a524a9873d3e4a45e6f9bd03b6e69f4384b701825c659817e017f2e8244f`) and `241_add_typesafe_platform.sql` (checksum `b6559525bf8d0b5d7c617e7415944f0d1fae8c408e9131ace84139795f66dcd2`). Database checksums match trimmed official source. These add a default-zero field and expand platform constraints; existing recharge configuration was preserved.
- Portal Core-client tests and Core migration, TypeSafe, and xAI package tests passed. Internal/public Core health, public admin, Portal health, and Imgtool health returned `200`; unauthenticated models returned expected `401`. Nginx validation passed with existing duplicate protocol-option warnings. Initial post-cutover logs had no ERROR/FATAL/PANIC or HTTP 5xx, and live model requests returned `200`; this short observation is not a full authenticated feature regression.

Operational record, 2026-09-30 (official `v0.2.11` deployment):

- Local `core/` was fast-forwarded 56 commits from `9a62841fd` to official `Wei-Shaw/sub2api` `main` `42bc7f6cf`, with a clean working tree. Release `v0.2.11` points to `96f4c115c9749078f90cbf210a01d39baf3f53b6`; main differs only in the VERSION file. No migration files changed since `v0.2.9`.
- Production runs the official `weishaw/sub2api:0.2.11` image, digest/image ID `sha256:2e3b7fab1e84d2e862cde664d09c1ae84fc897093357cf200cbeb0f64b2f2390`. Image revision and executable version were verified. Core started at 2026-09-30 22:37:54 Asia/Shanghai and is healthy with zero restarts. PostgreSQL, Redis, and standalone NewSub retained their original start times.
- Verified PostgreSQL custom-format backup, restore listing, Compose snapshot, dependency metadata, image metadata, and rollback image tag are under `/opt/backups/sub2api-core-v0.2.11-20260930-223659/`. SHA-256 checks passed; `rollback-image.txt` records the retained previous image. Deployment script: `artifacts/upgrade-core-20260930.sh`.
- Portal Core-client tests passed. Internal/public Core health, public admin, Portal health, and Imgtool health returned `200`; unauthenticated models returned expected `401`. Nginx configuration validation passed with existing duplicate protocol-option warnings. Initial client-side TLS/timeouts resolved on retry and server-side public checks passed.
- Live Nginx `/v1/` observation recorded 44 HTTP `200` and 4 HTTP `502` after cutover; the preceding ten-minute baseline already had 58 HTTP `200` and 6 HTTP `502`. Core logs show upstream account retry/failover and account-selection warnings, with no ERROR/FATAL/PANIC entries. Health checks pass, but upstream request failures remain present; this upgrade does not establish that the existing account-pool failures are resolved.
- Official defaults now enable API Key creation limits (200 active keys per user, 60 creations per hour) and in-flight balance reservation. Low-balance concurrent requests can be rejected earlier. These defaults were retained.

Operational record, 2026-09-28 (official `v0.2.9` deployment):

- Official `Wei-Shaw/sub2api` `main` advanced from `a3eb7ef30` (0.2.8) to `9a62841fd` (0.2.9), 70 commits. The signed `v0.2.9` tag points to `4c00df2e0`; local `core/` was fast-forwarded to `9a62841fd` with no uncommitted changes. The official release does not include the previous local four-file account-test prompt hotfix, so the deployed image is the pure official release.
- A verified PostgreSQL custom-format backup, Compose snapshot, dependency metadata, and rollback image tag were created under `/opt/backups/sub2api-core-v0.2.9-20260928/`. The backup SHA-256 manifest passed.
- Production Core now runs `weishaw/sub2api:0.2.9`, image digest `sha256:1f4a15d278fdedfa80188a0de538b81222bcc9a6bbe93bf91522c864737a90e3`, revision label `4c00df2e0183e2c70b7fa8ba45914205e36aad0c`. Only `sub2api` was recreated; PostgreSQL and Redis container IDs and start times were unchanged. The application is healthy with zero restarts and both local/public health checks returned `{"status":"ok"}`.

The Tokyo server runs the production Sub2API Core database restored from the final Singapore snapshot on 2026-08-23, not a copy of the local development database.

| Item | Value |
| --- | --- |
| Deployment directory | `/opt/sub2api` |
| Compose file | `/opt/sub2api/docker-compose.local.yml` |
| Secret environment file | `/opt/sub2api/.env` |
| Current Core version | `Sub2API 0.2.16` (official release `01842576b3d5c1e2545b5a99ad555d36e28ed306`) |
| Current image | `weishaw/sub2api:0.2.16` (`sha256:799e9b4ee9e3164f13fe74f4df93022048cb902eda533b98fe5751c883eb1a7c`) |
| Core container | `sub2api` |
| PostgreSQL container | `sub2api-postgres` |
| Redis container | `sub2api-redis` |
| Core host binding | `127.0.0.1:8080` |
| Nginx site | `/etc/nginx/sites-available/sub2api` |
| Nginx backup before Portal cutover | `/etc/nginx/sites-available/sub2api.bak-20260726-045152` |

PostgreSQL and Redis are Docker-internal only and must never be exposed publicly.

Operational record, 2026-09-23 (official `v0.2.8` deployment):

- Local `core/` was fast-forwarded by 207 commits from `1c0a69c0c` to official `Wei-Shaw/sub2api` `origin/main` at `a3eb7ef30`, version `0.2.8`; signed release tag `v0.2.8` points to `fd80b08c9`. The four uncommitted Portal exclusive-account prompt-forwarding hotfix files were preserved and reapplied without conflicts. Newsub source was not used.
- The focused Go 1.27 unit tests for OpenAI OAuth/API-key Responses prompt forwarding passed. The frontend locale-key suite (3 tests), TypeScript/Vue production build, Vite production bundle, and Go release build passed. Upstream's 1536 MiB frontend build heap was insufficient for this release, so only the isolated server build checkout used a 4096 MiB Node heap; the local source was not changed for that build-only adjustment.
- Production Core now runs `togoapi/sub2api:0.2.8-official-main-hotfix-20260923` (`sha256:43c2e6a40d743f3af4aa42ec9a61ee3a57365a5fb33bf00ffbd6fede12b8bce0`), version `0.2.8-official-main-hotfix`, revision `a3eb7ef302961cba716dc78b39b93b60c467db0e-hotfix`. Only `sub2api` was recreated at 2026-09-23 21:20:57 Asia/Shanghai; it became healthy with zero restarts.
- PostgreSQL container `1b96c2815e51` and Redis container `90c990d5694d` retained their 2026-08-23 start times and zero restarts. Standalone Newsub was not recreated and remained healthy on `togoapi/sub2api:2.7.8-20260921-030247`.
- Startup applied `240_affiliate_ledger_operation_id.sql`; its recorded checksum `3823bfea5f64feb58f5fcebc341c6877eaefc97834b4cd652c8e83ad08ed78da` matches the application's trimmed-content checksum. Compose changed only the Core image line; its SHA-256 is `ff9bedd570109b8c94979208122fc28350501eaf81032bf022762fac0cc69782`.
- The verified pre-upgrade backup is under `/opt/backups/sub2api-core-v0.2.8-20260923-211906/`. It contains a 392538237-byte PostgreSQL 18 custom-format dump, an 87875-byte restore listing, a 3644-byte non-secret deployment archive, pre-upgrade container/migration metadata, and a SHA-256 manifest that passed completely. The previous image is retained as `togoapi/sub2api:rollback-core-before-v0.2.8-20260923-211906` (`sha256:1d11fa973ce6d646a95dcf4ac013c734093a85e22a08ac0ca37309f307de054c`).
- Nginx validation passed. Internal Core health/admin returned `200` and unauthenticated models returned `401`; public Core health/root/admin/models returned `200`/`308`/`200`/`401`; public Portal, Imgtool, and standalone Newsub health returned `200`. Initial observation recorded 16 completed live model requests with HTTP `2xx`, zero completed HTTP `5xx`, and no error/fatal/panic or migration-checksum failures. Two transient upstream `502` warnings from account `733` were handled by normal failover and did not produce client-facing failures.

Operational record, 2026-09-22 (official `main` restored, newsub base removed from Core):

- Local `core/` was already fast-forwarded to official `Wei-Shaw/sub2api` `origin/main` at `1c0a69c0c`, version `0.2.7`. The latest official tag remains `v0.2.7`; this deployment includes the 30 commits after that tag. The four uncommitted Portal exclusive-account prompt-forwarding hotfix files were retained. Newsub source was not used.
- Production Core now runs `togoapi/sub2api:0.2.7-official-main-hotfix-20260922` (`sha256:1d11fa973ce6d646a95dcf4ac013c734093a85e22a08ac0ca37309f307de054c`), version `0.2.7-official-main-hotfix`, revision `1c0a69c0c-hotfix`. Only the `sub2api` container was recreated at 2026-09-22 01:47:42 Asia/Shanghai. It became healthy with zero restarts.
- PostgreSQL container `1b96c2815e51` and Redis container `90c990d5694d` retained their 2026-08-23 start times and zero restarts. Standalone Newsub was not recreated and remains healthy on `togoapi/sub2api:2.7.8-20260921-030247`.
- Existing migration checksums matched official `main`. Startup applied `238b_content_moderation_engine_meta.sql` and `239_channel_reasoning_effort_multipliers.sql`. Compose changed only the Core image line; its SHA-256 is `2fffaab4466467c2ab13e20cc7cf4defaa2d2f7fb5b42e2a68f552efafeb3431`.
- The verified pre-upgrade backup is under `/opt/backups/sub2api-core-official-main-20260922-014644/`. It contains a 404416237-byte PostgreSQL 18 custom-format dump, an 87679-byte restore listing, a 3650-byte non-secret deployment archive, and a SHA-256 manifest that passed completely. The previous Core image is retained as `togoapi/sub2api:rollback-core-before-official-main-20260922-014644` (`sha256:2b9e33d9a73d64a371e56ebd303caaa5b7abccaa87236286b61d04d3a545377b`).
- Internal Core health, root, and admin returned `200`; unauthenticated `/v1/models` returned `401`. Public Core health, root, and models returned `200`, `308`, and `401`. Public Portal and Imgtool returned `200`. Startup logs contained the existing allowlist, trusted-proxy, and CORS warnings, with no panic, fatal, or migration checksum error. Live `/v1/responses` and `/v1/chat/completions` requests resumed immediately.

Operational record, 2026-09-21 (`v2.7.8` synchronized upgrade):

- Synchronized both deployments to signed `ranxi2001/sub2api` tag `v2.7.8`, commit `9ac0645a232c942c54852973ffbbf0a65568ecff`. The release adds web-managed Codex harvest parameters, single-account manual harvesting, node-name display, OpenAI pre-request TLS retry/runtime proxy fallback, and DeepSeek relay adaptations. It introduces no database migration.
- Standalone NewSub runs the pure image `togoapi/sub2api:2.7.8-20260921-030247` (`sha256:9be42a2d5d4d338f75db76bb5ad594d96c447639e8ac9b907509a0496f359016`). Core runs `togoapi/sub2api:2.7.8-hotfix-exclusive-account-test-20260921` (`sha256:2b9e33d9a73d64a371e56ebd303caaa5b7abccaa87236286b61d04d3a545377b`), retaining the four-file Portal exclusive-account prompt-forwarding hotfix. The focused Go unit test passed, and both frontend production and Go release builds passed.
- The combined image archive was 84,333,056 bytes and matched SHA-256 `49ac46c33c1599dc3b7afcfcf60d46df7ac58626cccb0ad9000757343b7b0796` before loading. Pre-upgrade configuration, protected environment files, container/image metadata, and validated PostgreSQL custom-format dumps are under `/opt/backups/sub2api-newsub-v2.7.8-20260921-031400/`; every manifest checksum passed.
- Only `newsub-app` and `sub2api` were recreated, in that order. Core PostgreSQL/Redis retained IDs `1b96c2815e51`/`90c990d5694d` and their 2026-08-23 start times; standalone PostgreSQL/Redis retained IDs `79bb85b2334c`/`a389bd4234a0` and their original start times. All four data containers remain at zero restarts.
- Both application containers became healthy with zero restarts. Core health/admin, Portal, Imgtool, and standalone NewSub health/admin returned `200`; unauthenticated Core models returned the expected `401`. The initial observation captured 18 successful live `gpt-5.6-sol` Responses log entries after cutover, with zero HTTP `5xx`, panic, or fatal entries.
- Previous images remain tagged as `togoapi/sub2api:rollback-core-before-v2.7.8-20260921-031400` (`sha256:4c48b1e199b65e4df60f74da3f86d6446086e17e3c5fdf869b3c6814db9a0332`) and `togoapi/sub2api:rollback-newsub-before-v2.7.8-20260921-031400` (`sha256:7f6add460cd1463319d0b56de392266f33703c3a96a2e69fa8a6c042d42c9a0d`).

Operational record, 2026-09-21 (initial Core switch to NewSub `v2.7.7` base):

- Replaced only the production Core application image with `togoapi/sub2api:2.7.7-hotfix-exclusive-account-test-20260921`, built from the `ranxi2001/sub2api` signed `v2.7.7` release commit `cee3c547fe30efee23db4e9150feb297a090c55d` plus the existing four-file Portal exclusive-account test prompt-forwarding hotfix. The focused OpenAI API-key Responses test passed before the image was built. The running image ID is `sha256:4c48b1e199b65e4df60f74da3f86d6446086e17e3c5fdf869b3c6814db9a0332`, version `2.7.7-hotfix-exclusive-account-test`, revision `cee3c547-hotfix`.
- Core PostgreSQL and Redis were not recreated or restarted: their container IDs remain `1b96c2815e51` and `90c990d5694d`, with their original 2026-08-23 start times and zero restarts. The standalone NewSub deployment also remains isolated and unchanged on image `sha256:7f6add460cd1463319d0b56de392266f33703c3a96a2e69fa8a6c042d42c9a0d`.
- Core and NewSub migration SQL filenames and contents match exactly, and the production database was current through migration `238_*` before cutover. The pre-cutover backup is `/opt/backups/sub2api-core-newsub-v2.7.7-20260921-002200/`; its PostgreSQL 18 custom-format dump passed `pg_restore -l` validation and all manifest SHA-256 checks. The exact prior Core image is retained as `weishaw/sub2api:rollback-before-newsub-v2.7.7-20260921-002200` (`sha256:442199ff9432e0092e9004d68b484c44c551d462ee5eab7235a5f0b1de2a8ebf`).
- The replacement Core container became healthy with zero restarts on `127.0.0.1:8080`. Core health/admin, Portal, Imgtool, and standalone NewSub health checks returned `200`; live Core `gpt-5.6-sol` traffic also returned `200` after cutover.
- Core currently has Codex ticket probing enabled with fail-open behavior and a harvest proxy URL of `http://127.0.0.1:3101`, but no Core-managed proxy sidecar is listening there. Probe attempts therefore log `connection refused`; relay traffic remains available because `openai_codex_ticket_fail_closed=false`. Decide separately whether to disable Core ticket probing or provision a dedicated Core sidecar. Do not share the standalone NewSub sidecar credentials implicitly.

## Standalone NewSub deployment

The `ranxi2001/sub2api` `v2.7.8` release is deployed as a separate instance and must not be confused with or attached to the production Core account pool.

| Item | Value |
| --- | --- |
| Public URL | `https://newsub.43-165-190-2.sslip.io` |
| Deployment directory | `/opt/newsub` |
| Compose file | `/opt/newsub/docker-compose.yml` |
| Secret environment file | `/opt/newsub/.env` (mode `600`) |
| Version / commit | `Sub2API 2.7.8` / `9ac0645a232c942c54852973ffbbf0a65568ecff` |
| Image | `togoapi/sub2api:2.7.8-20260921-030247` (`sha256:9be42a2d5d4d338f75db76bb5ad594d96c447639e8ac9b907509a0496f359016`) |
| Application container / binding | `newsub-app` / `127.0.0.1:8081` |
| PostgreSQL / Redis containers | `newsub-postgres` / `newsub-redis` |
| Docker network | `newsub_newsub-network` |
| Data directories | `/opt/newsub/data`, `/opt/newsub/postgres_data`, `/opt/newsub/redis_data` |
| Nginx site | `/etc/nginx/sites-available/newsub` |
| TLS certificate | `/etc/letsencrypt/live/newsub.43-165-190-2.sslip.io/` (automatic renewal through `certbot.timer`) |

Operational record, 2026-09-20:

- Upgraded only the standalone `newsub-app` from `v2.7.6` to `v2.7.7` at 2026-09-21 00:15 Asia/Shanghai. The release fixes empty-data rendering and equal-height cards on the harvest-flow page and has no database migration or gateway behavior change. The image was built from signed release tag commit `cee3c547fe30efee23db4e9150feb297a090c55d`; its transferred archive passed SHA-256 verification (`582333a49d7d78998a374d2c766d19ab1b9fa58cdc5492558b626489337d790d`) before loading. Only `newsub-app` was recreated; `newsub-postgres` and `newsub-redis` retained container IDs `79bb85b2334c` and `a389bd4234a0`, original start times, and zero restarts. The new application became healthy with zero restarts; public health, admin, and harvest-flow pages returned `200`, unauthenticated models returned `401`, administrator login and version `2.7.7` were verified, and the managed harvest sidecar remained reachable. Rollback metadata and the previous Compose file are retained under `/opt/backups/newsub-upgrade-v2.7.7-20260921-000930/`; the prior `v2.7.6` image remains installed. The original Core container remained healthy on image ID `sha256:442199ff9432e0092e9004d68b484c44c551d462ee5eab7235a5f0b1de2a8ebf` with zero restarts.

- An initial deployment mistakenly replaced the existing Core application container with the `v2.7.6` image. It was immediately rolled back to the exact prior `0.2.7-hotfix-exclusive-account-test` image `sha256:442199ff9432e0092e9004d68b484c44c551d462ee5eab7235a5f0b1de2a8ebf`. Core PostgreSQL and Redis were never recreated or restarted, and their original container IDs remained unchanged. After restoration Core was healthy with zero restarts and continued serving successful production traffic.
- NewSub was then deployed independently. Its application, PostgreSQL, Redis, network, persistent directories, environment file, host binding, Nginx virtual host, administrator account, and TLS certificate are all separate from Core. Both applications are healthy with zero restarts; public NewSub health, administrator UI, and `/admin/harvest-flow` return `200`, unauthenticated `/v1/models` returns `401`, and administrator login was verified.
- The standalone administrator email is `admin@newsub.local`; its password remains only in `/opt/newsub/.env` and is not stored in this handoff document.

Operational record, 2026-09-19 (`0.2.7` plus retained account-test hotfix):

- Official upstream rewrote `main` after `0.2.6`, removing the withdrawn Codex ticket series. Local `core/` was therefore realigned exactly to official `origin/main` at `1a9d49e16`, version `0.2.7`, instead of merging the withdrawn commits back in. The former official pointer is retained locally as `backup/core-pre-v0.2.7-20260919`. The four uncommitted account-test prompt hotfix files were preserved and reapplied without conflicts.
- Official tag `v0.2.7` points to release commit `aea725f2e`. Official image `weishaw/sub2api:0.2.7` has digest `sha256:411d9ca53be0dded773f7418e570bbdc4834c2cf6ff4ecad4e5347f882b6e79b` and revision label `aea725f2ea644d5592d0bbb1d63b607efa7e200a`.
- Production runs `togoapi/sub2api:0.2.7-hotfix-exclusive-account-test-20260919` through the Compose name `weishaw/sub2api:latest`. The image ID is `sha256:442199ff9432e0092e9004d68b484c44c551d462ee5eab7235a5f0b1de2a8ebf`, with version `0.2.7-hotfix-exclusive-account-test` and revision `1a9d49e16e-hotfix`. The retained hotfix continues to forward the custom prompt used by Portal exclusive-account tests.
- The focused Go unit test for OpenAI API-key Responses prompt forwarding passed. The official frontend locale-key tests passed (3 tests), the TypeScript/Vue production build passed, and the Go release build passed. There are no migration or Compose changes from `v0.2.6` to `v0.2.7`.
- Only `sub2api` was recreated at `2026-09-19 14:31:42` Asia/Shanghai. It became healthy with zero restarts. PostgreSQL container `1b96c2815e51` and Redis container `90c990d5694d` retained their 2026-08-23 start times and zero restarts. The protected `.env`, Compose SHA-256, and migration list remained unchanged.
- The verified pre-upgrade backup is under `/opt/backups/sub2api-core-upgrade-20260919-140857/`. It contains a 428837595-byte PostgreSQL 18 custom-format dump, a 3833-byte non-secret deployment archive, restore/archive lists, before/after metadata, and a 14-file SHA-256 manifest that passed completely. The prior image is retained as `weishaw/sub2api:rollback-0.2.6-hotfix-20260919-140857`.
- Internal Core health returned `200`; public Core root, health, admin, and unauthenticated models checks returned `308`, `200`, `200`, and `401`; public Portal and Imgtool returned `200`; Nginx validation passed. The post-cutover observation recorded 13 successful real Responses/Chat Completions relay requests and zero HTTP `5xx`, `ERROR`, `FATAL`, or panic entries. Four existing startup warnings remain for the disabled URL allowlist, direct-peer trusted-proxy behavior, and missing CORS origins.

Operational record, 2026-09-18 (`0.2.6` plus retained account-test hotfix):

- Local `core/` was fast-forwarded to official upstream `main` at `8b69738d7`, version `0.2.6`, while preserving the four existing uncommitted account-test prompt hotfix files without conflicts. Official image `weishaw/sub2api:0.2.6` has digest `sha256:81691a5f8c47a30a31376cb978d0a37075847c0c1ec552b13a3686ec5736801d` and label revision `49a39b6dc1abed30fd227611e8af1108bc427610`.
- Production runs the retained hotfix rebuilt on the official `0.2.6` mainline as `togoapi/sub2api:0.2.6-hotfix-exclusive-account-test-20260918`, image ID `sha256:ab29b8de5bfb6d3a195c8a09968d549e19bef839779edd611bc6a840fc31c255`, with revision label `8b69738d782ccaa7fd26511e1cca26ba8d1b58db-hotfix`. The hotfix preserves custom prompt forwarding used by Portal exclusive-account tests; upstream `0.2.6` does not yet contain it.
- The focused Go unit test for OpenAI API-key Responses prompt forwarding passed with the `unit` build tag. The official frontend locale-key tests (3 tests), TypeScript/Vue production build, and Go release build also passed. The new official Codex 292-ticket feature remains at its default disabled setting; no secret or deployment configuration was changed.
- Only the `sub2api` container was recreated at 2026-09-18 19:09 Asia/Shanghai. PostgreSQL and Redis retained their container IDs and 2026-08-23 start times; Core is healthy with zero restarts. The protected `.env` and Compose SHA-256 values remained unchanged. This release has no new SQL migration files relative to `0.2.5`.
- The verified pre-upgrade backup is under `/opt/backups/sub2api-core-upgrade-20260918-183844/`. Its 442939770-byte PostgreSQL 18 custom-format dump, 16722-byte non-secret deployment archive, restore list, archive list, and SHA-256 manifests passed revalidation. The previous production image is retained as `weishaw/sub2api:rollback-0.2.5-hotfix-20260918-183844`.
- Internal Core and Portal health returned `200`; public Core root, health, admin, and unauthenticated models checks returned `308`, `200`, `200`, and `401`; public Portal and Imgtool returned `200`. Initial post-upgrade logs recorded 118 successful HTTP requests, including 14 real Responses/Chat Completions relay requests, with zero `5xx`, `ERROR`, `FATAL`, or panic entries. Nginx configuration validation passed.

Operational record, 2026-09-15 (`0.2.5`):

- Local `core/` was fast-forwarded to official upstream `main` at `881f32026`, version `0.2.5`; the official Docker image was pulled from `weishaw/sub2api:0.2.5` with digest `sha256:5d5c2cdd45e8c944fec9aa5d379b5361f704dfcdf6d3be4f3446242d2e133cc2` (image label revision `86f93c28ee34cc74b629dafb748bd5ac5ca8c5ea`).
- Production Compose was synchronized with the official `GATEWAY_FORCE_CODEX_CLI` and `GATEWAY_OPENAI_COMPACT_MODEL` variables; the compact-model default is now `gpt-5.5`. Only the `sub2api` container was recreated. PostgreSQL and Redis retained their existing container IDs and start times, and Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260915-203902/`; the 444103953-byte PostgreSQL custom-format dump and 3823-byte non-secret deployment archive passed restore-list, archive-list, and SHA-256 checks.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.2.4-20260915-203902`.
- Migrations `238_opencode_go_platform.sql` and `238_purge_unlimited_user_platform_quotas.sql` were applied and recorded. The latter removed 1,798 inert platform-quota rows whose daily, weekly, and monthly limits were all unset, leaving zero such rows.
- Internal Core and Portal health returned `200`; public Core health, Portal, and Imgtool checks returned `200`, public Core root returned the expected `308`, and unauthenticated `/v1/models` returned `401`. Post-upgrade logs recorded 67 successful requests, including 11 real `/v1/responses` requests, with no `ERROR`, `FATAL`, or panic entries.

Operational record, 2026-09-09 (`0.2.4`):

- Local `core/` was fast-forwarded to official upstream `main` at `98d86915b`, version `0.2.4`; the official Docker image was pulled from `weishaw/sub2api:0.2.4` with digest `sha256:ccf47a1c62e355f51f896e489f8253e119fe4101b103cd701ba458cc6c6f0f77` (image label revision `5de5e2bed035d43591a2e10e51f4200ef6a84eb98`).
- Production Compose was updated with the official `SUB2API_IMAGES_MAIN_MODEL` variable and default `gpt-5.6-luna`. Only the `sub2api` container was recreated; PostgreSQL and Redis retained their existing container IDs and start times. Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260909-163748/`; the 447194491-byte PostgreSQL dump and 15634-byte deployment archive passed integrity checks and have SHA-256 records.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.2.3-20260909-163748`.
- Migration `237_add_minimax_platform.sql` was applied and recorded in `schema_migrations`.
- Local Core and Portal health returned `200`; public Core root returned the expected `308`, unauthenticated `/v1/models` returned `401`, and public Portal/Imgtool checks returned `200`. No `ERROR`, `FATAL`, or panic entry appeared in the post-upgrade Core log scan.

Operational record, 2026-09-08 (`0.2.3`):

- Production had already been upgraded to `Sub2API 0.2.3` using image digest `sha256:ef61f1535e2396acd8420baf6a2d07a25db7e891448b1c03608b2b00354c37cf` (image label revision `8fa67d477d6651a744754392a8982ea589c26ae6`).
- Its pre-upgrade backup remains under `/opt/backups/sub2api-core-upgrade-20260908-111919/`, and the previous image remains tagged `weishaw/sub2api:rollback-0.2.2-20260908-111919`.
- Migration `236_group_model_allowlist_repair.sql` was present in production before the `0.2.4` upgrade.

Operational record, 2026-09-08 (`0.2.2`):

- Local `core/` was fast-forwarded to official upstream `main` at `b7dba6267`, version `0.2.2`; the official Docker image was pulled from `weishaw/sub2api:0.2.2` with digest `sha256:fae238660404dd01100fce30426173de6f1251c6cb710c88eb69b8a2748f7441` (image label revision `5485f368b29d05adb95a00f71801c7c23d8f48af`).
- Production was upgraded by recreating only the `sub2api` container. PostgreSQL and Redis retained their existing container IDs, start times, and healthy state. Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260908-000203/`; PostgreSQL dump and deployment archive both passed integrity checks.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.2.1-20260908-000203`.
- Migration `235_group_model_allowlist.sql` was applied and recorded in `schema_migrations`.
- Local Core and Portal health returned `200`; public Core root returned the expected `308`, unauthenticated `/v1/models` returned `401`, and public Portal/Imgtool checks returned `200`.
Operational record, 2026-09-05 (`0.2.1`):

- Local `core/` was fast-forwarded to official upstream `main` at `ab99d56e9`, version `0.2.1`; the official Docker image was pulled from `weishaw/sub2api:0.2.1` with digest `sha256:b3845aad81d728a5e4efa4d677a638f27947be286ed9a17788a42a1f07fe7e50` (image label revision `578785ee7fb35030b094b69624efe225670a36f5f`).
- Production was upgraded by recreating only the `sub2api` container. PostgreSQL and Redis retained their existing container IDs, start times, and healthy state. Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260905-192557/`; PostgreSQL dump and deployment archive both passed integrity checks.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.2.0-20260905-192557`.
- Migrations `232_add_usage_log_upstream_request_id.sql`, `233_add_usage_log_upstream_request_id_index_notx.sql`, `234_channel_max_reasoning_effort_multiplier.sql`, and `234_group_codex_models_manifest_config.sql` were applied and recorded in `schema_migrations`.
- Local Core and Portal health returned `200`; public Core root returned the expected `308`, unauthenticated `/v1/models` returned `401`, and public Portal/Imgtool checks returned `200`.

Operational record, 2026-09-02 (`0.2.0`):

- Local `core/` was fast-forwarded to official upstream `main` at `5097b3145`, version `0.2.0`; the official Docker image was pulled from `weishaw/sub2api:0.2.0` with digest `sha256:553864545ec1b446c4b3e3b394599523463de2ff93170b4a5b0ad00026c8b945` (image label revision `aa236488351eb71e120fc2b6fb32e336b0374c918`).
- Production was upgraded by recreating only the `sub2api` container. PostgreSQL and Redis retained their existing container IDs, start times, and healthy state. Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260902-150425/`; PostgreSQL dump and deployment archive both passed integrity checks.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.185-20260902-150425`.
- Migrations `232_channel_cache_write_1h_pricing.sql`, `232_group_force_openai_fast.sql`, `232_group_reasoning_effort_over_limit.sql`, and `233_group_free_openai_fast.sql` were applied and recorded in `schema_migrations`.
- Local Core and Portal health returned `200`; public Core root returned the expected `308`, unauthenticated `/v1/models` returned `401`, and public Portal/Imgtool checks returned `200`.

Operational record, 2026-09-01 (`0.1.185`):

- Local `core/` was fast-forwarded to official upstream `main` at `a2fb09260`, version `0.1.185`; the official Docker image was pulled from `weishaw/sub2api:0.1.185` with digest `sha256:7a62844c1fc526653da996ed5003bece5d8b52ffa89735fe632c76492b47f65b` (image label revision `2ac784c51a5d0925b324efef2ba6b3446c364781`).
- Production was upgraded by recreating only the `sub2api` container. PostgreSQL and Redis retained their existing container IDs, start times, and healthy state. Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260901-141035/`; PostgreSQL dump and deployment archive both passed integrity checks.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.184-20260901-141035`.
- This release added no database migration; the highest recorded migrations remain the three `231_*` files from `0.1.184`.
- Local Core and Portal health returned `200`; public Core root returned the expected `308`, unauthenticated `/v1/models` returned `401`, and public Portal/Imgtool checks returned `200`.

Operational record, 2026-08-31 (`0.1.184`):

- Local `core/` was fast-forwarded to official upstream `main` at `200602b41`, version `0.1.184`; the official Docker image was pulled from `weishaw/sub2api:0.1.184` with digest `sha256:f941bb239b9b5c844f1565b3630e989e565144afb8ca4ef458f9ef91de4ca4a1` (image label revision `e98ef32eb29aecd30d1def615912ec4dc93173f3`).
- Production was upgraded by recreating only the `sub2api` container. PostgreSQL and Redis retained their existing container IDs, start times, and healthy state. Core started healthy with zero restarts.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260831-222102/`; PostgreSQL dump and deployment archive both passed integrity checks.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.183-20260831-222102`.
- Migrations `231_add_usage_log_native_compaction_v2.sql`, `231_add_usage_log_requested_reasoning_effort.sql`, and `231_user_restrict_public_groups.sql` were applied and recorded in `schema_migrations`.
- Local Core and Portal health returned `200`; public Core root returned the expected `308`, unauthenticated `/v1/models` returned `401`, and public Portal/Imgtool checks returned `200`.

Operational record, 2026-08-26 (`0.1.183`):

- Local `core/` was fast-forwarded to official upstream `main` at `7634e3c23`, version `0.1.183`; official release `v0.1.183` points to build commit `e8cb019fabf8b55199436229044cbf9aa7a82564`.
- Production was upgraded to `Sub2API 0.1.183` using image digest `sha256:cff6bc3ed1a6eba7ea240bad8637cf12856161a4efb98be0882c2fa7aff371e3` (image label revision `e8cb019fabf8b55199436229044cbf9aa7a82564`). Only the Core container was recreated; PostgreSQL and Redis retained identical container IDs and start times.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260826-005731/`. It contains a 320031965-byte custom-format PostgreSQL dump, an 86804-byte restore listing, a non-secret deployment archive, pre-upgrade container/image/migration metadata, and SHA-256 checksums. Secrets and live data directories were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.182-20260826-005731` for immediate application rollback.
- This release added no database migration or Compose change; the production `schema_migrations` set was unchanged after the upgrade.
- The Core container became healthy with zero restarts. Internal Core and Portal health, public Core health/settings/admin, Portal, and Imgtool health returned HTTP `200`; the Core root returned its expected HTTP `308`, and unauthenticated `/v1/models` returned the expected HTTP `401`. Six real Responses requests completed with HTTP `200` during initial verification, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs. A local mainland client saw intermittent Cloudflare connection timeouts during repeat checks, while checks from the Tokyo origin through the public Cloudflare routes remained successful.

Operational record, 2026-08-25 (`0.1.182`):

- Local `core/` was fast-forwarded to official upstream `main` at `aa2c4e8d1`, version `0.1.182`; official release `v0.1.182` points to build commit `5a7d469622911a6b1291a692376df5fa03f9ac2e`.
- Production was upgraded to `Sub2API 0.1.182` using image digest `sha256:f5febc3022c2ecfaa4d15fdf464c8e7bdb4433dc19fae282c2d947d6e69a109c` (image label revision `5a7d469622911a6b1291a692376df5fa03f9ac2e`). Only the Core container was recreated; PostgreSQL and Redis retained identical container IDs and start times.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260825-130800/`. It contains a 308723505-byte custom-format PostgreSQL dump, an 86804-byte restore listing, a non-secret deployment archive, pre-upgrade container/image/migration metadata, and SHA-256 checksums. Secrets and live data directories were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.181-20260825-130800` for immediate application rollback.
- This release added no database migration or Compose change; the production `schema_migrations` set was unchanged after the upgrade.
- The Core container became healthy with zero restarts. Internal Core and Portal health, public Core health/settings/admin, Portal, and Imgtool health returned HTTP `200`; the Core root returned its expected HTTP `308`, and unauthenticated `/v1/models` returned the expected HTTP `401`. Seven real Responses requests completed with HTTP `200` during initial verification, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs.

Operational record, 2026-08-25:

- Local `core/` was fast-forwarded to official upstream `main` at `e2d9b823f`, version `0.1.181`; official release `v0.1.181` points to build commit `3af5443b224823ae507a50c7b113aa50604409c8`.
- Production was upgraded to `Sub2API 0.1.181` using image digest `sha256:b0b9dec5ab978e8b872cedc2706e695c944b5322ae56b20ba99ee9d3871fe810` (image label revision `3af5443b224823ae507a50c7b113aa50604409c8`). Only the Core container was recreated; PostgreSQL and Redis retained identical container IDs and start times.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260825-004015/`. It contains a 302866352-byte custom-format PostgreSQL dump, an 85069-byte restore listing, a non-secret deployment archive, pre-upgrade container/image/migration metadata, and SHA-256 checksums. Secrets and live data directories were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.179-20260825-004015` for immediate application rollback.
- Migrations `229_plugins.sql` and `230_plugin_artifacts.sql` were applied and recorded in `schema_migrations`.
- The Core container became healthy with zero restarts. Internal Core and Portal health, public Core health/settings/admin, Portal, and Imgtool health returned HTTP `200`; the Core root returned its expected HTTP `308`, and unauthenticated `/v1/models` returned the expected HTTP `401`. Multiple real `/v1/responses` requests completed with HTTP `200`, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.

Operational record, 2026-08-20:

- Local `core/` was fast-forwarded to official upstream `main` at `2bc139ab5`, version `0.1.179`; the official release `v0.1.179` was verified through GitHub and Docker metadata (build commit `75f88be5f75c27771836b586f7de1503afa0e3bc`).
- Production was upgraded to `Sub2API 0.1.179` using image digest `sha256:ecf9d61dbad7411e01be3a7e440a5895b0773456f5e602b5eff5fb252b67a68e` (image label revision `75f88be5f75c27771836b586f7de1503afa0e3bc`). Only the Core container was recreated; PostgreSQL and Redis remained running with unchanged container IDs and start times.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260820-183719/`. It contains a 204 MB custom-format PostgreSQL dump, an 83 KB restore listing, a 28 MB non-secret deployment archive, pre-upgrade container and image metadata, and SHA-256 checksums. Secrets and live data directories were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.178-20260820-183719` for immediate application rollback.
- Migrations `226_add_usage_log_effective_model_indexes_notx.sql`, `227_composite_routes_add_cn_providers.sql`, and `228_channel_pricing_multipliers.sql` were applied and recorded in `schema_migrations`; the existing `226_channel_monitor_quota_mode.sql` migration remains recorded separately.
- The Core container became healthy with zero restarts. Internal and public Core health, Core public settings, Portal home, the Portal settings adapter, and Imgtool health returned HTTP `200`; the Core root returned its expected HTTP `308`, and unauthenticated `/v1/models` returned the expected HTTP `401`. Post-upgrade real `/v1/responses` traffic returned HTTP `200`.
- Release behavior note: long-context pricing now activates when either the group or account switch is enabled. At deployment time, 19 of 20 groups had `long_context_pricing_enabled=true` and one had it disabled; no business settings were changed automatically.

Operational record, 2026-08-18:

- Local `core/` was fast-forwarded to official upstream `main` at `49504adc9`, version `0.1.178`; release `v0.1.178` points to build commit `e0c48a19ed794a565e3858662520afe0a1f9f0ba`.
- Production was upgraded to `Sub2API 0.1.178` using image digest `sha256:e0f019383025679bd3b0f912c21fe7d8afdba8e42613391fa7fa208cc0762e60` (image label revision `e0c48a19ed794a565e3858662520afe0a1f9f0ba`). Only the Core container was recreated; PostgreSQL and Redis remained running with unchanged container IDs and start times.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260818-185351/`. It contains a 155852156-byte custom-format PostgreSQL dump, an 83947-byte restore listing, a 28771845-byte non-secret deployment archive, pre-upgrade container and image metadata, and SHA-256 checksums. The secret `.env`, application data, database data, Redis data, and live logs were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.177-20260818-185351` for immediate application rollback.
- Migrations `224_user_platform_quotas_add_cn_providers.sql`, `225_backfill_codex_fingerprint_seed.sql`, `225_channel_model_time_pricing.sql`, and `226_channel_monitor_quota_mode.sql` were applied and recorded in `schema_migrations`.
- The Core container became healthy with zero restarts. Internal and public Core health, Core public settings, Portal home, the Portal settings adapter, and Imgtool health returned HTTP `200`; the Core root returned its expected HTTP `308`, and unauthenticated `/v1/models` returned the expected HTTP `401`. Live streaming and non-streaming `/v1/responses` traffic returned HTTP `200`, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.

Operational record, 2026-08-16:

- Local `core/` was fast-forwarded to official upstream `main` at `baeac1f3d`, version `0.1.177`; release `v0.1.177` points to build commit `073e92d17178a1ccdb0a27017f572f10c9c7ab62`.
- Production was upgraded to `Sub2API 0.1.177` using image digest `sha256:f0534541ab9a510b0d190e3fe25288b6b82fdcaea905634c38660c43fab1ff08` (image label revision `073e92d17178a1ccdb0a27017f572f10c9c7ab62`). Only the Core container was recreated; PostgreSQL and Redis remained running.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260816-151048/`. It contains a 144381283-byte custom-format PostgreSQL dump, a 14023-byte non-secret deployment archive, pre-upgrade container and image metadata, and SHA-256 checksums. The secret `.env`, database data directories, Redis data directory, and live logs were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.176-20260816-151048` for immediate application rollback.
- Migrations `222_group_usage_daily_rollups.sql` and `223_group_usage_rollup_timezone.sql` were applied. The two rollup tables, all three `usage_logs_group_rollup_*` invalidation triggers, and the `Asia/Shanghai` rollup state were verified.
- The Core container became healthy with zero restarts. Internal and public Core health, Core public settings, Portal home, and the Portal settings adapter returned HTTP `200`; unauthenticated `/v1/models` returned the expected HTTP `401`. Live `/v1/responses` traffic returned HTTP `200`, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.
- Release behavior note: Codex OAuth accounts without an explicitly configured fingerprint-convergence mode now default to convergence disabled. Accounts with an explicit mode retain their configured behavior.

Operational record, 2026-08-13:

- Local `core/` was fast-forwarded to official upstream `main` at `0e82efe48`, version `0.1.176`; release `v0.1.176` points to build commit `e803e3851c0a7e222cfadeafad7b8636ab959d11`.
- Production was upgraded to `Sub2API 0.1.176` using image digest `sha256:905baf250580334dacd902471f61da7b8b1e5da57e3c8c1769489952d51771a1` (image label revision `e803e3851c0a7e222cfadeafad7b8636ab959d11`). Only the Core container was recreated; PostgreSQL and Redis remained running.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260813-101359/`. It contains a 127088106-byte compressed PostgreSQL dump, a 28781965-byte non-secret deployment archive, pre-upgrade image metadata, and SHA-256 checksums. The secret `.env`, database data directories, Redis data directory, and live logs were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.175-20260813-101515` for immediate application rollback.
- The Core container became healthy with zero restarts. Internal and public Core health returned HTTP `200`, unauthenticated `/v1/models` returned the expected HTTP `401`, live `/v1/responses` and `/v1/messages` traffic returned HTTP `200`, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.

- Local `core/` was fast-forwarded to official upstream `main` at `5935e674a`, version `0.1.175`; release `v0.1.175` points to build commit `93c32fa1a2450351561abc46156d2e28cb5f74ca`.
- Production was upgraded to `Sub2API 0.1.175` using image digest `sha256:2fc3e1cf3c1dd133283d69db5e8142cf3b11d1b16a8343f140b90674bb719c88` (image label revision `93c32fa1a2450351561abc46156d2e28cb5f74ca`). Only the Core container was recreated; PostgreSQL and Redis remained running.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260813-010843/`. It contains a 125714662-byte compressed PostgreSQL dump, a 64040889-byte non-secret deployment archive, and SHA-256 checksums. The secret `.env`, database data directories, Redis data directory, and live logs were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.173-20260813-011006` for immediate application rollback.
- The Core container became healthy with zero restarts. Internal and public Core health returned HTTP `200`, unauthenticated `/v1/models` returned the expected HTTP `401`, live `/v1/responses` and `/v1/messages` traffic returned HTTP `200`, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.

Operational record, 2026-08-09:

- Local `core/` was fast-forwarded to official upstream `main` at `48eb3766d`, version `0.1.173`.
- Production was upgraded to `Sub2API 0.1.173` using image digest `sha256:7a924a8ecd5ad103d7b2730a3e5f94db5d4f09bb8273f0298ac3e13380bdb3a1` (image label revision `29009f0b2ea14edf3b11ae2564fb617ff91a03b4`). Only the Core container was recreated; PostgreSQL and Redis remained running.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260809-140306/`. It contains a 98693579-byte compressed PostgreSQL dump, a 28781565-byte non-secret deployment archive, and SHA-256 checksums. The secret `.env`, database data directories, Redis data directory, and live logs were excluded from the deployment archive.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.172-20260809-140549` for immediate application rollback.
- The Core container became healthy with zero restarts. Internal and public Core health returned HTTP `200`, live `/v1/responses` and `/v1/messages` traffic continued returning `allow` security-audit decisions, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.
- New `account_scheduling_thresholds` settings are absent in the existing production database, so the upgraded Core logs a warning and falls back to the official default thresholds (`openai`, `anthropic`, and `grok` all `100`). No business setting was changed during the upgrade.

- Local `core/` was fast-forwarded to official upstream `main` at `cc67b1aca`, version `0.1.172`.
- Production was upgraded to `Sub2API 0.1.172` (build commit `155c49496`) using the image digest above. Only the Core container was recreated; PostgreSQL and Redis remained running.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260809-011743/`. It contains a 95182624-byte compressed PostgreSQL dump, a 28781571-byte non-secret deployment archive, container metadata, the pre-upgrade version, and SHA-256 checksums. The secret `.env` was excluded.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.171-20260809` for immediate application rollback.
- The Core container became healthy with zero restarts. Migrations added the `usage_logs.upstream_response_model` and `usage_logs.upstream_model_mismatch` columns. Internal and public Core health, Portal health, Core public settings, and the Portal settings adapter returned HTTP `200`; unauthenticated `/v1/models` returned the expected HTTP `401`.
- Live production traffic returned HTTP `200` on `/v1/responses` and `/v1/messages` after the upgrade, and no `ERROR`, `FATAL`, or panic entries appeared in the post-upgrade Core logs during verification.

Operational record, 2026-08-05:

- Local `core/` was fast-forwarded to official upstream `main` at `00b859617`, version `0.1.171`.
- Production was upgraded to `Sub2API 0.1.171` (build commit `f0e7a9c7a`) using the image digest above. Only the Core container was recreated; PostgreSQL and Redis remained running.
- The verified pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260805-015908/`. It contains a 435353226-byte PostgreSQL dump, its SHA-256 checksum, and non-secret deployment metadata. The secret `.env` remains server-side with mode `600`.
- The previous image is retained locally as `weishaw/sub2api:rollback-0.1.170-20260805` for immediate application rollback.
- The Core container became healthy with zero restarts, internal and public health checks returned HTTP `200`, live `/v1/responses` traffic returned HTTP `200`, and the new Codex version synchronizer selected `0.146.0` after startup.
- The production frontend build passed locally. The upstream full test run exposed existing test-isolation issues: backend default-config tests load the ignored local `deploy/.env`, and several `GroupsView` tests omit the new `getLiveCapability` API mock. Neither issue affected the official release image, production startup, or runtime health checks.

Operational record, 2026-08-03:

- Local `core/` was fast-forwarded to official upstream `main` at `7e2e9ba05`, version `0.1.170`.
- Production was upgraded to `Sub2API 0.1.170` (build commit `c043c2477`) using the image digest above.
- The pre-upgrade backup is stored under `/opt/backups/sub2api-core-upgrade-20260803-025837/` and contains a PostgreSQL dump plus non-secret deployment files.
- Migrations added the `profit_control_enabled`, `profit_min_margin`, and `profit_safety_buffer` group columns. The Core container, internal health endpoint, public health endpoint, and admin UI were verified after the upgrade.

Operational record, 2026-08-01:

- Local `core/` was synced to official upstream `main` at `682c4fe0e`, version `0.1.169`.
- Production was checked on the Singapore origin and already ran `Sub2API 0.1.169` from the image digest above.
- Before the production check/pull, a server-side backup was created under `/opt/backups/sub2api-core-upgrade-20260801-043931/` containing a PostgreSQL dump and non-secret deployment files. The backup stays on the server and must not be copied into this repository.
- Public checks after the sync returned HTTP `200` for `https://api.togoapi.com/health`, `https://togoapi.com/`, and `https://img.togoapi.com/api/health`.

## Portal deployment

| Item | Value |
| --- | --- |
| Deployment directory | `/opt/portal` |
| Compose files | `docker-compose.yml` and `docker-compose.prod.yml` |
| Secret environment file | `/opt/portal/.env` (mode `600`) |
| Web container | `portal-portal-web-1` |
| API container | `portal-portal-api-1` |
| PostgreSQL container | `portal-portal-postgres-1` |
| Web host binding | `127.0.0.1:3000` |
| Core Docker network | `sub2api_sub2api-network` |
| Production Core URL | `http://sub2api:8080` |

Operational record, 2026-09-28 (temporarily hide account packages and exclusive channels):

- Portal frontend now hides account-package and exclusive-channel navigation, package administration, desktop/mobile package-wallet balances, the dashboard package balance, and the package recharge form. Recharge uses one column. Existing package/exclusive routes redirect to dashboards; existing payment-result verification remains available.
- Visibility switches are centralized in `portal/frontend/src/config/portalFeatures.ts`. Backend services, payment settlement, workers, and existing business data were not disabled or changed.
- Local type checking, 49 focused tests, and local/server production builds passed. Only `portal-portal-web-1` was recreated; all other running container IDs were verified unchanged. Portal Web/API/PostgreSQL and Core are healthy with zero restarts; public Portal/Core health checks passed.
- Public entry is `/assets/index-BC4vACP0.js`. Public index, AppLayout, DashboardView, and RechargeView JS responses were verified byte-for-byte against the deployed container.
- Active Web image: `sha256:25feb465c2ea1ee41351fc99f6594b81ba7cc30e6dcadf800fa8ed92c29d0bba`. Rollback image: `portal-portal-web:rollback-hide-features-20260928`. Source backup, transferred release checksum, and before/after container metadata are under `/opt/backups/portal-hide-features-20260928/`.

Operational record, 2026-09-21 (administrator-only pelican artwork controls, Asia/Shanghai):

- Administrators can delete individual Pelican artworks and see the actual Core upstream account used to generate each artwork. Each new run sends a unique client request ID and resolves its account through the Core administrator usage API; recent historical rows are backfilled by API Key, model, and completion time. Non-administrator responses omit the account fields, and the account label and delete action are rendered only for administrators.
- Additive migration `024_pelican_artwork_admin.sql` added `client_request_id`, `account_id`, and `account_name`. The administrator-only delete route is `DELETE /api/v1/admin/pelican-checks/artworks/{artworkID}`. No production artwork was deleted for acceptance testing.
- Release: `/opt/releases/portal-pelican-artwork-admin-20260921-1101/`; verified source/database backup: `/opt/backups/portal-pelican-artwork-admin-20260921-1101/`. Rollback tags: `portal-portal-api:rollback-portal-pelican-artwork-admin-20260921-1101` and `portal-portal-web:rollback-portal-pelican-artwork-admin-20260921-1101`. Release archive SHA-256: `1c7d182c622afb7553eff594afffc813205fcdc63a51f228fe3c89827cbf24b5`.
- Active images are API `sha256:395ed83ecc6b2760cfce349d88b9ceaad26cc9e7be44d6919a84497e1eae5e85` and Web `sha256:b44be10511d64866ef472681b4201204f3d0a0b3a25b175869ee08b34f72d733`. Only Portal API/Web were recreated; Core and Portal PostgreSQL were not restarted.
- The full Portal backend suite, frontend type checking, five focused Vue tests, and the production frontend build passed. Production API/Web/PostgreSQL are healthy with zero restarts; Portal health and the administrator page return HTTP `200`, unauthenticated reads and deletes return `401`, the deployed bundle contains both new UI/API markers, all three migration columns exist, no Pelican run is stuck in `running`, and the post-cutover critical-log scan is clean. Backup checksums and the PostgreSQL restore listing were revalidated after the observation window.

Operational record, 2026-09-20 (single pelican artwork deletion at 17:00, Asia/Shanghai):

- Deleted only `portal_pelican_check_runs` row `#148`, matching the requested newest artwork: completed `2026-09-20 16:59:37.927977 +08`, latency `168553 ms`, group `不降智分组0.20`, Key `检查用`, and 17,641 bytes of stored HTML.
- A pre-delete data-only table backup is stored at `/opt/backups/portal-pelican-single-delete-20260920-1700/portal_pelican_check_runs.sql` with mode `600`, size 2,627,341 bytes, and SHA-256 `9730cba33a12e0810e6b58913c36e8621a5574e11a4f83ecafabaa564d9848b6`.
- Production verification confirms row `#148` is absent; the six newest remaining artwork IDs are `147, 146, 145, 144, 143, 142`. The enabled monitor configuration remains unchanged for group `不降智分组0.20` and Key `检查用`.

Operational record, 2026-09-20 (single pelican artwork deletion and recovery, Asia/Shanghai):

- An over-broad artwork cleanup was reverted from the verified pre-change backup. The `最近作品` gallery, large preview, `artworks` / `latest_artwork` API fields, successful-run HTML persistence, and the original enabled monitor configuration are restored.
- Only the requested artwork/check row `#136` was permanently deleted: completed `2026-09-20 11:00:58 +08`, latency `338564 ms`, group `不降智分组0.20`, Key `检查用`. Production confirms that row `#136` is absent.
- All other 114 pre-existing artworks were restored, totaling 2,371,729 bytes. Subsequent automatic run `#139` succeeded in 224,705 ms and stored 19,345 bytes of HTML, confirming that new artwork persistence still works. Production now contains 115 non-empty artwork rows totaling 2,391,074 bytes.
- Recovery source/database/image backup remains at `/opt/backups/portal-pelican-artworks-retired-20260920-114323/`. Rollback tags remain `portal-portal-api:rollback-pelican-artworks-retired-20260920-114323` and `portal-portal-web:rollback-pelican-artworks-retired-20260920-114323`. The temporary migration `024_remove_pelican_artworks.sql` was removed from both production and the local workspace.
- Four focused Vue tests and frontend type checking passed after local source restoration. Production Portal health and both pelican pages return HTTP `200`, unauthenticated API access returns `401`, API/Web containers are healthy, and the six newest artwork IDs are `139, 137, 135, 134, 133, 132`, with `#136` absent.

Operational record, 2026-09-20 (explicit account-check cancellation, Asia/Shanghai):

- Replaced the misleading `停止等待` action with `终止检查`. An active-account action now calls Portal `DELETE /api/v1/admin/account-checks/accounts/{accountID}/test`; Portal atomically marks the current administrator's latest running row as `cancelled`, cancels the exact in-memory request context sent to Core, and preserves the terminal history as `已终止`. `停止全部` uses the same server cancellation for every running account and locally cancels unsent queue entries. Ordinary refresh/navigation still allows accepted checks to continue.
- Migration `023_admin_account_check_cancellation.sql` extends the manual-check state constraint with `cancelled`. Its idempotent constraint update was exercised repeatedly by the database test suite. Cancellation is serialized with start/finish by the existing per-admin/account advisory lock, and a late upstream result can update only a still-`running` row, so it cannot overwrite a cancellation or terminate a later run.
- Release: `/opt/releases/portal-account-checks-cancel-20260920-012300/`; verified source/database/image rollback backup: `/opt/backups/portal-account-checks-cancel-20260920-012300/`. Rollback tags: `portal-portal-api:rollback-portal-account-checks-cancel-20260920-012300` and `portal-portal-web:rollback-portal-account-checks-cancel-20260920-012300`. Active API: `sha256:caab938a3c1f01246149da8580bd090ba5482a70f38ef9a92a70e179bc03c0ad`; Web: `sha256:d76fdf29a6c8a86e01dbe62a65aed1540db78532ce7a040c28a64306909d332a`.
- Focused backend tests with an isolated PostgreSQL database verified that cancellation reaches the Core request context, persists `cancelled`, survives the handler's late completion path, and is idempotent. All 21 focused frontend tests, type checking, production frontend build, and production API/Web builds passed. Production UI verification issued one mocked cancellation and zero model calls; the live no-running endpoint returned `cancelled=false`, unauthenticated cancellation returned `401`, the deployed database constraint includes `cancelled`, and public page/health returned HTTP `200`. API/Web are healthy with zero restarts and no critical API logs. Core, PostgreSQL/Redis containers and production configuration remained unchanged.

Operational record, 2026-09-20 (account-check live catalog and persistent selection, Asia/Shanghai):

- The account-check page now treats the current paginated Core OpenAI account catalog as the source of truth. Historical rows for accounts deleted in Core remain stored but are excluded from the overview and cannot recreate cards; live account names and groups replace stale history snapshots.
- Administrator selections are stored in Portal table `portal_admin_account_check_selection` (additive migration `022_admin_account_check_selection.sql`), isolated per administrator. Removing one card, cancelling all, selecting one account, or selecting all is saved before the visible selection changes. Refresh restores an explicitly empty selection, while reselecting an account still restores its latest three historical checks. Failed selection writes leave the current cards unchanged and show an error.
- Release: `/opt/releases/portal-account-checks-selection-20260920-001500/`; verified source/database/image rollback backup: `/opt/backups/portal-account-checks-selection-20260920-001500/`. Rollback tags: `portal-portal-api:rollback-portal-account-checks-selection-20260920-001500` and `portal-portal-web:rollback-portal-account-checks-selection-20260920-001500`. Active API: `sha256:bf4d2a600870996b6d4fda6c25ea1b2b24d0fffecd09de15d7c611f1ca3d9f65`; Web: `sha256:dc5f6a94f595c75bac4ebd40ca593426cd6d957277c587fcb88c02a4edfe90cb`.
- Focused backend database tests and 20 focused frontend tests passed. Mocked browser checks cover deleted-account filtering, current identity/group data, removal and empty selection across refresh, history recovery after reselection, failed-save rollback, and select-all persistence. Production read-only verification found 14 live OpenAI accounts, nine live accounts with history, nine visible cards, no deleted screenshot IDs, and no write/model requests. Public page/health returned HTTP 200; API/Web are healthy with zero restarts. Core, PostgreSQL/Redis containers and production configuration remained unchanged.

Operational record, 2026-09-19 (account-check upstream failure diagnosis and handling, Asia/Shanghai):

- Account #722 (`https://xcpcai.com/`, API-key account with Core WebSocket mode off) failed in persisted runs #11/#15/#17 after approximately 60.7 seconds, with 5,380–5,844 characters of partial output and `websocket closed before terminal event (code 1008)`. Core access/error logs confirm the same account-test failures; this is an upstream-reported termination, not the Portal five-minute timeout. A controlled short-answer probe also returned upstream `503 upstream_unavailable`. The supplier's internal timeout/policy cause could not be established or repaired from Portal.
- Portal now retries only explicit 502/503/504 failures before any output, once after two seconds within the original five-minute deadline and history row; attempts are recorded. Policy close 1008, authentication errors and partial generations are never automatically replayed. Existing and new results show Chinese upstream-failure explanations with original technical details; incomplete HTML/SVG is folded as escaped raw output and never previewed. Historical failures were not deleted or relabeled as successful, and Core/account settings were not changed.
- Release: `/opt/releases/portal-account-checks-upstream-20260919-192900/`; verified source/database/image rollback backup: `/opt/backups/portal-account-checks-upstream-20260919-192900/`. Rollback tags: `portal-portal-api:rollback-portal-account-checks-upstream-20260919-192900` and `portal-portal-web:rollback-portal-account-checks-upstream-20260919-192900`. Active API: `sha256:646df30e46aa1b77df9d84c0303aff81f496c3b29285369bd721432eadb168e6`; Web: `sha256:9963fe065edb6c8facfbe855882cda60285e366eaf18f10157ac67c109a47193`.
- All 18 focused frontend tests, backend tests without DB integration, production builds and public health/route checks passed. Tests cover retry recovery/bounds, no replay for policy/auth/partial failures, and folded incomplete markup. A read-only browser check inspected actual production account #722 history, confirmed translated errors/folded raw code and issued no model requests. API/Web are healthy with zero restarts and no critical post-cutover logs. Core, database/Redis containers and production configuration remained unchanged; in-flight Portal requests drained before replacement.

Operational record, 2026-09-19 (persistent administrator account-check history, Asia/Shanghai):

- Manual account checks now persist in Portal table `portal_admin_account_checks` (additive migration `021_admin_account_check_history.sql`). Records are isolated by administrator and retain the latest three per account, including question, model, sanitized account snapshot, result and timestamps. The page restores cards and latest results after refresh/login and supports selecting older records. No historical results were available to backfill from the previous memory-only implementation.
- Accepted checks survive browser disconnects with a five-minute deadline; normal HTTP-server shutdown drains the handlers. History reads expire interrupted running rows after six minutes. Transaction advisory locks reject duplicate active-account submissions and serialize retention; completion conditionally updates running rows. Frontend restored-running records poll every five seconds. Browser queue entries not yet submitted are not persisted.
- Release: `/opt/releases/portal-account-checks-history-20260919-174600/`; verified source, rollback images and Portal PostgreSQL dump: `/opt/backups/portal-account-checks-history-20260919-174600/`. Rollback tags: `portal-portal-api:rollback-portal-account-checks-history-20260919-174600` and `portal-portal-web:rollback-portal-account-checks-history-20260919-174600`. The additive table is retained on image rollback to preserve records.
- Active API: `sha256:1ba846dfb3da87931aba47bef67b61ad7da975db1dc8dd5590206545f4543476`; Web: `sha256:7093136a1e4aace525d3df8c045c08b545512a877d9fd37f8dbbc0d02139ac7d`. Only Portal API/Web were replaced; Core, PostgreSQL/Redis container IDs/start times and production environment/Compose checksums were unchanged.
- Full backend tests passed using an isolated PostgreSQL database; focused persistence tests cover retention, administrator isolation, concurrent admission, stale recovery and browser disconnect. All 17 frontend tests, type checking and production builds passed. Headless browser checks with mocked model/history responses verified refresh restoration without new checks and history selection, with screenshots inspected. Authenticated local/public history reads and unauthenticated 401 responses passed. Both Portal containers are healthy with zero restarts and no critical post-release log entries.

Operational record, 2026-09-19 (individual account-card check action, Asia/Shanghai):

- Added `开始检查` beside each card's group/schedule controls. The action submits only that account with its current model and shared/individual question; a retry replaces only that account's result. Card and batch checks share a three-request queue, reject duplicate active-account submissions, and support cancellation. Missing questions on other cards do not block an individual check.
- Web-only release: `/opt/releases/portal-account-checks-single-20260919-171500/`; source/image backup: `/opt/backups/portal-account-checks-single-20260919-171500/`; rollback tag: `portal-portal-web:rollback-portal-account-checks-single-20260919-171500`. Active Web image: `sha256:2b5540ebe933d408fbfa87e63056dd9f05f23cef5213cbdf966c8c18cbf05296`. API, Core, databases, Redis and production configuration retained their previous state.
- All 16 focused frontend tests and local/server production builds passed. Browser checks confirmed target-only requests, result retention, individual questions and unchanged compact viewport behavior, using intercepted model responses without real model calls. Local Docker and production were updated; public route/health and action assets are verified, Web is healthy with zero restarts, and post-cutover API/Web logs contain no critical entries.

Operational record, 2026-09-19 (compact account cards and viewport scrolling, Asia/Shanghai):

- Account checks now opt into a viewport-sized layout: the account picker and card list scroll internally, while select-all and run controls remain visible. Each account card combines its model/group/schedule controls with its own result. HTML/SVG previews and text use a compact 160px result area, with full-screen expansion available.
- Web-only release: `/opt/releases/portal-account-checks-cards-20260919-165900/`; source/image rollback backup: `/opt/backups/portal-account-checks-cards-20260919-165900/`; rollback tag: `portal-portal-web:rollback-portal-account-checks-cards-20260919-165900`. Active Web image: `sha256:b8d8023ef16acfc24fc5726f0dc0682461b56c2c2526368850f8149229ac77dd`. Portal API, Core, both databases, Redis and deployment configuration were unchanged.
- All 13 focused frontend tests and local/server production builds passed. Headless browser checks verified 19-account select-all, results bound to the matching cards, non-overlapping cards, internal scrolling, and no outer overflow at 1920x1080, 1366x768, 960x640 and mobile 393x852. Text and animation full-screen expansion passed; screenshots were inspected. Model responses were mocked during these layout checks, with no real test runs or account mutations.
- Local Docker and production were updated. Public Portal/Core health and the account-check route return 200; deployed card/preview/select-all assets are verified. Web is healthy with zero restarts, and post-release API/Web logs contain no critical entries.

Operational record, 2026-09-19 (administrator account checks, Asia/Shanghai):

- Deployed the Portal-only `/admin/account-checks` page with OpenAI account multiselect/select-all, shared or individual questions, default model `gpt-6-astra`, compact responsive workspace, and sandboxed HTML/SVG plus text results. Administrators can edit group bindings and manage existing Core scheduled-test plans; plans retain three results and use the Core default probe rather than custom questions.
- Release/staged source: `/opt/releases/portal-account-checks-20260919-161921/`. Verified source and Portal PostgreSQL backups: `/opt/backups/portal-account-checks-20260919-161921/`. Rollback tags: `portal-portal-api:rollback-portal-account-checks-20260919-161921` and `portal-portal-web:rollback-portal-account-checks-20260919-161921`. The allowlisted 22-file archive SHA256 is `011880a1794ca610a82c451c5b5e659c6d62bbfba865af3ee12b3fb76a6cc867`.
- Activated at approximately 16:25 Asia/Shanghai. API image: `sha256:9083916725d3b8618a95068c6410f46b73748ccc036a6ddcbeb13afba0687333`; Web image: `sha256:94aa84639ed9e78034cadb8b4dccf60da96f25cf56466a629d64f653fbabe620`. Only Portal API/Web were recreated. Core, both PostgreSQL containers and Core Redis retained their IDs/start times; production environment and Compose checksums were unchanged. No migration was included.
- Validation passed 13 focused frontend tests, frontend type checking/build, the backend suite, and both server production image builds. Public Portal health/account-check page and Core health return 200; unauthenticated account and schedule APIs return 401. Authenticated production login, 19 OpenAI accounts, six groups and scheduled-plan reads passed. Deployed layout/default-model assets were verified; API/Web are healthy with zero restarts and no critical startup/runtime log entries. Browser visual verification was unavailable.

Operational record, 2026-09-18 (sidebar promotion cleanup, Asia/Shanghai):

- Removed the `NEW` markers from the account-package and balance-recharge sidebar entries and the `推荐` marker from the exclusive-channel entry in both regular-user and administrator personal navigation. The account-package group now stays collapsed on initial render, including on account-package routes, and still expands when clicked.
- This was a Web-only deployment. Release backup: `/opt/backups/portal-sidebar-cleanup-20260918-182452/`; rollback tag: `portal-portal-web:rollback-portal-sidebar-cleanup-20260918-182452`; active Web image: `sha256:ea3d32157506dcd0631b5df102473e9bd5b37b44822d3f427de8e27a52aeb062`.
- Activated at 2026-09-18 18:28 Asia/Shanghai. Only `portal-web` was recreated. Portal API and PostgreSQL retained their container IDs and start times; all three Portal containers are healthy with zero restarts.
- Verification passed 18 focused sidebar tests, frontend type checking, local and server production builds, source and public-asset checksum comparison, local/public Portal and Core health checks, deployed marker inspection, and post-deployment critical-log scanning.

Operational record, 2026-09-18 (exclusive account-test timeout recovery, Asia/Shanghai):

- Account-level `降智测试` now has a five-minute backend deadline. Polling and repeat admission atomically mark a still-`running` test older than six minutes as failed, release any matching request-ledger slot, and allow a retry. Completion writes are conditional on the request still owning a running row, so a late old result cannot overwrite a timeout or a newer run. The worker context is tied to the Portal application lifecycle.
- Production contained eight orphaned account-test rows, all admitted before the prior API container start and with no unfinished request-ledger rows. After the verified backup they were changed from `running` to `failed` with the retry-safe interruption message; saved prior HTML and generation timestamps were retained. The final account-test state is eight failed, one succeeded, and zero running.
- Release backup: `/opt/backups/portal-exclusive-test-timeout-20260918-155317/`; staged source: `/opt/releases/portal-exclusive-test-timeout-20260918-155317/`. The custom-format Portal database dump has a valid restore listing. Rollback image: `portal-portal-api:rollback-portal-exclusive-test-timeout-20260918-155317` (`sha256:6f0b5fbda58cd9ee59ecbdb7bf2c0f8d5db4e4c321cd57fadbfcd89a10295f74`).
- Active API image: `sha256:d16288c8c04b7b12911532d187e090c4e9863a5fe915739d769bf2158b3a050b`. Only Portal API was recreated; Portal Web, Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis were not restarted. No migration was required.
- The full local Portal backend test suite, server production build and Compose validation passed. The API is healthy with zero restarts; internal/public Portal health, public Core health and the exclusive-channel page return HTTP `200`; the unauthenticated downgrade-test endpoint returns `401`; post-release API logs contain no error, fatal, or panic entries.

Operational record, 2026-09-18 (pelican scheduler lifecycle and actual-run display):

- Replaced the fixed 48-slot timeline with one point per actual run in the last 24 hours across all Keys. Counters are derived from the same returned records; multiple checks within one half hour remain separate, and absent checks produce no gray placeholders. Tooltips identify run ID, time, Key, result, retry count and failure reason.
- Pelican now uses its own transaction-scoped advisory lock (`734210093`), distinct from the exclusive-channel lock (`734210092`). Manual and scheduled checks share one runner and global in-progress admission guard. Persistent `next_run_at` no longer depends on history cleanup; the worker polls every 5 seconds and persists `scheduler_seen_at`. The UI uses server clock offset and explicit running, delayed and unavailable states.
- Accepted jobs have an 8-minute overall limit, survive HTTP disconnects, and are drained before normal API shutdown closes the database. Abandoned jobs older than 9 minutes are marked failed rather than remaining running indefinitely. Transient connection/SSE failures are retried once within the same recorded check; persistent failures remain visible with their reason. No production run records were deleted for this release.
- Prior run `#38` was a real upstream failure (`stream_read_error`), corroborated by Core request `d99ead85-e2c2-4ed1-a6c9-e293be60c7a2` on account `631`. Prior deployment-related cancellations were separate from that failure; removing failure history did not fix these causes.
- Additive columns are applied idempotently by migration `020_pelican_checks.sql`. Full source/database backups are under `/opt/backups/pelican-reliability-20260918-1316/` (dump restore-list validated, mode 600); staged source is `/opt/releases/pelican-reliability-20260918-1316/`. Rollback tags: `portal-portal-api:rollback-pelican-reliability-20260918-1316` and `portal-portal-web:rollback-pelican-reliability-20260918-1316`.
- Deployed API `sha256:6f0b5fbda58cd9ee59ecbdb7bf2c0f8d5db4e4c321cd57fadbfcd89a10295f74` and Web `sha256:6bbd749479a01274e18f1412f8c3493ac5267d34757ca4cdb876f161f6bd2ca2`. Scheduling was briefly paused, no active runs remained at cutover, and the original enabled group `57` / Key `593` was restored after health checks. Core and Portal PostgreSQL were not restarted.
- Validation: 5 focused Go/PostgreSQL regression tests passed against a disposable isolated database; 4 Vue component tests, Vue type checking, changed-file ESLint and both production builds passed. Public member/admin routes return 200, unauthenticated records API returns 401, and deployed assets contain the actual-record presentation. Browser visual inspection was not performed.
- Automatic run `#40` started at `13:26:16 +08` without a manual click and succeeded in `212746 ms` on the first attempt. The database contains 21 succeeded records, no failed/running records; next scheduled run remains `13:56:16 +08`, with scheduler heartbeat updating every 5 seconds.

Operational record, 2026-09-18 (pelican timeline cross-Key scope, Asia/Shanghai):

- Unified the 24-hour timeline with the summary scope: timeline points now include runs from all historical Keys, while the headline status and title remain tied to the currently configured group and Key. Half-hour cells are aligned to wall-clock `:00`/`:30` boundaries so adjacent runs do not collapse into one rolling-window cell.
- The 19 retained successful runs map to 19 distinct half-hour cells and therefore render as 19 green points. Successful history was not modified. Three non-successful rows created around deployment/retry (`#36`, `#37`, and `#38`) were removed individually; the final table contains 19 succeeded rows and no failed or running rows.
- Source backup: `/opt/backups/portal-pelican-timeline-scope-20260918-123637/`. Rollback images are `portal-portal-api:rollback-pelican-timeline-scope-20260918-123637` and `portal-portal-web:rollback-pelican-timeline-scope-20260918-123637`.
- Active images are API `sha256:acf913cca113b021963088865e7364bd0b622d178afdf2d4194b565283098e29` and Web `sha256:93ee4c7c70cd32e12f68ba517db1be6d40d634c9a7cc3968d6686467a6533c2e`. Portal PostgreSQL and Core were not recreated.
- Production API/Web builds passed. Portal API, Web, and PostgreSQL are healthy with zero restarts; local/public `/admin/pelican-checks` and the local health endpoint return HTTP `200`; post-cleanup logs contain no API error, fatal, or panic entries. No test suite was run, per request.

Operational record, 2026-09-18 (pelican running-card details, Asia/Shanghai):

- The running check card now shows the active operation (`正在生成鹈鹕骑自行车 HTML / SVG`), selected group and masked Key, and a live elapsed-time counter updated every second, filling the previously empty area while generation is in progress.
- Release backup: `/opt/backups/portal-pelican-running-detail-20260918-120235/`. Rollback image: `portal-portal-web:rollback-pelican-running-detail-20260918-120235`.
- Active Web image: `sha256:92fed6002fabadfd7035aac91236173966c777a811c315da6100058e94a7c826`. Only Portal Web was recreated; Portal API remained on `sha256:bb733e97d498ea606abbfa86a6d5d387bb1c4dcca6a62c3f51e279dbe4c132eb` and was not restarted.
- Web and API are healthy with zero restarts, the internal health endpoint and `/admin/pelican-checks` return HTTP `200`, and the deployed asset contains the running-operation marker. Manual run `#35` succeeded in 236225 ms; the table now contains 19 succeeded runs and no failed or running rows. No test suite was run, per request.

Operational record, 2026-09-18 (pelican run-now semantics and cross-Key history, Asia/Shanghai):

- Fixed the administrator `立即检查` action so unsaved group/Key/enable selections are persisted first, then exactly one check is queued. The API now validates the configured Key, acquires the pelican check lock, creates the `running` row synchronously, and only then acknowledges that the check started; stale/moved Keys and an already-running check return explicit conflicts instead of a false success.
- Saving configuration no longer implicitly starts a second check. The explicit run action owns immediate execution, while the existing 30-minute server monitor remains unchanged.
- The 24-hour summary and recent artwork gallery now span successful historical runs across Key changes. Current status and next-run scheduling still follow the currently configured Key, so switching Keys no longer makes earlier successful artwork appear deleted.
- The selected active Key `#593` (`检查用`, masked suffix `6734`) was persisted for group `#57`. Only failed pelican runs were deleted; the final data state was 18 succeeded, 0 failed, and 0 running. Pre-cleanup table/config backup: `/opt/backups/portal-pelican-failed-cleanup-20260918-112436/`.
- Release backup: `/opt/backups/portal-pelican-run-now-20260918-113739/`; staged source: `/opt/releases/portal-pelican-run-now-20260918-113739/`. Rollback tags are `portal-portal-api:rollback-pelican-run-now-20260918-113739` and `portal-portal-web:rollback-pelican-run-now-20260918-113739`.
- Active images are API `sha256:bb733e97d498ea606abbfa86a6d5d387bb1c4dcca6a62c3f51e279dbe4c132eb` and Web `sha256:24f5406e76a7da026d841bf9fd8fe86df7a373fa3747b72e3e65015872bd40a1`. Only Portal API and Web were recreated; Portal PostgreSQL, Core, Core PostgreSQL, and Redis retained their container IDs and start times.
- Frontend type checking, local/server production builds, PostgreSQL restore-list validation, public health/route checks, deployed authenticated read traffic, container health, and critical-log checks passed. No test suite was run, per request.

Operational record, 2026-09-18 (exclusive-channel graceful shutdown and stuck reclamation recovery, Asia/Shanghai):

- Channel `#10` / Core group `#55` remained in `releasing` because lease `#27` had two `running` request-ledger rows left by earlier Portal API replacements. Core access logs proved both requests completed with HTTP `200` at `2026-09-17 23:56:12.602+08` and `2026-09-18 00:19:30.083+08`; their matching Core usage rows were already billed and no upstream work remained active.
- After a verified Portal PostgreSQL backup, the two rows were reconciled to `finished` with their actual Core completion times. The worker closed lease `#27` at `01:21:35`, completed the configured 60-second cooldown, and returned the channel to `available` at `01:22:35`. Open exclusive leases and requests were both zero before the application release.
- Portal now handles `SIGTERM` with `http.Server.Shutdown`, cancels background workers through the application context, and allows up to 600 seconds for active requests to drain. Compose gives the API container 660 seconds before forced termination, preventing normal deployments from orphaning admitted exclusive requests.
- Recovery backup: `/opt/backups/portal-exclusive-stuck-recovery-20260918-0110/`; release backup: `/opt/backups/portal-exclusive-graceful-shutdown-20260918-0125/`; staged source: `/opt/releases/portal-exclusive-graceful-shutdown-20260918-0125/`. The database dump restore list contains 217 entries. Rollback image: `portal-portal-api:rollback-exclusive-graceful-shutdown-20260918-0125`.
- Active API image: `sha256:6160ae367227b7d4c0ccf5e5fde497d0f266c59a6fd73a49b403f0e0fcb95252`. Only Portal API was recreated; Web, Portal PostgreSQL, Core, Core PostgreSQL and Core Redis retained their prior start times and remained healthy with zero restarts.
- Server-side API compilation, production Compose validation, source checksum matching, local/public Portal and Core health checks, and post-release error-log inspection passed. A controlled API restart logged both `Portal API draining active requests` and `Portal API shutdown complete`; the recreated container exposes `stop=660` and is healthy with zero restarts.

Operational record, 2026-09-18 (pelican viewer access, Asia/Shanghai):

- Added the authenticated `/pelican-checks` route and sidebar entry so ordinary active members can view the 24-hour status timeline and recent artwork cards. Full administrators retain the `/admin/pelican-checks` management entry.
- Split the permissions at both UI and API layers: only full administrators can load Core group/Key choices, change the selected group/Key or enable state, and trigger an immediate check; the status/artwork read endpoint accepts any active authenticated member. No plaintext Key is returned by the read endpoint.
- Release backup: `/opt/backups/portal-pelican-viewer-access-20260918-004948/`; staged source: `/opt/releases/portal-pelican-viewer-access-20260918-004948/`. Rollback tags are `portal-portal-api:rollback-pelican-viewer-access-20260918-004948` and `portal-portal-web:rollback-pelican-viewer-access-20260918-004948`.
- Active images are API `sha256:fcd33c7f9815c6a9d6010a14ebf004bf946fc51f7407488d5083742412ddc47a` and Web `sha256:7805cf324f338394ea3bce030764c3aafc784015c5a6ba1688022e4f6cdfda7f`. Only Portal API and Web were recreated; Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times.
- Frontend type checking, local/server production builds, PostgreSQL backup restore-list validation, deployed route/asset checks, health checks, authenticated administrator loading, and unauthenticated rejection for both read and management endpoints passed. Both replacement containers are healthy with zero restarts, and post-deployment logs contain no runtime error, fatal, or panic entries.

Operational record, 2026-09-18 (compact pelican artwork cards, Asia/Shanghai):

- Replaced the oversized, cropped latest-artwork panel with a compact gallery of the six most recent successful pelican pages. Each 380px card renders a 16:9 preview through a four-times virtual iframe viewport scaled to fit, so the complete generated page is visible without an inner scrollbar; clicking a card opens the existing large preview.
- The administrator response now includes the six most recent successful artworks for the selected group and Key while retaining `latest_artwork` for compatibility. Failed runs remain visible only in the monitoring timeline and are not shown as artwork cards. No database migration was required.
- Release backup: `/opt/backups/portal-pelican-artwork-cards-20260918-003357/`; staged source: `/opt/releases/portal-pelican-artwork-cards-20260918-003357/`. Rollback tags are `portal-portal-api:rollback-pelican-artwork-cards-20260918-003357` and `portal-portal-web:rollback-pelican-artwork-cards-20260918-003357`.
- Active images are API `sha256:8a68e6b6b233849169d7f7bec524cd2478d473beeec7b14523fecceeaef314f4` and Web `sha256:4bf1d8be33e514527b8bc998fbf2c3adb8884df4e5c02e54c2c29189f793e36d`. Only Portal API and Web were recreated; Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times.
- Frontend type checking, local production build, server API/Web builds, PostgreSQL backup restore-list validation, authenticated artwork API loading, deployed asset-marker checks, health checks, and unauthenticated administrator rejection all passed. Both replacement containers are healthy with zero restarts and post-deployment logs contain no error, fatal, or panic entries.

Operational record, 2026-09-18 (pelican streaming check fix, Asia/Shanghai):

- Production failures were caused by the selected Core account accepting streaming Responses requests only while the monitor sent `stream: false`; Core returned HTTP `400 non_stream_not_allowed`. The monitor now sends `stream: true`, consumes Responses SSE completion/delta events, reports safe Core error details, and rejects failed or incomplete streams.
- The administrator page now polls every two seconds after a manual run or enabled configuration save, up to the six-minute request deadline, so short-lived `running` states and final results are visible without waiting for the normal 30-minute refresh.
- Release backup: `/opt/backups/portal-pelican-stream-fix-20260918-001052/`; staged source: `/opt/releases/portal-pelican-stream-fix-20260918-001052/`. Rollback tags are `portal-portal-api:rollback-pelican-stream-fix-20260918-001052` and `portal-portal-web:rollback-pelican-stream-fix-20260918-001052`.
- Active images are API `sha256:0bf2782891142ff69d4a81d89a5fd5de26d2f76732c80ba3917ca960282cac02` and Web `sha256:039110cd54c1acede94c96ff3a83d2257b7f574d6d13669d5d3832d0b9bf0fd4`. Only Portal API and Web were recreated; Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times.
- The PostgreSQL custom-format backup produced a valid restore listing. Frontend type checking and both production image builds passed; the replacement containers are healthy, Portal/Core health and `/admin/pelican-checks` return HTTP `200`, the unauthenticated administrator endpoint returns `401`, and startup logs contain no error, fatal, or panic entries. Live run `#7` advanced from `running` to `succeeded` in 233466 ms and stored 18859 bytes of generated HTML/SVG; the six pre-fix `400` runs remain as historical records.

Operational record, 2026-09-17 (administrator pelican drawing checks, Asia/Shanghai):

- Added the full-administrator-only `鹈鹕绘图检查` page. An administrator selects one active OpenAI Core group and API Key, explicitly enables monitoring, can trigger an immediate run, and can inspect the 24-hour timeline and latest sandboxed HTML/SVG result. The Portal database stores only the selected IDs and display names; it resolves the current Key from Core at execution time and does not persist the plaintext Key.
- Enabled checks run the fixed pelican bicycle HTML/SVG prompt every 30 minutes. Migration `020_pelican_checks.sql` created the singleton configuration and run-history tables. The production configuration is disabled by default and contained no run history immediately after deployment.
- Release backup: `/opt/backups/portal-pelican-checks-20260917-234345/`; staged source: `/opt/releases/portal-pelican-checks-20260917-234345/`. Rollback tags are `portal-portal-api:rollback-pelican-checks-20260917-234345` and `portal-portal-web:rollback-pelican-checks-20260917-234345`.
- Active images are API `sha256:ac2a4b1268f13c51baff936effe6f439ed61caa4594e46f2f67a43e02b773427` and Web `sha256:f77c0a3a18f3023e03d1fa5a0e4abcb440c1133fee7ca0599b4fc80aaf25ed66`. Only Portal API and Web were recreated; Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times.
- The PostgreSQL custom-format backup produced a valid restore listing. API and Web production builds passed, both replacement containers became healthy with zero restarts, public Portal/Core health and `/admin/pelican-checks` returned HTTP `200`, unauthenticated pelican administrator endpoints returned `401`, deployed asset markers were present, and post-deployment API logs contained no error, fatal, or panic entries.

Operational record, 2026-09-17 (compact exclusive-channel cards, Asia/Shanghai):

- The customer exclusive-channel lobby now uses content-height cards in capped 520-620px columns instead of stretching every card across the row. Card padding, account rows, API Key usage panels, and claim-action spacing were tightened to remove the large blank regions shown in the previous layout.
- Account downgrade-test details and the complete-page preview now share a two-column card layout when the card is at least 540px wide. The preview remains single-column on narrow screens and is capped at 340px on wider cards.
- This was a Web-only deployment. Release backup: `/opt/backups/portal-exclusive-cards-compact-20260917-215728/`; rollback tag: `portal-portal-web:rollback-portal-exclusive-cards-compact-20260917-215728`; active Web image: `sha256:4b71a46ea4958b721bea84a237518c58db2a75a06ac38e57d3f27382dbe9f197`.
- Activated at 2026-09-17 21:59 Asia/Shanghai. Only `portal-web` was recreated. Portal API, Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times and remained healthy with zero restarts.
- Verification passed 27 focused frontend tests, frontend type checking, local and server production builds, production source checksum comparison, deployed responsive-layout bundle marker checks, public Portal/Core health checks, and post-deployment Web log inspection.

Operational record, 2026-09-17 (pending exclusive pricing display, Asia/Shanghai):

- The administrator channel table now displays a saved pending multiplier as the primary value and labels it `待生效 · 当前 X×`. Pending image pricing and pending text-pricing mode are also shown separately from the active values, so deferred edits no longer appear to have been lost while a channel is active, releasing, or cooling.
- Production channel `#10` remained active with current multiplier `0.25`; its existing pending configuration was verified unchanged with multiplier `0.18`, account `#697`, image price `0.05`, and image multiplier `1`.
- This was a Web-only deployment. Release backup: `/opt/backups/portal-pending-pricing-display-20260917-201228/`; rollback tag: `portal-portal-web:rollback-portal-pending-pricing-display-20260917-201228`; active Web image: `sha256:7a120dbb1acf88c67618f25c8f01e7abcd8fb2e89fad794f7308cc236d96041e`.
- Activated at 2026-09-17 20:14 Asia/Shanghai. Only `portal-web` was recreated. Portal API, Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times and remained healthy with zero restarts.
- Verification passed 26 focused frontend tests, frontend type checking, local and server production builds, deployed bundle marker checks, production pending-configuration inspection, local/public health checks, and post-deployment Web log inspection.

Operational record, 2026-09-17 (exclusive ingress request-body limit, Asia/Shanghai):

- Fixed origin-generated HTML `413 Request Entity Too Large` responses for `https://togoapi.com/exclusive/v1/*`. The apex `togoapi.com` HTTPS server had retained Nginx's default 1 MiB body limit even though Portal Web allowed 16 MiB on the exclusive route; production failures were recorded for request bodies from approximately 3.1 MB through 11.6 MB.
- Added `client_max_body_size 256m` to the apex edge proxy so application-specific limits remain authoritative. The existing gift and lottery location includes were preserved, `nginx -t` passed, and Nginx was hot-reloaded without restarting any application or database container.
- Release backup: `/opt/backups/portal-exclusive-nginx-body-limit-20260917-195933/`. A 12,582,940-byte request was verified both against the local TLS origin and through public Cloudflare routing; both reached Portal API and returned structured JSON `401 INVALID_CHANNEL_KEY` instead of Nginx `413`. Subsequent real exclusive requests returned successful `200` responses, with no new oversized-body errors after the reload.
- Portal Web, Portal API, Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times and remained healthy with zero restarts.

Operational record, 2026-09-17 (full-page downgrade preview, Asia/Shanghai):

- The inline account downgrade-test preview is now a compact, right-aligned 4:3 window capped at 460px. The generated page reports its rendered dimensions to the trusted sandbox wrapper, which scales and centers the complete page inside the preview without exposing the former scrollable top-only crop. The enlarged and fullscreen preview behavior is unchanged.
- This was a Web-only deployment. Release backup: `/opt/backups/portal-downgrade-preview-fullpage-20260917-191751/`; rollback tag: `portal-portal-web:rollback-portal-downgrade-preview-fullpage-20260917-191751`; active Web image: `sha256:231abfd66d0b425aab0f4641e1b4b70f536a16db215b3d07c3d23e5e744652c3`.
- Activated at 2026-09-17 19:19 Asia/Shanghai. Only `portal-web` was recreated. Portal API, Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times and remained healthy with zero restarts.
- Verification passed 26 focused frontend tests, frontend type checking, local and server production builds, deployed scaling/CSS bundle marker checks, local/public Portal and Core health checks, and post-deployment Web log inspection.

Operational record, 2026-09-17 (inline downgrade-test preview, Asia/Shanghai):

- Saved account downgrade-test pages now load automatically inside each account card in a stable 190px compact preview. Starting a new test shows generation progress inline and keeps polling even when the optional enlarged preview is closed; the previous full-size and fullscreen preview remains available through `放大预览`.
- Equivalent account metadata returned by the channel-list poll does not refetch the saved HTML, preventing repeated preview requests while still reloading when the page state or generation timestamp changes.
- This was a Web-only deployment. Release backup: `/opt/backups/portal-inline-downgrade-preview-20260917-184711/`; rollback tag: `portal-portal-web:rollback-portal-inline-downgrade-preview-20260917-184711`; active Web image: `sha256:7ad04c6812601cff4c21f0531efa7b7dc59c38469c5ec3990d4a8ca897bab974`.
- Activated at 2026-09-17 18:54 Asia/Shanghai. Only `portal-web` was recreated. Portal API, Portal PostgreSQL, Core, Core PostgreSQL, and Core Redis retained their container IDs and start times and remained healthy with zero restarts.
- Verification passed 25 focused frontend tests, frontend type checking, local and server production builds, deployed bundle marker checks, local/public Portal and Core health checks, Nginx validation, and post-deployment Web log inspection.

Operational record, 2026-09-17 (exclusive API Key shared groups, Asia/Shanghai):

- API Key accounts may now remain bound to ordinary Core groups while also joining one Portal exclusive channel. Exclusive synchronization adds only the channel's Core group and preserves every pre-existing group; replacement, removal, retry, and channel deletion remove only that exclusive group. OAuth accounts retain the existing exclusive-group isolation rules.
- Candidate selection and publish/update validation no longer reject an otherwise healthy API Key account merely because it already has Core groups. Portal account-package reservations and ownership by another Portal exclusive channel remain conflicts.
- Release backup: `/opt/backups/portal-exclusive-apikey-shared-groups-20260917-1705/`. API rollback tag: `portal-portal-api:rollback-portal-exclusive-apikey-shared-groups-20260917-1705`.
- Active images: API `sha256:36c6f2c9e160f8c2b4d102f2acdc8f943fb29b90c4783d8d3b12da59cfeb5a2f`; Web remains `sha256:21a9729e1e2573283ea680b6722d01dfe8a9d418d2162fb4b870e9c315541f59`. The retained API release tag is `release-portal-exclusive-apikey-shared-groups-20260917-1705`.
- Activated at 2026-09-17 17:14 Asia/Shanghai. Only Portal API was recreated and is healthy with zero restarts. Portal Web, PostgreSQL, and Core retained their existing container IDs and start times. Channels `#10` and `#11` remain in their previous error state until the administrator explicitly retries synchronization.
- Verification passed all focused `TestExclusive*` tests against an isolated PostgreSQL container, including publish, retry, candidate selection, replacement, and deletion coverage for shared API Key groups; the server API production build, database backup restore-list validation, Nginx validation, local/public health checks, unauthenticated administrator API rejection, exact deployed-source checksums, and post-deployment log inspection also passed.

Operational record, 2026-09-17 (administrator account downgrade test, Asia/Shanghai):

- The administrator exclusive-channel table now has a beaker action that opens the channel's bound accounts and allows a direct account-level `降智测试` without claiming the channel first. Existing saved-page preview and progress polling are reused.
- The API permits full administrators to start a bound account test on an available channel or while another user holds it. Administrator tests do not create lease request rows or apply holder billing/concurrency admission checks. Ordinary users remain limited to their own active, unexpired lease, and unresolved duplicate tests still return `409` before the lease concurrency limit.
- Release backup: `/opt/backups/portal-admin-downgrade-test-20260917-1630/`. Rollback tags are `portal-portal-api:rollback-portal-admin-downgrade-test-20260917-1630` and `portal-portal-web:rollback-portal-admin-downgrade-test-20260917-1630`.
- Active images: API `sha256:e8e045241495acdb977de56d6d1518918c90dba31ec8d81403b7c8635d012f73`; Web `sha256:21a9729e1e2573283ea680b6722d01dfe8a9d418d2162fb4b870e9c315541f59`. Matching retained release tags use `release-portal-admin-downgrade-test-20260917-1630`.
- Activated at 2026-09-17 16:41 Asia/Shanghai. Only Portal API/Web were recreated; both are healthy with zero restarts. Portal PostgreSQL and Core retained their existing container IDs and start times. No migration or production data update was required; channel `#10` remained active with account `#697`.
- Verification passed 23 focused frontend tests, frontend type checking and production build, all `TestExclusive*` backend tests plus the duplicate-admission regression against an isolated PostgreSQL container, server API/Web production builds, backup restore-list validation, Nginx validation, local/public health checks, unauthenticated downgrade-test rejection, deployed bundle/binary markers, and post-deployment log inspection.

Operational record, 2026-09-17 (exclusive-channel occupied editor, Asia/Shanghai):

- Administrator channel editing now remains usable while a channel is active, releasing, or cooling. Name, durations, and cooldown changes remain immediate; account, multiplier, and pricing edits are stored as a pending configuration and are synchronized to Core automatically after the current lease closes and cooldown completes. Reopening the editor restores pending values and pending account selections.
- Migration `019_exclusive_pending_config.sql` adds the nullable `portal_exclusive_channels.pending_config` JSONB column. Existing channels and leases are unchanged; production channel `#10` was `available` with no pending configuration after deployment verification.
- Release backup: `/opt/backups/portal-exclusive-editor-20260917-1557/`. Rollback tags are `portal-portal-api:rollback-portal-exclusive-editor-20260917-1557` and `portal-portal-web:rollback-portal-exclusive-editor-20260917-1557`.
- Active images: API `sha256:f20b0b9c1dec14e838b64a8f92fe5f83c4c9e25aeba78dac24c2c895b8b4832b`; Web `sha256:c0ac5c10f775ef7a37930fc01fff2b3f100086dde96c9c9ade63d13df8098217`. Matching retained release tags use `release-portal-exclusive-editor-20260917-1557`.
- Activated at 2026-09-17 16:14 Asia/Shanghai. Only Portal API/Web were recreated; both are healthy with zero restarts. Portal PostgreSQL and Core retained their existing container IDs and start times.
- Verification passed 15 focused frontend tests, frontend type checking and production build, all focused `TestExclusive*` Go tests against an isolated PostgreSQL container, server API/Web production builds, PostgreSQL backup restore-list validation, Nginx validation, public and local health checks, unauthenticated administrator API rejection, production migration inspection, and public delivery of the updated exclusive-channel bundle. Post-deployment API/Web logs contain no error, fatal, or panic entries.

Operational record, 2026-09-17 (exclusive API Key account hotfix, Asia/Shanghai):

- API Key accounts can be selected and bound to exclusive channels without being rejected by subscription-expiry checks. Exclusive account availability now keeps the active, schedulable, independent-account requirements while treating API Key credentials separately from OAuth subscription expiry.
- The exclusive-channel account view no longer calls or displays OAuth 5-hour/7-day subscription quota windows for API Key accounts. The account's configured upstream Base URL remains separate from the customer-facing `https://togoapi.com/exclusive/v1` ingress.
- Release backup: `/opt/backups/portal-exclusive-apikey-hotfix-20260917-1436/`. Rollback tags are `portal-portal-api:rollback-portal-exclusive-apikey-hotfix-20260917-1436` and `portal-portal-web:rollback-portal-exclusive-apikey-hotfix-20260917-1436`.
- Active images: API `sha256:4fac904575ea425ff71df028a26bb85bd3fb2889bf5e03f2f162df8383861361`; Web `sha256:ab8a78e951ffdaeabba300c19bc130c11c92630618cce751863408521a19c1e9`. Matching retained release tags use `release-portal-exclusive-apikey-hotfix-20260917-1436`.
- Activated at 2026-09-17 14:39 Asia/Shanghai. Only Portal API/Web were recreated; both are healthy with zero restarts. Portal PostgreSQL and Core retained their previous container IDs and start times, and the protected production `.env` checksum and mode remained unchanged.
- Verification passed the focused Go regression test, 14 exclusive-channel frontend tests, frontend type checking, server API/Web production builds, Nginx validation, public Portal/Core health checks, exclusive-ingress invalid-key rejection, and public delivery of the updated exclusive-channel bundle. Post-deployment API/Web logs contain no error, fatal, or panic entries.

Operational record, 2026-09-17 (workspace consolidation and administrator account-package deduction, Asia/Shanghai):

- Rebuilt and published the complete current Portal workspace so changes from all local sessions remain in one production release. This includes the previously deployed account-package, exclusive-channel, payment, usage/statistics and account-test work, plus full-administrator account-package balance deduction with overdraft rejection, before/after balance audit data, acting administrator ID, required ledger direction, and manual-adjustment history display.
- Release backup: `/opt/backups/portal-all-workspace-20260917-124727/`. The pre-deployment PostgreSQL custom-format dump, restore listing, previous source archive and SHA-256 manifest were verified. Rollback tags are `portal-portal-api:rollback-all-workspace-20260917-124727` and `portal-portal-web:rollback-all-workspace-20260917-124727`.
- Active images: API `sha256:ea4673484c4a08af9eef9af0b862be74035322b96ea1429ad6392f75d7527fe7`; Web `sha256:2bfc8f8f040ad5d6c414c7e9cc434ac1939136909c735ef27f0272f0ac2694a6`. Matching retained release tags use `release-all-workspace-20260917-124727`.
- Verification passed all 833 frontend tests across 137 files, ESLint, the production Vue/TypeScript build, and the full Go suite against an isolated PostgreSQL container. The uploaded source archive SHA-256 was `65e67af88914e9c243fa403a8e8723e34b4689f43f1913bf792f5d1aa7fb94a5` and matched on the server.
- Activated at 2026-09-17 12:55 Asia/Shanghai. Only Portal API/Web were recreated; both are healthy with zero restarts. Portal PostgreSQL and Core retained their prior container IDs and start times. The protected production `.env` checksum and mode remained unchanged, and Imgtool's local COS timeout source hash still matches production, so Imgtool was not restarted.
- Public Portal, recharge, administrator SPA, Core, Imgtool and gift checks passed. Lottery retains its expected redirect response before following redirects. Public administrator deduction/history APIs reject unauthenticated requests with `401`; the active API binary contains the debit ledger marker, and the public `AccountUserLookupAdminView-Kwcb_NIE.js` contains the deduction and manual-adjustment UI. Nginx validation passed, and post-deployment API/Web logs contain no error/fatal/panic entries.

Operational record, 2026-09-17 (all-session Portal release, Asia/Shanghai):

- Published all current shared-workspace Portal changes, including account-level `降智测试`, channel account rebinding/selection, deletion of eligible channels, CCS import/copy feedback, and reuse of the package usage page for exclusive request records, charts, filters and CSV export. Exclusive usage is restricted to the authenticated holder's lease Key IDs and includes closed leases.
- Release: `portal-all-changes-20260917-020926`. Backup/build evidence: `/opt/backups/portal-all-changes-20260917-020926/`; staged source: `/opt/releases/portal-all-changes-20260917-020926/`. The Portal custom-format database dump, restore listing, previous source, server-only environment backup and release archive were verified. Release archive SHA-256: `ba27ccd2d02a27db04bab94b3b63cb22428490bd51be9c7dc4012d7042b7058e`.
- API image: `sha256:68a11a0b36c797cbe050194719bfd9e69aee0d8d3cb06cb8af5c5b4810825d15`; Web image: `sha256:121839df5960ebb04fa87d05c79e2a441f5bd21d4a3965f3d5ea2fd4a184ef75`. Rollback tags: `portal-portal-api:rollback-portal-all-changes-20260917-020926` and `portal-portal-web:rollback-portal-all-changes-20260917-020926`. Retag those images to their respective `:latest` tags and recreate only Portal API/Web with the production Compose pair for application rollback; migration 018 is additive and does not require restoring live database data for that rollback.
- Passed: 833 frontend tests across 137 files, ESLint, production Vue/TypeScript build, and the full Go suite with PostgreSQL-backed tests in isolated local schemas. The release also fixes missing customer-response sanitization for account usage and account-test JSON/SSE. Two stale route-test assumptions were corrected: Core expiry checks legitimately read account metadata, and model-list mocks must return an array.
- Activated at 2026-09-17 02:14 Asia/Shanghai (2026-09-16 18:14 UTC). Only Portal API/Web were recreated, both healthy with zero restarts. Core, Core PostgreSQL, Core Redis and Portal PostgreSQL retain their previous IDs/start times. Migration 018 created `portal_exclusive_account_tests`. Production environment values, network names and public bindings were checked unchanged. Imgtool's local COS timeout change already matches the deployed source and binary; Core has no uncommitted local changes.
- Public Portal, Core, Imgtool, gift and lottery checks passed. New usage/statistics/downgrade-test and admin routes reject unauthenticated access with 401. Public `index-CHAA-0YH.js` and `ExclusiveChannelsView-9S9v8MxT.js` match the release; account downgrade-test and embedded exclusive usage feature markers were verified. API/Web logs contain no error/fatal/panic entries. Browser visual and authenticated production business/inference acceptance were not performed.

Operational record, 2026-09-16 (exclusive channels):

- Deployed the Portal-owned exclusive-channel mode without changing Core source. Administrators publish a named channel backed by a Core group and selected OpenAI accounts; users claim one channel for a selected duration, receive an isolated channel credential, and release it manually or through expiry/30-minute no-consumption recovery. Release waits for in-flight requests to finish before the channel becomes available again.
- The customer page exposes real account usage windows, refresh controls, lease/release countdowns, platform labels, channel multiplier, per-model text pricing overrides, image pricing (`CNY 0.05/image` and image multiplier `1` by default), the recommended marker, and the per-channel generated-page diagnostic. Migrations `013` through `017` created the exclusive-channel, lease, request, diagnostic, multiplier, and pricing schema. The production channel table was empty immediately after deployment.
- Exclusive ingress is enabled and isolated at `https://togoapi.com/exclusive/v1`; its server-generated secret remains only in `/opt/portal/.env`. Unauthenticated customer/admin APIs return `401`, and an invalid exclusive-channel credential returns `401` without reaching Core.
- Verified backup and rollback files are under `/opt/backups/portal-exclusive-channels-20260916-192348/`. Rollback images are `portal-portal-api:rollback-exclusive-channels-20260916-192348` and `portal-portal-web:rollback-exclusive-channels-20260916-192348`.
- Active images are API `sha256:601a6650ad64c1d415d44f4d78aff3a71b4bce563aafddbb3afc92557555b75e` and Web `sha256:6c7d66fdbd3f6ac1b7adc793f96b4bf35983fdb8752fb9a112fd23bede936d73`. Only Portal API and Web were recreated. Portal PostgreSQL retained container ID `16ebe43fca47` and start time `2026-08-23T09:03:06Z`; Core, Core PostgreSQL, and Core Redis were not restarted.
- Local and public Portal health returned HTTP `200`; `/exclusive-channels` and `/admin/exclusive-channels` serve the production SPA; Nginx configuration validation passed; Portal containers are healthy with zero restarts; and post-deployment API/Web logs contain no error, fatal, or panic entries.

Operational record, 2026-09-14 (administrator account-package balance recharge):

- Added `充值套餐余额` to the administrator user-package lookup page. Full administrators can credit only the selected user's Portal account-package wallet, with a required positive amount, a per-operation maximum of CNY 1,000,000, two-decimal validation, an optional 200-character note, and a clear post-credit balance preview. The operation does not modify the user's Core relay balance.
- Credits run in a database transaction and write an `adjustment` ledger row with `reference_type=admin_account_recharge`, the acting administrator ID, before/after balances, amount, and audit note. The administrator finance page includes a separate manual-adjustment tab searchable by user ID, email, username, or note.
- Full administrators can also deduct from a selected user's account-package wallet. Deductions reject overdrafts, run under the same row lock and transaction as credits, and write a negative `adjustment` ledger row with `reference_type=admin_account_debit`, the acting administrator ID, before/after balances, and audit note. The finance tab shows both directions as manual adjustments.
- Release and source backups are under `/opt/backups/portal-account-admin-manual-recharge-20260914-194050/`. Rollback images are `portal-portal-api:rollback-portal-account-admin-manual-recharge-20260914-194050` and `portal-portal-web:rollback-portal-account-admin-manual-recharge-20260914-194050`. Active images are API `sha256:3e2ae3db204b098fc836c1c7e57ac039ce7338b72a4ac8be714f9d07e716695a` and Web `sha256:c0fc9950d5b68247a35fb07cf18b49c1fc659102303e4a232a01161727fef9bd`.
- Only Portal API and Web were recreated. No migration was required; PostgreSQL retained image `sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2` and start time `2026-08-23T09:03:06Z`. All Portal containers are healthy with zero restarts.
- Verification passed the PostgreSQL-backed administrator credit/audit tests, invalid-precision and unauthenticated rejection tests, ESLint, type checking, all 765 frontend tests, and local/server production builds. Public Portal/Core health returned HTTP `200`, unauthenticated credit and adjustment-history routes returned `401`, and the authenticated administrator UI showed the balance preview and loaded the new history tab. No production user balance was changed during deployment verification.

Operational record, 2026-09-14 (account administrator insights):

- Added two full-administrator entries under `账号管理`: `充值 / 兑换记录` and `用户套餐查询`. Administrators can view online account-wallet recharge records and redeem-code usage, filter by status, and search by user ID, email, username, order number, or redeem-code suffix.
- The user lookup page combines Core relay balance, Portal account-package balance, current-month usage, active/expired package state, validity dates, assigned account IDs, auto-renew status, cumulative spending, and recent recharge/redemption history.
- Release and source backup files are under `/opt/backups/portal-account-admin-insights-20260914-175658/`; rollback images are `portal-portal-api:rollback-portal-account-admin-insights-20260914-175658` and `portal-portal-web:rollback-portal-account-admin-insights-20260914-175658`. Active images are API `sha256:54815ab1477edda2f4a46618b4169e251deb429649299e8456d2094370587b81` and Web `sha256:0358a13c18496f1e13ee1de99faf1bdb7ec5b4674f67a0a4788949dc43a2df44`.
- Only Portal API and Web were recreated. PostgreSQL remained on `sha256:d3e1620b530c944afa6e887d22eb899824da68e19c52024bf98f5220c88a65b2` with its original `2026-08-23T09:03:06Z` start time. All three Portal containers are healthy with zero restarts.
- Verification passed 20 focused frontend tests, the full 765-test frontend suite, ESLint, type checking, local/server production builds, and focused PostgreSQL-backed Go tests for the new administrator endpoints. Public Portal/Core health returned HTTP `200`; unauthenticated new routes returned `401`; authenticated browser checks loaded real payment and redemption data, user search, both balances, usage, active package details, and recent history.

Operational record, 2026-09-14 (account-package direct recharge):

- Deployed Portal API/Web for direct Alipay recharge into the separate account-package wallet. The existing EasyPay merchant instance 1 is reused without changing Core payment settings or crediting Core balances.
- Configured the public Portal origin, pinned provider and stable account-payment encryption secret in the protected server-side environment. No secret contents are stored in this repository. Migration 012 adds the dedicated payment-order table and idempotency constraints.
- Backup and release files: `/opt/backups/portal-account-payments-20260914-143224/`. The Portal custom-format dump passed `pg_restore --list`; source/environment backups and checksums are retained. Rollback tags: `portal-portal-api:rollback-account-payments-20260914-143224` and `portal-portal-web:rollback-account-payments-20260914-143224`.
- Active image IDs: API `sha256:1bbe7ca5a4cde9a29484c7dbc1c781fb8bf3f36aba687e16333582e16c7b4837`, Web `sha256:16c89d8af6bdad8b0cb3537a25e87496709e6441f79aead1f16c1c4193b93a32`. Both services are healthy with zero restarts. Core and both PostgreSQL containers were not recreated or restarted.
- Verified authenticated checkout/history through the user's current Portal session, desktop/mobile forms, public health/recharge 200 responses, checkout authentication (401), callback routing (400 for malformed input), and the gateway's read-only order query. The public Web entry is `/assets/index-D1HbTKkr.js`.
- Thirteen focused payment test groups, the full Go suite, 755 frontend tests and production builds passed. Real payment creation, deduction and live settlement are still awaiting explicit user confirmation; do not describe the live-payment acceptance as completed. Once real orders exist, retain the active encryption secret and a settlement-capable API during rollback. See `portal/docs/account-payments.md`.

Operational record, 2026-09-14 (card-shop retirement):

- After direct online account-package recharge became available, removed the user-facing `购买卡密` entry and its `NEW` marker from both regular-user and administrator personal navigation. The account marketplace now links to `/recharge` for direct balance recharge; existing card redemption and recharge-history views remain available.
- Legacy card-purchase URLs `/card-shop`, `/account-business/balance/purchase`, `/account-business/recharge`, and `/account-recharge` now redirect to `/recharge`, preventing bookmarked links from reopening the retired shop while preserving route compatibility.
- This was a Web-only deployment. Release and source backup files are under `/opt/backups/portal-card-shop-retire-20260914-155434/`; rollback image: `portal-portal-web:rollback-card-shop-retire-20260914-155434`. Active Web image: `sha256:9de627ed28965065e76545f84692e2a2598cd1cfdd2bd3baf8ac3d3ec6c70bb9`; API remained on `sha256:1bbe7ca5a4cde9a29484c7dbc1c781fb8bf3f36aba687e16333582e16c7b4837` and API/PostgreSQL containers were not recreated.
- Verification passed 37 focused frontend tests, the full 755-test frontend suite, ESLint on changed files, production build, and `git diff --check` apart from expected CRLF warnings. The active production bundle `/assets/index-CzBxCc3u.js` contains the `/card-shop` redirect and no `购买卡密` text; public `/card-shop` serves the current SPA bundle. Portal Web/API/PostgreSQL are healthy with zero restarts.

Operational record, 2026-09-14 (recharge `NEW` markers):

- Added the prominent `NEW` marker to the top-level `余额充值` navigation entry for regular users and administrators, and to the `账号套餐余额充值` heading inside the recharge page. Desktop and development H5 layouts were visually checked with no marker overlap or truncation.
- This was a Web-only deployment. Release and source backup files are under `/opt/backups/portal-recharge-new-badges-20260914-161404/`; rollback image: `portal-portal-web:rollback-recharge-new-badges-20260914-161404`. Active Web image: `sha256:63502f6730cecddb9fc3f138d3254ca92267ecc36768eac200c43ca656541a74`; API remained on `sha256:1bbe7ca5a4cde9a29484c7dbc1c781fb8bf3f36aba687e16333582e16c7b4837`, and API/PostgreSQL were not recreated or restarted.
- Verification passed 32 focused tests, the full 756-test frontend suite, ESLint on changed files, `git diff --check` apart from expected CRLF warnings, local and server production builds, and public health checks. The active public assets are `/assets/index-DJSxQ70t.js`, `/assets/AppLayout.vue_vue_type_script_setup_true_lang-BXYOBkko.js`, and `/assets/RechargeView-C4FCzMnQ.js`; the latter two contain the sidebar and account-recharge markers respectively. Portal Web/API/PostgreSQL are healthy with zero restarts.

Operational record, 2026-09-14 (dual wallet balance display):

- Updated the desktop header, H5 user menu, and user dashboard balance card to show both `中转余额` and `套餐余额` at the same time. Separate link/grid icons and emerald/sky colors distinguish the two wallets; an unavailable account-package balance displays `--` instead of falling back to the relay balance.
- This was a Web-only deployment. Release, source backup, checksum, and container baseline files are under `/opt/backups/portal-dual-wallet-balances-20260914-164435/`; rollback image: `portal-portal-web:rollback-dual-wallet-balances-20260914-164435`. Active Web image: `sha256:473bdb7e3ba49218e393b753971c2151c0208dde6df6d52ad2ce7ad5efea38ba`; API remained on `sha256:1bbe7ca5a4cde9a29484c7dbc1c781fb8bf3f36aba687e16333582e16c7b4837`, and API/PostgreSQL were not recreated or restarted.
- Verification passed 22 focused tests, the full 762-test frontend suite, ESLint on changed files, the local and server production builds, and `git diff --check` apart from expected CRLF warnings. Public Portal health, recharge, and Core health returned HTTP `200`; Docker-internal Portal API health returned `ok`. The active public entry is `/assets/index-C6SrsARs.js`, while `/assets/AppLayout.vue_vue_type_script_setup_true_lang-Db6PiqXz.js` and `/assets/DashboardView-DcdmV2Wx.js` contain the two wallet labels. Portal Web/API/PostgreSQL are healthy with zero restarts.

Operational record, 2026-09-13:

- Portal was rebuilt and deployed with the account-package marketplace, rental and account product modes, configurable rental validity, 30-day-only auto-renewal, uploaded product images, account-package card redemption, and the top-level card shop at `/card-shop`.
- Only `portal-api` and `portal-web` were recreated. PostgreSQL retained its existing container and volume; Portal migrations `007` through `011` completed during API startup.
- Rollback images: `portal-portal-api:rollback-20260913-221213` and `portal-portal-web:rollback-20260913-221213`. The verified database dump, deployment archive, release archive, and checksums are stored under `/opt/backups/portal-account-business-20260913-221213/`.
- Pre-deployment verification passed 736 frontend tests, frontend lint and production build, server-side Go tests, and isolated API/Web image builds. Post-deployment checks confirmed healthy containers with zero restarts, Core network connectivity, public Portal/Core/Imgtool HTTP `200` responses, the account-product API, legacy card-purchase route compatibility, and the production card-shop UI.
- A same-day Web-only hotfix changed uploaded account-product advertising images from cropped fixed-height banners to complete square previews in both customer and administrator views. The rollback image is `portal-portal-web:rollback-ad-image-20260913-225645`; the source backup and release checksum are under `/opt/backups/portal-ad-image-hotfix-20260913-225645/`.
- A later Web-only hotfix added the existing prominent `NEW` marker to the top-level `购买卡密` menu entry for both regular users and administrators. Only `portal-web` was recreated; the rollback image is `portal-portal-web:rollback-card-shop-new-20260913-233602`, and the source backup plus checksums are under `/opt/backups/portal-card-shop-new-20260913-233602/`. The replacement image is `sha256:24eef8c2d198878cf09acaeabe3e48f909f2ca5ff09e52b3e9b179e903e40ff1`; post-deployment checks confirmed a healthy Web container, local and public health responses, and the production sidebar marker.

Operational record, 2026-09-10:

- Portal was rebuilt from `output/portal-ppt-retire-20260910.tar.gz` to remove the retired AIPPT SSO and AI-config routes.
- Only `portal-api` and `portal-web` were recreated. PostgreSQL kept its existing container.
- Rollback images: `portal-portal-api:rollback-20260910-155836` and `portal-portal-web:rollback-20260910-155836`. File backup: `/opt/backups/portal-ppt-retire-20260910-155836/`.
- Verified locally and through Cloudflare: Portal health `200`, public settings `200`, Imgtool SSO still present (`401` unauthenticated), PPT SSO `404`. Core health remained `200`.

Portal uses the production override to join Core's internal Docker network. Do not use `host.docker.internal:8080` in production because Core intentionally binds its published port to host loopback. Portal's Admin API Key and Imgtool SSO secret are stored only in `/opt/portal/.env` and are deliberately omitted here.

## Nginx and origin HTTPS

Current behavior:

- `api.togoapi.com:443` redirects only the exact root path `/` to `https://togoapi.com/`; `/admin`, `/v1/*`, `/api/*`, and all other Core paths proxy to `127.0.0.1:8080`.
- `togoapi.com:443` proxies to `127.0.0.1:3000`.
- `img.togoapi.com:443` serves `/opt/imgtool/web/dist` and proxies `/api/*` and `/sso` to `127.0.0.1:18082`.
- `togoapi.com/gift/` proxies to `127.0.0.1:18083`; `/lottery/` proxies to `127.0.0.1:18084`.
- Unknown-Host HTTPS requests return `444`; plain HTTP sent to the HTTPS listener returns `400`.
- TLS permits only TLS 1.2 and TLS 1.3.
- Internal Core proxy validation returns HTTP `200`.

Origin certificate:

| Item | Value |
| --- | --- |
| Issuer | Let's Encrypt |
| Names | `api.togoapi.com`, `togoapi.com` |
| Full chain | `/etc/nginx/ssl/togoapi/fullchain.pem` |
| Private key | `/etc/nginx/ssl/togoapi/privkey.pem` |
| Valid until | 2026-10-22 |
| Renewal | Manual DNS-01; automatic renewal is not configured |

Renew or automate the certificate before expiry. The temporary DNSPod `_acme-challenge` TXT records used for issuance can be removed.

Imgtool uses a separate Let's Encrypt certificate:

| Item | Value |
| --- | --- |
| Name | `img.togoapi.com` |
| Full chain | `/etc/letsencrypt/live/img.togoapi.com/fullchain.pem` |
| Private key | `/etc/letsencrypt/live/img.togoapi.com/privkey.pem` |
| Valid until | 2026-10-23 |
| Renewal | Manual DNS-01; `certbot.timer` is disabled |

The manual renewal scripts are under `/usr/local/sbin/imgtool-certbot-*`; the corresponding source scripts are under `imgtool/deploy/`. Renew before expiry, update the temporary DNS TXT challenge, verify the certificate, and reload Nginx.

## Imgtool deployment

| Item | Value |
| --- | --- |
| Deployment directory | `/opt/imgtool` |
| Secret environment file | `/opt/imgtool/.env` (mode `600`) |
| Binary | `/opt/imgtool/dist/imgtool-linux-amd64` |
| Frontend | `/opt/imgtool/web/dist` |
| Data | `/opt/imgtool/data/imgtool.sqlite` |
| Host binding | `127.0.0.1:18082` |
| systemd unit | `imgtool.service` |
| Nginx site | `/etc/nginx/sites-available/imgtool` |

Operational record, 2026-09-14 (COS upload timeout):

- Increased the shared Tencent COS HTTP client and per-upload context timeout from 2 minutes to 10 minutes after a generated 2.13 MB image stalled during multipart upload and hit the former 120-second deadline. The image provider had completed successfully; the failure occurred only while persisting the result to the Guangzhou COS bucket.
- Backend tests passed under Go 1.24. The deployed Linux amd64 binary SHA-256 is `0624d0605eedaa425d457e2d63831bc18f23dc266da6114d7fd88a372a351e78`.
- The previous binary and source files are backed up under `/opt/imgtool/.deploy-backups/20260914-201144-timeout10/`. The service restarted at `2026-09-14 20:11:44 CST`, remained active with zero restarts, and both local and public Imgtool health checks returned HTTP `200`.

## Gift campaign deployment

The lightweight lucky-code campaign is deployed on the apex site under a path prefix:

| Item | Value |
| --- | --- |
| Public page | `https://togoapi.com/gift` |
| Admin page | `https://togoapi.com/gift/admin` |
| Deployment directory | `/opt/lucky-code-campaign` |
| Binary | `/opt/lucky-code-campaign/lucky-code-campaign-linux-amd64` |
| Secret environment file | `/opt/lucky-code-campaign/.env` (mode `600`) |
| SQLite database | `/opt/lucky-code-campaign/data/activity.db` |
| Host binding | `127.0.0.1:18083` |
| systemd unit | `lucky-code-campaign.service` |
| Nginx locations | `/etc/nginx/snippets/gift.locations.conf` |
| Nginx rate limits | `/etc/nginx/conf.d/gift-rate-limit.conf` |

The service was deployed and publicly verified on 2026-07-26. The production prize pool was left empty so no test codes or claims exist. The administrator password is stored only in the production environment file and is deliberately omitted here.

Routine checks:

```bash
sudo systemctl status lucky-code-campaign.service --no-pager
sudo journalctl -u lucky-code-campaign.service -n 100 --no-pager
curl -fsS http://127.0.0.1:18083/gift/api/status
sudo nginx -t
```

## Retired sidecars

On 2026-09-10 these Tokyo sidecars were stopped and their Nginx sites were disabled. Do not start them again.

| Service | Former location | Notes |
| --- | --- | --- |
| Presenton / PPT Workbench | `/opt/presenton`, `/opt/ppt-workbench` | Nginx site `presenton` disabled; `ppt.togoapi.com` is retired |
| QQ status bot / NapCat | `/opt/qq-status-bot` | Containers stopped; leftover data may still be on disk |
| kiro-rs | `/opt/kiro-rs` | Container stopped; Nginx site `kiro` disabled; host port `8990` closed |

Enabled Nginx sites are only `sub2api` and `imgtool`. Core, Portal, Imgtool, Gift, and Lottery remain in service.

## Lottery deployment

| Service | Deployment / Compose | Host binding | Public route |
| --- | --- | --- | --- |
| Lottery campaign | `/opt/lottery-campaign/deploy/docker-compose.production.yml` | `127.0.0.1:18084` | `https://togoapi.com/lottery/` |

## DNS and Cloudflare

The `togoapi.com` zone is active on Cloudflare. These three records are proxied and point to the Tokyo origin:

| Name | Type | Content | Proxy | TTL |
| --- | --- | --- | --- | --- |
| `togoapi.com` | A | `43.165.190.2` | Proxied | Auto |
| `api.togoapi.com` | A | `43.165.190.2` | Proxied | Auto |
| `img.togoapi.com` | A | `43.165.190.2` | Proxied | Auto |

Do not change unrelated mail, SES, COS download, DKIM, SPF, DMARC, or ACME records during an origin migration. Cloudflare must remain in proxied mode. The former EdgeOne CNAME/origin-group configuration is historical and is not the active production path.

The 2026-08-23 cutover was verified with unique query markers: all markers appeared only in the Tokyo Nginx access log, all public health/pages returned HTTP `200`, and live production model requests completed with HTTP `200`.

Cloudflare hides the origin from normal DNS lookups, but it does not make the origin IP secret. Historical DNS and TLS scans can still discover it; enforce a Cloudflare-only origin firewall after the rollback window.

## Payment business rules

TogoAPI uses a promotional 1:1 recharge model rather than a foreign-exchange conversion model:

- `1 CNY` paid grants `1 USD` of account balance.
- Set **Balance Recharge Multiplier** to `1`.
- Set **Subscription USD to CNY Rate** to `0` so a plan whose price is `10` is charged as `10 CNY`; do not set this field to a market exchange rate.
- Set the recharge fee rate independently according to whether payment-provider fees are absorbed by TogoAPI or charged to the user.

This is an intentional product pricing rule. Do not replace the multiplier with `1 / USD-CNY exchange rate` during deployment or maintenance.

## Registration email

Sub2API has native SMTP support for registration verification codes and notification emails. The public Portal password-reset flow is separate and is documented below.

Current provider configuration:

| Item | Value |
| --- | --- |
| Provider | Alibaba Cloud DirectMail |
| Console region | China (Hangzhou) |
| Sending domain | `mail.togoapi.com` |
| Domain status | Verified |
| Sending address | `no-reply@mail.togoapi.com` |
| Address type | Triggered/transactional email |
| SMTP host | `smtpdm.aliyun.com` |
| SMTP port | `465` |
| TLS | Enabled; required with port 465 |
| SMTP username | `no-reply@mail.togoapi.com` |
| From address | `no-reply@mail.togoapi.com` |
| From name | `TogoAPI` |

The SMTP password is deliberately omitted. It was set in Alibaba Cloud and must be retrieved/reset there if lost; never add it to this repository or this document.

Authoritative DNS records verified publicly on 2026-07-25:

| Host | Type | Value |
| --- | --- | --- |
| `mail` | MX, priority 10 | `mx01.dm.aliyun.com` |
| `mail` | TXT | `v=spf1 include:spf1.dm.aliyun.com -all` |
| `aliyun-cn-hangzhou._domainkey.mail` | TXT | Alibaba-generated DKIM public key; full value is in DNSPod and the DirectMail console |
| `_dmarc.mail` | TXT | `v=DMARC1; p=none` |

Alibaba Cloud reports the domain verification as passed. A TLS SMTP connection and test-email delivery were successfully verified from the local Core on 2026-07-25.

Deployment state:

- Local Core: SMTP configured and test email received.
- Production Core: configured and in use for registration and native notifications. SMTP settings and password are stored in the production settings database.
- Public registration should remain closed until production email is verified and anti-abuse controls such as Turnstile are enabled.
- After production setup, test registration, verification-code expiry/resend behavior, login, and password reset end to end.

### Portal low-balance email monitor

Portal owns a low-balance fallback monitor so users do not have to add their
registration email as a separate Core notification recipient. It is enabled in
production with `PORTAL_BALANCE_ALERT_ENABLED=true` and polls Core's official
public-settings and paginated admin-user APIs every 60 seconds. It does not read
the Core database.

The monitor honors Core's global switch, default threshold, recharge URL,
per-user switch, fixed/percentage threshold, and account status. When a user
already has any verified, enabled Core notification email, Portal skips that
user and leaves delivery to Core to prevent duplicate messages. Otherwise,
Portal sends a Chinese low-balance message to the primary account email.

Crossing and delivery state is stored in the Portal PostgreSQL table
`portal_balance_alert_states`. The first observation establishes a baseline;
mail is sent only after a later downward crossing. Failed SMTP deliveries stay
pending for retry. Relevant Portal logs contain `Portal balance alert`.

## Portal password reset

The public password-reset flow is owned by Portal and does not require Core code changes:

1. `https://togoapi.com/forgot-password` accepts the account email and a Cloudflare Turnstile response.
2. Portal sends a six-digit code through Alibaba Cloud DirectMail.
3. The user submits the email, code, new password, and confirmation on the same Portal page.
4. Portal validates the code and calls Core's existing scoped Admin API to update the user's password.

Operational controls:

- Codes expire after 10 minutes and may be resent after 60 seconds.
- Each active reset session permits at most 5 sends and 5 verification attempts.
- Portal PostgreSQL stores only an HMAC digest in `portal_password_reset_codes`; plaintext codes are not persisted.
- Unknown email addresses receive the same public response and do not trigger email delivery.
- Runtime secrets are stored only in `/opt/portal/.env`: `PORTAL_PASSWORD_RESET_SECRET`, `PORTAL_TURNSTILE_SECRET`, and the `PORTAL_SMTP_*` variables.
- Legacy `/reset-password` links direct users to the Portal verification-code flow.

The existing Core `frontend_url=https://api.togoapi.com` setting belongs to Core's native link-based reset path. The Portal flow does not use it. Do not change Core code, restart Core, or alter that setting when maintaining the Portal verification-code flow.

Tencent Cloud SES was evaluated first, but the current personal-verified Tencent account is not authorized to create SMTP credentials. Its API/console sending capability and previously approved template do not make it compatible with Sub2API's native SMTP path. DNS has been migrated to Alibaba Cloud. The obsolete `qcloud._domainkey.mail` TXT record may still exist and can be deleted after confirming no system still sends through Tencent SES.

## Firewall

### Cloudflare ingress hardening (2026-08-04)

The four public web hostnames are proxied through Cloudflare. Cloudflare uses `Full (strict)`, HTTP/2 to the origin, and bypasses cache for `api.togoapi.com`, `/api/`, and `/sso`. HTTP/3 and 0-RTT are disabled while mainland route stability is evaluated.

Nginx loads the current official Cloudflare networks from `/etc/nginx/conf.d/cloudflare-real-ip.conf` and trusts `CF-Connecting-IP` only when the TCP peer belongs to those networks. The public proxy locations overwrite `X-Forwarded-For` and `CF-Connecting-IP` with the parsed `$remote_addr`, so direct clients cannot spoof the application client IP.

`/etc/nginx/conf.d/ingress-observe.conf` defines conservative per-client thresholds of 60 requests/second with a 200-request burst and 100 concurrent requests. Public locations include `/etc/nginx/snippets/togoapi-ingress-observe-location.conf`, which puts both request and connection limiting in `dry-run` mode: hits are logged at notice level but no request is rejected. Keeping dry-run at location scope preserves the existing enforced limits on sensitive gift endpoints. Review at least 24 hours of logs before enabling enforcement.

The corresponding tracked deployment files are:

- `portal/deploy/nginx-cloudflare-real-ip.conf`
- `portal/deploy/nginx-ingress-observe.conf`
- `portal/deploy/nginx-ingress-observe-location.conf`
- `portal/deploy/sshd-togoapi-hardening.conf`

Tencent Cloud firewall rules currently expected:

| Protocol / port | Source | Purpose |
| --- | --- | --- |
| TCP `22` | All IPv4 | SSH administration; restrict to current administrator IP after the migration observation window |
| TCP `443` | All IPv4 | HTTPS origin for Cloudflare; added during the 2026-08-23 cutover |
| TCP `80` | All IPv4 | HTTP redirect and diagnostics |
| ICMP | All IPv4 | Ping diagnostics |

Tokyo currently relies on Tencent Cloud Lighthouse firewall rules plus Nginx unknown-host `444` handling. After the rollback window, replace the all-IPv4 web rules with the current official Cloudflare IPv4/IPv6 source ranges and restrict SSH to trusted administrator addresses. Keep an emergency access procedure before applying the restriction.

Local maintenance artifacts:

```text
C:\Users\Administrator\Downloads\togoapi-edgeone-firewall.zip
C:\Users\Administrator\Downloads\edgeone-https-443-firewall-2026-07-25.csv
C:\Users\Administrator\Downloads\togoapi-cloud-firewall-final.csv
```

The old EdgeOne firewall artifacts are historical and must not be applied to Tokyo. Cloudflare publishes different address lists; obtain the current official Cloudflare ranges before creating origin restrictions.

## Resolved incident

The former origin IP `43.135.137.58` was blocked by Tencent Cloud after a UDP flood peaked at approximately `36.29 Gbps`. The provider console reported automatic unblock at:

```text
2026-07-26 03:06:12 Asia/Shanghai
```

While blocked, public requests reached EdgeOne but returned:

```text
HTTP 522 Connect origin timed out
Server: TencentEdgeOne
```

The public IP was first replaced with the US origin `170.106.190.46`. On 2026-08-01, production was migrated to the Singapore origin `43.160.240.196` because mainland WiFi users still reported unstable access through the US origin. On 2026-08-23, production was migrated again to Tokyo `43.165.190.2` and the four active Cloudflare A records were updated.

Do not restore `43.135.137.58` or `170.106.190.46` in active DNS. During the current rollback window, only Singapore `43.160.240.196` may be used as an emergency rollback target, and Tokyo writers must be stopped before restarting Singapore writers.

## Current verification

Use this sequence for routine deployment verification:

1. Check the Core locally on the server:

   ```bash
   curl -fsS http://127.0.0.1:8080/health
   curl -kI --resolve api.togoapi.com:443:127.0.0.1 https://api.togoapi.com/
   curl -kI --resolve api.togoapi.com:443:43.165.190.2 https://api.togoapi.com/health
   ```

2. From a client, verify Cloudflare and HTTPS:

   ```powershell
   curl.exe -I http://api.togoapi.com/
   curl.exe -I https://api.togoapi.com/
   curl.exe -I https://api.togoapi.com/admin
   curl.exe -I https://api.togoapi.com/v1/models
   curl.exe -I http://togoapi.com/
   curl.exe -I https://togoapi.com/
   curl.exe -I https://img.togoapi.com/api/health
   curl.exe -I https://togoapi.com/gift/
   curl.exe -L -I https://togoapi.com/lottery/
   ```

   Expected: HTTP redirects to HTTPS with `301`; `https://api.togoapi.com/` returns `308` with `Location: https://togoapi.com/`; `/admin` still serves the Core administrator UI; API paths continue returning their normal authenticated or unauthenticated responses.

3. Test `/v1/*`, streaming responses, WebSocket behavior if used, and PUT/PATCH/DELETE/OPTIONS requests.
4. Re-test SMTP connection, registration verification, and password reset after changing any production email or frontend URL setting.
5. After the rollback window, restrict TCP `22`, TCP `80`, direct TCP `443`, and ICMP as appropriate; use Cloudflare's official source ranges for the web origin.
6. If the public IP changes, update the three proxied Cloudflare A records (`@`, `api`, and `img`), SSH details, firewall records, and this document.

## Routine operations

After connecting to the primary Tokyo origin over SSH:

```bash
ssh togoapi-tokyo
cd /opt/sub2api

# Service status and Core logs
docker compose --env-file .env -f docker-compose.local.yml ps
docker compose --env-file .env -f docker-compose.local.yml logs --tail 200 sub2api

# Restart Core only
docker compose --env-file .env -f docker-compose.local.yml restart sub2api

# Pull and apply current images
backup_dir="/opt/backups/sub2api-core-upgrade-$(date +%Y%m%d-%H%M%S)"
sudo mkdir -p "$backup_dir"
sudo chown ubuntu:ubuntu "$backup_dir"
set -a
. ./.env
set +a
: "${POSTGRES_USER:=sub2api}"
: "${POSTGRES_DB:=sub2api}"
docker compose --env-file .env -f docker-compose.local.yml exec -T postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > "$backup_dir/postgres.sql.gz"
tar --exclude='./postgres_data' --exclude='./redis_data' --exclude='./.env' -czf "$backup_dir/sub2api-deploy-files.tgz" .
chmod 600 "$backup_dir"/*
docker compose --env-file .env -f docker-compose.local.yml pull sub2api
docker compose --env-file .env -f docker-compose.local.yml up -d sub2api

# Local checks
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:3000/health
sudo nginx -t
sudo systemctl reload nginx

# Firewall and public listeners
sudo ufw status verbose
sudo ss -lntp | grep -E ':(22|80|443) '
```

Portal operations use both Compose files:

```bash
cd /opt/portal
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env ps
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env logs --tail 200 portal-api portal-web
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file .env up -d --build
```

Never run `docker compose down -v`. Never delete Core data directories or the `portal_portal_postgres_data` Docker volume; these contain persistent application data.

The Singapore application and bot containers must remain stopped during the rollback window. Its `sub2api-postgres` and `portal-portal-postgres-1` containers may remain healthy for emergency rollback. Never allow writers on both origins at the same time.

## Secrets

- SSH private key: local path listed above; never commit or upload it.
- Core runtime secrets and database password: `/opt/sub2api/.env`, which should remain mode `600`.
- Core administrator password: deliberately omitted.
- Core Admin API Keys: deliberately omitted; store them only in ignored secret files.
- Alibaba Cloud DirectMail SMTP password: deliberately omitted; keep it in a password manager and enter it only in the relevant Core settings database.
- Portal secrets must remain separate from Core secrets.
- Portal runtime secrets: `/opt/portal/.env`, mode `600`; never print, commit, or copy them into documentation.

## Portal pelican gallery update — 2026-10-09 14:53 Asia/Shanghai

- Portal now shows only the latest six successful HTML/SVG artworks. Pass-rate / error counters and the failure timeline are removed from the page and gallery response. Internal Portal failure records remain for diagnostics.
- The server reads eligible accounts in the configured Core group through official admin APIs, then probes selected upstreams directly using Portal's account adapter. Each run rotates its starting account and tries up to three distinct accounts; timeout, HTTP/SSE errors and invalid output advance to the next account. A unique session ID is used per attempt. Each account receives six minutes, the run receives nineteen minutes, stale recovery is twenty minutes, shutdown drain is twenty minutes, and container stop grace is twenty-one minutes. Terminal SSE events close the probe immediately. Direct probes incur upstream usage but do not create Core inference usage records or Core billing; successful account identity is recorded in Portal.
- Only Portal API/Web were recreated. Core container `f8c4053c2d14` (`weishaw/sub2api:0.2.15`, started 2026-10-09 14:38:31 Asia/Shanghai) was already running at deployment preparation and was unchanged by this update. No Core code, account configuration, or database changes were made.
- Images: `portal-portal-api:pelican-gallery-20261009`, `portal-portal-web:pelican-gallery-20261009`. Backup: `/opt/backups/pelican-gallery-20261009/` with verified Portal dump, restore list, source archive, SHA-256 manifest and matching Core before/after identity. Rollback tags: `portal-portal-api:rollback-pelican-gallery-20261009`, `portal-portal-web:rollback-pelican-gallery-20261009`.
- Backend full suite passed on Linux with an isolated PostgreSQL; focused pelican integration tests passed, frontend gallery tests passed (6), and TypeScript / production build passed. Public gallery assets contain the gallery and no pass-rate/timeline. Portal/Core health endpoints returned 200; Portal containers are healthy with zero restarts. The temporary validation database was removed.
- Details: `portal/docs/pelican-gallery.md`. Deployment script: `artifacts/deploy-pelican-gallery-20261009.sh`. Rollback must restore the two image tags and Portal source/Compose snapshot, then recreate only `portal-api portal-web` with `--no-deps --no-build`; preserve the Portal database and existing artwork history.

线上真实验证：2026-10-09 14:56:34 触发新版任务 #559，于 15:01:31 成功完成；首次尝试账号 #752（chenlijia-0.133），耗时约 297 秒，保存 16387 字符 HTML/SVG，账号归属准确，未修改 Core。

## Portal order support removal — 2026-10-09

- Deployed removal of the recharge-history invoice/refund request buttons, support dialog, clipboard handler, and dedicated Chinese/English strings.
- Only Portal Web was recreated; all other running container IDs were verified unchanged. Portal Web/API/PostgreSQL and Core were healthy with zero restarts; public Portal/Core health checks passed.
- Public orders entry and referenced OrdersView-J_t8_iTo.js verified: refresh/recharge retained; invoice/refund support entries and support contact absent.
- Image: portal-portal-web:remove-order-support-20261009, SHA-256 04780afb856049a43f602a57741ebeba4e85bac4c7c43c9332ea9c6e270d9b7f.
- Backup: /opt/backups/portal-remove-order-support-20261009/; rollback image: portal-portal-web:rollback-remove-order-support-20261009. Four source files were transferred after matching production against their pre-change baseline; archive checksum verified.
- Local order test and local/server type checks and production builds passed. Rollback: restore the frontend source backup, tag the rollback image as portal-portal-web:latest, then run the existing production Compose pair with up -d --no-deps --no-build portal-web.
## Portal documentation removal — 2026-10-09

- Removed header/sidebar documentation links, the /docs route, DocsView and dedicated docs-assets from production.
- Only portal-portal-web-1 was recreated; all other container IDs and start times matched the pre-deployment baseline.
- Image: portal-portal-web:remove-docs-20261009 (manifest sha256:3a63ab62f8f28e2ccebafaf433163e8df7f9aedc3523dbb6d94bef751b3c1f31). Backup: /opt/backups/portal-remove-docs-20261009/; rollback image: portal-portal-web:rollback-remove-docs-20261009.
- Local 85 focused tests and production build passed; server production build passed. Verified public index-BbWPk3rl.js and AppLayout-CFMSNTyv.js have no documentation route/navigation, and the active image has no DocsView chunks or docs-assets directory. Portal/Core health checks returned 200; containers healthy.
- Rollback: restore affected frontend files and deleted documentation assets from source.tar.gz, tag the rollback Web image as portal-portal-web:latest, and recreate only portal-web using the production Compose pair with --no-deps --no-build.

## Portal drawing check configuration — 2026-10-15

- Deployed administrator-editable model ID and drawing prompt to Tokyo Portal API/Web. Migration 025 adds defaults for existing configuration; scheduled and manual probes use saved settings and account filtering respects the selected model.
- Built from the production source snapshot plus only the 11 files listed in `artifacts/pelican-config-20261015/files.txt`. Images: `portal-portal-api:pelican-config-20261015` and `portal-portal-web:pelican-config-20261015`.
- Backup: `/opt/backups/pelican-config-20261015/` (Portal database dump, verified restore listing, source archive, hashes, test/build logs). Rollback image tags: `portal-portal-api:rollback-pelican-config-20261015` and `portal-portal-web:rollback-pelican-config-20261015`. Restore these tags to `latest` and recreate only Portal API/Web using the existing production Compose pair; retain additive database columns.
- Production builds and 11 focused backend tests against an isolated PostgreSQL passed. Local frontend 8 tests and type checking passed. Waited for artwork #852 to complete before switching. A CRLF file-list issue stopped the first activation before source/image changes; the automatic rollback check succeeded, the list was normalized, and the retry succeeded.
- Portal API/Web healthy with zero restarts; public Portal/Core health endpoints passed. All other container IDs/images/start times unchanged. Browser verified the new fields, original values, and successful save (updated timestamp 17:08:58 Asia/Shanghai), retaining original model/prompt/group/key. Screenshot: `artifacts/pelican-config-20261015/deployed.png`.

## Portal drawing model and effort selectors — 2026-10-15

- Deployed `portal-portal-api:pelican-options-20261015` and `portal-portal-web:pelican-options-20261015`. Model dropdown loads the selected Key's `/v1/models` catalog on the server; browser never receives its secret. Effort dropdown supports none/minimal/low/medium/high/xhigh and propagates to Responses and Chat Completions payloads. Migration 026 defaults existing configurations to high.
- Frontend 8 tests and type check passed; production builds and 13 Pelican backend/Core-client tests with isolated PostgreSQL passed. Public health checks and Portal container health passed. Protected non-Portal container identities unchanged.
- Browser confirmed 22 model choices and 6 effort choices. Existing production configuration was not changed by verification. Backup and rollback tags use `pelican-options-20261015`; backup directory `/opt/backups/pelican-options-20261015/`. Deployment scripts and screenshot are in `artifacts/pelican-options-20261015/`.

## Pelican configuration autosave — 2026-10-15

- Published frontend-only `portal-portal-web:pelican-autosave-20261015`; all other container identities including API unchanged. Model/prompt/effort edits save after 800 ms, route leave waits for save, session drafts preserve edits across refresh, visible save/failure status added. Save no longer overwrites edits using a subsequent status request.
- Ten focused frontend tests and type checking passed; server production build and public health passed. Backup `/opt/backups/pelican-autosave-20261015/`, rollback image `portal-portal-web:rollback-pelican-autosave-20261015`.
