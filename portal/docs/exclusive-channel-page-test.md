# Channel page generation acceptance

Scope: Portal only; no Core source changes. Each channel retains one latest
successful HTML page and its completion time, visible to authenticated channel
viewers even after the holder releases the channel. No history or automatic rating.

- Only the active, unexpired holder may start a test. Admin privileges do not
  override ownership. Billing freshness and lease concurrency limits still apply.
- Use the existing lease upstream key and `/v1/responses`, model `gpt-6-astra`,
  reasoning effort `high`, and exact user prompt:
  `生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画，不用进行测试`
- Admit the job and normal in-flight ledger atomically. One running job per channel.
  Closing the modal or disconnecting does not cancel generation. Expiry/release
  waits for completion. Ambiguous upstream termination keeps the lease locked.
- A successful, completed HTML response replaces the prior page. Failed, incomplete
  or oversized responses preserve it. Persist result before finishing the ledger.
  Never automatically retry a request after restart (avoids duplicate billing).
- Lobby: two columns on desktop, one on mobile; inline account/platform/quota,
  countdowns, latest result/time and view action. No embedded active previews.
- My channel: existing credentials/quota and a dedicated test section. Only here
  expose generation to the active holder. Admin list has view-only access.
- Preview: large modal, model/effort/time/status, fullscreen and close. Generated
  content is isolated from Portal, credentials and network. Retain inline SVG,
  CSS and JavaScript animations. Never evaluate generated markup in Portal DOM.
- Verify ownership/races, persistence/replacement, request draining, release
  blocking, parser failures, UI states, mobile sizing and hostile preview content.

Operational limitation: restart or transport failure with an admitted unresolved
request requires operator reconciliation, just like ordinary exclusive requests.
No unsafe automatic unlock or request replay is introduced.

## Verification (2026-09-16)

- Docker Go regression run: `TestExclusive|TestRental` passed using isolated DB
  schemas. Added page tests cover ownership (including non-holder admin), expiry,
  idle deadline, releasing/unclaimed leases, stale billing, concurrency limits,
  concurrent duplicate submissions, restart replay prevention, exact model/prompt/
  reasoning/key, release waiting, successful replacement and failure retention.
- Frontend: 8 tests passed across the channel view and page-test component.
- Canonical local build (`local.ps1 up`) passed TypeScript/Vite and health checks.
- Browser inspection: desktop 1440px and iPhone 13 390px; no horizontal overflow;
  two-column lobby on desktop, one on mobile; holder test action visible without
  scrolling through account quotas. Inline SVG animation changes between captures.
- Preview probe in the actual modal: Portal localStorage access denied; fetch,
  external image and self-navigation blocked by CSP, with no external requests.
  The trusted outer frame owns `frame-src 'none'`; both iframe layers are sandboxed
  without `allow-same-origin`. No generated code is run in the Portal document.
- No real provider generation was invoked. Availability of `gpt-6-astra` in an
  operator's Core channel remains dependent on that channel's upstream configuration.

## HTML Extraction Fix (2026-09-16)

- Keep the original user prompt, model and reasoning level unchanged. Add a
  separate output-format instruction requesting self-contained HTML only.
- Extract full documents from explanatory prose and HTML code fences. Prefer
  final-answer messages and assemble text parts within each message. Use the HTML
  tokenizer so closing-tag strings inside scripts do not truncate the document.
- Accept a closed body with an omitted closing html tag, or wrap a complete SVG
  fragment for preview. Incomplete model responses and incomplete markup still
  fail without replacing a successful page. Preview isolation remains unchanged.
- Retain the latest parser-failure diagnostic encrypted in `response_secret`.
  It contains assistant output and response shape, not reasoning or credentials.
  It is not returned by user/list APIs. Successful generation clears it.
- Failures before this fix did not save their original output and cannot be
  recovered retroactively. Deployment does not retry billable requests.
