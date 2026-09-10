export const PASSWORD_MIN_LENGTH = 8

export function meetsPasswordLength(value: string): boolean {
  return value.length >= PASSWORD_MIN_LENGTH
}
