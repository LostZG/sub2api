<template>
  <div
    class="auth-neon dark relative flex min-h-screen items-center justify-center overflow-hidden p-4"
  >
    <!-- Neon Background -->
    <div class="auth-bg" aria-hidden="true">
      <div class="auth-grid"></div>
      <div class="auth-orb auth-orb--cyan"></div>
      <div class="auth-orb auth-orb--magenta"></div>
      <div class="auth-orb auth-orb--blue"></div>
      <div class="auth-noise"></div>
      <div class="auth-scan"></div>
      <div class="auth-crt"></div>
      <div class="auth-vignette"></div>

      <!-- HUD corner marks -->
      <span class="auth-hud-corner auth-hud-corner--tl"></span>
      <span class="auth-hud-corner auth-hud-corner--tr"></span>
      <span class="auth-hud-corner auth-hud-corner--bl"></span>
      <span class="auth-hud-corner auth-hud-corner--br"></span>
    </div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <template v-if="settingsLoaded">
          <div
            class="auth-logo-mark mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="auth-title mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="auth-subtitle text-sm">{{ siteSubtitle }}</p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-card rounded-2xl p-8">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="auth-footer mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="auth-copyright mt-8 text-center text-xs">
        <span class="auth-copyright-prompt">$</span>
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const siteSubtitle = computed(
  () => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform'
)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style>
/* Cyber-neon display fonts (kept in a non-scoped block so the @import is hoisted) */
@import url('https://fonts.googleapis.com/css2?family=Chakra+Petch:wght@500;600;700&family=Sora:wght@300;400;500;600&family=JetBrains+Mono:wght@400;500&display=swap');
</style>

<style scoped>
/* ============ Base ============ */
.auth-neon {
  background: #050608;
  color: #e2e8f0;
  font-family: 'Sora', 'PingFang SC', 'Microsoft YaHei', system-ui, sans-serif;
}

/* ============ Background layers ============ */
.auth-bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

.auth-grid {
  position: absolute;
  inset: -2px;
  background-image:
    linear-gradient(rgba(45, 212, 191, 0.1) 1px, transparent 1px),
    linear-gradient(90deg, rgba(45, 212, 191, 0.1) 1px, transparent 1px);
  background-size: 56px 56px;
  background-position: center center;
  -webkit-mask-image: radial-gradient(ellipse 80% 60% at 50% 50%, #000 0%, transparent 75%);
  mask-image: radial-gradient(ellipse 80% 60% at 50% 50%, #000 0%, transparent 75%);
}

.auth-orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  animation: auth-orb-drift 18s ease-in-out infinite;
  will-change: transform;
}
.auth-orb--cyan {
  width: 460px;
  height: 460px;
  top: -110px;
  left: -90px;
  background: radial-gradient(circle, #14b8a6, transparent 70%);
  opacity: 0.62;
}
.auth-orb--magenta {
  width: 500px;
  height: 500px;
  bottom: -140px;
  right: -120px;
  background: radial-gradient(circle, #d946ef, transparent 70%);
  opacity: 0.42;
  animation-delay: -6s;
}
.auth-orb--blue {
  width: 360px;
  height: 360px;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: radial-gradient(circle, #0ea5e9, transparent 70%);
  opacity: 0.32;
  animation-delay: -12s;
}

@keyframes auth-orb-drift {
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

.auth-scan {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent 10%, rgba(45, 212, 191, 0.85) 50%, transparent 90%);
  box-shadow:
    0 0 12px rgba(45, 212, 191, 0.6),
    0 0 24px rgba(45, 212, 191, 0.3);
  animation: auth-scan-sweep 9s linear infinite;
  opacity: 0;
}

@keyframes auth-scan-sweep {
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

.auth-crt {
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

.auth-vignette {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at center, transparent 55%, rgba(0, 0, 0, 0.55) 100%);
}

/* Noise texture: gives the dark background grain instead of flat black */
.auth-noise {
  position: absolute;
  inset: 0;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
  opacity: 0.08;
  mix-blend-mode: screen;
}

/* HUD corner marks: frame the viewport with L-shaped accents */
.auth-hud-corner {
  position: absolute;
  width: 14px;
  height: 14px;
  border: 0 solid rgba(45, 212, 191, 0.5);
  z-index: 1;
}
.auth-hud-corner--tl {
  top: 24px;
  left: 24px;
  border-top-width: 1px;
  border-left-width: 1px;
}
.auth-hud-corner--tr {
  top: 24px;
  right: 24px;
  border-top-width: 1px;
  border-right-width: 1px;
}
.auth-hud-corner--bl {
  bottom: 24px;
  left: 24px;
  border-bottom-width: 1px;
  border-left-width: 1px;
}
.auth-hud-corner--br {
  bottom: 24px;
  right: 24px;
  border-bottom-width: 1px;
  border-right-width: 1px;
}

/* ============ Brand ============ */
.auth-logo-mark {
  border: 1px solid rgba(45, 212, 191, 0.4);
  box-shadow:
    0 0 20px rgba(45, 212, 191, 0.35),
    inset 0 0 14px rgba(45, 212, 191, 0.1);
}

.auth-title {
  font-family: 'Chakra Petch', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  font-weight: 700;
  letter-spacing: 0.02em;
  background: linear-gradient(135deg, #5eead4 0%, #2dd4bf 35%, #67e8f9 65%, #2dd4bf 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent;
  filter: drop-shadow(0 0 20px rgba(45, 212, 191, 0.45));
}

.auth-subtitle {
  color: #94a3b8;
  letter-spacing: 0.04em;
}

/* ============ Card (dark glassmorphism) ============ */
.auth-card {
  background: rgba(30, 41, 59, 0.92);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(45, 212, 191, 0.5);
  box-shadow:
    0 0 0 1px rgba(45, 212, 191, 0.12),
    0 0 24px rgba(45, 212, 191, 0.18),
    0 24px 70px -20px rgba(0, 0, 0, 0.8),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
  position: relative;
}

/* glowing top edge on the card */
.auth-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 20%;
  right: 20%;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(45, 212, 191, 0.6), transparent);
  opacity: 0.7;
}

/* Footer & copyright */
.auth-footer {
  color: #94a3b8;
}
.auth-footer :deep(a) {
  color: #2dd4bf;
}
.auth-copyright {
  color: #475569;
  font-family: 'JetBrains Mono', monospace;
  letter-spacing: 0.04em;
}
.auth-copyright-prompt {
  color: #2dd4bf;
  margin-right: 6px;
}

/* ============ Form input adaptation ============ */
/* Ensure inputs/labels render cleanly on the dark glass card */
.auth-card :deep(.input-label) {
  color: #cbd5e1;
}
.auth-card :deep(.input) {
  background-color: rgba(2, 6, 23, 0.75);
  border-color: rgba(45, 212, 191, 0.35);
  color: #e2e8f0;
  box-shadow: inset 0 0 12px rgba(45, 212, 191, 0.06);
}
.auth-card :deep(.input::placeholder) {
  color: #475569;
}
.auth-card :deep(.input:focus) {
  border-color: rgba(45, 212, 191, 0.7);
  box-shadow:
    inset 0 0 14px rgba(45, 212, 191, 0.15),
    0 0 0 3px rgba(45, 212, 191, 0.15),
    0 0 16px rgba(45, 212, 191, 0.2);
}
/* keep linked helper text legible */
.auth-card :deep(a) {
  color: #2dd4bf;
}

@media (max-width: 768px) {
  .auth-hud-corner {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-orb,
  .auth-scan {
    animation: none;
  }
}
</style>
