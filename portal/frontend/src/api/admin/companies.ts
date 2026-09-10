import { apiClient } from '../client'

export interface AdminCompany {
  id: number
  name: string
  status: string
  created_at?: string
  updated_at?: string
}

export interface CompanyMember {
  user_id: number
  email: string
  username?: string
  role: string
  status: string
  balance: number
  api_key_count: number
  total_cost: number
  request_count: number
  last_request_at?: string
  company_id: number
  company_name: string
}

export interface CompanyManager {
  company_id: number
  user_id: number
  email: string
  username?: string
  role: string
  status: string
  created_at?: string
}

export interface PaginatedCompanyMembers {
  items: CompanyMember[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface CompanyMembersParams {
  page?: number
  page_size?: number
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export interface AllocateBalanceRequest {
  amount: number
  notes?: string
}

export interface AssignCompanyMemberRequest {
  user_id: number
}

export interface CreateCompanyMemberRequest {
  email: string
  password: string
  username: string
  notes?: string
  balance?: number
  concurrency?: number
  rpm_limit?: number
}

export interface AddCompanyManagerRequest {
  user_id: number
}

export interface CompanyMemberMutationResult {
  company_id: number
  user_id: number
  updated_members: number
  updated_api_keys?: number
}

export interface CreateCompanyRequest {
  name: string
  status?: string
}

export interface UpdateCompanyRequest {
  name: string
  status: string
}

export async function getAll(): Promise<AdminCompany[]> {
  const { data } = await apiClient.get<AdminCompany[]>('/admin/companies/all')
  return data
}

export async function create(body: CreateCompanyRequest): Promise<AdminCompany> {
  const { data } = await apiClient.post<AdminCompany>('/admin/companies', body)
  return data
}

export async function update(companyId: number, body: UpdateCompanyRequest): Promise<AdminCompany> {
  const { data } = await apiClient.put<AdminCompany>(`/admin/companies/${companyId}`, body)
  return data
}

export async function deactivate(companyId: number): Promise<AdminCompany> {
  const { data } = await apiClient.delete<AdminCompany>(`/admin/companies/${companyId}`)
  return data
}

export async function assignMember(companyId: number, body: AssignCompanyMemberRequest): Promise<CompanyMemberMutationResult> {
  const { data } = await apiClient.post<CompanyMemberMutationResult>(`/admin/companies/${companyId}/members`, body)
  return data
}

export async function createMember(companyId: number, body: CreateCompanyMemberRequest) {
  const { data } = await apiClient.post(`/admin/companies/${companyId}/members/create`, body)
  return data
}

export async function removeMember(companyId: number, userId: number): Promise<CompanyMemberMutationResult> {
  const { data } = await apiClient.delete<CompanyMemberMutationResult>(`/admin/companies/${companyId}/members/${userId}`)
  return data
}

export async function getMembers(companyId: number, params: CompanyMembersParams = {}): Promise<PaginatedCompanyMembers> {
  const { data } = await apiClient.get<PaginatedCompanyMembers>(`/admin/companies/${companyId}/members`, {
    params,
  })
  return data
}

export async function getManagers(companyId: number): Promise<CompanyManager[]> {
  const { data } = await apiClient.get<CompanyManager[]>(`/admin/companies/${companyId}/managers`)
  return data
}

export async function addManager(companyId: number, body: AddCompanyManagerRequest) {
  const { data } = await apiClient.post(`/admin/companies/${companyId}/managers`, body)
  return data
}

export async function removeManager(companyId: number, userId: number) {
  const { data } = await apiClient.delete(`/admin/companies/${companyId}/managers/${userId}`)
  return data
}

export async function allocateBalance(companyId: number, userId: number, body: AllocateBalanceRequest) {
  const { data } = await apiClient.post(`/admin/companies/${companyId}/members/${userId}/balance`, body)
  return data
}

export default { getAll, create, update, deactivate, assignMember, createMember, removeMember, getMembers, getManagers, addManager, removeManager, allocateBalance }
