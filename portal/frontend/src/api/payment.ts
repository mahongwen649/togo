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
