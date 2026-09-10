export type PaymentOrderStatus =
  | 'PENDING' | 'PAID' | 'RECHARGING' | 'COMPLETED' | 'EXPIRED'
  | 'CANCELLED' | 'FAILED' | 'REFUND_REQUESTED' | 'REFUNDING'
  | 'REFUND_PENDING' | 'PARTIALLY_REFUNDED' | 'REFUNDED' | 'REFUND_FAILED'

export interface PaymentMethodLimit {
  display_name?: string
  single_min: number
  single_max: number
  fee_rate: number
  available?: boolean
}

export interface PaymentCheckoutInfo {
  methods: Record<string, PaymentMethodLimit>
  global_min: number
  global_max: number
  balance_disabled: boolean
  balance_recharge_multiplier: number
  recharge_fee_rate: number
  help_text: string
}

export interface CreatePaymentOrderResult {
  order_id: number
  out_trade_no?: string
  pay_url?: string
  amount: number
  pay_amount: number
  fee_rate: number
  expires_at: string
  campaign_bonus?: number
  credited_amount?: number
  campaign_bonus_status?: 'PENDING' | 'APPLYING' | 'APPLIED' | 'FAILED'
}

export interface PaymentOrder {
  id: number
  amount: number
  pay_amount: number
  currency?: string
  fee_rate: number
  payment_type: string
  out_trade_no: string
  status: PaymentOrderStatus
  order_type: string
  created_at: string
  expires_at: string
  paid_at?: string
  completed_at?: string
  campaign_bonus?: number
  credited_amount?: number
  campaign_bonus_status?: 'PENDING' | 'APPLYING' | 'APPLIED' | 'FAILED'
}

export interface PaymentOrderPage {
  items: PaymentOrder[]
  total: number
  page: number
  page_size: number
  pages: number
}
