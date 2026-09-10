/**
 * Vue Router configuration for Sub2API frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { useRoutePrefetch } from '@/composables/useRoutePrefetch'
import { getSetupStatus } from '@/api/setup'
import { resolveCompletedSetupRedirectPath } from './setupRedirect'
import { resolveDocumentTitle } from './title'

/**
 * Route definitions with lazy loading
 */
const routes: RouteRecordRaw[] = [
  // ==================== Setup Routes ====================
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup'
    }
  },

  // ==================== Public Routes ====================
  {
    path: '/home',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Home'
    }
  },
  {
    path: '/login',
    name: 'Login',
    redirect: (to) => ({
      path: '/home',
      query: { ...to.query, mode: 'login' }
    }),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'home.login'
    }
  },
  {
    path: '/register',
    name: 'Register',
    redirect: (to) => ({
      path: '/home',
      query: { ...to.query, mode: 'register' }
    }),
    meta: {
      requiresAuth: false,
      title: 'Register',
      titleKey: 'auth.createAccount'
    }
  },
  {
    path: '/email-verify',
    name: 'EmailVerify',
    component: () => import('@/views/auth/EmailVerifyView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Verify Email'
    }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/auth/ForgotPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Forgot Password',
      titleKey: 'auth.forgotPasswordTitle'
    }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/auth/ResetPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Reset Password'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
    }
  },
  {
    path: '/legal/:documentId',
    name: 'LegalDocument',
    component: () => import('@/views/public/LegalDocumentView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Legal Document'
    }
  },

  // ==================== User Routes ====================
  {
    path: '/',
    redirect: '/home'
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('@/views/user/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Dashboard',
      titleKey: 'dashboard.title',
      descriptionKey: 'dashboard.welcomeMessage'
    }
  },
  {
    path: '/keys',
    name: 'Keys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'API Keys',
      titleKey: 'keys.title',
      descriptionKey: 'keys.description'
    }
  },
  {
    path: '/usage',
    name: 'Usage',
    component: () => import('@/views/user/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Usage Records',
      titleKey: 'usage.title',
      descriptionKey: 'usage.description'
    }
  },
  {
    path: '/docs',
    name: 'Docs',
    component: () => import('@/views/user/DocsView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Docs',
      titleKey: 'nav.docs'
    }
  },
  {
    path: '/models',
    name: 'ModelMarket',
    component: () => import('@/views/user/ModelMarketView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Model Market',
      titleKey: 'modelMarket.title',
      descriptionKey: 'modelMarket.description'
    }
  },
  {
    path: '/monitor',
    name: 'ChannelStatus',
    component: () => import('@/views/user/ChannelStatusView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Channel Status',
      titleKey: 'nav.channelStatus'
    }
  },
  {
    path: '/developers',
    name: 'Developers',
    component: () => import('@/views/user/DevelopersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Developer Tools',
      titleKey: 'developers.title'
    }
  },
  {
    path: '/redeem',
    name: 'Redeem',
    component: () => import('@/views/user/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Redeem Code',
      titleKey: 'redeem.title',
      descriptionKey: 'redeem.description'
    }
  },
  {
    path: '/affiliate',
    name: 'Affiliate',
    component: () => import('@/views/user/AffiliateView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Affiliate Rebates',
      titleKey: 'affiliate.title',
      descriptionKey: 'affiliate.description'
    }
  },
  {
    path: '/recharge',
    name: 'Recharge',
    component: () => import('@/views/user/RechargeView.vue'),
    meta: { requiresAuth: true, requiresAdmin: false, title: 'Recharge', titleKey: 'payment.portal.recharge' }
  },
  {
    path: '/subscriptions',
    name: 'Subscriptions',
    component: () => import('@/views/user/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Subscriptions',
      titleKey: 'userSubscriptions.title',
      descriptionKey: 'userSubscriptions.description'
    }
  },
  {
    path: '/orders',
    name: 'PaymentOrders',
    component: () => import('@/views/user/OrdersView.vue'),
    meta: { requiresAuth: true, requiresAdmin: false, title: 'Recharge history', titleKey: 'payment.portal.ordersTitle' }
  },
  {
    path: '/payment/result',
    name: 'PaymentResult',
    component: () => import('@/views/user/PaymentResultView.vue'),
    meta: { requiresAuth: true, requiresAdmin: false, title: 'Payment result', titleKey: 'payment.portal.statusTitle' }
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/user/ProfileView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Profile',
      titleKey: 'profile.title',
      descriptionKey: 'profile.description'
    }
  },

  // ==================== Admin Routes ====================
  {
    path: '/admin',
    redirect: '/admin/dashboard'
  },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresManagement: true,
      title: 'Admin Dashboard',
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description'
    }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('@/views/admin/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresFullAdmin: true,
      title: 'User Management',
      titleKey: 'admin.users.title',
      descriptionKey: 'admin.users.description'
    }
  },
  {
    path: '/admin/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Announcements',
      titleKey: 'admin.announcements.title',
      descriptionKey: 'admin.announcements.description'
    }
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresManagement: true,
      title: 'Usage Records',
      titleKey: 'admin.usage.title',
      descriptionKey: 'admin.usage.description'
    }
  },
  {
    path: '/admin/companies',
    name: 'AdminCompanies',
    component: () => import('@/views/admin/CompaniesView.vue'),
    meta: {
      requiresAuth: true,
      requiresManagement: true,
      title: 'Companies',
      titleKey: 'admin.companies.title',
      descriptionKey: 'admin.companies.description'
    }
  },
  {
    path: '/admin/finance',
    name: 'AdminFinance',
    component: () => import('@/views/admin/FinanceView.vue'),
    meta: {
      requiresAuth: true,
      requiresManagement: true,
      requiresCompanyManagement: true,
      title: 'Finance',
      titleKey: 'admin.finance.title',
      descriptionKey: 'admin.finance.description'
    }
  },
  {
    path: '/admin/recharges',
    name: 'AdminRecharges',
    component: () => import('@/views/admin/RechargesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: '充值记录'
    }
  },
  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found'
    }
  }
]

