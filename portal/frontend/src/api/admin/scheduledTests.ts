/**
 * Customer-scoped scheduled test plan API.
 *
 * The Portal routes keep the Core scheduled-test contract while requiring the
 * owning account ID on every operation so the backend can enforce package
 * ownership before proxying to Core with its server-side admin credential.
 */

import { apiClient } from '../client'
import type {
  CreateScheduledTestPlanRequest,
  ScheduledTestPlan,
  ScheduledTestResult,
  UpdateScheduledTestPlanRequest
} from '@/types'

const plansPath = (accountId: number) => `/account-market/accounts/${accountId}/scheduled-test-plans`

export async function listByAccount(accountId: number): Promise<ScheduledTestPlan[]> {
  const { data } = await apiClient.get<ScheduledTestPlan[]>(plansPath(accountId))
  return data ?? []
}

export async function create(req: CreateScheduledTestPlanRequest): Promise<ScheduledTestPlan> {
  const { data } = await apiClient.post<ScheduledTestPlan>(plansPath(req.account_id), req)
  return data
}

export async function update(
  accountId: number,
  id: number,
  req: UpdateScheduledTestPlanRequest
): Promise<ScheduledTestPlan> {
  const { data } = await apiClient.put<ScheduledTestPlan>(`${plansPath(accountId)}/${id}`, req)
  return data
}

export async function deletePlan(accountId: number, id: number): Promise<void> {
  await apiClient.delete(`${plansPath(accountId)}/${id}`)
}

export async function listResults(
  accountId: number,
  planId: number,
  limit?: number
): Promise<ScheduledTestResult[]> {
  const { data } = await apiClient.get<ScheduledTestResult[]>(
    `${plansPath(accountId)}/${planId}/results`,
    { params: limit ? { limit } : undefined }
  )
  return data ?? []
}

export const scheduledTestsAPI = {
  listByAccount,
  create,
  update,
  delete: deletePlan,
  listResults
}

export default scheduledTestsAPI
