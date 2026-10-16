# Account-Package Alipay Recharge

Status: deployed to production on 2026-09-14 at 14:38 Asia/Shanghai. Authenticated checkout and history load successfully against production, and the desktop/mobile recharge forms are visible. No live payment has been submitted; user-authorized small-payment acceptance remains pending.

Release: `portal-account-payments-20260914-143224`. Verified backup: `/opt/backups/portal-account-payments-20260914-143224/`, including the Portal database dump, restore listing, source files, protected environment snapshots and checksums. Rollback images: `portal-portal-api:rollback-account-payments-20260914-143224` and `portal-portal-web:rollback-account-payments-20260914-143224`. Preserve `environment.active` and the payment tables if any real orders have been created.

Production checks: API/Web healthy with zero restarts; Core and both PostgreSQL containers retained their IDs and start times; public health and recharge page return 200, unauthenticated checkout returns 401, malformed callback returns 400. The configured gateway's read-only order-query API responds successfully. Thirteen payment test groups (including isolated PostgreSQL and HTTP-route settlement tests), the full Go suite, 755 frontend tests and production builds passed. These checks do not establish a completed live payment.

## Ownership

- Relay recharge continues to use Core `/payment/orders` and the existing Core fulfillment.
- Account-package recharge uses `/account-market/payments/orders`, the same configured EasyPay Alipay merchant, and a separate `acct_` order namespace.
- Only the Portal account wallet is credited. No Core order is created, and no Core balance is credited or transferred.
- Account-package credit is CNY 1:1. Relay promotion bonuses and the Core balance multiplier do not apply. The existing provider fee percentage is charged separately, rounding upward to cents.
- The Core connection is used only for reading configuration and provider usage. Core code, tables, and credentials are not modified.

## Activation

Deploy both Portal API and Web. API startup applies migration `012_account_payments.sql` without changing existing wallet balances. Configure these in the protected Portal `.env`:

```dotenv
PORTAL_PUBLIC_URL=https://togoapi.com
PORTAL_ACCOUNT_PAYMENT_SECRET=<stable random secret of at least 32 characters>
PORTAL_ACCOUNT_PAYMENT_PROVIDER_ID=0
```

Keep the secret out of logs, Git and browser responses. Back it up with the protected environment file; changing it prevents pending snapshots from being decrypted. `0` selects the first enabled EasyPay instance supporting Alipay, ordered by configured sort order and ID. Use an explicit Core instance ID when multiple providers exist. Existing `CORE_DATABASE_URL` is required.

The selected Core provider must store readable JSON configuration containing `pid`, `pkey`, and an HTTPS `apiBase`. Old encrypted configurations must be re-saved through Core administration first. Official direct-Alipay and non-CNY providers are not supported by this integration. Misconfiguration disables account-package payments, with no fallback to relay recharge.

The global payment switch, Alipay routing switch/source, amount limits, pending-order cap and fee are respected. Account-package daily user limits are tracked separately. Provider daily usage checks include Core and Portal orders, but a shared hard quota still needs enforcement at the gateway because the two databases cannot atomically reserve one provider quota.

For legacy Core settings with no configured Alipay source, the sole enabled provider family is used; ambiguous provider families are rejected. Once a source is configured, its explicit enable switch is enforced. This matches the existing legacy checkout without rewriting Core settings.

## Settlement

Notify URL: `/api/v1/account-market/payments/webhook/easypay` (GET or form POST).

The handler validates the merchant-bound signature, payment method, exact paid cents, trade number and stored order. A supplied merchant ID must match; ZPAY callbacks can omit it, in which case the original merchant key and encrypted order snapshot bind the merchant. Duplicate form fields and wrong signatures are rejected. The gateway return URL is for navigation only and never grants credit. The result page queries the authenticated account-payment API to confirm actual completion.

ZPAY compatibility: merchant order numbers are at most 32 characters; the return URL has no preset query string (the gateway supplies the signed result parameters); reconciliation uses the documented HTTPS GET order-query API. Never log the query URL because it contains the merchant key.

Each order's merchant configuration is encrypted using AES-GCM with the order number as authenticated data. Pending-order callbacks remain verifiable after provider configuration changes. The wallet increment, ledger entry and order completion commit in a single PostgreSQL transaction. Row locking and a unique gateway transaction constraint prevent replay and double credit. Verified payments arriving after local expiry still settle.

A 30-second worker reconciles recent unfinished orders, including expired orders, for up to 30 days. Per-order durable leases throttle provider queries. Signed callbacks can settle older paid orders as well. For payments older than 30 days with no callback, use the authenticated history status check; never grant credit based on a screenshot or return URL alone.

## UI And Operations

`/recharge` shows separate Alipay forms and explicit destination wallets. `/account-business/balance/history` defaults to online payments, with the original card history retained in its own tab. Pending online orders can be resumed without creating another order. The original dedicated card redemption page remains available, but card redemption is not the account-package recharge form.

Before production activation, back up the Portal database and API/Web images. Keep migration 012 and the payment secret when rolling back application images. Once real orders exist, do not remove their records or the secret. Older API versions do not service the new callbacks: pause new orders and retain a settlement-capable API until all outstanding payments are reconciled.

After deployment, verify health, authenticated checkout, merchant selection, public callback routing and a user-authorized small live payment. Check exactly one `account_payment` wallet-ledger entry, unchanged Core balance, correct history, and duplicate callback behavior. Local tests use an isolated schema and an HTTP test gateway; they are not a substitute for this live acceptance check.
