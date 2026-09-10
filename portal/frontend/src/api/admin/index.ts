/**
 * Admin API barrel export
 * Centralized exports for all admin API modules
 */

import dashboardAPI from './dashboard'
import usersAPI from './users'
import groupsAPI from './groups'
import redeemAPI from './redeem'
import announcementsAPI from './announcements'
import systemAPI from './system'
import usageAPI from './usage'
import userAttributesAPI from './userAttributes'
import apiKeysAPI from './apiKeys'
import financeAPI from './finance'
import companiesAPI from './companies'
import scopeAPI from './scope'
import rechargesAPI from './recharges'

/**
 * Unified admin API object for convenient access
 */
export const adminAPI = {
  dashboard: dashboardAPI,
  users: usersAPI,
  groups: groupsAPI,
  redeem: redeemAPI,
  announcements: announcementsAPI,
  system: systemAPI,
  usage: usageAPI,
  userAttributes: userAttributesAPI,
  apiKeys: apiKeysAPI,
  finance: financeAPI,
  companies: companiesAPI,
  scope: scopeAPI,
  recharges: rechargesAPI
}

export {
  dashboardAPI,
  usersAPI,
  groupsAPI,
  redeemAPI,
  announcementsAPI,
  systemAPI,
  usageAPI,
  userAttributesAPI,
  apiKeysAPI,
  financeAPI,
  companiesAPI,
  scopeAPI,
  rechargesAPI
}

export default adminAPI

// Re-export types used by components
export type { BalanceHistoryItem } from './users'
