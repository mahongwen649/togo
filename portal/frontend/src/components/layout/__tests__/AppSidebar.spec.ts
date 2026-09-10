import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar mobile drawer', () => {
  it('uses a compact width and closes from the collapse action', () => {
    expect(componentSource).toContain("isCompactNavigation ? 'w-[min(56vw,11.5rem)]' : 'w-64'")
    expect(componentSource).toContain('const { isCompactNavigation } = useDeviceMode()')
    expect(componentSource).toContain('closeMobile()')
  })
})

describe('AppSidebar header styles', () => {
  it('does not show a version badge in the brand area', () => {
    expect(componentSource).not.toContain('<VersionBadge')
    expect(componentSource).not.toContain("import VersionBadge from '@/components/common/VersionBadge.vue'")
    expect(componentSource).not.toContain('const siteVersion = computed(() => appStore.siteVersion)')
  })
})

describe('AppSidebar Portal entries', () => {
  it('links the gift campaign outside the Portal router', () => {
    expect(componentSource).toContain("path: 'gift:campaign', href: '/gift'")
    expect(componentSource).toContain("item.href ? 'a'")
    expect(componentSource).toContain(':href="item.href"')
  })

  it('keeps redeem and site monitor while removing retired user domains', () => {
    expect(componentSource).toContain("{ path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true }")
    expect(componentSource).toContain("{ path: '/subscriptions', label: t('nav.mySubscriptions'), icon: CreditCardIcon, hideInSimpleMode: true }")
    expect(componentSource).toContain("{ path: '/models', label: t('nav.modelMarket')")
    expect(componentSource).toContain("path: '/affiliate'")
    expect(componentSource).toContain('affiliate_enabled')
    expect(componentSource).toContain("{ path: '/monitor', label: t('nav.siteMonitor')")
    expect(componentSource).toContain('channel_monitor_enabled !== false')
    expect(componentSource).not.toContain("{ path: '/purchase'")
    expect(componentSource).not.toContain("{ path: '/orders'")
  })

  it('exposes redeem to regular users outside simple mode', () => {
    expect(componentSource).not.toContain('fullAdminOnly')
  })

  it('renders external SSO actions as buttons instead of fake current-page links', () => {
    expect(componentSource).toContain(":is=\"item.href ? 'a' : isActionPath(item.path) ? 'button' : 'router-link'\"")
    expect(componentSource).not.toContain("isActionPath(item.path) ? route.fullPath : item.path")
  })

  it('keeps only the image-generation SSO action in the personal menu', () => {
    expect(componentSource).toContain("path: 'imgtool:sso'")
    expect(componentSource).not.toContain("path: 'ppt:sso'")
    expect(componentSource).not.toContain("path: '/developers'")
    expect(componentSource).not.toContain('createPPTSSOURL')
  })

  it('does not expose retired administrator domains', () => {
    expect(componentSource).not.toContain("{ path: '/admin/ops'")
    expect(componentSource).not.toContain("{ path: '/admin/settings'")
    expect(componentSource).not.toContain("{ path: '/admin/proxies'")
  })
})

describe('AppSidebar limited admin entries', () => {
  it('does not show company or finance entries', () => {
    expect(componentSource).not.toContain("path: '/admin/companies'")
    expect(componentSource).not.toContain("path: '/admin/finance'")
  })
})
