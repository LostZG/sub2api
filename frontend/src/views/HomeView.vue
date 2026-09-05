<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
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

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page · Cyber Neon -->
  <div v-else class="neon-home">
    <!-- Background layers -->
    <div class="neon-bg" aria-hidden="true">
      <div class="neon-grid"></div>
      <div class="neon-orb neon-orb--cyan"></div>
      <div class="neon-orb neon-orb--magenta"></div>
      <div class="neon-orb neon-orb--blue"></div>
      <div class="neon-scan"></div>
      <div class="neon-crt"></div>
      <div class="neon-vignette"></div>
    </div>

    <!-- Header -->
    <header class="neon-header">
      <nav class="neon-nav">
        <div class="neon-logo">
          <div class="neon-logo-mark">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" />
          </div>
          <span class="neon-logo-text">{{ siteName }}</span>
        </div>

        <div class="neon-actions">
          <LocaleSwitcher class="neon-action" />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="neon-action neon-icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <!-- Model Plaza Link -->
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="neon-action neon-icon-btn"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
          </router-link>

          <!-- Theme Toggle -->
          <button
            @click="toggleTheme"
            class="neon-action neon-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="neon-action neon-btn neon-btn--sm"
          >
            <span class="neon-btn-dot"></span>
            <span>{{ t('home.dashboard') }}</span>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="neon-action neon-btn neon-btn--sm"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main -->
    <main class="neon-main">
      <!-- Hero -->
      <section class="neon-hero">
        <div class="neon-badge">
          <span class="neon-badge-dot"></span>
          <span class="neon-badge-text">AI API GATEWAY</span>
        </div>

        <h1 class="neon-title">{{ siteName }}</h1>

        <p class="neon-subtitle">{{ heroTagline }}</p>

        <div class="neon-cta">
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="neon-btn neon-btn--lg"
          >
            <span>{{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}</span>
            <Icon name="arrowRight" size="md" class="neon-btn-arrow" :stroke-width="2" />
          </router-link>
        </div>
      </section>

      <!-- Feature cards -->
      <section class="neon-cards">
        <article class="neon-card">
          <span class="neon-card-corner neon-card-corner--tl"></span>
          <span class="neon-card-corner neon-card-corner--br"></span>
          <div class="neon-card-icon">
            <Icon name="server" size="lg" :stroke-width="1.8" />
          </div>
          <div class="neon-card-label">UNIFIED GATEWAY</div>
          <div class="neon-card-title">{{ t('home.features.unifiedGateway') }}</div>
        </article>

        <article class="neon-card">
          <span class="neon-card-corner neon-card-corner--tl"></span>
          <span class="neon-card-corner neon-card-corner--br"></span>
          <div class="neon-card-icon">
            <Icon name="shield" size="lg" :stroke-width="1.8" />
          </div>
          <div class="neon-card-label">AUTO FAILOVER</div>
          <div class="neon-card-title">{{ t('home.features.multiAccount') }}</div>
        </article>

        <article class="neon-card">
          <span class="neon-card-corner neon-card-corner--tl"></span>
          <span class="neon-card-corner neon-card-corner--br"></span>
          <div class="neon-card-icon">
            <Icon name="chart" size="lg" :stroke-width="1.8" />
          </div>
          <div class="neon-card-label">PAY PER USE</div>
          <div class="neon-card-title">{{ t('home.features.balanceQuota') }}</div>
        </article>
      </section>
    </main>

    <!-- Footer -->
    <footer class="neon-footer">
      <div class="neon-footer-inner">
        <p class="neon-footer-text">
          <span class="neon-footer-prompt">$</span>
          <span
            >&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span
          >
        </p>
        <div class="neon-footer-links">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ t('home.docs') }}
          </a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(
  () => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API'
)
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
    allowRelative: true,
    allowDataUrl: true,
  })
)
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const heroTagline = computed(() => siteSubtitle.value || t('home.heroSubtitle'))
const docUrl = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
)
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme (reflects global preference; the neon home is always dark by design)
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme (affects the rest of the app; neon home stays dark)
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
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

<style>
/* Cyber-neon display fonts (non-scoped so the @import is hoisted) */
@import url('https://fonts.googleapis.com/css2?family=Chakra+Petch:wght@500;600;700&family=Sora:wght@300;400;500;600&family=JetBrains+Mono:wght@400;500&display=swap');
</style>

<style scoped>
/* ============ Base ============ */
.neon-home {
  position: relative;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #050608;
  color: #e2e8f0;
  font-family: 'Sora', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif;
}