/**
 * Create router instance
 */
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(_to, _from, savedPosition) {
    // Scroll to saved position when using browser back/forward
    if (savedPosition) {
      return savedPosition
    }
    // Scroll to top for new routes
    return { top: 0 }
  }
})

/**
 * Navigation guard: Authentication check
 */
let authInitialized = false

// 初始化导航加载状态和预加载
const navigationLoading = useNavigationLoadingState()
// 延迟初始化预加载，传入 router 实例
let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null
const BACKEND_MODE_ALLOWED_PATHS = ['/home', '/login', '/key-usage', '/setup', '/legal', '/docs']
const BACKEND_MODE_PENDING_AUTH_PATHS = ['/register', '/email-verify']
const REMOVED_PORTAL_PATH_PREFIXES = [
  '/available-channels',
  '/purchase',
  '/custom',
  '/admin/ops',
  '/admin/audit-logs',
  '/admin/groups',
  '/admin/channels',
  '/admin/subscriptions',
  '/admin/accounts',
  '/admin/proxies',
  '/admin/redeem',
  '/admin/promo-codes',
  '/admin/settings',
  '/admin/risk-control',
  '/admin/security-audit',
  '/admin/affiliates',
  '/admin/orders',
]

function isBackendModePublicRouteAllowed(path: string, hasPendingAuthSession: boolean): boolean {
  if (BACKEND_MODE_ALLOWED_PATHS.some((allowedPath) => path === allowedPath || path.startsWith(allowedPath))) {
    return true
  }

  if (hasPendingAuthSession && BACKEND_MODE_PENDING_AUTH_PATHS.some((allowedPath) => path === allowedPath)) {
    return true
  }

  return false
}

