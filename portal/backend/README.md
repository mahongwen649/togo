# Portal Backend

Planned scope:

- same-origin proxy and DTO adaptation for official Core APIs;
- Core session handling without a second password database;
- company, membership, company manager, and finance services;
- company-scoped authorization before calling Core admin APIs;
- health and static frontend serving.

Explicitly excluded:

- independent identity/password implementation;
- OAuth, TOTP, invitations, Portal settings, payment, subscriptions, IP management;
- Portal actor/HMAC protocols and Core-specific business patches;
- direct Core database access.

## Portal balance alerts

Set `PORTAL_BALANCE_ALERT_ENABLED=true` to run the Portal-owned low-balance
monitor. It polls the official Core public settings and paginated admin user
API, honors the global and per-user notification switches and thresholds, and
sends a Chinese alert to the primary account email when no verified Core
notification recipient exists. Crossing state is stored in Portal PostgreSQL,
so restarts do not resend alerts. The first scan establishes a baseline and
does not send historical low-balance alerts.

`PORTAL_BALANCE_ALERT_INTERVAL_SECONDS` defaults to `60`.

## Username reconciliation

`cmd/username-reconcile` compares the Portal username registry with the official
Core admin user list. It is dry-run by default and reports matches, safe imports,
orphaned registry rows, conflicts, and invalid Core usernames without including
email addresses or credentials.

Set `CORE_BASE_URL`, `CORE_ADMIN_API_KEY`, and `PORTAL_DATABASE_URL`, then run:

```text
go run ./cmd/username-reconcile
go run ./cmd/username-reconcile --apply
```

Apply is refused when any conflict exists. Conflict-free apply only inserts
previously unmapped Core usernames and removes registry rows whose Core user no
longer exists; it never changes a Core user.
