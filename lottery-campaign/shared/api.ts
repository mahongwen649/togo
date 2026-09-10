export type ApiEnvelope<T> = { code: string | number; message?: string; data: T }

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: string | number = status
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

const apiErrorMessages: Record<string, string> = {
  ACCOUNT_NOT_FOUND: '未找到该注册账户，请确认邮箱或先完成注册。',
  ALREADY_REGISTERED: '该账户已报名本期活动。',
  ACTIVE_CAMPAIGN_EXISTS: '当前已有一期活动处于发布状态。',
  CAMPAIGN_LOCKED: '报名开始后不能修改活动配置。',
  CAMPAIGN_NOT_CANCELLABLE: '当前活动状态不允许取消。',
  CAMPAIGN_NOT_DRAWABLE: '只有已发布的活动才能开奖。',
  CAMPAIGN_NOT_FOUND: '活动不存在或已被移除。',
  INVALID_EMAIL: '请输入有效的账户邮箱。',
  INVALID_ID: '活动编号无效。',
  INVALID_BUDGET: '总预算无法满足当前随机额度区间。',
  INVALID_REQUEST: '请求参数有误，请检查后重试。',
  LOTTERY_ERROR: '活动服务请求失败，请稍后重试。',
  LOTTERY_UNAVAILABLE: '活动服务暂不可用，请稍后重试。',
  NO_ACTIVE_CAMPAIGN: '当前暂无开放活动。',
  PARTICIPANT_LIMIT_REACHED: '本期报名人数已达上限。',
  REGISTRATION_CLOSED: '本期报名已截止。',
  REGISTRATION_NOT_STARTED: '本期报名尚未开始。',
  RESULT_NOT_AVAILABLE: '开奖结果暂未生成，请稍后再试。',
  RATE_LIMITED: '请求过于频繁，请稍后再试。',
  METHOD_NOT_ALLOWED: '当前操作不被支持。',
  UNAUTHORIZED: '登录状态已失效，请重新登录。',
  FORBIDDEN: '你没有执行此操作的权限。'
}

export function localizeApiMessage(code: string | number, message: string, status: number) {
  const mapped = apiErrorMessages[String(code)]
  if (mapped) return mapped
  if (status === 401) return '邮箱或密码错误，请重新输入。'
  if (status === 403) return '你没有执行此操作的权限。'
  if (status === 404) return '请求的内容不存在。'
  if (status >= 500) return '服务暂时繁忙，请稍后重试。'
  return message
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const token = window.localStorage.getItem('portal-access-token')
  if (token && !headers.has('Authorization')) headers.set('Authorization', `Bearer ${token}`)
  let response: Response
  try {
    response = await fetch(path, { ...init, headers })
  } catch {
    throw new ApiError('无法连接活动服务，请稍后重试', 0, 'NETWORK_ERROR')
  }
  const payload = await response.json().catch(() => ({})) as ApiEnvelope<T> & { error?: string }
  if (!response.ok || (payload.code !== undefined && payload.code !== 0)) {
    const code = payload.code ?? response.status
    const message = payload.message || payload.error || `请求失败（${response.status}）`
    throw new ApiError(localizeApiMessage(code, message, response.status), response.status, code)
  }
  return payload.data
}
