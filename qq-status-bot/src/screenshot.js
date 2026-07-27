import { chromium } from 'playwright-core'
import { loginSite } from './site.js'

export async function captureStatusPage(config) {
  const login = await loginSite(config)
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
    headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  })

  try {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 1000 },
      deviceScaleFactor: 1,
      locale: 'zh-CN',
      timezoneId: 'Asia/Shanghai',
      colorScheme: 'light',
    })
    await context.addInitScript(({ accessToken, refreshToken, user }) => {
      localStorage.setItem('auth_token', accessToken)
      if (refreshToken) localStorage.setItem('refresh_token', refreshToken)
      if (user) localStorage.setItem('auth_user', JSON.stringify(user))
      localStorage.setItem('channel-status-auto-refresh', JSON.stringify({ enabled: false, interval: 60 }))
      localStorage.setItem('theme', 'light')
    }, {
      accessToken: login.access_token,
      refreshToken: login.refresh_token || '',
      user: login.user || null,
    })

    const page = await context.newPage()
    await page.goto(`${config.siteBaseUrl}/monitor`, { waitUntil: 'networkidle', timeout: 45_000 })
    if (!page.url().includes('/monitor')) throw new Error(`Monitor page redirected to ${page.url()}`)
    await page.locator('main button.group').first().waitFor({ state: 'visible', timeout: 30_000 })
    await page.evaluate(() => document.fonts.ready)
    return await page.locator('main').screenshot({ animations: 'disabled' })
  } finally {
    await browser.close()
  }
}
