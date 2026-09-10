<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="home-shell flex min-h-screen flex-col text-slate-900 dark:text-white"
    :class="{ 'home-mobile': isMobileH5 }"
  >
    <div class="home-backdrop" aria-hidden="true"></div>
    <div class="home-particles" aria-hidden="true">
      <span v-for="particle in particles" :key="particle" :style="particleStyle(particle)"></span>
    </div>

    <!-- Main Content -->
    <main
      class="home-main relative z-10 flex min-h-screen items-start"
      :class="isMobileH5 ? 'px-3 pb-4 pt-3' : 'px-4 pb-8 pt-10 sm:px-6 md:pt-14 lg:pt-16'"
    >
      <div class="mx-auto w-full max-w-6xl">
        <!-- Hero Section -->
        <div class="home-grid grid items-center gap-8 md:grid-cols-[minmax(0,1fr)_480px] md:gap-8 lg:grid-cols-[minmax(0,1fr)_500px] lg:gap-14">
          <!-- Left: Text Content -->
          <div class="home-intro home-reveal text-center lg:text-left">
            <div class="home-brand mb-6 flex items-center justify-center gap-4 lg:justify-start">
              <div class="home-brand-logo h-12 w-12 overflow-hidden rounded-2xl shadow-lg shadow-teal-500/20">
                <img :src="siteLogo || '/logo.png?v=togo-20260725'" alt="Logo" class="h-full w-full object-contain" />
              </div>
              <div class="text-left">
                <p class="text-2xl font-black text-slate-950 dark:text-white">{{ siteName }}</p>
                <p class="text-sm font-medium text-slate-500 dark:text-slate-400">让 AI 接入更简单、更稳定</p>
              </div>
            </div>

            <p class="home-tagline mb-3 text-sm font-black text-teal-600 dark:text-teal-300">
              一个入口 · 自动优选 · 费用清楚
            </p>
            <p class="home-subtitle mx-auto mb-6 max-w-2xl text-lg font-medium leading-8 text-slate-600 dark:text-slate-300 md:text-xl lg:mx-0">
              {{ commercialSubtitle }}
            </p>

            <div class="gateway-card mx-auto lg:mx-0">
              <div class="gateway-card-header">
                <div>
                  <span class="status-dot"></span>
                  <span>服务随时可用</span>
                </div>
                <strong>省心接入</strong>
              </div>

              <div class="gateway-stage">
                <span class="energy-ring ring-a"></span>
                <span class="energy-ring ring-b"></span>
                <span class="energy-ring ring-c"></span>
                <div class="gateway-core">
                  <Icon name="server" size="lg" />
                  <span>Togo</span>
                </div>
                <div
                  v-for="node in gatewayNodes"
                  :key="node.name"
                  class="gateway-node"
                  :class="node.className"
                >
                  <span class="provider-mark" :class="`provider-${node.icon}`" aria-hidden="true">
                    <svg v-if="node.icon === 'gemini'" viewBox="0 0 24 24" role="img">
                      <path
                        d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z"
                        fill="#3186FF"
                      />
                      <path
                        d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z"
                        fill="url(#homeGeminiGreen)"
                      />
                      <path
                        d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z"
                        fill="url(#homeGeminiRed)"
                      />
                      <path
                        d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z"
                        fill="url(#homeGeminiYellow)"
                      />
                      <defs>
                        <linearGradient id="homeGeminiGreen" gradientUnits="userSpaceOnUse" x1="7" x2="11" y1="15.5" y2="12">
                          <stop stop-color="#08B962" />
                          <stop offset="1" stop-color="#08B962" stop-opacity="0" />
                        </linearGradient>
                        <linearGradient id="homeGeminiRed" gradientUnits="userSpaceOnUse" x1="8" x2="11.5" y1="5.5" y2="11">
                          <stop stop-color="#F94543" />
                          <stop offset="1" stop-color="#F94543" stop-opacity="0" />
                        </linearGradient>
                        <linearGradient id="homeGeminiYellow" gradientUnits="userSpaceOnUse" x1="3.5" x2="17.5" y1="13.5" y2="12">
                          <stop stop-color="#FABC12" />
                          <stop offset=".46" stop-color="#FABC12" stop-opacity="0" />
                        </linearGradient>
                      </defs>
                    </svg>
                    <ModelIcon v-else-if="node.model" :model="node.model" size="25px" />
                    <svg v-else viewBox="0 0 32 32" role="img">
                      <path d="M15 7h2v8h8v2h-8v8h-2v-8H7v-2h8V7z" fill="currentColor" />
                    </svg>
                  </span>
                  <span class="provider-copy">
                    <strong>{{ node.name }}</strong>
                    <span>{{ node.label }}</span>
                  </span>
                </div>
                <span class="gateway-path path-a"></span>
                <span class="gateway-path path-b"></span>
                <span class="gateway-path path-c"></span>
                <span class="gateway-path path-d"></span>
                <span
                  v-for="pulse in energyPulses"
                  :key="pulse"
                  class="energy-pulse"
                  :class="`pulse-${pulse}`"
                ></span>
                <span class="gateway-orbit orbit-a"></span>
                <span class="gateway-orbit orbit-b"></span>
              </div>

              <div class="gateway-flow">
                <div class="flow-item">
                  <span>一个账号</span>
                  <strong>全站可用</strong>
                </div>
                <div class="flow-wave" aria-hidden="true">
                  <i></i>
                  <i></i>
                  <i></i>
                </div>
                <div class="flow-item flow-item-accent">
                  <span>自动优选</span>
                  <strong>稳定省心</strong>
                </div>
              </div>
            </div>
          </div>

          <!-- Right: Home Auth -->
          <div class="home-auth-wrap home-reveal home-delay-1 flex justify-center md:justify-end">
            <HomeAuthPanel />
          </div>
        </div>

      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAuthStore, useAppStore } from '@/stores'
