# Exclusive Channels Implementation Contract

Status: implemented and enabled in the canonical local environment after real Core and guarded-ingress verification. Production has not been deployed or enabled.

## Ownership

- Do not modify Core source or write directly to Core databases.
- Portal owns channels, account reservations, leases, cleanup state, cooldown configuration and audit history.
- Use existing Core administrator APIs for group creation, account binding and scoped permission management.
- Core remains the source of truth for relay balances and actual charges. Do not charge the account-package wallet or estimate charges from token counts in Portal.
- Keep the existing relay and account-package modes separate. Preserve the standalone fake-data demo.

## Agreed Product Requirements

1. Admin enters a channel name, selects available accounts and publishes. Provision a dedicated standard Core group and bind exactly the selected accounts; do not ask the admin to pre-create a group.
2. Reject unavailable, duplicate and already-reserved account selections. Do not take accounts from existing groups or account-package reservations.
3. One channel has one holder; one user can hold one channel. Enforce this server-side under concurrent requests.
4. Duration is selected before claim and defaults to 24 hours. Claim itself is free.
5. Admin configures per-channel cooldown, including zero. Capture its deadline when cleanup completes; changing configuration does not shorten an existing cooldown.
6. Manual return, selected-duration expiry, 30-minute no-spending timeout and admin recovery all enter the same reclamation process.
7. Stop new admissions when reclamation begins. Allow admitted requests to finish. Do not make the channel available until completion is confirmed and cleanup and cooldown finish.
8. Start the no-spending clock at claim. Reset it only on a positive actual Core charge for this lease. Other relay traffic, zero-cost calls and request starts do not reset it.
9. A late charge during reclamation belongs to the original holder and does not reactivate the lease.
10. Lobby and My Channel display each account's remaining 5-hour and 7-day quota directly, without a second dialog. Missing or stale upstream quota must not be represented as full availability.
11. Display expiry and no-spending countdowns for occupied channels. Claim confirmation uses the yellow no-spending warning approved in the demo.
12. History distinguishes manual, expiry, idle and administrator reclamation. No other user's credentials or upstream account secrets appear in the lobby.
13. Editing bound account membership is outside the agreed initial scope. Unpublishing does not free account reservations.
14. Newly created channel groups enable image generation by default (`allow_image_generation=true`), matching account-package groups. Batch image generation remains disabled. This changes creation defaults only, not existing groups or supported ingress protocols. Image generation through supported Responses requests still depends on the bound account's upstream capabilities; dedicated Images and asynchronous task endpoints are not enabled by this flag.

## Architecture Gate

The current Core management interfaces do not provide a reliable drain acknowledgement:

- `core/backend/internal/service/ops_concurrency.go` returns zero load on concurrency-read failures (best effort for monitoring).
- `core/backend/internal/handler/admin/account_handler.go` initializes current concurrency to zero and retains it if the read fails.
- `portal/deploy/nginx-sub2api-production.conf` forwards the public Core host directly to Core.
- Core users can modify their own key status, expiry and IP rules through existing key-management APIs. Hiding a key or button only in Portal is not an access-control boundary.

Approved: add a dedicated Portal ingress for exclusive-channel requests, with corresponding ingress and credential controls that prevent bypass. Existing relay balances and billing remain in Core. Do not infer completion from a monitoring zero or a fixed waiting period.

## Deployment and Protocol

- `PORTAL_EXCLUSIVE_ENABLED` defaults to false. Set `PORTAL_EXCLUSIVE_PUBLIC_URL` to the public Portal origin and provide a stable secret of at least 32 bytes in `PORTAL_EXCLUSIVE_SECRET`. Changing this secret without migrating ciphertext makes existing credentials unreadable.
- Before setting `PORTAL_EXCLUSIVE_INGRESS_ISOLATED=true`, replace the public Core catch-all proxy with `deploy/nginx-exclusive-core-locations.conf` and block all direct external access to Core, including alternate hosts and IPv6. This flag is an operator attestation, not an automatic firewall configuration.
- The normal public relay is forwarded through `/core-ingress/`; managed credential prefixes are rejected there. Public `/api/` routes use Portal's allowlist and managed-key restrictions. Keep Core administration accessible only through a private administrator connection.
- Channel clients use `/exclusive/v1` with the Portal-issued credential. The Core key is encrypted in Portal and is never intentionally returned in user responses.
- Initial exclusive protocol support is HTTP/SSE Responses, Chat Completions, Messages and Models. WebSocket, asynchronous/background tasks and other endpoints are rejected before admission because their completion cannot be inferred from an HTTP response EOF. Ordinary relay WebSocket upgrades use the normal reverse proxy.
- In-flight request rows survive restart. An unconfirmed or broken upstream response remains locked; no automatic TTL deletes its evidence. Operational resolution requires verification that upstream work has ended before changing an uncertain ledger entry. No UI button bypasses this check.
- Core usage is asynchronous. Billing is periodically reconciled by the original per-lease key, including closed leases for 24 hours. Core is still the billing authority; displayed totals may lag. Unknown billing state blocks admission and idle reclamation.

