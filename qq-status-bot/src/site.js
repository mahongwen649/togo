function unwrap(body) {
  if (body && typeof body === 'object' && 'code' in body) {
    if (body.code !== 0) throw new Error(body.message || `Site API error: ${body.code}`)
    return body.data
  }
  return body
}

async function requestJson(url, options = {}) {
  const response = await fetch(url, {
    ...options,
    signal: AbortSignal.timeout(30_000),
    headers: { 'Content-Type': 'application/json', ...options.headers },
  })
  const body = await response.json().catch(() => null)
  if (!response.ok) throw new Error(body?.message || `Site HTTP ${response.status}`)
  return unwrap(body)
}

export async function fetchMonitorStatus(config) {
  const login = await loginSite(config)

  const result = await requestJson(`${config.siteBaseUrl}/api/v1/channel-monitors?timezone=Asia%2FShanghai`, {
    headers: {
      Authorization: `Bearer ${login.access_token}`,
      'X-Portal-User-UI': '1',
    },
  })
  return Array.isArray(result?.items) ? result.items : []
}

export async function loginSite(config) {
  const login = await requestJson(`${config.siteBaseUrl}/api/v1/auth/login`, {
    method: 'POST',
    body: JSON.stringify({ identifier: config.siteIdentifier, password: config.sitePassword }),
  })
  if (!login?.access_token) {
    throw new Error(login?.requires_2fa ? 'Bot site account has 2FA enabled' : 'Site login returned no access token')
  }

  return login
}

export { unwrap }