import ModelIcon from '@/components/common/ModelIcon.vue'
import HomeAuthPanel from '@/components/auth/HomeAuthPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { useDeviceMode } from '@/composables/useDeviceMode'

const authStore = useAuthStore()
const appStore = useAppStore()
const { isMobileH5 } = useDeviceMode()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'TogoAPI')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
void docUrl.value
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '让 AI 接入更简单、更稳定')
const commercialSubtitle = computed(() => {
  const subtitle = siteSubtitle.value.trim()
  if (!subtitle || /subscription\s+to\s+api/i.test(subtitle)) {
    return '中转 · 官方直连 · 成本可控 · 企业级稳定'
  }
  return subtitle
})
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const particles = Array.from({ length: 22 }, (_, index) => index + 1)
const gatewayNodes = [
  { name: 'Claude', label: 'Anthropic', icon: 'claude', model: 'claude-3-5-sonnet', className: 'node-claude' },
  { name: 'GPT', label: 'OpenAI', icon: 'openai', model: 'gpt-4o', className: 'node-gpt' },
  { name: 'Gemini', label: 'Google', icon: 'gemini', model: 'gemini-2.5-pro', className: 'node-gemini' },
  { name: '更多', label: '持续接入', icon: 'more', model: '', className: 'node-more' }
]
const energyPulses = [1, 2, 3, 4]

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

