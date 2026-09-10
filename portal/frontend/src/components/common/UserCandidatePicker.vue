<template>
  <div ref="pickerRef" class="relative">
    <label v-if="label" class="input-label">{{ label }}</label>
    <div class="relative">
      <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
      <input
        :value="search"
        type="text"
        class="input w-full pl-9"
        autocomplete="off"
        :placeholder="placeholder"
        :data-test="`${dataTestPrefix}-candidate-search`"
        @input="handleSearchInput"
        @focus="openDropdown"
        @keydown.down.prevent="moveFocus(1)"
        @keydown.up.prevent="moveFocus(-1)"
        @keydown.enter.prevent="confirmFocusedCandidate"
        @keydown.esc.prevent="dropdownOpen = false"
      />
    </div>

    <div
      v-if="dropdownOpen"
      :class="[
        'absolute z-50 mt-1 w-full overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-700 dark:bg-dark-800',
        dropdownClass
      ]"
    >
      <div v-if="loading" class="px-4 py-3 text-sm text-gray-500 dark:text-dark-300">
        {{ loadingText }}
      </div>
      <template v-else>
        <button
          v-for="(candidate, index) in candidates"
          :key="candidate.id"
          type="button"
          :data-test="`${dataTestPrefix}-candidate-${candidate.id}`"
          :class="[
            'flex w-full items-start gap-3 px-4 py-2.5 text-left text-sm transition-colors',
            index === focusedIndex
              ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
              : 'text-gray-700 hover:bg-gray-50 dark:text-dark-200 dark:hover:bg-dark-700'
          ]"
          @mousedown.prevent.stop
          @click.prevent.stop="toggleCandidate(candidate)"
        >
          <span
            v-if="multiple"
            :class="[
              'mt-0.5 flex h-4 w-4 flex-shrink-0 items-center justify-center rounded border',
              isSelected(candidate.id)
                ? 'border-primary-500 bg-primary-500 text-white'
                : 'border-gray-300 bg-white dark:border-dark-600 dark:bg-dark-900'
            ]"
          >
            <Icon v-if="isSelected(candidate.id)" name="check" size="xs" />
          </span>
          <span class="min-w-0 flex-1">
            <span class="block truncate font-medium">
              {{ candidate.username || candidate.email || `#${candidate.id}` }}
              <span class="ml-1 text-xs font-normal text-gray-500">#{{ candidate.id }}</span>
            </span>
            <span class="block truncate text-xs text-gray-500">{{ candidate.email }}</span>
          </span>
          <Icon v-if="!multiple && isSelected(candidate.id)" name="check" size="sm" class="mt-0.5 flex-shrink-0 text-primary-500" />
        </button>
      </template>
      <div v-if="!loading && candidates.length === 0" class="px-4 py-6 text-center text-sm text-gray-500">
        {{ emptyText }}
      </div>
    </div>

    <div v-if="selectedCandidates.length > 0" class="mt-2 flex flex-wrap gap-2">
      <span
        v-for="candidate in selectedCandidates"
        :key="candidate.id"
        class="inline-flex max-w-full items-center gap-1 rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/25 dark:text-primary-200"
        :data-test="`${dataTestPrefix}-selected-${candidate.id}`"
      >
        <span class="truncate">{{ candidate.username || candidate.email || `#${candidate.id}` }} #{{ candidate.id }}</span>
        <button
          v-if="multiple"
          type="button"
          class="rounded-full p-0.5 text-primary-500 hover:bg-primary-100 hover:text-primary-700 dark:hover:bg-primary-900/40"
          @click="removeCandidate(candidate.id)"
        >
          <Icon name="x" size="xs" />
        </button>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import Icon from '@/components/icons/Icon.vue'

export interface UserCandidate {
  id: number
  email: string
  username?: string
  role?: string
  status?: string
}

