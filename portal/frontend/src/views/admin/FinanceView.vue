<template>
  <AppLayout>
    <div class="space-y-5">
      <div :class="[isMobileH5 ? 'grid grid-cols-2 gap-2' : 'flex flex-wrap items-center gap-3', 'rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800']">
        <div :class="isMobileH5 ? 'col-span-2 min-w-0' : ''">
          <DateRangePicker
            v-model:start-date="startDate"
            v-model:end-date="endDate"
            @change="reload"
          />
        </div>
        <div ref="companyFilterRef" :class="['relative', isMobileH5 ? 'min-w-0' : 'w-full sm:w-80']">
          <button
            type="button"
            class="select-trigger"
            :class="companyFilterOpen && 'select-trigger-open'"
            data-test="finance-company-filter-trigger"
            :aria-expanded="companyFilterOpen"
            aria-haspopup="listbox"
            @click.stop="toggleCompanyFilter"
          >
            <span class="select-value">{{ companyFilterLabel }}</span>
            <Icon
              name="chevronDown"
              size="md"
              :class="['flex-shrink-0 text-gray-400 transition-transform duration-200', companyFilterOpen && 'rotate-180']"
            />
          </button>
          <div
            v-if="companyFilterOpen"
            class="absolute left-0 top-full z-50 mt-1 w-full overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg shadow-black/10 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30"
            role="listbox"
            aria-multiselectable="true"
            @click.stop
          >
            <div class="flex items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-dark-700">
              <Icon name="search" size="sm" class="text-gray-400" />
              <input
                v-model="companySearchQuery"
                type="text"
                :placeholder="t('common.searchPlaceholder')"
                class="flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400"
              />
            </div>
            <div class="flex items-center justify-between border-b border-gray-100 px-3 py-2 text-xs dark:border-dark-700">
              <button type="button" class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="() => selectAllCompanies()">
                {{ t('admin.finance.selectAllCompanies') }}
              </button>
              <button type="button" class="font-medium text-gray-500 hover:text-gray-700 dark:text-dark-300 dark:hover:text-white" @click="clearCompanies">
                {{ t('admin.finance.clearCompanies') }}
              </button>
            </div>
            <div class="max-h-80 overflow-y-auto py-1">
              <label
                class="flex cursor-pointer items-center justify-between gap-3 px-4 py-2.5 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                :class="areAllCompaniesSelected && 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'"
              >
                <span class="truncate">{{ t('admin.finance.allCompanies') }}</span>
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
                  :checked="areAllCompaniesSelected"
                  @change="() => selectAllCompanies()"
                />
              </label>
              <label
                v-for="company in filteredCompanies"
                :key="company.id"
                class="flex cursor-pointer items-center justify-between gap-3 px-4 py-2.5 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                :class="selectedCompanyIds.includes(company.id) && 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'"
              >
                <span class="min-w-0 flex-1 truncate">{{ company.name }}</span>
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
                  :checked="selectedCompanyIds.includes(company.id)"
                  :data-test="`finance-company-option-${company.id}`"
                  @change="toggleCompany(company.id, ($event.target as HTMLInputElement).checked)"
                />
              </label>
              <div v-if="filteredCompanies.length === 0" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                {{ t('common.noOptionsFound') }}
              </div>
            </div>
          </div>
        </div>
        <select
          v-model="selectedPlatform"
          :class="['select-trigger', isMobileH5 ? 'min-w-0' : 'w-full sm:w-48']"
          data-test="finance-platform-filter"
          @change="reload"
        >
          <option v-for="option in platformOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
        <div ref="groupFilterRef" :class="['relative', isMobileH5 ? 'min-w-0' : 'w-full sm:w-80']">
          <button
            type="button"
            class="select-trigger"
            :class="groupFilterOpen && 'select-trigger-open'"
            data-test="finance-group-filter-trigger"
            :aria-expanded="groupFilterOpen"
            aria-haspopup="listbox"
            @click.stop="toggleGroupFilter"
          >
            <span class="select-value">{{ groupFilterLabel }}</span>
            <Icon
              name="chevronDown"
              size="md"
              :class="['flex-shrink-0 text-gray-400 transition-transform duration-200', groupFilterOpen && 'rotate-180']"
            />
          </button>
          <div
            v-if="groupFilterOpen"
            class="absolute left-0 top-full z-50 mt-1 w-full overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg shadow-black/10 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30"
            role="listbox"
            aria-multiselectable="true"
            @click.stop
          >
            <div class="flex items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-dark-700">
              <Icon name="search" size="sm" class="text-gray-400" />
              <input
                v-model="groupSearchQuery"
                type="text"
                :placeholder="t('common.searchPlaceholder')"
                class="flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400"
              />
            </div>
            <div class="flex items-center justify-between border-b border-gray-100 px-3 py-2 text-xs dark:border-dark-700">
              <button type="button" class="font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="() => selectAllGroups()">
                {{ t('admin.finance.selectAllGroups') }}
              </button>
              <button type="button" class="font-medium text-gray-500 hover:text-gray-700 dark:text-dark-300 dark:hover:text-white" @click="clearGroups">
                {{ t('admin.finance.clearGroups') }}
              </button>
            </div>
            <div class="max-h-80 overflow-y-auto py-1">
              <label
                class="flex cursor-pointer items-center justify-between gap-3 px-4 py-2.5 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                :class="areAllGroupsSelected && 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'"
              >
                <span class="truncate">{{ t('admin.finance.allGroups') }}</span>
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
                  :checked="areAllGroupsSelected"
                  data-test="finance-all-groups-option"
                  @change="() => selectAllGroups()"
                />
              </label>
              <label
                v-for="group in filteredGroups"
                :key="group.id"
                class="flex cursor-pointer items-center justify-between gap-3 px-4 py-2.5 text-sm text-gray-700 transition-colors hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700"
                :class="selectedGroupIds.includes(group.id) && 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'"
              >
                <span class="min-w-0 flex-1 truncate">{{ group.name }}</span>
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-500 focus:ring-primary-500 dark:border-dark-500"
                  :checked="selectedGroupIds.includes(group.id)"
                  :data-test="`finance-group-option-${group.id}`"
                  @change="toggleGroup(group.id, ($event.target as HTMLInputElement).checked)"
                />
              </label>
              <div v-if="filteredGroups.length === 0" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">
                {{ t('common.noOptionsFound') }}
              </div>
            </div>
          </div>
        </div>
        <button :class="['btn btn-secondary', { 'w-full': isMobileH5 }]" :disabled="loading" @click="reload">
          {{ t('common.refresh') }}
        </button>
        <button :class="['btn btn-primary', isMobileH5 ? 'w-full' : 'ml-auto']" :disabled="exporting" @click="exportReport">
          <span v-if="exporting" class="inline-block h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
          {{ t('admin.finance.export') }}
        </button>
      </div>

      <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
        <div class="card p-4" data-test="finance-stat-card">
          <div class="flex items-center gap-3" data-test="finance-stat-content">
            <div class="rounded-lg bg-green-100 p-2 dark:bg-green-900/30" data-test="finance-stat-icon">
              <Icon name="dollar" size="md" class="text-green-600 dark:text-green-400" :stroke-width="2" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.totalCharge') }}</p>
              <p class="text-xl font-bold text-green-600 dark:text-green-400" data-test="finance-stat-value">{{ formatCurrency(totalCharge) }}</p>
            </div>
          </div>
        </div>
        <div class="card p-4" data-test="finance-stat-card">
          <div class="flex items-center gap-3" data-test="finance-stat-content">
            <div class="rounded-lg bg-blue-100 p-2 dark:bg-blue-900/30" data-test="finance-stat-icon">
              <Icon name="chart" size="md" class="text-blue-600 dark:text-blue-400" :stroke-width="2" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.requestCount') }}</p>
              <p class="text-xl font-bold text-gray-900 dark:text-white" data-test="finance-stat-value">{{ formatInteger(totalRequests) }}</p>
            </div>
          </div>
        </div>
        <div class="card p-4" data-test="finance-stat-card">
          <div class="flex items-center gap-3" data-test="finance-stat-content">
            <div class="rounded-lg bg-emerald-100 p-2 dark:bg-emerald-900/30" data-test="finance-stat-icon">
              <Icon name="users" size="md" class="text-emerald-600 dark:text-emerald-400" :stroke-width="2" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.memberCount') }}</p>
              <p class="text-xl font-bold text-gray-900 dark:text-white" data-test="finance-stat-value">{{ formatInteger(totalMembers) }}</p>
            </div>
          </div>
        </div>
        <div class="card p-4" data-test="finance-stat-card">
          <div class="flex items-center gap-3" data-test="finance-stat-content">
            <div class="rounded-lg bg-amber-100 p-2 dark:bg-amber-900/30" data-test="finance-stat-icon">
              <Icon name="cube" size="md" class="text-amber-600 dark:text-amber-400" :stroke-width="2" />
            </div>
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.finance.tokenCount') }}</p>
              <p class="text-xl font-bold text-gray-900 dark:text-white" data-test="finance-stat-value">{{ formatInteger(totalTokens) }}</p>
            </div>
          </div>
        </div>
      </div>

      <section class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.companySummary') }}</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-700/60 dark:text-dark-300">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.finance.company') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.finance.platform') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.memberCount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.requestCount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.inputTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.outputTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.cacheCreationTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.cacheReadTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.imageOutputTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.actualChargedAmount') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loading">
                <td colspan="10" class="px-4 py-8 text-center text-gray-500">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="summaryRows.length === 0">
                <td colspan="10" class="px-4 py-8 text-center text-gray-500">{{ t('common.noData') }}</td>
              </tr>
              <tr v-for="row in summaryRows" v-else :key="summaryRowKey(row)" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
                <td class="whitespace-nowrap px-4 py-3 font-medium text-gray-900 dark:text-white">{{ displayCompanyName(row) }}</td>
                <td class="whitespace-nowrap px-4 py-3">{{ formatPlatform(row.platform) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.member_count) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.request_count) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.input_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.output_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.cache_creation_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.cache_read_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.image_output_tokens) }}</td>
                <td class="px-4 py-3 text-right font-semibold text-green-600 dark:text-green-400">{{ formatCurrency(row.actual_charged_amount) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.finance.memberSummary') }}</h2>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-700/60 dark:text-dark-300">
              <tr>
                <th class="px-4 py-3 text-left">{{ t('admin.finance.company') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.finance.platform') }}</th>
                <th class="px-4 py-3 text-left">{{ t('admin.finance.user') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.requestCount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.inputTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.outputTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.cacheCreationTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.cacheReadTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.imageOutputTokens') }}</th>
                <th class="px-4 py-3 text-right">{{ t('admin.finance.actualChargedAmount') }}</th>
                <th class="px-4 py-3 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="membersLoading">
                <td colspan="11" class="px-4 py-8 text-center text-gray-500">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="memberRows.length === 0">
                <td colspan="11" class="px-4 py-8 text-center text-gray-500">{{ t('common.noData') }}</td>
              </tr>
              <tr v-for="row in memberRows" v-else :key="memberRowKey(row)" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
                <td class="whitespace-nowrap px-4 py-3 font-medium text-gray-900 dark:text-white">{{ displayCompanyName(row) }}</td>
                <td class="whitespace-nowrap px-4 py-3">{{ formatPlatform(row.platform) }}</td>
                <td class="whitespace-nowrap px-4 py-3">
                  <span class="font-medium text-gray-900 dark:text-white">{{ displayUserName(row) }}</span>
                  <span class="ml-2 text-xs text-gray-500">#{{ row.user_id }}</span>
                </td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.request_count) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.input_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.output_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.cache_creation_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.cache_read_tokens) }}</td>
                <td class="px-4 py-3 text-right">{{ formatInteger(row.image_output_tokens) }}</td>
                <td class="px-4 py-3 text-right font-semibold text-green-600 dark:text-green-400">{{ formatCurrency(row.actual_charged_amount) }}</td>
                <td class="px-4 py-3 text-right">
                  <button type="button" class="btn btn-secondary btn-sm inline-flex items-center gap-1.5" @click="openMemberDetail(row)">
                    <Icon name="eye" size="sm" />
                    {{ t('admin.finance.detail') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="memberPagination.total > 0"
          :page="memberPagination.page"
          :total="memberPagination.total"
          :page-size="memberPagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>

      <div v-if="detailOpen" class="fixed inset-0 z-50 flex justify-end bg-black/30" @click.self="closeMemberDetail">
        <aside class="flex h-full w-full max-w-5xl flex-col bg-white shadow-xl dark:bg-dark-800">
          <div class="flex items-start justify-between gap-4 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('admin.finance.memberDetailTitle', { user: selectedMember ? displayUserName(selectedMember) : '-' }) }}
              </h2>
              <p v-if="selectedMember" class="mt-1 text-sm text-gray-500 dark:text-dark-400">#{{ selectedMember.user_id }} · {{ displayCompanyName(selectedMember) }}</p>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-primary inline-flex items-center gap-1.5" :disabled="detailExporting || !selectedMember" @click="exportMemberDetail">
                <Icon name="download" size="sm" />
                {{ t('admin.finance.exportMemberDetail') }}
              </button>
              <button type="button" class="rounded-lg p-2 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-white" @click="closeMemberDetail">
                <Icon name="x" size="md" />
              </button>
            </div>
          </div>
          <div class="flex-1 overflow-auto p-5">
            <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-700/60 dark:text-dark-300">
                  <tr>
                    <th class="px-4 py-3 text-left">{{ t('admin.finance.company') }}</th>
                    <th class="px-4 py-3 text-left">{{ t('admin.finance.platform') }}</th>
                    <th class="px-4 py-3 text-left">{{ t('admin.finance.group') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.requestCount') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.inputTokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.outputTokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.cacheCreationTokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.cacheReadTokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.imageOutputTokens') }}</th>
                    <th class="px-4 py-3 text-right">{{ t('admin.finance.actualChargedAmount') }}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                  <tr v-if="detailLoading">
                    <td colspan="10" class="px-4 py-8 text-center text-gray-500">{{ t('common.loading') }}</td>
                  </tr>
                  <tr v-else-if="detailRows.length === 0">
                    <td colspan="10" class="px-4 py-8 text-center text-gray-500">{{ t('common.noData') }}</td>
                  </tr>
                  <tr v-for="row in detailRows" v-else :key="detailRowKey(row)" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
                    <td class="whitespace-nowrap px-4 py-3 font-medium text-gray-900 dark:text-white">{{ displayCompanyName(row) }}</td>
                    <td class="whitespace-nowrap px-4 py-3">{{ formatPlatform(row.platform) }}</td>
                    <td class="whitespace-nowrap px-4 py-3">{{ row.group_name || '-' }}</td>
                    <td class="px-4 py-3 text-right">{{ formatInteger(row.request_count) }}</td>
                    <td class="px-4 py-3 text-right">{{ formatInteger(row.input_tokens) }}</td>
                    <td class="px-4 py-3 text-right">{{ formatInteger(row.output_tokens) }}</td>
                    <td class="px-4 py-3 text-right">{{ formatInteger(row.cache_creation_tokens) }}</td>
                    <td class="px-4 py-3 text-right">{{ formatInteger(row.cache_read_tokens) }}</td>
                    <td class="px-4 py-3 text-right">{{ formatInteger(row.image_output_tokens) }}</td>
                    <td class="px-4 py-3 text-right font-semibold text-green-600 dark:text-green-400">{{ formatCurrency(row.actual_charged_amount) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import financeAPI from '@/api/admin/finance'
import companiesAPI from '@/api/admin/companies'
import { groupsAPI } from '@/api/admin/groups'
import type { FinanceMemberRow, FinanceSummaryRow } from '@/api/admin/finance'
import type { AdminCompany } from '@/api/admin/companies'
import type { AdminGroup } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useDeviceMode } from '@/composables/useDeviceMode'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { isMobileH5 } = useDeviceMode()

const startDate = ref(formatMonthStart())
const endDate = ref(formatDateOffset(0))
const selectedCompanyIds = ref<number[]>([])
const selectedGroupIds = ref<number[]>([])
const selectedPlatform = ref('')
const companyFilterOpen = ref(false)
const groupFilterOpen = ref(false)
const companySearchQuery = ref('')
const groupSearchQuery = ref('')
const companyFilterRef = ref<HTMLElement | null>(null)
const groupFilterRef = ref<HTMLElement | null>(null)
const companies = ref<AdminCompany[]>([])
const groups = ref<AdminGroup[]>([])
const summaryRows = ref<FinanceSummaryRow[]>([])
const memberRows = ref<FinanceMemberRow[]>([])
const detailRows = ref<FinanceMemberRow[]>([])
const selectedMember = ref<FinanceMemberRow | null>(null)
const loading = ref(false)
const membersLoading = ref(false)
const exporting = ref(false)
const detailLoading = ref(false)
const detailExporting = ref(false)
const detailOpen = ref(false)

const memberPagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
  pages: 1
})

const platformOptions = computed(() => [
  { value: '', label: t('admin.finance.allPlatforms') },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
])

const filteredCompanies = computed(() => {
  const query = companySearchQuery.value.trim().toLowerCase()
  if (!query) return companies.value
  return companies.value.filter((company) =>
    company.name.toLowerCase().includes(query)
  )
})

const filteredGroups = computed(() => {
  const query = groupSearchQuery.value.trim().toLowerCase()
  if (!query) return groups.value
  return groups.value.filter((group) =>
    group.name.toLowerCase().includes(query) || group.description?.toLowerCase().includes(query)
  )
})

const areAllCompaniesSelected = computed(() =>
  companies.value.length > 0 && selectedCompanyIds.value.length === companies.value.length
)

const areAllGroupsSelected = computed(() =>
  selectedGroupIds.value.length === 0 || (groups.value.length > 0 && selectedGroupIds.value.length === groups.value.length)
)

const companyFilterLabel = computed(() => {
  if (selectedCompanyIds.value.length === 0) return t('admin.finance.noCompaniesSelected')
  if (areAllCompaniesSelected.value) return t('admin.finance.allCompanies')
  if (selectedCompanyIds.value.length === 1) {
    return companies.value.find((company) => company.id === selectedCompanyIds.value[0])?.name ?? t('admin.finance.selectedCompanies', { count: 1 })
  }
  return t('admin.finance.selectedCompanies', { count: selectedCompanyIds.value.length })
})

const groupFilterLabel = computed(() => {
  if (areAllGroupsSelected.value) return t('admin.finance.allGroups')
  if (selectedGroupIds.value.length === 1) {
    return groups.value.find((group) => group.id === selectedGroupIds.value[0])?.name ?? t('admin.finance.selectedGroups', { count: 1 })
  }
  return t('admin.finance.selectedGroups', { count: selectedGroupIds.value.length })
})

const totalCharge = computed(() =>
  summaryRows.value.reduce((sum, row) => sum + row.actual_charged_amount, 0)
)
const totalRequests = computed(() =>
  summaryRows.value.reduce((sum, row) => sum + row.request_count, 0)
)
const totalMembers = computed(() =>
  summaryRows.value.reduce((sum, row) => sum + row.member_count, 0)
)
const totalTokens = computed(() =>
  summaryRows.value.reduce((sum, row) => sum + row.input_tokens + row.output_tokens + row.cache_creation_tokens + row.cache_read_tokens + row.image_output_tokens, 0)
)

function formatDateOffset(days: number): string {
  const date = new Date()
  date.setDate(date.getDate() + days)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatMonthStart(): string {
  const date = new Date()
  date.setDate(1)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  return `${year}-${month}-01`
}

function requestFilters() {
  const companyIDs = selectedCompanyIds.value
  const groupIDs = areAllGroupsSelected.value ? [] : selectedGroupIds.value
  return {
    start_date: startDate.value || undefined,
    end_date: endDate.value || undefined,
    company_id: companyIDs.length === 1 ? companyIDs[0] : undefined,
    company_ids: companyIDs.length > 0 ? [...companyIDs] : undefined,
    platform: selectedPlatform.value || undefined,
    group_id: groupIDs.length === 1 ? groupIDs[0] : undefined,
    group_ids: groupIDs.length > 0 ? [...groupIDs] : undefined
  }
}

function formatInteger(value: number): string {
  return new Intl.NumberFormat().format(value || 0)
}

function formatCurrency(value: number): string {
  return `¥${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 4,
    maximumFractionDigits: 6
  }).format(value || 0)}`
}

async function loadGroups(): Promise<void> {
  if (authStore.isCompanyManager && !authStore.isFullAdmin) {
    groups.value = []
    selectAllGroups(false)
    return
  }

  try {
    groups.value = await groupsAPI.getAll()
    selectAllGroups(false)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.failedToLoadGroups'))
  }
}

async function loadCompanies(): Promise<void> {
  try {
    companies.value = await companiesAPI.getAll()
    selectAllCompanies(false)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.failedToLoadCompanies'))
  }
}

async function loadSummary(): Promise<void> {
  if (selectedCompanyIds.value.length === 0) {
    summaryRows.value = []
    return
  }
  loading.value = true
  try {
    summaryRows.value = await financeAPI.summary(requestFilters())
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.failedToLoad'))
  } finally {
    loading.value = false
  }
}

async function loadMembers(): Promise<void> {
  if (selectedCompanyIds.value.length === 0) {
    memberRows.value = []
    memberPagination.total = 0
    memberPagination.pages = 1
    return
  }
  membersLoading.value = true
  try {
    const response = await financeAPI.members({
      ...requestFilters(),
      page: memberPagination.page,
      page_size: memberPagination.page_size
    })
    memberRows.value = response.items
    memberPagination.total = response.total
    memberPagination.pages = response.pages
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.failedToLoad'))
  } finally {
    membersLoading.value = false
  }
}

function reload(): void {
  memberPagination.page = 1
  if (detailOpen.value) closeMemberDetail()
  void Promise.all([loadSummary(), loadMembers()])
}

function handleGroupChange(): void {
  reload()
}

function handleCompanyChange(): void {
  reload()
}

function toggleCompanyFilter(): void {
  companyFilterOpen.value = !companyFilterOpen.value
  if (companyFilterOpen.value) groupFilterOpen.value = false
}

function toggleGroupFilter(): void {
  groupFilterOpen.value = !groupFilterOpen.value
  if (groupFilterOpen.value) companyFilterOpen.value = false
}

function toggleCompany(companyId: number, checked: boolean): void {
  selectedCompanyIds.value = checked
    ? Array.from(new Set([...selectedCompanyIds.value, companyId]))
    : selectedCompanyIds.value.filter((id) => id !== companyId)
  handleCompanyChange()
}

function selectAllCompanies(shouldReload = true): void {
  selectedCompanyIds.value = companies.value.map((company) => company.id)
  if (shouldReload) handleCompanyChange()
}

function clearCompanies(): void {
  selectedCompanyIds.value = []
  handleCompanyChange()
}

function toggleGroup(groupId: number, checked: boolean): void {
  selectedGroupIds.value = checked
    ? Array.from(new Set([...selectedGroupIds.value, groupId]))
    : selectedGroupIds.value.filter((id) => id !== groupId)
  handleGroupChange()
}

function selectAllGroups(shouldReload = true): void {
  selectedGroupIds.value = groups.value.map((group) => group.id)
  if (shouldReload) handleGroupChange()
}

function clearGroups(): void {
  selectedGroupIds.value = []
  handleGroupChange()
}

function handleClickOutside(event: MouseEvent): void {
  if (!companyFilterRef.value?.contains(event.target as Node)) {
    companyFilterOpen.value = false
  }
  if (!groupFilterRef.value?.contains(event.target as Node)) {
    groupFilterOpen.value = false
  }
}

function handlePageChange(page: number): void {
  memberPagination.page = Math.max(1, Math.min(page, memberPagination.pages || 1))
  void loadMembers()
}

function handlePageSizeChange(pageSize: number): void {
  memberPagination.page_size = pageSize
  memberPagination.page = 1
  void loadMembers()
}

async function exportReport(): Promise<void> {
  exporting.value = true
  try {
    const blob = await financeAPI.exportCsv(requestFilters())
    saveAs(blob, `finance-report-${startDate.value}-${endDate.value}.csv`)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.exportFailed'))
  } finally {
    exporting.value = false
  }
}

async function openMemberDetail(row: FinanceMemberRow): Promise<void> {
  selectedMember.value = row
  detailOpen.value = true
  detailRows.value = []
  detailLoading.value = true
  try {
    detailRows.value = await financeAPI.memberBreakdown(row.user_id, requestFilters())
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.failedToLoadMemberDetail'))
  } finally {
    detailLoading.value = false
  }
}

function closeMemberDetail(): void {
  detailOpen.value = false
  selectedMember.value = null
  detailRows.value = []
}

async function exportMemberDetail(): Promise<void> {
  if (!selectedMember.value) return
  detailExporting.value = true
  try {
    const blob = await financeAPI.exportMemberCsv(selectedMember.value.user_id, requestFilters())
    saveAs(blob, `finance-member-detail-${safeFilePart(displayUserName(selectedMember.value))}-${selectedMember.value.user_id}-${startDate.value}-${endDate.value}.csv`)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || error.message || t('admin.finance.exportMemberDetailFailed'))
  } finally {
    detailExporting.value = false
  }
}

function displayCompanyName(row: Pick<FinanceSummaryRow, 'company_name' | 'group_name'>): string {
  return row.company_name || row.group_name || '-'
}

function displayUserName(row: Pick<FinanceMemberRow, 'username' | 'user_email' | 'user_id' | 'user_deleted_at'>): string {
  const rawName = row.username?.trim() || row.user_email?.trim() || ''
  const idText = String(row.user_id)
  const name = rawName && rawName !== idText ? rawName : ''
  if (row.user_deleted_at) {
    return name ? t('admin.finance.deletedUserName', { name }) : t('admin.finance.deletedUnknownUserName')
  }
  return name || idText
}

function safeFilePart(value: string): string {
  return value.replace(/[\\/:*?"<>|]/g, '_').replace(/\s+/g, '_').slice(0, 80) || 'user'
}

function formatPlatform(platform?: string): string {
  switch ((platform || '').toLowerCase()) {
    case 'openai':
      return 'OpenAI'
    case 'anthropic':
      return 'Claude'
    case 'gemini':
      return 'Gemini'
    default:
      return platform || t('admin.finance.allPlatforms')
  }
}

function summaryRowKey(row: FinanceSummaryRow): string {
  return `${row.company_id || 0}-${row.platform || 'all'}-${row.group_id || 0}`
}

function memberRowKey(row: FinanceMemberRow): string {
  return `${row.company_id || 0}-${row.platform || 'all'}-${row.user_id}`
}

function detailRowKey(row: FinanceMemberRow): string {
  return `${row.company_id || 0}-${row.platform || 'all'}-${row.group_id || 0}-${row.user_id}`
}

onMounted(async () => {
  document.addEventListener('click', handleClickOutside)
  await Promise.all([loadCompanies(), loadGroups()])
  reload()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.select-trigger {
  @apply flex w-full cursor-pointer items-center justify-between gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm text-gray-900 transition-all duration-200 hover:border-gray-300 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100 dark:hover:border-dark-500;
}

.select-trigger-open {
  @apply border-primary-500 ring-2 ring-primary-500/30;
}

.select-value {
  @apply flex-1 truncate text-left;
}
</style>