function particleStyle(index: number) {
  const left = (index * 37) % 100
  const top = (index * 53) % 100
  const size = 3 + (index % 4)
  const delay = (index % 9) * -0.7
  const duration = 9 + (index % 7)

  return {
    left: `${left}%`,
    top: `${top}%`,
    width: `${size}px`,
    height: `${size}px`,
    animationDelay: `${delay}s`,
    animationDuration: `${duration}s`
  }
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-shell {
  position: relative;
  overflow: hidden;
  background:
    linear-gradient(135deg, rgba(248, 250, 252, 0.98), rgba(240, 253, 250, 0.86) 42%, rgba(248, 250, 252, 0.98)),
    #f8fafc;
  font-family:
    Outfit,
    'Noto Sans SC',
    ui-sans-serif,
    system-ui,
    sans-serif;
}

.dark .home-shell {
  background:
    linear-gradient(135deg, #020617, #07111d 45%, #020617),
    #020617;
}

.home-backdrop {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
  opacity: 0.8;
  background-image:
    linear-gradient(rgba(20, 184, 166, 0.065) 1px, transparent 1px),
    linear-gradient(90deg, rgba(13, 148, 136, 0.06) 1px, transparent 1px),
    radial-gradient(ellipse at 18% 8%, rgba(45, 212, 191, 0.2), transparent 34%),
    radial-gradient(ellipse at 78% 16%, rgba(13, 148, 136, 0.17), transparent 32%),
    radial-gradient(ellipse at 52% 58%, rgba(37, 99, 235, 0.08), transparent 36%);
  background-size:
    72px 72px,
    72px 72px,
    100% 100%,
    100% 100%,
    100% 100%;
}

.home-backdrop::before {
  content: '';
  position: absolute;
  inset: -10%;
  background: conic-gradient(
    from 180deg at 52% 38%,
    transparent,
    rgba(45, 212, 191, 0.12),
    rgba(13, 148, 136, 0.12),
    rgba(37, 99, 235, 0.08),
    transparent
  );
  filter: blur(50px);
  opacity: 0.78;
  animation: field-flow 24s ease-in-out infinite alternate;
}

.dark .home-backdrop {
  opacity: 0.65;
  background-image:
    linear-gradient(rgba(45, 212, 191, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(13, 148, 136, 0.05) 1px, transparent 1px),
    radial-gradient(ellipse at 22% 8%, rgba(45, 212, 191, 0.12), transparent 35%),
    radial-gradient(ellipse at 78% 16%, rgba(13, 148, 136, 0.1), transparent 34%),
    radial-gradient(ellipse at 52% 58%, rgba(37, 99, 235, 0.08), transparent 38%);
}

.home-particles {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 1;
  overflow: hidden;
}

.home-particles span {
  position: absolute;
  display: block;
  border-radius: 9999px;
  background: rgba(20, 184, 166, 0.52);
  box-shadow:
    0 0 0 6px rgba(20, 184, 166, 0.08),
    0 0 22px rgba(13, 148, 136, 0.38);
  animation: particle-rise ease-in-out infinite;
}

.gateway-card,
.gateway-node,
.flow-item {
  border: 1px solid rgba(255, 255, 255, 0.76);
  background: rgba(255, 255, 255, 0.72);
  box-shadow: 0 18px 54px rgba(125, 179, 203, 0.18);
  backdrop-filter: blur(20px);
}

.dark .gateway-card,
.dark .gateway-node,
.dark .flow-item {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.06);
  box-shadow: 0 18px 54px rgba(0, 0, 0, 0.22);
}

.gateway-card {
  position: relative;
  overflow: hidden;
  width: min(100%, 600px);
  border-radius: 1.8rem;
  padding: 1.1rem;
  background:
    radial-gradient(circle at 20% 15%, rgba(45, 212, 191, 0.16), transparent 26%),
    radial-gradient(circle at 86% 74%, rgba(37, 99, 235, 0.1), transparent 28%),
    rgba(255, 255, 255, 0.58);
  min-height: 382px;
}

.gateway-card::before {
  content: '';
  position: absolute;
  inset: 1.1rem;
  background:
    linear-gradient(rgba(13, 148, 136, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(13, 148, 136, 0.055) 1px, transparent 1px);
  background-size: 32px 32px;
  border-radius: 1.25rem;
  pointer-events: none;
}

.gateway-card-header {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.1rem 0.25rem 0.9rem;
  color: #0f172a;
  font-size: 0.86rem;
  font-weight: 900;
}

.gateway-card-header div {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
}

.gateway-card-header strong {
  border-radius: 9999px;
  background: rgba(20, 184, 166, 0.1);
  padding: 0.35rem 0.65rem;
  color: #0f766e;
  font-size: 0.72rem;
}

.status-dot {
  position: relative;
  width: 0.78rem;
  height: 0.78rem;
  border-radius: 9999px;
  background: #10b981;
  box-shadow:
    0 0 0 7px rgba(16, 185, 129, 0.14),
    0 0 24px rgba(16, 185, 129, 0.65);
  animation: status-breathe 1.8s ease-in-out infinite;
}

.status-dot::after {
  content: '';
  position: absolute;
  inset: -0.45rem;
  border-radius: inherit;
  border: 1px solid rgba(16, 185, 129, 0.35);
  animation: status-ripple 1.8s ease-out infinite;
}

.gateway-stage {
  position: relative;
  z-index: 2;
  overflow: hidden;
  height: 286px;
  border-radius: 1.35rem;
  background:
    radial-gradient(circle at 50% 50%, rgba(45, 212, 191, 0.16), transparent 30%),
    radial-gradient(circle at 18% 72%, rgba(13, 148, 136, 0.12), transparent 25%),
    radial-gradient(circle at 82% 24%, rgba(20, 184, 166, 0.12), transparent 25%),
    linear-gradient(rgba(255, 255, 255, 0.36), rgba(255, 255, 255, 0.1)),
    rgba(255, 255, 255, 0.22);
}

.gateway-node {
  position: absolute;
  z-index: 5;
  display: flex;
  align-items: center;
  min-width: 112px;
  gap: 0.62rem;
  border-radius: 1.1rem;
  padding: 0.76rem 0.82rem;
  animation: node-float 5.2s ease-in-out infinite;
}

.provider-mark {
  display: inline-flex;
  flex: 0 0 auto;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: 0.85rem;
  background: rgba(255, 255, 255, 0.82);
  box-shadow:
    0 10px 22px rgba(13, 148, 136, 0.12),
    inset 0 0 0 1px rgba(15, 23, 42, 0.06);
}

.provider-mark svg {
  width: 1.48rem;
  height: 1.48rem;
}

.provider-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.12rem;
}

.provider-copy strong {
  color: #0f172a;
  font-size: 0.88rem;
  font-weight: 950;
  line-height: 1.1;
}

.provider-copy span {
  color: #64748b;
  font-size: 0.7rem;
  font-weight: 800;
  line-height: 1.2;
}

.node-claude {
  left: 6%;
  top: 13%;
}

.node-claude .provider-mark {
  color: #f97316;
}

.node-gpt {
  right: 6%;
  top: 16%;
  animation-delay: -1.1s;
}

.node-gpt .provider-mark {
  color: #111827;
}

.node-gemini {
  left: 7%;
  bottom: 13%;
  animation-delay: -2.2s;
}

.node-gemini .provider-mark {
  color: #4285f4;
}

.node-more {
  right: 7%;
  bottom: 11%;
  animation-delay: -3.2s;
}

.node-more .provider-mark {
  color: #0f766e;
}

.gateway-core {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 5;
  display: flex;
  width: 90px;
  height: 90px;
  transform: translate(-50%, -50%);
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 0.25rem;
  border-radius: 9999px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.95), rgba(240, 253, 250, 0.72));
  color: #0f766e;
  box-shadow:
    0 20px 46px rgba(13, 148, 136, 0.24),
    0 0 0 13px rgba(45, 212, 191, 0.08),
    0 0 42px rgba(20, 184, 166, 0.28);
  animation: core-breathe 3.4s ease-in-out infinite;
}

