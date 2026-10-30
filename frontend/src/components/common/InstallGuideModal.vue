<template>
  <Teleport to="body">
    <div class="guide-overlay" :class="'theme-' + theme">
      <div class="guide-card">
        <div class="guide-header">
          <span class="guide-title">{{ t('guide.title') }}</span>
          <button class="guide-close" :title="t('guide.close')" @click="$emit('close')">✕</button>
        </div>
        <div class="guide-tabs">
          <button
            v-for="item in platforms"
            :key="item.id"
            type="button"
            class="guide-tab"
            :class="{ active: active === item.id }"
            @click="active = item.id"
          >{{ item.label }}</button>
        </div>
        <div class="guide-body">
          <p class="guide-intro">{{ t('guide.intro') }}</p>
          <ol class="guide-steps">
            <li v-for="step in steps" :key="step">{{ step }}</li>
          </ol>
          <div v-if="command" class="guide-command">
            <code>{{ command }}</code>
            <button type="button" class="guide-copy" @click="copyCommand">
              {{ copied ? t('guide.copied') : t('guide.copy') }}
            </button>
          </div>
          <p class="guide-hint">{{ t('guide.hint') }}</p>
        </div>
        <div class="guide-footer">
          <button type="button" class="guide-btn" @click="$emit('close')">{{ t('guide.close') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'

defineEmits(['close'])

const settingsStore = useSettingsStore()
const { t } = useI18n()
const theme = computed(() => settingsStore.themeMode || 'light')

function detectPlatform() {
  const ua = (navigator.userAgent || '') + ' ' + (navigator.platform || '')
  if (/win/i.test(ua)) return 'windows'
  if (/linux/i.test(ua)) return 'linux'
  return 'macos'
}
const active = ref(detectPlatform())

const platforms = computed(() => [
  { id: 'macos', label: t('guide.macos') },
  { id: 'windows', label: t('guide.windows') },
  { id: 'linux', label: t('guide.linux') },
])

const steps = computed(() => {
  if (active.value === 'windows') {
    return [t('guide.winStep1'), t('guide.winStep2'), t('guide.winStep3')]
  }
  if (active.value === 'linux') {
    return [t('guide.linuxStep1'), t('guide.linuxStep2'), t('guide.linuxStep3')]
  }
  return [t('guide.macStep1'), t('guide.macStep2'), t('guide.macStep3'), t('guide.macStep4')]
})

const command = computed(() => {
  if (active.value === 'macos') return 'xattr -dr com.apple.quarantine /Applications/LiteRedis.app'
  if (active.value === 'linux') return 'tar -xzf LiteRedis-linux-amd64.tar.gz && chmod +x LiteRedis && ./LiteRedis'
  return ''
})

const copied = ref(false)
async function copyCommand() {
  if (!command.value) return
  const ok = await copyToClipboard(command.value)
  if (!ok) return
  copied.value = true
  setTimeout(() => { copied.value = false }, 1500)
}
</script>

<style scoped>
.guide-overlay {
  position: fixed;
  inset: 0;
  z-index: 10072;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.45);
  animation: guideFade 160ms ease;
}
@keyframes guideFade {
  from { background: rgba(15, 23, 42, 0); }
  to { background: rgba(15, 23, 42, 0.45); }
}
.guide-card {
  width: min(620px, 92vw);
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-lg);
  background: #fff;
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: var(--shadow-lg);
  animation: guidePop 200ms var(--ease-out);
}
@keyframes guidePop {
  from { opacity: 0; transform: translateY(10px) scale(0.985); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.guide-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(239, 246, 255, 0.9));
  border-radius: var(--radius-lg) var(--radius-lg) 0 0;
}
.guide-title {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
}
.guide-close {
  width: 26px;
  height: 26px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: #64748b;
  cursor: pointer;
}
.guide-close:hover {
  background: #fff;
  border-color: rgba(203, 213, 225, 0.9);
  color: #1e293b;
}
.guide-tabs {
  display: flex;
  gap: 6px;
  padding: 10px 16px 0;
}
.guide-tab {
  height: 26px;
  padding: 0 12px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-sm);
  background: #fff;
  color: #64748b;
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out), color var(--motion-fast) var(--ease-out);
}
.guide-tab:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.guide-tab.active {
  background: rgba(239, 246, 255, 0.96);
  border-color: rgba(147, 197, 253, 0.96);
  color: #1d4ed8;
}
.guide-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 16px 4px;
}
.guide-intro {
  margin: 0 0 10px;
  font-size: 11.5px;
  color: #64748b;
}
.guide-steps {
  margin: 0 0 12px;
  padding-left: 18px;
  font-size: 12.5px;
  line-height: 1.8;
  color: #334155;
}
.guide-command {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 8px 10px;
  border-radius: var(--radius-md);
  background: #0f172a;
}
.guide-command code {
  flex: 1;
  min-width: 0;
  color: #dbeafe;
  font: 11px/1.6 ui-monospace, Menlo, monospace;
  white-space: pre-wrap;
  word-break: break-all;
}
.guide-copy {
  flex: 0 0 auto;
  height: 24px;
  padding: 0 10px;
  border: 1px solid rgba(96, 165, 250, 0.5);
  border-radius: var(--radius-sm);
  background: rgba(30, 64, 175, 0.28);
  color: #dbeafe;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
}
.guide-copy:hover {
  background: rgba(30, 64, 175, 0.45);
}
.guide-hint {
  margin: 0 0 8px;
  padding: 8px 10px;
  border-radius: var(--radius-md);
  font-size: 11px;
  line-height: 1.65;
  color: #64748b;
  background: rgba(248, 250, 252, 0.96);
  border: 1px dashed rgba(203, 213, 225, 0.9);
}
.guide-footer {
  display: flex;
  justify-content: flex-end;
  padding: 10px 16px 14px;
}
.guide-btn {
  height: 30px;
  padding: 0 16px;
  border-radius: var(--radius-md);
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}
.guide-btn:hover {
  border-color: #60a5fa;
  color: #1d4ed8;
  background: linear-gradient(180deg, #f8fbff, #eff6ff);
}
/* 深色 */
.guide-overlay.theme-dark .guide-card {
  background: rgba(15, 23, 42, 0.99);
  border-color: rgba(51, 65, 85, 0.92);
}
.guide-overlay.theme-dark .guide-header {
  border-bottom-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.99));
}
.guide-overlay.theme-dark .guide-title { color: #e2e8f0; }
.guide-overlay.theme-dark .guide-close { color: #94a3b8; }
.guide-overlay.theme-dark .guide-close:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #e2e8f0;
}
.guide-overlay.theme-dark .guide-tab {
  background: rgba(15, 23, 42, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #94a3b8;
}
.guide-overlay.theme-dark .guide-tab.active {
  background: rgba(30, 64, 175, 0.3);
  border-color: rgba(96, 165, 250, 0.5);
  color: #dbeafe;
}
.guide-overlay.theme-dark .guide-intro,
.guide-overlay.theme-dark .guide-hint { color: #94a3b8; }
.guide-overlay.theme-dark .guide-steps { color: #cbd5e1; }
.guide-overlay.theme-dark .guide-hint {
  background: rgba(11, 18, 32, 0.9);
  border-color: rgba(71, 85, 105, 0.9);
}
.guide-overlay.theme-dark .guide-btn {
  background: rgba(15, 23, 42, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #93c5fd;
}
</style>
