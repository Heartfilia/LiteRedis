<template>
  <Teleport to="body">
    <div class="welcome-overlay" :class="'theme-' + theme">
      <div class="welcome-card">
        <div class="welcome-hero">
          <span class="welcome-logo" aria-hidden="true">
            <svg viewBox="0 0 32 32" fill="none">
              <rect x="4" y="6" width="24" height="6" rx="3" fill="currentColor" opacity="0.9" />
              <rect x="4" y="14" width="24" height="6" rx="3" fill="currentColor" opacity="0.6" />
              <rect x="4" y="22" width="24" height="6" rx="3" fill="currentColor" opacity="0.35" />
            </svg>
          </span>
          <h2 class="welcome-title">{{ t('welcome.title') }}</h2>
          <p class="welcome-subtitle">{{ t('welcome.subtitle') }}</p>
        </div>

        <ul class="welcome-features">
          <li v-for="item in features" :key="item">
            <span class="feature-check" aria-hidden="true">
              <svg viewBox="0 0 16 16" fill="none">
                <path d="M3.5 8.5 6.5 11.5 12.5 5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </span>
            <span>{{ item }}</span>
          </li>
        </ul>

        <div class="welcome-tip">{{ t('welcome.hint') }}</div>

        <div class="welcome-actions">
          <button class="welcome-btn ghost" @click="$emit('skip')">{{ t('welcome.skip') }}</button>
          <button class="welcome-btn primary" @click="$emit('create')">{{ t('welcome.create') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'

defineEmits(['skip', 'create'])

const settingsStore = useSettingsStore()
const { t } = useI18n()
const theme = computed(() => settingsStore.themeMode || 'light')
const features = computed(() => [
  t('welcome.feature1'),
  t('welcome.feature2'),
  t('welcome.feature3'),
  t('welcome.feature4'),
])
</script>

<style scoped>
.welcome-overlay {
  position: fixed;
  inset: 0;
  z-index: 10070;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.45);
  animation: welcomeFade 160ms ease;
}
@keyframes welcomeFade {
  from { background: rgba(15, 23, 42, 0); }
  to { background: rgba(15, 23, 42, 0.45); }
}
.welcome-card {
  width: min(560px, 92vw);
  padding: 26px 28px 22px;
  border-radius: var(--radius-lg);
  background: #fff;
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: var(--shadow-lg);
  animation: welcomePop 200ms var(--ease-out);
}
@keyframes welcomePop {
  from { opacity: 0; transform: translateY(10px) scale(0.98); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.welcome-hero {
  text-align: center;
  margin-bottom: 18px;
}
.welcome-logo {
  display: inline-flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  color: #2563eb;
  background: linear-gradient(180deg, #eff6ff, #dbeafe);
  border: 1px solid rgba(191, 219, 254, 0.9);
  margin-bottom: 10px;
}
.welcome-logo svg {
  width: 24px;
  height: 24px;
  display: block;
}
.welcome-title {
  margin: 0 0 6px;
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
}
.welcome-subtitle {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.65;
  color: #64748b;
}
.welcome-features {
  margin: 0 0 14px;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 9px;
}
.welcome-features li {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  font-size: 12.5px;
  line-height: 1.6;
  color: #334155;
}
.feature-check {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  margin-top: 1px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #15803d;
  background: rgba(220, 252, 231, 0.95);
  border: 1px solid rgba(134, 239, 172, 0.9);
}
.feature-check svg {
  width: 10px;
  height: 10px;
  display: block;
}
.welcome-tip {
  margin-bottom: 18px;
  padding: 9px 12px;
  border-radius: var(--radius-md);
  font-size: 11.5px;
  line-height: 1.6;
  color: #64748b;
  background: rgba(248, 250, 252, 0.96);
  border: 1px dashed rgba(203, 213, 225, 0.9);
}
.welcome-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.welcome-btn {
  height: 32px;
  padding: 0 16px;
  border-radius: var(--radius-md);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #475569;
  transition: background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out), color var(--motion-fast) var(--ease-out);
}
.welcome-btn.ghost:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.welcome-btn.primary {
  background: linear-gradient(180deg, #2563eb, #1d4ed8);
  border-color: #1d4ed8;
  color: #fff;
}
.welcome-btn.primary:hover {
  background: linear-gradient(180deg, #1d4ed8, #1e40af);
}
/* 深色 */
.welcome-overlay.theme-dark .welcome-card {
  background: rgba(15, 23, 42, 0.99);
  border-color: rgba(51, 65, 85, 0.92);
}
.welcome-overlay.theme-dark .welcome-logo {
  color: #93c5fd;
  background: rgba(30, 64, 175, 0.3);
  border-color: rgba(96, 165, 250, 0.4);
}
.welcome-overlay.theme-dark .welcome-title { color: #e2e8f0; }
.welcome-overlay.theme-dark .welcome-subtitle,
.welcome-overlay.theme-dark .welcome-tip { color: #94a3b8; }
.welcome-overlay.theme-dark .welcome-features li { color: #cbd5e1; }
.welcome-overlay.theme-dark .feature-check {
  color: #6ee7b7;
  background: rgba(6, 78, 59, 0.9);
  border-color: rgba(16, 185, 129, 0.45);
}
.welcome-overlay.theme-dark .welcome-tip {
  background: rgba(11, 18, 32, 0.9);
  border-color: rgba(71, 85, 105, 0.9);
}
.welcome-overlay.theme-dark .welcome-btn {
  background: rgba(15, 23, 42, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #cbd5e1;
}
.welcome-overlay.theme-dark .welcome-btn.ghost:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
.welcome-overlay.theme-dark .welcome-btn.primary {
  background: linear-gradient(180deg, #2563eb, #1d4ed8);
  border-color: #1d4ed8;
  color: #fff;
}
</style>
