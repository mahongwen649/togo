<template>
  <div class="min-h-screen bg-gradient-to-b from-teal-50/70 via-emerald-50/35 to-white text-slate-900 dark:from-dark-950 dark:via-dark-950 dark:to-dark-900 dark:text-white">
    <div
      class="mx-auto grid min-h-screen w-full items-center px-4 py-8 lg:px-8"
      :class="showBrandPanel ? 'max-w-7xl gap-8 lg:grid-cols-[1fr_440px]' : 'max-w-md'"
    >
      <section v-if="showBrandPanel" class="hidden lg:block">
        <template v-if="settingsLoaded">
          <div class="mb-10 flex items-center gap-3">
            <div class="flex h-11 w-11 items-center justify-center overflow-hidden rounded-xl shadow-sm shadow-teal-500/20">
              <img :src="siteLogo || '/logo.png?v=togo-20260725'" :alt="t('auth.layout.logoAlt')" class="h-full w-full object-contain" />
            </div>
            <div>
              <h1 class="text-xl font-semibold text-slate-950 dark:text-white">{{ siteName }}</h1>
              <p class="text-sm text-slate-500 dark:text-slate-400">{{ t('auth.layout.brandDirection') }}</p>
            </div>
          </div>

          <p class="mb-4 text-sm font-semibold uppercase text-teal-600 dark:text-teal-300">
            {{ t('auth.layout.gatewayEyebrow') }}
          </p>
          <h2 class="max-w-2xl bg-gradient-to-r from-slate-950 via-teal-950 to-teal-700 bg-clip-text text-5xl font-bold leading-tight text-transparent dark:from-white dark:via-teal-100 dark:to-emerald-300">
            {{ t('auth.layout.heroTitle') }}
          </h2>
          <p v-if="showFeatureSummary" class="mt-5 max-w-xl text-base leading-7 text-slate-600 dark:text-slate-300">
            {{ siteSubtitle }}
          </p>
          <div v-if="showFeatureSummary" class="mt-8 grid max-w-xl gap-3 sm:grid-cols-3">
            <div v-for="feature in features" :key="feature.title" class="rounded-lg border border-teal-100 bg-white p-4 shadow-sm shadow-teal-500/5 dark:border-dark-700 dark:bg-dark-900">
              <div class="text-2xl font-semibold text-slate-950 dark:text-white">{{ feature.title }}</div>
              <div class="mt-1 text-xs text-slate-500">{{ feature.description }}</div>
            </div>
          </div>
        </template>
      </section>

      <main class="w-full">
        <div v-if="showBrandPanel" class="mb-6 text-center lg:hidden">
          <template v-if="settingsLoaded">
            <div class="mb-3 inline-flex h-12 w-12 items-center justify-center overflow-hidden rounded-xl shadow-sm shadow-teal-500/20">
              <img :src="siteLogo || '/logo.png?v=togo-20260725'" :alt="t('auth.layout.logoAlt')" class="h-full w-full object-contain" />
            </div>
            <h1 class="text-2xl font-semibold text-slate-950 dark:text-white">{{ siteName }}</h1>
            <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ siteSubtitle }}</p>
          </template>
        </div>

        <div class="rounded-lg border border-teal-100 bg-white p-6 shadow-card shadow-teal-500/5 dark:border-dark-700 dark:bg-dark-900 sm:p-8">
          <slot />
        </div>

        <div class="mt-6 text-center text-sm">
          <slot name="footer" />
        </div>

        <div class="mt-8 text-center text-xs text-slate-400 dark:text-slate-500">
          {{ t('auth.layout.copyright', { year: currentYear, siteName }) }}
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

withDefaults(defineProps<{
  showFeatureSummary?: boolean
  showBrandPanel?: boolean
}>(), {
  showFeatureSummary: true,
  showBrandPanel: true,
})

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'TogoAPI')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || t('auth.layout.defaultSubtitle'))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())
const features = computed(() => [
  { title: t('auth.layout.relayTitle'), description: t('auth.layout.relayDescription') },
  { title: t('auth.layout.quotaTitle'), description: t('auth.layout.quotaDescription') },
  { title: t('auth.layout.billingTitle'), description: t('auth.layout.billingDescription') },
])

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
