# Exclusive Channels Frontend Demo

## Frozen preview scope

- Standalone frontend preview; no production feature, authentication, HTTP business APIs, Core operations, databases, or real balances.
- Existing Portal entry, navigation and deployment remain unchanged.
- Reuse Portal's Vue, styling, logo and icons. Demo data lives in this browser tab's session storage only.

## Product behavior

1. A channel represents one exclusive standard group with multiple accounts.
2. One user holds one channel; one channel has one holder.
3. Select duration before claiming, default 24 hours. Admin options must include 24 hours.
4. Claiming is free. Simulated requests deduct only the simulated relay balance.
5. Early return, expiry and admin force return use the same cleanup path.
6. Cleanup rejects new requests and waits for existing requests to finish before handoff.
7. Admin sets per-channel cooldown in seconds, including zero. It starts after cleanup.
8. A cooling deadline is captured when cooling starts; later edits do not change it.
9. Closing dialogs or switching demo users does not release a channel.
10. No extension, queue, multiple claims, package-wallet billing or real credentials.
11. Reclaim after 30 consecutive minutes without positive spending, initially measured from claim and reset only when a billed request completes. Request starts and zero-cost completions do not reset it. Existing requests still drain before cooldown; later completions do not cancel reclamation. Expiry wins when both deadlines coincide. A positive charge at the idle deadline resets the idle timer before evaluation.

## Surfaces

- Lobby: search, status filter, account counts, inline per-account remaining 5-hour/7-day quota bars, models, sample rates and usage. Active channels show a live expiry countdown; draining channels show cleanup status rather than promise immediate availability.
- My channel: demo credentials, expiry, in-flight requests, inline per-account remaining quota bars without a secondary dialog, usage, early return.
- My history: selected demo user's sessions only.
- Admin: enter channel name, select one or more available sample accounts from a searchable dropdown, then publish. Publishing creates a new simulated Core group and binds the selected accounts. Edit name/durations/cooldown, pause/unpublish idle channels, force return, events.
- Preview controls: identity, simulated request, jump to expiry/next transition, reset and theme.

## Acceptance

- Default duration is 24 hours; selecting another duration changes the deadline.
- Double claim, a second channel, another user's return and new requests during cleanup are rejected by the local model.
- Expiry uses the simulated clock, independent of the current page.
- In-flight requests finish and charge their original user before cooldown; no double billing.
- Zero cooldown still waits for cleanup and in-flight requests.
- Idle reclaim covers no usage, repeated positive spending, zero-cost requests, in-flight draining, restored sessions and large clock jumps. Lobby and My Channel show the idle countdown; history identifies idle reclamation.
- Cooling cannot be bypassed by switching users or editing its setting.
- Admin cannot reassign an occupied channel or bypass cleanup by maintenance/unpublish.
- Publishing no longer selects an existing group. Empty, duplicate, unavailable or already-bound account selections fail without creating a group or channel. Bound accounts stay reserved after unpublishing and refreshing. Editing existing account membership is outside this preview scope; the current bindings are read-only.
- Model tests cover transitions, guards, balances, cooldown snapshots and admin validation.
- Browser checks cover claiming, returning, expiry, user switching, config, persistence and reset.
- Desktop/mobile have no page-level horizontal overflow or overlapping controls.
- Network checks show no business API calls. Core and Portal backend remain untouched by this work.

## Run

From `portal/frontend`: `pnpm demo:channels`.
Open `http://127.0.0.1:3010/exclusive-channels-demo.html`.
Build independently with `pnpm demo:channels:build`.
