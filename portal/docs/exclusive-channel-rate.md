# Exclusive Channel Rate

- Portal administrator may set the Core group's `rate_multiplier` when publishing
  or editing a channel. Default 1; positive finite numbers only. Input precision
  follows Core's 0.001 step. Core performs the actual relay billing, not Portal.
- Show the multiplier in the admin list, lobby, holder view and claim confirmation.
  Claims from the updated UI include the displayed rate and reject changed quotes.
- Only available/maintenance/unpublished channels without any open lease may change
  rate. All other configuration edits preserve the multiplier when omitted.
- Persist the desired rate and suspend claims before touching Core. Verify Core's
  returned rate before reopening. Failed/ambiguous updates remain locked and can
  use the existing retry-sync action. Preserve maintenance/unpublished state.
- Existing channels migrate to 1, matching their original creation default.
  Core source and account multipliers are unchanged. OpenAI image pricing is now
  managed separately with independent default rate 1; see
  `exclusive-channel-pricing.md` for the model/image pricing extension.
- Acceptance: publish with fractional rate, edit and verify upstream payload,
  retained state, invalid values, occupied rejection, lost-response recovery,
  stale claim rejection and frontend input/display coverage. No billable inference
  is needed for validation.
