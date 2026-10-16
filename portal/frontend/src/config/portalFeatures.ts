// Temporarily hide these Portal features while preserving existing business data.
export const ACCOUNT_PACKAGES_ENABLED = false
export const EXCLUSIVE_CHANNELS_ENABLED = false

export function isHiddenPortalPath(path: string): boolean {
  const roots = [
    ...(!ACCOUNT_PACKAGES_ENABLED ? ['/account-business', '/account-market', '/admin/account-business', '/admin/account-market'] : []),
    ...(!EXCLUSIVE_CHANNELS_ENABLED ? ['/exclusive-channels', '/admin/exclusive-channels'] : []),
  ]
  return roots.some(root => path === root || path.startsWith(`${root}/`))
}
