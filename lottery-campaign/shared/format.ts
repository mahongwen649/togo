import type { Campaign } from './types'

export function maskEmail(value: string) {
  const [name, domain] = value.trim().toLowerCase().split('@')
  if (!name || !domain) return value
  return `${name.slice(0, Math.min(2, name.length))}***@${domain}`
}

export function scaledBudget(campaign: Pick<Campaign, 'randomBudget' | 'randomLimit'>, participants: number) {
  if (campaign.randomLimit <= 0) return 0
  const winners = Math.min(Math.max(participants, 0), campaign.randomLimit)
  return Math.round((campaign.randomBudget * winners / campaign.randomLimit) * 100) / 100
}

export function formatAmount(value: number) {
  return Number.isFinite(value) ? value.toFixed(2) : '0.00'
}

export function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '--'
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false
  }).format(date).replace(/\//g, '.')
}

export function formatFullDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '--'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false
  }).format(date).replace(/\//g, '-')
}