const props = withDefaults(defineProps<{
  modelValue: number | number[] | null
  search: string
  candidates: UserCandidate[]
  label?: string
  placeholder?: string
  emptyText?: string
  loadingText?: string
  loading?: boolean
  multiple?: boolean
  autoOpen?: boolean
  dropdownClass?: string
  dataTestPrefix?: string
}>(), {
  label: '',
  placeholder: '',
  emptyText: '',
  loadingText: '',
  loading: false,
  multiple: false,
  autoOpen: false,
  dropdownClass: 'max-h-72',
  dataTestPrefix: 'user'
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: number | number[] | null): void
  (e: 'update:search', value: string): void
  (e: 'search', value: string): void
  (e: 'open'): void
}>()

const dropdownOpen = ref(false)
const focusedIndex = ref(-1)
const pickerRef = ref<HTMLElement | null>(null)
const selectedCache = ref<Map<number, UserCandidate>>(new Map())

const selectedIds = computed<number[]>(() => {
  if (props.multiple) return Array.isArray(props.modelValue) ? props.modelValue : []
  return typeof props.modelValue === 'number' ? [props.modelValue] : []
})

const selectedCandidates = computed(() =>
  selectedIds.value
    .map((id) => selectedCache.value.get(id) ?? props.candidates.find((candidate) => candidate.id === id))
    .filter((candidate): candidate is UserCandidate => !!candidate)
)

watch(
  () => props.candidates,
  (candidates) => {
    const next = new Map(selectedCache.value)
    for (const candidate of candidates) {
      if (selectedIds.value.includes(candidate.id)) next.set(candidate.id, candidate)
    }
    selectedCache.value = next
    focusedIndex.value = candidates.length > 0 ? Math.max(0, Math.min(focusedIndex.value, candidates.length - 1)) : -1
  },
  { immediate: true }
)

const debouncedSearch = useDebounceFn((value: string) => {
  emit('search', value)
}, 250)

function handleSearchInput(event: Event): void {
  const value = (event.target as HTMLInputElement).value
  emit('update:search', value)
  dropdownOpen.value = true
  debouncedSearch(value)
}

function openDropdown(): void {
  dropdownOpen.value = true
  emit('open')
  if (props.candidates.length > 0 && focusedIndex.value < 0) focusedIndex.value = 0
}

function isSelected(id: number): boolean {
  return selectedIds.value.includes(id)
}

function candidateLabel(candidate: UserCandidate): string {
  return candidate.username || candidate.email || `#${candidate.id}`
}

function toggleCandidate(candidate: UserCandidate): void {
  const nextCache = new Map(selectedCache.value)
  nextCache.set(candidate.id, candidate)
  selectedCache.value = nextCache

  if (props.multiple) {
    const current = selectedIds.value
    const next = isSelected(candidate.id)
      ? current.filter((id) => id !== candidate.id)
      : [...current, candidate.id]
    emit('update:modelValue', next)
    return
  }

  emit('update:modelValue', candidate.id)
  emit('update:search', candidateLabel(candidate))
  dropdownOpen.value = false
}

function removeCandidate(id: number): void {
  if (!props.multiple) return
  emit('update:modelValue', selectedIds.value.filter((selectedId) => selectedId !== id))
}

function moveFocus(delta: number): void {
  if (!dropdownOpen.value) {
    openDropdown()
    return
  }
  if (props.candidates.length === 0) return
  const next = focusedIndex.value + delta
  focusedIndex.value = (next + props.candidates.length) % props.candidates.length
}

function confirmFocusedCandidate(): void {
  if (!dropdownOpen.value) {
    openDropdown()
    return
  }
  const candidate = props.candidates[focusedIndex.value]
  if (candidate) toggleCandidate(candidate)
}

function handleDocumentClick(event: MouseEvent): void {
  const target = event.target as Node | null
  if (target && pickerRef.value?.contains(target)) return
  dropdownOpen.value = false
}

onMounted(() => {
  document.addEventListener('mousedown', handleDocumentClick)
  if (props.autoOpen) openDropdown()
})

onUnmounted(() => {
  document.removeEventListener('mousedown', handleDocumentClick)
})
</script>
