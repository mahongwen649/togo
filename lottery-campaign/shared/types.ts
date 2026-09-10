export type CampaignStatus = 'draft' | 'scheduled' | 'registering' | 'drawing' | 'completed' | 'cancelled'

export interface Campaign {
  id: number
  name: string
  subtitle: string
  registrationStart: string
  drawAt: string
  randomLimit: number
  participantLimit: number
  randomMin: number
  randomMax: number
  randomBudget: number
  guaranteeAmount: number
  participants: number
  randomWinners: number
  guaranteedWinners: number
  credited: number
  failed: number
  randomPrize?: number
  guaranteePrize?: number
  status: CampaignStatus
  published: boolean
}

export interface PayoutRecord {
  id: number
  campaign: string
  email: string
  prizeType: 'random' | 'guarantee'
  amount: number
  status: 'pending' | 'credited' | 'processing' | 'failed'
  createdAt: string
}
