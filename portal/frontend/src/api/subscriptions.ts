/** User subscription API. */

import { apiClient } from './client'
import type { UserSubscription, SubscriptionProgress } from '@/types'

export async function getMySubscriptions(): Promise<UserSubscription[]> {
  const { data } = await apiClient.get<UserSubscription[]>('/subscriptions')
  return data
}

export async function getActiveSubscriptions(): Promise<UserSubscription[]> {
  const { data } = await apiClient.get<UserSubscription[]>('/subscriptions/active')
  return data
}

export async function getSubscriptionsProgress(): Promise<SubscriptionProgress[]> {
  const { data } = await apiClient.get<SubscriptionProgress[]>('/subscriptions/progress')
  return data
}

export async function getSubscriptionProgress(subscriptionId: number): Promise<SubscriptionProgress> {
  const { data } = await apiClient.get<SubscriptionProgress>(`/subscriptions/${subscriptionId}/progress`)
  return data
}

export default {
  getMySubscriptions,
  getActiveSubscriptions,
  getSubscriptionsProgress,
  getSubscriptionProgress,
}
