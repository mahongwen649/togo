export interface UserDisplayIdentity {
  email?: string | null
  username?: string | null
}

export function getUserAvatarInitial(user: UserDisplayIdentity | null | undefined): string {
  const source = user?.email?.trim() || user?.username?.trim() || 'U'
  return source.charAt(0).toUpperCase()
}
