import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..')
const docsViewPath = resolve(repoRoot, 'src/views/user/DocsView.vue')
const routerPath = resolve(repoRoot, 'src/router/index.ts')

describe('DocsView source contract', () => {
  it('contains the required tutorial sections and interactions', () => {
    expect(existsSync(docsViewPath)).toBe(true)
    const source = readFileSync(docsViewPath, 'utf8')

    for (const text of ['快速开始', 'CCSwitch 使用', 'CLI 配置教程', '第三方接入', '常见问题']) {
      expect(source).toContain(text)
    }

    expect(source).toContain('settings.json')
    expect(source).toContain('config.toml')
    expect(source).toContain('auth.json')
    expect(source).toContain('copyCode')
    expect(source).toContain('activeTabs')
  })

  it('lets tab and code block styles apply to render-function child content', () => {
    const source = readFileSync(docsViewPath, 'utf8')

    expect(source).not.toContain('<style scoped>')
    expect(source).toContain('detectCodeLanguage')
    expect(source).toContain('docs-code-lang')
    expect(source).toContain('highlightCode')
  })

  it('separates CLI configuration by target tool', () => {
    const source = readFileSync(docsViewPath, 'utf8')

    expect(source).toContain("group=\"cliTool\"")
    expect(source).toContain("activeTabs.cliTool === 'Claude Code'")
    expect(source).toContain("activeTabs.cliTool === 'Codex'")
    expect(source).toContain('Claude Code 配置内容')
    expect(source).toContain('Codex 配置内容')
    expect(source).toContain('第三方通用请求头')
  })

  it('does not embed competitor or secret strings', () => {
    if (!existsSync(docsViewPath)) return
    const source = readFileSync(docsViewPath, 'utf8')

    const competitorDomain = [
      String.fromCharCode(116, 111, 107, 101, 110),
      String.fromCharCode(100, 105, 97, 108, 111, 103, 117, 101, 100, 117, 105),
      String.fromCharCode(99, 111, 109)
    ].join('.')
    const competitorBrand = String.fromCharCode(23567, 30333) + 'Code'

    expect(source).not.toContain(competitorDomain)
    expect(source).not.toContain(competitorBrand)
    expect(source).not.toMatch(/sk-[A-Za-z0-9]{12,}/)
  })

	it('derives every documented API endpoint from the configured public gateway', () => {
    const source = readFileSync(docsViewPath, 'utf8')

		expect(source).toContain('const apiBaseUrl = getPublicGatewayBaseURL()')
    expect(source).toContain('Base URL: ${apiBaseUrl}')
    expect(source).toContain('base_url = "${apiBaseUrl}/v1"')
    expect(source).toContain('curl ${apiBaseUrl}/v1/chat/completions')
    expect(source).not.toContain('https://code.wsurprise.com')
  })

  it('cache-busts local docs screenshots after sanitization updates', () => {
    const source = readFileSync(docsViewPath, 'utf8')

    expect(source).toContain('docsAssetVersion')
    expect(source).toContain('versionedSrc')
    expect(source).toContain('?v=')
  })

  it('prevents inline code chip styles from leaking into code blocks', () => {
    const source = readFileSync(docsViewPath, 'utf8')

    expect(source).toContain('.docs-code-card code {')
    expect(source).toContain('background: transparent;')
    expect(source).toContain('.docs-code-card code span {')
  })
})

describe('docs route contract', () => {
  it('registers the /docs route as an authenticated user page', () => {
    const routerSource = readFileSync(routerPath, 'utf8')

    expect(routerSource).toContain("path: '/docs'")
    expect(routerSource).toContain("name: 'Docs'")
    expect(routerSource).toContain("component: () => import('@/views/user/DocsView.vue')")
  })
})