.gateway-core span {
  color: #0f172a;
  font-size: 0.7rem;
  font-weight: 950;
}

.gateway-path {
  position: absolute;
  z-index: 3;
  height: 2px;
  transform-origin: center;
  overflow: hidden;
  border-radius: 9999px;
  background: rgba(13, 148, 136, 0.12);
}

.gateway-path::after {
  content: '';
  position: absolute;
  inset: 0;
  width: 45%;
  border-radius: inherit;
  background: linear-gradient(90deg, transparent, rgba(20, 184, 166, 0.86), rgba(52, 211, 153, 0.9), transparent);
  filter: drop-shadow(0 0 8px rgba(13, 148, 136, 0.55));
  animation: energy-travel 2.4s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

.path-a {
  left: 22%;
  top: 33%;
  width: 188px;
  transform: rotate(18deg);
}

.path-b {
  right: 22%;
  top: 34%;
  width: 184px;
  transform: rotate(-18deg);
}

.path-c {
  left: 22%;
  bottom: 31%;
  width: 188px;
  transform: rotate(-18deg);
}

.path-d {
  right: 22%;
  bottom: 31%;
  width: 184px;
  transform: rotate(18deg);
}

.path-b::after,
.path-d::after {
  animation-delay: -0.8s;
}

.path-c::after {
  animation-delay: -1.4s;
}

.gateway-orbit {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 2;
  display: block;
  width: 164px;
  height: 164px;
  transform: translate(-50%, -50%);
  border-radius: 9999px;
  border: 1px solid rgba(20, 184, 166, 0.18);
  animation: orbit-breathe 5.5s ease-in-out infinite;
}

.orbit-b {
  width: 114px;
  height: 114px;
  animation-duration: 9s;
}

.energy-ring {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 1;
  display: block;
  width: 92px;
  height: 92px;
  transform: translate(-50%, -50%);
  border-radius: 9999px;
  border: 1px solid rgba(20, 184, 166, 0.28);
  opacity: 0;
  animation: energy-ring 3.6s ease-out infinite;
}

.ring-b {
  animation-delay: -1.2s;
}

.ring-c {
  animation-delay: -2.4s;
}

.energy-pulse {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 4;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 9999px;
  background: #2dd4bf;
  box-shadow:
    0 0 0 5px rgba(45, 212, 191, 0.1),
    0 0 18px rgba(13, 148, 136, 0.65);
}

.pulse-1 {
  animation: pulse-to-a 2.8s ease-in-out infinite;
}

.pulse-2 {
  animation: pulse-to-b 2.8s ease-in-out infinite;
  animation-delay: -0.7s;
}

.pulse-3 {
  animation: pulse-to-c 2.8s ease-in-out infinite;
  animation-delay: -1.4s;
}

.pulse-4 {
  animation: pulse-to-d 2.8s ease-in-out infinite;
  animation-delay: -2.1s;
}

.gateway-flow {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 88px minmax(0, 1fr);
  gap: 0.75rem;
  align-items: center;
  margin-top: 0.95rem;
}

.flow-item {
  min-width: 0;
  border-radius: 1.15rem;
  border: 1px solid rgba(255, 255, 255, 0.78);
  background: rgba(255, 255, 255, 0.58);
  padding: 0.72rem 0.82rem;
  box-shadow: 0 12px 30px rgba(13, 148, 136, 0.1);
}

.flow-item span {
  display: block;
  color: #64748b;
  font-size: 0.7rem;
  font-weight: 800;
}

.flow-item strong {
  display: block;
  margin-top: 0.18rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #0f766e;
  font-size: 0.92rem;
  font-weight: 950;
}

.flow-item-accent {
  background:
    linear-gradient(135deg, rgba(20, 184, 166, 0.1), rgba(13, 148, 136, 0.08)),
    rgba(255, 255, 255, 0.6);
}

.flow-wave {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 0.28rem;
}

.flow-wave i {
  display: block;
  width: 0.42rem;
  height: 1.45rem;
  border-radius: 9999px;
  background: linear-gradient(180deg, rgba(20, 184, 166, 0.22), rgba(13, 148, 136, 0.78));
  box-shadow: 0 0 18px rgba(13, 148, 136, 0.28);
  animation: wave-breathe 1.4s ease-in-out infinite;
}

.flow-wave i:nth-child(2) {
  height: 2.05rem;
  animation-delay: -0.25s;
}

.flow-wave i:nth-child(3) {
  animation-delay: -0.5s;
}

.home-reveal {
  animation: reveal-rise 0.8s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.home-delay-1 {
  animation-delay: 0.12s;
}

.home-delay-2 {
  animation-delay: 0.22s;
}

.home-delay-3 {
  animation-delay: 0.34s;
}

@keyframes field-flow {
  0% {
    transform: translate3d(-2%, -1%, 0) rotate(0deg) scale(1);
  }
  100% {
    transform: translate3d(2%, 1%, 0) rotate(8deg) scale(1.04);
  }
}

@keyframes status-breathe {
  0%,
  100% {
    transform: scale(0.92);
    box-shadow:
      0 0 0 7px rgba(16, 185, 129, 0.12),
      0 0 18px rgba(16, 185, 129, 0.5);
  }
  50% {
    transform: scale(1.08);
    box-shadow:
      0 0 0 10px rgba(16, 185, 129, 0.18),
      0 0 30px rgba(16, 185, 129, 0.72);
  }
}

@keyframes status-ripple {
  0% {
    opacity: 0.85;
    transform: scale(0.58);
  }
  100% {
    opacity: 0;
    transform: scale(1.75);
  }
}

@keyframes node-float {
  0%,
  100% {
    transform: translate3d(0, 0, 0);
  }
  50% {
    transform: translate3d(0, -8px, 0);
  }
}

@keyframes core-breathe {
  0%,
  100% {
    transform: translate(-50%, -50%) scale(0.96);
  }
  50% {
    transform: translate(-50%, -50%) scale(1.04);
  }
}

@keyframes energy-travel {
  0% {
    opacity: 0;
    transform: translateX(115%);
  }
  18% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translateX(-95%);
  }
}

@keyframes orbit-breathe {
  0%,
  100% {
    opacity: 0.35;
    transform: translate(-50%, -50%) scale(0.96);
  }
  50% {
    opacity: 0.85;
    transform: translate(-50%, -50%) scale(1.04);
  }
}

@keyframes energy-ring {
  0% {
    opacity: 0.52;
    transform: translate(-50%, -50%) scale(0.76);
  }
  100% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(2.25);
  }
}

