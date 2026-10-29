<template>
  <Teleport to="body">
    <div class="wn-overlay" :class="'theme-' + theme">
      <div class="wn-card">
        <div class="wn-header">
          <div class="wn-title-wrap">
            <span class="wn-title">{{ t('whatsNew.title') }}</span>
            <span class="wn-version">v{{ version }}</span>
          </div>
          <button class="wn-close" :title="t('keyEditor.close')" @click="close">✕</button>
        </div>
        <div class="wn-body">
          <div class="wn-lead">{{ t('whatsNew.lead') }}</div>
          <div class="wn-notes" v-html="notesHtml" @click="onNotesClick"></div>
        </div>
        <div class="wn-footer">
          <button v-if="url" class="wn-btn ghost" @click="openRelease">{{ t('whatsNew.openRelease') }}</button>
          <button class="wn-btn primary" @click="close">{{ t('whatsNew.gotIt') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { renderReleaseNotes } from '../../utils/markdown.js'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime.js'

const props = defineProps({
  version: { type: String, default: '' },
  body: { type: String, default: '' },
  url: { type: String, default: '' },
})
const emit = defineEmits(['close'])

const settingsStore = useSettingsStore()
const { t } = useI18n()
const theme = computed(() => settingsStore.themeMode || 'light')
const notesHtml = computed(() => renderReleaseNotes(props.body))

let disposeEscape = null
onMounted(() => { disposeEscape = registerEscapeHandler(close) })
onBeforeUnmount(() => { if (disposeEscape) disposeEscape() })

function close() {
  emit('close')
}

function openRelease() {
  if (props.url) BrowserOpenURL(props.url)
}

// 说明里的链接交给系统浏览器打开，避免 WebView 自己跳转走
function onNotesClick(event) {
  const link = event.target && event.target.closest ? event.target.closest('a') : null
  if (!link) return
  event.preventDefault()
  const href = link.getAttribute('href') || ''
  if (/^https?:\/\//i.test(href)) BrowserOpenURL(href)
}
</script>

<style scoped>
.wn-overlay {
  position: fixed;
  inset: 0;
  z-index: 10080;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.45);
  animation: wnFadeIn 160ms ease;
}
@keyframes wnFadeIn {
  from { background: rgba(15, 23, 42, 0); }
  to { background: rgba(15, 23, 42, 0.45); }
}
.wn-card {
  width: min(660px, 92vw);
  max-height: min(76vh, 720px);
  display: flex;
  flex-direction: column;
  border-radius: 14px;
  background: #fff;
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: 0 28px 60px rgba(15, 23, 42, 0.24), 0 8px 20px rgba(15, 23, 42, 0.08);
  overflow: hidden;
  animation: wnPop 220ms cubic-bezier(0.22, 0.9, 0.24, 1);
}
@keyframes wnPop {
  from { opacity: 0; transform: translateY(10px) scale(0.97); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.wn-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 18px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(239, 246, 255, 0.9));
}
.wn-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.wn-title {
  font-size: 14px;
  font-weight: 700;
  color: #1e293b;
}
.wn-version {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 11px;
  font-weight: 600;
  color: #2563eb;
  background: rgba(219, 234, 254, 0.9);
  border: 1px solid rgba(147, 197, 253, 0.7);
  border-radius: 999px;
  padding: 3px 8px;
  line-height: 1;
}
.wn-close {
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: #64748b;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.14s, color 0.14s, border-color 0.14s;
}
.wn-close:hover {
  background: #fff;
  border-color: rgba(203, 213, 225, 0.9);
  color: #1e293b;
}
.wn-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 20px 6px;
}
.wn-lead {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 10px;
}
.wn-notes {
  font-size: 12.5px;
  line-height: 1.75;
  color: #334155;
  word-break: break-word;
}
.wn-notes :deep(h3),
.wn-notes :deep(h4) {
  margin: 14px 0 6px;
  font-size: 12.5px;
  font-weight: 700;
  color: #0f172a;
}
.wn-notes :deep(h3:first-child) {
  margin-top: 0;
}
.wn-notes :deep(p) {
  margin: 6px 0;
}
.wn-notes :deep(ul),
.wn-notes :deep(ol) {
  margin: 6px 0;
  padding-left: 20px;
}
.wn-notes :deep(li) {
  margin: 3px 0;
}
.wn-notes :deep(code) {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  padding: 1px 5px;
  border-radius: 5px;
  background: rgba(241, 245, 249, 0.96);
  border: 1px solid rgba(226, 232, 240, 0.9);
}
.wn-notes :deep(a) {
  color: #2563eb;
  text-decoration: none;
}
.wn-notes :deep(a:hover) {
  text-decoration: underline;
}
.wn-notes :deep(hr) {
  border: none;
  border-top: 1px dashed rgba(203, 213, 225, 0.9);
  margin: 12px 0;
}
.wn-notes :deep(strong) {
  color: #0f172a;
}
.wn-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.9), rgba(248, 250, 252, 0.96));
}
.wn-btn {
  height: 30px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #475569;
  transition: background 0.14s, border-color 0.14s, color 0.14s;
}
.wn-btn.ghost:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.wn-btn.primary {
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  border-color: rgba(191, 219, 254, 0.96);
}
.wn-btn.primary:hover {
  background: linear-gradient(180deg, #f8fbff, #eff6ff);
  border-color: #60a5fa;
  color: #1d4ed8;
}

/* 深色 */
.wn-overlay.theme-dark .wn-card {
  background: rgba(15, 23, 42, 0.99);
  border-color: rgba(51, 65, 85, 0.92);
  box-shadow: 0 28px 60px rgba(2, 6, 23, 0.6);
}
.wn-overlay.theme-dark .wn-header {
  border-bottom-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.99));
}
.wn-overlay.theme-dark .wn-title { color: #e2e8f0; }
.wn-overlay.theme-dark .wn-version {
  color: #93c5fd;
  background: rgba(30, 64, 175, 0.34);
  border-color: rgba(96, 165, 250, 0.45);
}
.wn-overlay.theme-dark .wn-close { color: #94a3b8; }
.wn-overlay.theme-dark .wn-close:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #e2e8f0;
}
.wn-overlay.theme-dark .wn-lead { color: #94a3b8; }
.wn-overlay.theme-dark .wn-notes { color: #cbd5e1; }
.wn-overlay.theme-dark .wn-notes :deep(h3),
.wn-overlay.theme-dark .wn-notes :deep(h4),
.wn-overlay.theme-dark .wn-notes :deep(strong) { color: #e2e8f0; }
.wn-overlay.theme-dark .wn-notes :deep(code) {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(71, 85, 105, 0.9);
  color: #dbeafe;
}
.wn-overlay.theme-dark .wn-notes :deep(a) { color: #93c5fd; }
.wn-overlay.theme-dark .wn-notes :deep(hr) { border-top-color: rgba(71, 85, 105, 0.9); }
.wn-overlay.theme-dark .wn-footer {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(11, 18, 32, 0.99), rgba(8, 15, 29, 1));
}
.wn-overlay.theme-dark .wn-btn {
  background: rgba(15, 23, 42, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #cbd5e1;
}
.wn-overlay.theme-dark .wn-btn.ghost:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
.wn-overlay.theme-dark .wn-btn.primary {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
}
.wn-overlay.theme-dark .wn-btn.primary:hover {
  border-color: rgba(96, 165, 250, 0.5);
  color: #dbeafe;
}
</style>