router.beforeEach(async (to, _from, next) => {
  // 开始导航加载状态
  navigationLoading.startNavigation()

  const authStore = useAuthStore()

  // Restore auth state from localStorage on first navigation (page refresh)
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  if (REMOVED_PORTAL_PATH_PREFIXES.some((path) => to.path === path || to.path.startsWith(`${path}/`))) {
    next(authStore.canAccessAdminArea ? '/admin/dashboard' : '/dashboard')
    return
  }

  // Set page title
  const appStore = useAppStore()
  // For custom pages, use menu item label as document title
  if (to.name === 'CustomPage') {
    const id = to.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
    if (menuItem?.label) {
      const siteName = appStore.siteName || 'TogoAPI'
      document.title = `${menuItem.label} - ${siteName}`
    } else {
      document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string)
    }
  } else {
    document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string)
  }

  // Check if route requires authentication
  const requiresAuth = to.meta.requiresAuth !== false // Default to true
  const requiresManagement = to.meta.requiresManagement === true || to.meta.requiresAdmin === true
  const requiresGroupManagement = to.meta.requiresGroupManagement === true
  const requiresCompanyManagement = to.meta.requiresCompanyManagement === true
  const requiresFullAdmin = to.meta.requiresFullAdmin === true || (
    to.meta.requiresAdmin === true && to.meta.requiresManagement !== true
  )
  const managementHomePath = '/admin/dashboard'

  if (to.path === '/setup') {
    try {
      const status = await getSetupStatus()
      if (!status.needs_setup) {
        next(resolveCompletedSetupRedirectPath(authStore.isAuthenticated, authStore.canAccessAdminArea))
        return
      }
    } catch {
      // If setup status cannot be determined, keep the setup page reachable.
    }
  }

  // If route doesn't require auth, allow access
  if (!requiresAuth) {
    // If already authenticated and trying to access login/register, redirect to appropriate dashboard
    if (authStore.isAuthenticated && (to.path === '/login' || to.path === '/register')) {
      // In backend mode, non-admin users should NOT be redirected away from login
      // (they are blocked from all protected routes, so redirecting would cause a loop)
      if (appStore.backendModeEnabled && !authStore.canAccessAdminArea) {
        next()
        return
      }
      // Management users go to admin dashboard, regular users go to user dashboard
      next(authStore.canAccessAdminArea ? managementHomePath : '/dashboard')
      return
    }
    // Backend mode: block public pages for unauthenticated users (except login, key-usage, setup)
    if (appStore.backendModeEnabled && !authStore.isAuthenticated) {
      const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
      if (!isAllowed) {
        next('/login')
        return
      }
    }
    next()
    return
  }

  // Route requires authentication
  if (!authStore.isAuthenticated) {
    // Not authenticated, redirect to login
    next({
      path: '/login',
      query: { redirect: to.fullPath } // Save intended destination
    })
    return
  }

  if (typeof authStore.refreshManagementScope === 'function') {
    await authStore.refreshManagementScope().catch(() => {})
  }

  // Check scoped management/full-admin requirements
  if (requiresManagement && !authStore.canAccessAdminArea) {
    // User is authenticated but has no management scope, redirect to user dashboard
    next('/dashboard')
    return
  }

  if (requiresGroupManagement && !authStore.isFullAdmin && !authStore.isGroupManager) {
    next(authStore.canAccessAdminArea ? managementHomePath : '/dashboard')
    return
  }

  if (requiresCompanyManagement && !authStore.isFullAdmin && !authStore.isCompanyManager) {
    next(authStore.canAccessAdminArea ? managementHomePath : '/dashboard')
    return
  }

  if (requiresFullAdmin && !authStore.isFullAdmin) {
    next(authStore.canAccessAdminArea ? managementHomePath : '/dashboard')
    return
  }

  if (to.path === '/dashboard' && authStore.canAccessAdminArea) {
    next(managementHomePath)
    return
  }

  // 简易模式下限制访问某些页面
  if (authStore.isSimpleMode) {
    const restrictedPaths = [
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/redeem',
      '/subscriptions',
      '/redeem'
    ]

    if (restrictedPaths.some((path) => to.path.startsWith(path))) {
      // 简易模式下访问受限页面,重定向到仪表板
      next(authStore.canAccessAdminArea ? managementHomePath : '/dashboard')
      return
    }
  }

  // Backend mode: management users get management access, regular users remain blocked
  if (appStore.backendModeEnabled) {
    if (authStore.isAuthenticated && authStore.canAccessAdminArea) {
      next()
      return
    }
    const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
    if (!isAllowed) {
      next('/login')
      return
    }
  }

  // All checks passed, allow navigation
  next()
})

/**
 * Navigation guard: End loading and trigger prefetch
 */
router.afterEach((to) => {
  // 结束导航加载状态
  navigationLoading.endNavigation()

  // 懒初始化预加载（首次导航时创建，传入 router 实例）
  if (!routePrefetch) {
    routePrefetch = useRoutePrefetch(router)
  }
  // 触发路由预加载（在浏览器空闲时执行）
  routePrefetch.triggerPrefetch(to)
})

/**
 * Navigation guard: Error handling
 * Handles dynamic import failures caused by deployment updates
 */
router.onError((error) => {
  console.error('Router error:', error)

  // Check if this is a dynamic import failure (chunk loading error)
  const isChunkLoadError =
    error.message?.includes('Failed to fetch dynamically imported module') ||
    error.message?.includes('Loading chunk') ||
    error.message?.includes('Loading CSS chunk') ||
    error.name === 'ChunkLoadError'

  if (isChunkLoadError) {
    // Avoid infinite reload loop by checking sessionStorage
    const reloadKey = 'chunk_reload_attempted'
    const lastReload = sessionStorage.getItem(reloadKey)
    const now = Date.now()

    // Allow reload if never attempted or more than 10 seconds ago
    if (!lastReload || now - parseInt(lastReload) > 10000) {
      sessionStorage.setItem(reloadKey, now.toString())
      console.warn('Chunk load error detected, reloading page to fetch latest version...')
      window.location.reload()
    } else {
      console.error('Chunk load error persists after reload. Please clear browser cache.')
    }
  }
})

export default router
