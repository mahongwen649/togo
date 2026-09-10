export type RouteQueryValue = string | null | Array<string | null> | undefined

export function getSingleRouteQueryValue(value: RouteQueryValue): string {
  const candidate = Array.isArray(value)
    ? value.find((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : value

  return typeof candidate === 'string' ? candidate.trim() : ''
}