Admission is committed before forwarding. The worker only releases admitted requests whose upstream response completed cleanly; uncertain records remain locked across restarts. Local real Core HTTP/SSE and client-disconnect validation passed. Core crash/shutdown and uncertain-operation recovery still require operational verification before production rollout. A Portal reverse proxy alone is not proof that every upstream failure has been resolved.

An unfinished claim never exposes the client credential. Cancellation or its deadline reconciles the Core key inventory, unbinds any created key and verifies the response before closing the lease and starting cooldown. Core inventory or unbind errors preserve the reservation. Claim retries, cancellation and worker recovery share the lease allocator lock.

## Acceptance Gates

- Concurrent claim and account reservation tests demonstrate a single winner and no double binding.
- Provisioning retries and failures cannot publish partially configured groups or expose credentials before success.
- Unauthorized users cannot read, alter, return or use another lease; direct upstream access cannot bypass lease controls.
- No-spending tests cover first use, repeated charges, zero cost, delayed billing, other-mode traffic and equal expiry deadlines.
- Reclamation tests cover running requests, queued work, streaming disconnects, monitoring failures, restart recovery and zero cooldown. Unknown completion keeps the channel unavailable.
- Tests verify original-user billing and no account-package balance mutation.
- Integration tests exercise real Core administrator contracts in the local environment with dedicated test resources, not real customer accounts.
- Frontend tests cover approved demo flows, mobile layout, loading/error/stale states and direct quota display.
- Production enablement stays blocked until ingress isolation and drain guarantees are verified end to end.

## Verification Record (2026-09-15)

- Passed: PostgreSQL isolated-schema `TestExclusive|TestRental`, including partial publication retry, credential redaction, duplicate credential fields, holder uniqueness, idle timing, billing outage, uncertain-request retention, cleanup-before-cooldown and lost-create-response recovery.
- Passed: three real Vue view tests for inline quota/default duration and warning, administrator publication with selected accounts/cooldown, and failed publication feedback.
- Passed: frontend type checking and production build.
- Browser checked with intercepted test responses: desktop 1440px lobby and 390px H5 claim/admin dialogs. Quota and warning are inline; H5 has no horizontal overflow. This is frontend verification, not a real business or billing test. The shared layout intentionally chooses H5 by device detection (or the existing development `h5=1` switch), not desktop window width alone.
- Full PostgreSQL backend suite is not green: `TestAccountCustomerOperationsRejectUnauthorizedAccountIDs` and `TestAccountCustomerSingleRoutesRequireAuthorizationAndUseExpectedMethods` report failures in the account-package routes (expired account authorization, sensitive-field filtering and models response). These routes were not changed as part of this channel follow-up; a clean baseline comparison was not performed.
- Passed: `TestExclusiveAgainstLocalCore` against the canonical Core and, separately, through deployed Portal Web/Nginx. The test creates a temporary administrator and ordinary user, publishes through the real admin endpoint, claims with the ordinary user's JWT, checks hidden/immutable Core keys and blocked alternate group keys, exercises ordinary and streaming Responses plus streaming Chat Completions, and verifies real Core charges against the original relay balance with no account-package wallet mutation.
- Passed: a client disconnect through Nginx does not cancel the synthetic upstream request. Both connected and disconnected streams remain admitted during reclamation and are billed to the original holder before cleanup. The provider response is synthetic; Core routing, authentication, accounting, Portal state and Nginx are real.
- Passed at initial isolation verification: Core had no host port; ports 3000 and 8080 terminated at Portal Web and rejected managed keys on ordinary ingress. A subsequent user request restored trusted local Core administration at `127.0.0.1:8082`. This is a direct Core endpoint, not an admin-only filter: local processes/users can bypass Portal there. The setup now assumes a trusted local operator and must not be considered isolation from untrusted users on the same machine. Do not expose or forward port 8082 externally or deploy this local override to production.
- Passed: generated Portal channels/leases, Core users (including the temporary admin after demotion), accounts and groups are removed after testing. No real business account was assigned to a test channel.
- Local startup now automatically applies exclusive isolation overrides when the feature is enabled in `portal/.env`. The stable encryption secret stays in the ignored local environment file. Use `local.ps1` for future restarts.
- Remaining production gates: production network/alternate-host isolation, trusted original-client-IP forwarding through the production proxy chain, Core shutdown/crash behavior, recovery of genuinely uncertain operations, and compatibility beyond the tested Responses/Chat HTTP/SSE protocols. No production configuration was applied.

The live test is opt-in using `PORTAL_RENTAL_LIVE_CORE_URL`,
`PORTAL_RENTAL_LIVE_CORE_KEY`, `PORTAL_RENTAL_TEST_DATABASE_URL`, and
`PORTAL_EXCLUSIVE_TEST_UPSTREAM_URL=http://portal-api:18181`. Run the test process
in the canonical Portal API network namespace so Core can reach the synthetic
upstream. Without a public URL, Portal state uses an isolated database schema.
For deployed local ingress verification only, set
`PORTAL_EXCLUSIVE_TEST_PUBLIC_URL=http://portal-web:8080` and supply the local
Portal secret as `PORTAL_EXCLUSIVE_TEST_SECRET`; this mode uses only its generated
channel records in the canonical Portal database and verifies ownership before
cleaning them up. The test never writes to the Core database.
