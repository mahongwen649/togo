# TogoAPI Server Handoff

Last updated: 2026-09-10 (Asia/Shanghai)

This is the deployment handoff for future sessions. It deliberately contains no passwords, private-key contents, Admin API Keys, database credentials, or application secrets.

## Architecture

| Public name | Responsibility | Current state |
| --- | --- | --- |
| `https://api.togoapi.com` | Sub2API Core, `/v1/*`, and internal account-pool administration | Deployed and healthy through Cloudflare |
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

The Tokyo server runs the production Sub2API Core database restored from the final Singapore snapshot on 2026-08-23, not a copy of the local development database.

| Item | Value |
| --- | --- |
| Deployment directory | `/opt/sub2api` |
| Compose file | `/opt/sub2api/docker-compose.local.yml` |
| Secret environment file | `/opt/sub2api/.env` |
| Current Core version | `Sub2API 0.2.4` |
| Current image | `weishaw/sub2api:latest` (`sha256:ccf47a1c62e355f51f896e489f8253e119fe4101b103cd701ba458cc6c6f0f77`) |
| Core container | `sub2api` |
| PostgreSQL container | `sub2api-postgres` |
| Redis container | `sub2api-redis` |
| Core host binding | `127.0.0.1:8080` |
| Nginx site | `/etc/nginx/sites-available/sub2api` |
| Nginx backup before Portal cutover | `/etc/nginx/sites-available/sub2api.bak-20260726-045152` |

PostgreSQL and Redis are Docker-internal only and must never be exposed publicly.

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
