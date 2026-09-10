/**
 * Type definitions for Vue Router meta fields
 * Extends the RouteMeta interface with custom properties
 */

import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /**
     * Whether this route requires authentication
     * @default true
     */
    requiresAuth?: boolean

    /**
     * Whether this route requires admin role
     * @default false
     */
    requiresAdmin?: boolean

    /**
     * Whether this route requires full administrator access.
     * @default false
     */
    requiresFullAdmin?: boolean

    /**
     * Whether this route allows full administrators or scoped managers.
     * @default false
     */
    requiresManagement?: boolean

    /**
     * Whether this route requires full administrators or scoped group managers.
     * Company managers are management users, but they should not enter legacy
     * group-member user management pages.
     * @default false
     */
    requiresGroupManagement?: boolean

    /**
     * Whether this route requires full administrators or scoped company managers.
     * Group managers are management users, but they should not enter company
     * finance/company-member workflows.
     * @default false
     */
    requiresCompanyManagement?: boolean

    /**
     * Page title for this route
     */
    title?: string

    /**
     * Optional breadcrumb items for navigation
     */
    breadcrumbs?: Array<{
      label: string
      to?: string
    }>

    /**
     * Icon name for this route (for sidebar navigation)
     */
    icon?: string

    /**
     * Whether to hide this route from navigation menu
     * @default false
     */
    hideInMenu?: boolean

    /**
     * Whether this route requires internal payment system to be enabled
     * @default false
     */
    requiresPayment?: boolean

    /**
     * 是否要求风控中心功能开关已启用
     * @default false
     */
    requiresRiskControl?: boolean

    /**
     * i18n key for the page title
     */
    titleKey?: string

    /**
     * i18n key for the page description
     */
    descriptionKey?: string
  }
}