/* ============ Background layers ============ */
.neon-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.neon-grid {
  position: absolute;
  inset: -2px;
  background-image:
    linear-gradient(rgba(45, 212, 191, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(45, 212, 191, 0.07) 1px, transparent 1px);
  background-size: 56px 56px;
  background-position: center center;
  -webkit-mask-image: radial-gradient(ellipse 80% 60% at 50% 38%, #000 0%, transparent 75%);
  mask-image: radial-gradient(ellipse 80% 60% at 50% 38%, #000 0%, transparent 75%);
}

.neon-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.5;
  animation: orb-drift 18s ease-in-out infinite;
  will-change: transform;
}
.neon-orb--cyan {
  width: 480px;
  height: 480px;
  top: -120px;
  left: -100px;
  background: radial-gradient(circle, #14b8a6, transparent 70%);
}
.neon-orb--magenta {
  width: 520px;
  height: 520px;
  bottom: -160px;
  right: -120px;
  background: radial-gradient(circle, #d946ef, transparent 70%);
  animation-delay: -6s;
  opacity: 0.32;
}
.neon-orb--blue {
  width: 360px;
  height: 360px;
  top: 45%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: radial-gradient(circle, #0ea5e9, transparent 70%);
  animation-delay: -12s;
  opacity: 0.22;
}

@keyframes orb-drift {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }
  33% {
    transform: translate(40px, -30px) scale(1.05);
  }
  66% {
    transform: translate(-30px, 40px) scale(0.95);
  }
}

.neon-scan {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent 10%, rgba(45, 212, 191, 0.85) 50%, transparent 90%);
  box-shadow:
    0 0 12px rgba(45, 212, 191, 0.6),
    0 0 24px rgba(45, 212, 191, 0.3);
  animation: scan-sweep 9s linear infinite;
  opacity: 0;
}

@keyframes scan-sweep {
  0% {
    transform: translateY(-10vh);
    opacity: 0;
  }
  8% {
    opacity: 0.7;
  }
  92% {
    opacity: 0.7;
  }
  100% {
    transform: translateY(110vh);
    opacity: 0;
  }
}

.neon-crt {
  position: absolute;
  inset: 0;
  background: repeating-linear-gradient(
    0deg,
    rgba(0, 0, 0, 0) 0,
    rgba(0, 0, 0, 0) 2px,
    rgba(0, 0, 0, 0.18) 3px,
    rgba(0, 0, 0, 0) 4px
  );
  opacity: 0.4;
  mix-blend-mode: multiply;
}

.neon-vignette {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at center, transparent 45%, rgba(0, 0, 0, 0.7) 100%);
}

/* ============ Header ============ */
.neon-header {
  position: relative;
  z-index: 20;
  padding: 20px 32px;
  border-bottom: 1px solid rgba(45, 212, 191, 0.08);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.neon-nav {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.neon-logo {
  display: flex;
  align-items: center;
  gap: 12px;
}

.neon-logo-mark {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid rgba(45, 212, 191, 0.4);
  box-shadow:
    0 0 16px rgba(45, 212, 191, 0.3),
    inset 0 0 12px rgba(45, 212, 191, 0.1);
}
.neon-logo-mark img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.neon-logo-text {
  font-family: 'Chakra Petch', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  font-weight: 600;
  font-size: 15px;
  letter-spacing: 0.12em;
  color: #2dd4bf;
  text-transform: uppercase;
}

.neon-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.neon-action {
  display: inline-flex;
  align-items: center;
}

.neon-icon-btn {
  padding: 8px 10px;
  border-radius: 6px;
  color: #64748b;
  transition:
    color 0.2s,
    background 0.2s;
}
.neon-icon-btn:hover {
  color: #2dd4bf;
  background: rgba(45, 212, 191, 0.08);
}

/* Let LocaleSwitcher inherit the dark neon palette */
.neon-action :deep(*) {
  color: #94a3b8;
}
.neon-action :deep(*:hover) {
  color: #2dd4bf;
}

/* ============ Buttons ============ */
.neon-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: #2dd4bf;
  background: rgba(45, 212, 191, 0.04);
  border: 1px solid rgba(45, 212, 191, 0.4);
  padding: 12px 22px;
  cursor: pointer;
  transition:
    color 0.25s,
    background 0.25s,
    border-color 0.25s,
    box-shadow 0.25s,
    transform 0.25s;
  text-decoration: none;
  box-shadow:
    0 0 0 1px rgba(45, 212, 191, 0.05),
    0 0 20px rgba(45, 212, 191, 0.15),
    inset 0 0 16px rgba(45, 212, 191, 0.04);
}

.neon-btn:hover {
  color: #5eead4;
  background: rgba(45, 212, 191, 0.1);
  border-color: rgba(45, 212, 191, 0.7);
  box-shadow:
    0 0 0 1px rgba(45, 212, 191, 0.15),
    0 0 36px rgba(45, 212, 191, 0.35),
    inset 0 0 24px rgba(45, 212, 191, 0.08);
  transform: translateY(-1px);
}

.neon-btn--sm {
  padding: 7px 16px;
  font-size: 11px;
}

.neon-btn--lg {
  padding: 15px 34px;
  font-size: 13px;
}

.neon-btn-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #2dd4bf;
  box-shadow: 0 0 8px #2dd4bf;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}

.neon-btn-arrow {
  transition: transform 0.25s ease;
}
.neon-btn:hover .neon-btn-arrow {
  transform: translateX(4px);
}

/* ============ Main ============ */
.neon-main {
  position: relative;
  z-index: 10;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 32px 60px;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

/* ============ Hero ============ */
.neon-hero {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  animation: hero-enter 1s ease-out;
}

@keyframes hero-enter {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.neon-badge {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 6px 14px;
  margin-bottom: 32px;
  border: 1px solid rgba(45, 212, 191, 0.25);
  background: rgba(45, 212, 191, 0.04);
  border-radius: 4px;
}
.neon-badge-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 8px #22c55e;
  animation: pulse 2s ease-in-out infinite;
}
.neon-badge-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.28em;
  color: #5eead4;
}

.neon-title {
  font-family: 'Chakra Petch', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  font-weight: 700;
  font-size: clamp(48px, 9vw, 112px);
  line-height: 0.95;
  letter-spacing: 0.01em;
  margin: 0 0 24px;
  background: linear-gradient(135deg, #5eead4 0%, #2dd4bf 35%, #67e8f9 65%, #2dd4bf 100%);
  background-size: 200% 200%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
  filter: drop-shadow(0 0 28px rgba(45, 212, 191, 0.45));
  animation: title-shimmer 8s ease-in-out infinite;
}

@keyframes title-shimmer {
  0%,
  100% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
}

.neon-subtitle {
  font-family: 'Sora', 'PingFang SC', sans-serif;
  font-weight: 300;
  font-size: clamp(15px, 1.6vw, 19px);
  color: #94a3b8;
  margin: 0 0 40px;
  letter-spacing: 0.04em;
  max-width: 520px;
}

.neon-cta {
  margin-bottom: 80px;
}

/* ============ Feature cards ============ */
.neon-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
  width: 100%;
  max-width: 900px;
}

.neon-card {
  position: relative;
  padding: 28px 24px;
  background: rgba(10, 15, 25, 0.5);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border: 1px solid rgba(45, 212, 191, 0.12);
  transition:
    border-color 0.3s,
    background 0.3s,
    transform 0.3s,
    box-shadow 0.3s;
  overflow: hidden;
}

.neon-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 15%;
  right: 15%;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(45, 212, 191, 0.6), transparent);
  opacity: 0.6;
}

.neon-card:hover {
  border-color: rgba(45, 212, 191, 0.35);
  background: rgba(15, 23, 42, 0.65);
  transform: translateY(-4px);
  box-shadow: 0 12px 40px -8px rgba(45, 212, 191, 0.25);
}

.neon-card-corner {
  position: absolute;
  width: 14px;
  height: 14px;
  transition: opacity 0.3s ease;
  opacity: 0.4;
}
.neon-card-corner--tl {
  top: -1px;
  left: -1px;
  border-top: 2px solid #2dd4bf;
  border-left: 2px solid #2dd4bf;
}
.neon-card-corner--br {
  bottom: -1px;
  right: -1px;
  border-bottom: 2px solid #2dd4bf;
  border-right: 2px solid #2dd4bf;
}
.neon-card:hover .neon-card-corner {
  opacity: 1;
}

.neon-card-icon {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 18px;
  color: #2dd4bf;
  background: rgba(45, 212, 191, 0.08);
  border: 1px solid rgba(45, 212, 191, 0.2);
  border-radius: 8px;
  box-shadow:
    0 0 16px rgba(45, 212, 191, 0.15),
    inset 0 0 8px rgba(45, 212, 191, 0.05);
  filter: drop-shadow(0 0 6px rgba(45, 212, 191, 0.4));
}

.neon-card-label {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.2em;
  color: #475569;
  margin-bottom: 6px;
}

.neon-card-title {
  font-family: 'Sora', 'PingFang SC', sans-serif;
  font-size: 17px;
  font-weight: 500;
  color: #e2e8f0;
  letter-spacing: 0.02em;
}

/* ============ Footer ============ */
.neon-footer {
  position: relative;
  z-index: 10;
  padding: 24px 32px;
  border-top: 1px solid rgba(45, 212, 191, 0.08);
}

.neon-footer-inner {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.neon-footer-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  color: #475569;
  margin: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.neon-footer-prompt {
  color: #2dd4bf;
}

.neon-footer-links {
  display: flex;
  gap: 20px;
}
.neon-footer-links a {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  color: #475569;
  text-decoration: none;
  transition: color 0.2s;
}
.neon-footer-links a:hover {
  color: #2dd4bf;
}

/* ============ Responsive ============ */
@media (max-width: 768px) {
  .neon-header {
    padding: 16px 20px;
  }
  .neon-main {
    padding: 60px 20px 40px;
  }
  .neon-cards {
    grid-template-columns: 1fr;
    gap: 14px;
  }
  .neon-cta {
    margin-bottom: 60px;
  }
  .neon-logo-text {
    display: none;
  }
  .neon-footer-inner {
    flex-direction: column;
    text-align: center;
  }
}

@media (prefers-reduced-motion: reduce) {
  .neon-orb,
  .neon-scan,
  .neon-title,
  .neon-btn-dot,
  .neon-badge-dot,
  .neon-hero {
    animation: none;
  }
}
</style>
