/**
 * Normalize upstream ChatGPT plan names before applying display labels.
 */
export function normalizePlanType(value?: string | null): string {
  return (value || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
}

/**
 * OpenAI-only plan labels. Other platforms use their own tier naming.
 */
export function openAIPlanTypeLabel(value?: string | null): string {
  switch (normalizePlanType(value)) {
    case 'plus':
      return 'Plus'
    case 'chatgptpro':
    case 'pro':
      return 'Pro 20x'
    case 'prolite':
      return 'Pro 5x'
    case 'selfservebusinessprolite':
      return 'Business Premium'
    case 'team':
      return 'Business Standard'
    case 'free':
      return 'Free'
    default:
      return ''
  }
}
