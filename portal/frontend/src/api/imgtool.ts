import apiClient from './client'

export interface ImgtoolSSOURLResponse {
  redirect_url: string
}

export async function createImgtoolSSOURL(): Promise<ImgtoolSSOURLResponse> {
  const response = await apiClient.post<ImgtoolSSOURLResponse>('/integrations/imgtool/sso-url')
  return response.data
}
