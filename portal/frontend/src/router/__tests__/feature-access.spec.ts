import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>
) => Promise<void>

const routerHarness = vi.hoisted(() => ({ guard: null as NavigationGuard | null }))
const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  refreshManagementScope: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isFullAdmin: false,
  isGroupManager: false,
  isCompanyManager: false,
  canAccessAdminArea: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))
const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null,
  fetchPublicSettings: vi.fn(),
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => { routerHarness.guard = guard }),
    afterEach: vi.fn(),
    onError: vi.fn(),
  })),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(), endNavigation: vi.fn(), isLoading: { value: false },
  }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(), cancelPendingPrefetch: vi.fn(), resetPrefetchState: vi.fn(),
  }),
}))

function runGuard(path: string, meta: Record<string, unknown> = {}) {
  if (!routerHarness.guard) throw new Error('router guard was not registered')
  const next = vi.fn()
  const navigation = routerHarness.guard(
    { path, fullPath: path, name: 'Route', params: {}, meta: { requiresAuth: true, ...meta } },
    {},
    next,
  )
  return { navigation, next }
}

describe('reduced Portal route guard', () => {
  beforeAll(async () => { await import('@/router') })

  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    authStore.isFullAdmin = false
    authStore.isGroupManager = false
    authStore.isCompanyManager = false
    authStore.canAccessAdminArea = false
    authStore.isSimpleMode = false
    authStore.refreshManagementScope.mockReset().mockResolvedValue(undefined)
    appStore.fetchPublicSettings.mockReset()
  })

  it.each(['/purchase', '/admin/settings'])('redirects removed route %s for users', async (path) => {
    const { navigation, next } = runGuard(path)
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith('/dashboard')
  })

  it('allows the retained site monitor route', async () => {
    const { navigation, next } = runGuard('/monitor')
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it('redirects removed administrator routes to the management dashboard', async () => {
    authStore.isAdmin = true
    authStore.isFullAdmin = true
    authStore.canAccessAdminArea = true
    const { navigation, next } = runGuard('/admin/ops')
    await navigation
    expect(next).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('allows retained finance management without settings or compliance preloads', async () => {
    authStore.isAdmin = true
    authStore.isFullAdmin = true
    authStore.canAccessAdminArea = true
    const { navigation, next } = runGuard('/admin/finance', { requiresManagement: true })
    await navigation
    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledWith()
  })
})
