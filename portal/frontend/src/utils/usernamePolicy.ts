const NEW_USERNAME_PATTERN = /^[A-Za-z0-9_\-\u4e00-\u9fff]+$/

export function normalizeNewUsername(username: string): string {
  return username.trim()
}

export function validateNewUsername(username: string): boolean {
  const normalized = normalizeNewUsername(username)
  const length = Array.from(normalized).length
  return length >= 2 && length <= 30 && NEW_USERNAME_PATTERN.test(normalized)
}
