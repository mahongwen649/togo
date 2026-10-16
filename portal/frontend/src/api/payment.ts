import { apiClient } from './client'
import type { CreatePaymentOrderResult, PaymentCheckoutInfo, PaymentOrder, PaymentOrderPage } from '@/types/payment'
import { isMobileDevice } from '@/utils/device'

export async function getPaymentCheckoutInfo(): Promise<PaymentCheckoutInfo> {
  const { data } = await apiClient.get<PaymentCheckoutInfo>('/payment/checkout-info')
  return data
}

export async function createPaymentOrder(amount: number): Promise<CreatePaymentOrderResult> {
  const { data } = await apiClient.post<CreatePaymentOrderResult>('/payment/orders', {
    amount,
    payment_type: 'alipay',
    order_type: 'balance',
    payment_source: 'hosted_redirect',
    return_url: `${window.location.origin}/payment/result`,
    is_mobile: isMobileDevice(),
  })
  return data
}

export async function getPaymentOrder(id: number): Promise<PaymentOrder> {
  const { data } = await apiClient.get<PaymentOrder>(`/payment/orders/${id}`)
  return data
}

export async function verifyPaymentOrder(outTradeNo: string): Promise<PaymentOrder> {
  if (outTradeNo.trim().startsWith('acct_')) {
    return accountPaymentAPI.verify(outTradeNo.trim())
  }
  const { data } = await apiClient.post<PaymentOrder>('/payment/orders/verify', { out_trade_no: outTradeNo.trim() })
  return data
}

export async function getMyPaymentOrders(page = 1, pageSize = 20): Promise<PaymentOrderPage> {
  const { data } = await apiClient.get<PaymentOrderPage>('/payment/orders/my', {
    params: { page, page_size: pageSize, order_type: 'balance' },
  })
  return data
}

export async function cancelPaymentOrder(id: number): Promise<void> {
  await apiClient.post(`/payment/orders/${id}/cancel`)
}

export const accountPaymentAPI = {
  checkout() {
    return apiClient.get<PaymentCheckoutInfo>('/account-market/payments/checkout-info').then(({ data }) => data)
  },
  create(amount: number) {
    return apiClient.post<CreatePaymentOrderResult>('/account-market/payments/orders', {
      amount, payment_type: 'alipay', is_mobile: isMobileDevice(),
    }).then(({ data }) => data)
  },
  get(id: number) {
    return apiClient.get<PaymentOrder>(`/account-market/payments/orders/${id}`).then(({ data }) => data)
  },
  verify(outTradeNo: string) {
    return apiClient.post<PaymentOrder>('/account-market/payments/verify', { out_trade_no: outTradeNo }).then(({ data }) => data)
  },
  list(page = 1) {
    return apiClient.get<PaymentOrderPage>('/account-market/payments/orders', { params: { page } }).then(({ data }) => data)
  },
}