@keyframes pulse-to-a {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.55);
  }
  16% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate(calc(-50% - 154px), calc(-50% - 62px)) scale(1.08);
  }
}

@keyframes pulse-to-b {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.55);
  }
  16% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate(calc(-50% + 154px), calc(-50% - 58px)) scale(1.08);
  }
}

@keyframes pulse-to-c {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.55);
  }
  16% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate(calc(-50% - 152px), calc(-50% + 62px)) scale(1.08);
  }
}

@keyframes pulse-to-d {
  0% {
    opacity: 0;
    transform: translate(-50%, -50%) scale(0.55);
  }
  16% {
    opacity: 1;
  }
  100% {
    opacity: 0;
    transform: translate(calc(-50% + 152px), calc(-50% + 64px)) scale(1.08);
  }
}

@keyframes wave-breathe {
  0%,
  100% {
    opacity: 0.48;
    transform: scaleY(0.72);
  }
  50% {
    opacity: 1;
    transform: scaleY(1.08);
  }
}

@keyframes particle-rise {
  0%,
  100% {
    opacity: 0.2;
    transform: translate3d(0, 0, 0) scale(0.9);
  }
  50% {
    opacity: 0.9;
    transform: translate3d(10px, -26px, 0) scale(1.2);
  }
}

