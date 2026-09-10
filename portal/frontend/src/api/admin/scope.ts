import { apiClient } from '../client'

export interface ManagementScope {
  user_id: number
  is_admin: boolean
  is_group_manager: boolean
  is_company_manager: boolean
  managed_group_ids: number[]
  managed_company_ids: number[]
  can_access_admin_area: boolean
}

export async function getManagementScope(): Promise<ManagementScope> {
  const { data } = await apiClient.get<ManagementScope>('/admin/management/scope')
  return data
}

export default { getManagementScope }
