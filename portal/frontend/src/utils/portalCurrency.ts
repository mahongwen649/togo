export const PORTAL_USD_CNY_RATE = 7

/** Customer balances and actual charges are already denominated 1:1 in CNY. */
export function customerAmountCny(amount: number | null | undefined): number {
  return finiteAmount(amount)
}

/** Upstream account costs are stored in USD and need conversion for Portal display. */
export function upstreamCostCny(amountUsd: number | null | undefined): number {
  return finiteAmount(amountUsd) * PORTAL_USD_CNY_RATE
}

/** Effective customer multiplier against the official USD price converted to CNY. */
export function customerEffectiveMultiplier(multiplier: number | null | undefined): number {
  return finiteAmount(multiplier) / PORTAL_USD_CNY_RATE
}

function finiteAmount(amount: number | null | undefined): number {
  return typeof amount === 'number' && Number.isFinite(amount) ? amount : 0
}
