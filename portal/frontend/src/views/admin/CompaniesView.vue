<template>
  <AppLayout>
    <div class="space-y-5">
      <div v-if="authStore.isFullAdmin" class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.companies.masterData') }}</h2>
          <button class="btn btn-primary" data-test="company-create-button" @click="openCompanyDialog()">
            <Icon name="plus" size="sm" />
            {{ t('admin.companies.createCompany') }}
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900/60">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.companyName') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.status') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="company in companies" :key="company.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-white">{{ company.name }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ company.status === 'active' ? t('admin.companies.active') : t('admin.companies.disabled') }}</td>
                <td class="px-4 py-3">
                  <div class="flex justify-end gap-2">
                    <button class="btn btn-sm btn-secondary" :data-test="`company-edit-${company.id}`" @click="openCompanyDialog(company)">
                      <Icon name="edit" size="sm" />
                      {{ t('admin.companies.editCompany') }}
                    </button>
                    <button class="btn btn-sm btn-danger" :disabled="savingCompany" :data-test="`company-deactivate-${company.id}`" @click="deactivateCompany(company)">
                      <Icon name="trash" size="sm" />
                      {{ t('admin.companies.deactivate') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3 rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <Select
          v-model="selectedCompanyId"
          class="w-full sm:w-72"
          data-test="company-selector"
          :options="companyOptions"
          :searchable="companyOptions.length > 5"
          @change="handleCompanySelectionChange"
        />
        <button class="btn btn-secondary" :disabled="loading" @click="loadCompanyDetails">
          <Icon name="refresh" size="sm" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.companies.managers') }}</h2>
          <button v-if="authStore.isFullAdmin" class="btn btn-primary" :disabled="!selectedCompanyId" data-test="company-add-manager-button" @click="openManagerDialog">
            <Icon name="userPlus" size="sm" />
            {{ t('admin.companies.addManager') }}
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900/60">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.user') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.status') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loadingManagers">
                <td colspan="3" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="managers.length === 0">
                <td colspan="3" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('admin.companies.noManagers') }}</td>
              </tr>
              <tr v-for="manager in managers" v-else :key="manager.user_id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="px-4 py-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ manager.email || manager.username || `#${manager.user_id}` }}</p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ manager.user_id }} · {{ manager.role }}</p>
                  </div>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ manager.status }}</td>
                <td class="px-4 py-3 text-right">
                  <button
                    v-if="authStore.isFullAdmin"
                    class="btn btn-sm btn-danger"
                    :disabled="savingManager"
                    :data-test="`company-manager-remove-${manager.user_id}`"
                    @click="removeCompanyManager(manager)"
                  >
                    <Icon name="trash" size="sm" />
                    {{ t('admin.companies.removeManager') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.companies.members') }}</h2>
          <button v-if="authStore.isFullAdmin || authStore.isCompanyManager" class="btn btn-primary" :disabled="!selectedCompanyId" data-test="company-add-member-button" @click="openMemberDialog">
            <Icon name="userPlus" size="sm" />
            {{ t('admin.companies.addMember') }}
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900/60">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.user') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.balance') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.apiKeys') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.totalCost') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.requests') }}</th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('admin.companies.lastRequest') }}</th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loading">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="members.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-dark-300">{{ t('admin.companies.noMembers') }}</td>
              </tr>
              <tr v-for="member in members" v-else :key="member.user_id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="px-4 py-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ member.email || member.username || `#${member.user_id}` }}</p>
                    <p class="text-xs text-gray-500 dark:text-dark-400">ID {{ member.user_id }} · {{ member.status }}</p>
                  </div>
                </td>
                <td class="px-4 py-3 text-right text-sm font-semibold text-gray-900 dark:text-white">{{ formatCurrency(member.balance) }}</td>
                <td class="px-4 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{{ member.api_key_count }}</td>
                <td class="px-4 py-3 text-right text-sm font-medium text-green-600 dark:text-green-400">{{ formatCurrency(member.total_cost) }}</td>
                <td class="px-4 py-3 text-right text-sm text-gray-700 dark:text-gray-300">{{ member.request_count }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ member.last_request_at ? formatDateTime(member.last_request_at) : '-' }}</td>
                <td class="px-4 py-3 text-right">
                  <div class="flex justify-end gap-2">
                    <button
                      class="btn btn-sm btn-primary"
                      :disabled="member.role === 'admin'"
                      :data-test="`company-member-allocate-${member.user_id}`"
                      @click="openAllocateDialog(member)"
                    >
                      <Icon name="plus" size="sm" />
                      {{ t('admin.companies.allocate') }}
                    </button>
                    <button
                      v-if="authStore.isFullAdmin || authStore.isCompanyManager"
                      class="btn btn-sm btn-danger"
                      :disabled="savingMember"
                      :data-test="`company-member-remove-${member.user_id}`"
                      @click="removeCompanyMember(member)"
                    >
                      <Icon name="trash" size="sm" />
                      {{ t('admin.companies.removeMember') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="companyDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="closeCompanyDialog">
      <div class="w-full max-w-md rounded-lg bg-white p-5 shadow-xl dark:bg-dark-800">
        <div class="mb-4 flex items-start justify-between gap-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingCompany ? t('admin.companies.editCompany') : t('admin.companies.createCompany') }}</h3>
          <button class="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-white" @click="closeCompanyDialog">
            <Icon name="x" size="md" />
          </button>
        </div>
        <form class="space-y-4" data-test="company-form" @submit.prevent="submitCompany">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.companies.companyName') }}</label>
            <input v-model.trim="companyForm.name" class="input w-full" required data-test="company-form-name" />
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.companies.status') }}</label>
            <select v-model="companyForm.status" class="input w-full" data-test="company-form-status">
              <option value="active">{{ t('admin.companies.active') }}</option>
              <option value="disabled">{{ t('admin.companies.disabled') }}</option>
            </select>
          </div>
          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="closeCompanyDialog">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="savingCompany || !companyForm.name">
              {{ savingCompany ? t('common.saving') : t('admin.companies.saveCompany') }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <UserCreateModal
      :show="memberDialogOpen"
      :submit-override="createCompanyMember"
      :success-message="null"
      :show-role="false"
      @close="closeMemberDialog"
      @success="handleMemberCreated"
    />

    <BaseDialog :show="managerDialogOpen" :title="t('admin.companies.addManager')" width="wide" @close="closeManagerDialog">
      <form class="min-h-[30rem] space-y-4" data-test="company-manager-form" @submit.prevent="submitManagerAssignment">
        <UserCandidatePicker
          v-model="selectedManagerCandidateId"
          v-model:search="managerCandidateSearch"
          :candidates="managerCandidates"
          :loading="managerCandidateLoading"
          :label="t('admin.companies.candidate')"
          :placeholder="t('admin.companies.managerCandidatePlaceholder')"
          :empty-text="t('admin.companies.noCandidates')"
          :loading-text="t('common.loading')"
          dropdown-class="max-h-[24rem]"
          auto-open
          data-test-prefix="company-manager"
          @open="openManagerCandidateDropdown"
          @search="(value) => loadManagerCandidates(value)"
        />
        <div class="relative z-40 flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeManagerDialog">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="savingManager || !selectedManagerCandidateId">
            {{ savingManager ? t('common.saving') : t('common.confirm') }}
          </button>
        </div>
      </form>
    </BaseDialog>

    <div v-if="allocationTarget" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="closeAllocateDialog">
      <div class="w-full max-w-md rounded-lg bg-white p-5 shadow-xl dark:bg-dark-800">
        <div class="mb-4 flex items-start justify-between gap-4">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.companies.allocate') }}</h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ allocationTarget.email || allocationTarget.username }}</p>
          </div>
          <button class="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-white" @click="closeAllocateDialog">
            <Icon name="x" size="md" />
          </button>
        </div>
        <form class="space-y-4" data-test="company-allocation-form" @submit.prevent="submitAllocation">
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.companies.amount') }}</label>
            <input v-model.number="allocationForm.amount" type="number" min="0.01" step="0.01" class="input w-full" required data-test="company-allocation-amount" />
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.companies.notes') }}</label>
            <textarea v-model="allocationForm.notes" rows="3" class="input w-full resize-none" data-test="company-allocation-notes"></textarea>
          </div>
          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-secondary" @click="closeAllocateDialog">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="allocating || allocationForm.amount <= 0">
              {{ allocating ? t('common.saving') : t('common.confirm') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import UserCandidatePicker, { type UserCandidate } from '@/components/common/UserCandidatePicker.vue'
import UserCreateModal from '@/components/admin/user/UserCreateModal.vue'
import companiesAPI from '@/api/admin/companies'
import type { AdminCompany, CompanyManager, CompanyMember, CreateCompanyMemberRequest } from '@/api/admin/companies'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const companies = ref<AdminCompany[]>([])
const selectedCompanyId = ref<number | null>(null)
const members = ref<CompanyMember[]>([])
const managers = ref<CompanyManager[]>([])
const loading = ref(false)
const loadingManagers = ref(false)
const allocating = ref(false)
const savingCompany = ref(false)
const savingMember = ref(false)
const savingManager = ref(false)
const companyDialogOpen = ref(false)
const memberDialogOpen = ref(false)
const managerDialogOpen = ref(false)
const editingCompany = ref<AdminCompany | null>(null)
const companyForm = reactive({ name: '', status: 'active' })
const allocationTarget = ref<CompanyMember | null>(null)
const allocationForm = reactive({ amount: 0, notes: '' })

const managerCandidateSearch = ref('')
const managerCandidateLoading = ref(false)
const managerCandidates = ref<UserCandidate[]>([])
const selectedManagerCandidateId = ref<number | null>(null)
let managerCandidateRequestSeq = 0

const visibleCompanies = computed(() => {
  if (authStore.isFullAdmin) return companies.value
  const allowed = new Set(authStore.managedCompanyIds)
  return companies.value.filter((company) => allowed.has(company.id))
})

const companyOptions = computed<SelectOption[]>(() =>
  visibleCompanies.value.map((company) => ({
    value: company.id,
    label: company.name,
  }))
)

watch(visibleCompanies, (items) => {
  if (!selectedCompanyId.value && items.length > 0) {
    selectedCompanyId.value = items[0].id
  }
})

async function loadCompanies() {
  try {
    companies.value = await companiesAPI.getAll()
    if (!selectedCompanyId.value && visibleCompanies.value.length > 0) {
      selectedCompanyId.value = visibleCompanies.value[0].id
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.failedToLoadCompanies'))
  }
}

async function loadMembers() {
  if (!selectedCompanyId.value) {
    members.value = []
    return
  }
  loading.value = true
  try {
    const data = await companiesAPI.getMembers(selectedCompanyId.value, { page: 1, page_size: 100 })
    members.value = data.items
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.failedToLoadMembers'))
  } finally {
    loading.value = false
  }
}

async function loadManagers() {
  if (!selectedCompanyId.value) {
    managers.value = []
    return
  }
  loadingManagers.value = true
  try {
    managers.value = await companiesAPI.getManagers(selectedCompanyId.value)
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.failedToLoadManagers'))
  } finally {
    loadingManagers.value = false
  }
}

async function loadCompanyDetails() {
  await Promise.all([loadMembers(), loadManagers()])
}

function handleCompanySelectionChange(value: string | number | boolean | null) {
  selectedCompanyId.value = typeof value === 'number' ? value : null
  void loadCompanyDetails()
}

function openCompanyDialog(company?: AdminCompany) {
  editingCompany.value = company ?? null
  companyForm.name = company?.name ?? ''
  companyForm.status = company?.status ?? 'active'
  companyDialogOpen.value = true
}

function closeCompanyDialog() {
  companyDialogOpen.value = false
  editingCompany.value = null
}

async function submitCompany() {
  if (!companyForm.name) return
  savingCompany.value = true
  try {
    if (editingCompany.value) {
      await companiesAPI.update(editingCompany.value.id, {
        name: companyForm.name,
        status: companyForm.status,
      })
      appStore.showSuccess(t('admin.companies.updateSuccess'))
    } else {
      await companiesAPI.create({
        name: companyForm.name,
        status: companyForm.status,
      })
      appStore.showSuccess(t('admin.companies.createSuccess'))
    }
    closeCompanyDialog()
    await loadCompanies()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.saveFailed'))
  } finally {
    savingCompany.value = false
  }
}

async function deactivateCompany(company: AdminCompany) {
  savingCompany.value = true
  try {
    await companiesAPI.deactivate(company.id)
    appStore.showSuccess(t('admin.companies.deactivateSuccess'))
    if (selectedCompanyId.value === company.id) {
      selectedCompanyId.value = null
      members.value = []
      managers.value = []
    }
    await loadCompanies()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.deactivateFailed'))
  } finally {
    savingCompany.value = false
  }
}

function openMemberDialog() {
  memberDialogOpen.value = true
}

function closeMemberDialog() {
  memberDialogOpen.value = false
}

function openManagerDialog() {
  resetManagerCandidatePicker()
  managerDialogOpen.value = true
  void loadManagerCandidates()
}

function closeManagerDialog() {
  managerDialogOpen.value = false
  resetManagerCandidatePicker()
}

function resetManagerCandidatePicker() {
  managerCandidateSearch.value = ''
  managerCandidates.value = []
  selectedManagerCandidateId.value = null
}

function excludedManagerCandidateIds(): Set<number> {
  return new Set(managers.value.map((item) => item.user_id))
}

function filterEligibleManagerCandidates(users: UserCandidate[]): UserCandidate[] {
  const excluded = excludedManagerCandidateIds()
  return users.filter((user) => user.role === 'user' && user.status === 'active' && !excluded.has(user.id))
}

function memberToCandidate(member: CompanyMember): UserCandidate {
  return {
    id: member.user_id,
    email: member.email,
    username: member.username,
    role: member.role,
    status: member.status,
  }
}

function candidateMatchesSearch(candidate: UserCandidate, search: string): boolean {
  const query = search.trim().toLowerCase()
  if (!query) return true
  return [
    candidate.email,
    candidate.username ?? '',
    String(candidate.id),
    `#${candidate.id}`,
  ].some((value) => value.toLowerCase().includes(query))
}

async function loadManagerCandidates(search = managerCandidateSearch.value) {
  if (!authStore.isFullAdmin) return
  const seq = ++managerCandidateRequestSeq
  managerCandidateLoading.value = true
  try {
    if (seq !== managerCandidateRequestSeq) return
    managerCandidates.value = filterEligibleManagerCandidates(members.value.map(memberToCandidate))
      .filter((candidate) => candidateMatchesSearch(candidate, search))
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.candidatesLoadFailed'))
  } finally {
    if (seq === managerCandidateRequestSeq) {
      managerCandidateLoading.value = false
    }
  }
}

function openManagerCandidateDropdown() {
  if (managerCandidates.value.length === 0 && !managerCandidateLoading.value) void loadManagerCandidates()
}

async function createCompanyMember(payload: CreateCompanyMemberRequest) {
  if (!selectedCompanyId.value) return
  await companiesAPI.createMember(selectedCompanyId.value, payload)
}

async function handleMemberCreated() {
  appStore.showSuccess(t('admin.companies.addMemberSuccess'))
  closeMemberDialog()
  await loadMembers()
}

async function removeCompanyMember(member: CompanyMember) {
  if (!selectedCompanyId.value) return
  savingMember.value = true
  try {
    await companiesAPI.removeMember(selectedCompanyId.value, member.user_id)
    appStore.showSuccess(t('admin.companies.removeMemberSuccess'))
    await loadMembers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.memberMutationFailed'))
  } finally {
    savingMember.value = false
  }
}

async function submitManagerAssignment() {
  if (!selectedCompanyId.value || !selectedManagerCandidateId.value) return
  savingManager.value = true
  try {
    await companiesAPI.addManager(selectedCompanyId.value, { user_id: selectedManagerCandidateId.value })
    appStore.showSuccess(t('admin.companies.addManagerSuccess'))
    closeManagerDialog()
    await loadManagers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.managerMutationFailed'))
  } finally {
    savingManager.value = false
  }
}

async function removeCompanyManager(manager: CompanyManager) {
  if (!selectedCompanyId.value) return
  savingManager.value = true
  try {
    await companiesAPI.removeManager(selectedCompanyId.value, manager.user_id)
    appStore.showSuccess(t('admin.companies.removeManagerSuccess'))
    await loadManagers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.managerMutationFailed'))
  } finally {
    savingManager.value = false
  }
}

function openAllocateDialog(member: CompanyMember) {
  allocationTarget.value = member
  allocationForm.amount = 0
  allocationForm.notes = ''
}

function closeAllocateDialog() {
  allocationTarget.value = null
}

async function submitAllocation() {
  if (!selectedCompanyId.value || !allocationTarget.value || allocationForm.amount <= 0) return
  allocating.value = true
  try {
    await companiesAPI.allocateBalance(selectedCompanyId.value, allocationTarget.value.user_id, {
      amount: allocationForm.amount,
      notes: allocationForm.notes,
    })
    appStore.showSuccess(t('admin.companies.allocateSuccess'))
    closeAllocateDialog()
    await loadMembers()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('admin.companies.allocateFailed'))
  } finally {
    allocating.value = false
  }
}

onMounted(async () => {
  await authStore.refreshManagementScope().catch(() => {})
  await loadCompanies()
  await loadCompanyDetails()
})
</script>