@keyframes reveal-rise {
  0% {
    opacity: 0;
    transform: translateY(36px);
  }
  100% {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 640px) {
  .gateway-card {
    min-height: 380px;
  }

  .gateway-stage {
    height: 265px;
  }

  .gateway-node {
    min-width: 98px;
    padding: 0.65rem;
    gap: 0.45rem;
  }

  .provider-mark {
    width: 1.75rem;
    height: 1.75rem;
    border-radius: 0.7rem;
    font-size: 0.78rem;
  }

  .provider-copy strong {
    font-size: 0.78rem;
  }

  .provider-copy span {
    font-size: 0.68rem;
  }

  .gateway-flow {
    grid-template-columns: 1fr;
    gap: 0.55rem;
  }

  .flow-wave {
    height: 1.2rem;
    transform: rotate(90deg);
  }
}

.home-shell.home-mobile .home-grid {
  gap: 0.75rem;
}

.home-shell.home-mobile .home-brand {
  margin-bottom: 0.35rem;
  gap: 0.65rem;
}

.home-shell.home-mobile .home-brand-logo {
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 0.75rem;
}

.home-shell.home-mobile .home-brand p:first-child {
  font-size: 1.25rem;
  line-height: 1.5rem;
}

.home-shell.home-mobile .home-brand p:last-child {
  font-size: 0.72rem;
}

.home-shell.home-mobile .home-tagline {
  margin-bottom: 0.1rem;
  font-size: 0.72rem;
}

.home-shell.home-mobile .home-subtitle {
  margin-bottom: 0.45rem;
  font-size: 0.8rem;
  line-height: 1.15rem;
}

.home-shell.home-mobile .gateway-card {
  min-height: 0;
  padding: 0.55rem 0.7rem;
  border-radius: 0.9rem;
}

.home-shell.home-mobile .gateway-card::before,
.home-shell.home-mobile .gateway-stage,
.home-shell.home-mobile .gateway-flow {
  display: none;
}

.home-shell.home-mobile .gateway-card-header {
  padding: 0;
  font-size: 0.74rem;
}

.home-shell.home-mobile .gateway-card-header div {
  gap: 0.5rem;
}

.home-shell.home-mobile .status-dot {
  width: 0.55rem;
  height: 0.55rem;
}

.home-shell.home-mobile .home-auth-wrap {
  align-items: flex-start;
}

@media (prefers-reduced-motion: reduce) {
  .home-backdrop::before,
  .home-particles span,
  .home-reveal,
  .status-dot,
  .status-dot::after,
  .gateway-node,
  .gateway-core,
  .gateway-path::after,
  .gateway-orbit,
  .energy-ring,
  .energy-pulse,
  .flow-wave i {
    animation: none;
  }
}
</style>
