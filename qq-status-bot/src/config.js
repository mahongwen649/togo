function required(env, name) {
  const value = String(env[name] || '').trim()
  if (!value) throw new Error(`Missing required environment variable: ${name}`)
  return value
}

export function loadConfig(env = process.env) {
  const groupIds = new Set(
    String(env.ALLOWED_GROUP_IDS || '')
      .split(',')
      .map(value => value.trim())
      .filter(Boolean),
  )
  if (groupIds.size === 0) throw new Error('ALLOWED_GROUP_IDS must contain at least one QQ group ID')

  return {
    onebotWsUrl: required(env, 'ONEBOT_WS_URL'),
    onebotToken: required(env, 'ONEBOT_TOKEN'),
    siteBaseUrl: required(env, 'SITE_BASE_URL').replace(/\/+$/, ''),
    siteIdentifier: required(env, 'SITE_IDENTIFIER'),
    sitePassword: required(env, 'SITE_PASSWORD'),
    allowedGroupIds: groupIds,
    commands: new Set(
      String(env.COMMANDS || '状态检查,站点状态')
        .split(',')
        .map(value => value.trim())
        .filter(Boolean),
    ),
    cooldownMs: Math.max(5, Number(env.COOLDOWN_SECONDS || 30)) * 1000,
  }
}
