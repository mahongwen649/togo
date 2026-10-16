# OpenAI Exclusive Channel Pricing

## Requirements

- No Core source edits. Core remains the sole billing ledger; Portal writes and
  verifies managed group settings using the administrator API.
- Text models use exact model identifiers. Input, output and cache-read prices
  are independently optional. Empty fields inherit Core default token prices;
  explicit zero means free. Removing a row restores that model's defaults.
- The editor uses relay balance units per million tokens and converts once to
  Core's per-token representation. Portal displays the existing balance unit as
  yuan; this feature does not introduce any currency exchange conversion.
- Image price is a separate flat price per generated image, default 0.05, across
  all image sizes. Final image fee is image count * unit price * image rate.
  The independent image rate defaults to 1 and can be set separately. Text rates
  continue to use the channel rate. Existing Core service-tier
  and reasoning policies remain unchanged.
- Managed group model cards reserve gpt-image-* and dall-e-* for image billing.
  These patterns cannot be used in text overrides. Core's OpenAI Responses image
  tool selects its image billing model independently of the text request model.
- Configure all three group image size prices and enable independent image
  rate (default 1). No new inference protocols are exposed and batch image stays disabled.
- Price changes require an unoccupied available/maintenance/unpublished channel.
  Persist desired prices and block claims before Core changes. Verify the full
  response; failures stay blocked and reuse retry sync. Restore prior visibility.
- Claims carry a pricing revision and show the quoted text/image prices. Reject
  stale quotes. Existing leases are never repriced during deployment. Legacy
  channels block new claims until the worker can synchronize while unoccupied.
- Cards use restrained gradient backgrounds, pointer tilt and press feedback;
  touch and reduced-motion users get stable layouts without tilt.

## Acceptance

- New channel has 0.05 image price, image billing cards and inherited text rates.
- Create/edit/read back partial overrides; preserve null versus zero; convert
  million-token prices exactly once; restore default prices and remove rows.
- Invalid, duplicate, wildcard and image-model text entries are rejected.
- Changes while occupied are rejected by API as well as disabled in the UI.
- Lost Core responses remain fail-closed and retry restores prior state.
- Pending legacy settings wait for lease cleanup and invalidate older quotes.
- Synthetic local Core inference verifies image counts and final balance charges
  without calling a paid upstream provider or changing real users' balances.
- Frontend tests plus desktop/mobile, dark/light, pointer and reduced-motion QA.

## Verified Locally

- Synthetic OpenAI Responses returned three images with different sizes. With
  image price 0.05 and image rate 1, their cost was 0.15 even when text rate was 2.
  Three text requests at the configured token prices cost 0.0024. Both Core's
  usage ledger and the temporary user's balance change equaled 0.1524.
- The test creates and cleans up temporary users, accounts and groups, uses a
  dedicated Portal test database, and never sends a request to a paid provider.
