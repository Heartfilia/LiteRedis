<template>
  <Teleport to="body">
    <div v-if="rendered" class="expand-overlay" :class="[themeClass, { 'is-closing': closing }]">
      <div ref="modalEl" class="expand-modal">
        <div class="expand-header">
          <span class="expand-title" :title="title">{{ title }}</span>
          <span v-if="jsonTip" class="json-tip" :class="jsonTipTone">{{ jsonTip }}</span>
          <div class="expand-header-actions">
            <button
              v-if="editable && !saving"
              class="btn-json-modal"
              :class="{ 'json-flash-error': jsonError }"
              :disabled="!localContent.trim()"
              :title="formatApplied ? t('keyEditor.jsonRestore') : t('keyEditor.jsonFormat')"
              @click="toggleJson"
            >{{ formatApplied ? t('keyEditor.jsonRestore') : t('keyEditor.jsonFormat') }}</button>
            <button
              v-if="editable && !saving"
              class="btn-json-modal"
              :class="{ 'json-flash-error': jsonError }"
              :disabled="!localContent.trim()"
              :title="treeMode ? t('keyEditor.jsonTextView') : t('keyEditor.jsonTreeView')"
              @click="toggleTreeView"
            >{{ treeMode ? t('keyEditor.jsonTextView') : t('keyEditor.jsonTreeView') }}</button>
            <button
              v-if="editable && !saving"
              class="btn-save-modal"
              :class="{ 'success-flash': saveFlashing, 'error-flash': errorFlashing }"
              :title="t('keyEditor.save')"
              @click="onSave"
            >
              <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
                <path d="M6.75 4.75h8.9l2.6 2.6v9.9A1.75 1.75 0 0 1 16.5 19H7.5a1.75 1.75 0 0 1-1.75-1.75V6.5A1.75 1.75 0 0 1 7.5 4.75Z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" />
                <path d="M9 4.75v4.5h5.5v-4.5M9.25 14.25h5.5" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </button>
            <button class="btn-copy-modal" :class="{ copied }" :title="copied ? t('keyEditor.copied') : t('keyEditor.copy')" @click="copy">
              <svg v-if="!copied" viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
                <path d="M9 7.25h8.25A1.75 1.75 0 0 1 19 9v8.25A1.75 1.75 0 0 1 17.25 19H9A1.75 1.75 0 0 1 7.25 17.25V9A1.75 1.75 0 0 1 9 7.25Z" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round" />
                <path d="M5.75 15V6.75A1.75 1.75 0 0 1 7.5 5h8.25" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
              <svg v-else viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
                <path d="M6.75 12.5 10.2 16l7.05-8" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round" />
              </svg>
            </button>
            <button class="btn-close-modal" @click="onClose" :title="t('keyEditor.close')">
              <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
                <path d="M6.7 6.7l10.6 10.6M17.3 6.7L6.7 17.3" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" />
              </svg>
            </button>
          </div>
        </div>
        <div class="expand-body">
          <JsonViewer v-if="treeMode" :data="treeData" />
          <textarea
            v-else-if="editable"
            v-model="localContent"
            class="expand-textarea"
            :disabled="saving"
          />
          <pre v-else class="expand-content">{{ content }}</pre>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { useI18n } from '../../i18n/index.js'
import { useSettingsStore } from '../../stores/settings.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import JsonViewer from '../common/JsonViewer.vue'

const { t } = useI18n()
const settingsStore = useSettingsStore()

const props = defineProps({
  show:    { type: Boolean, default: false },
  title:   { type: String,  default: '' },
  content: { type: String,  default: '' },
  editable:{ type: Boolean, default: false },
  saving:  { type: Boolean, default: false },
  // 触发编辑的那个按钮的位置，用于从按钮位置放大 / 收回
  anchor:  { type: Object,  default: null },
})

const emit = defineEmits(['close', 'save'])
let disposeEscape = null
onMounted(() => { disposeEscape = registerEscapeHandler(() => onClose()) })
onBeforeUnmount(() => { if (disposeEscape) disposeEscape() })

const localContent = ref(props.content)
const copied = ref(false)
const saveFlashing = ref(false)
const errorFlashing = ref(false)
const themeClass = computed(() => `theme-${settingsStore.themeMode || 'light'}`)
let saveFlashTimer = null
let errorFlashTimer = null

// ---- 弹出 / 收回动画 ----
const rendered = ref(props.show)
const closing = ref(false)
const modalEl = ref(null)
const openOrigin = ref('50% 50%')
const openScale = ref(0.86)
let closeTimer = null

function measureAnchor() {
  const el = modalEl.value
  const rect = el ? el.getBoundingClientRect() : null
  const a = props.anchor
  if (!rect || !rect.width || !rect.height) {
    openOrigin.value = '50% 50%'
    openScale.value = 0.88
    return
  }
  if (!a || !a.width) {
    openOrigin.value = '50% 50%'
    openScale.value = 0.9
    return
  }
  const originX = a.left + a.width / 2 - rect.left
  const originY = a.top + a.height / 2 - rect.top
  openOrigin.value = `${originX}px ${originY}px`
  // 以编辑按钮尺寸为起点放大，限制下限避免缩成一个点
  openScale.value = Math.max(0.05, Math.min(0.32, (a.width * 1.5) / rect.width))
}

function playOpen() {
  const el = modalEl.value
  if (!el) return
  measureAnchor()
  el.style.transformOrigin = openOrigin.value
  el.style.transition = 'none'
  el.style.transform = `scale(${openScale.value})`
  el.style.opacity = '0'
  void el.offsetWidth
  el.style.transition = 'transform 260ms cubic-bezier(0.22, 0.9, 0.24, 1), opacity 170ms ease'
  el.style.transform = 'scale(1)'
  el.style.opacity = '1'
}

function resetModalStyle() {
  const el = modalEl.value
  if (!el) return
  el.style.transform = ''
  el.style.opacity = ''
  el.style.transition = ''
  el.style.transformOrigin = ''
}

function playClose() {
  const el = modalEl.value
  closing.value = true
  const finish = () => {
    rendered.value = false
    closing.value = false
    resetModalStyle()
  }
  if (!el) {
    finish()
    return
  }
  el.style.transition = 'transform 200ms cubic-bezier(0.4, 0, 0.75, 0.2), opacity 170ms ease'
  el.style.transform = `scale(${openScale.value})`
  el.style.opacity = '0'
  if (closeTimer) clearTimeout(closeTimer)
  closeTimer = setTimeout(finish, 205)
}

watch(() => props.show, async (v) => {
  if (v) {
    if (closeTimer) { clearTimeout(closeTimer); closeTimer = null }
    closing.value = false
    rendered.value = true
    await nextTick()
    playOpen()
  } else if (rendered.value) {
    playClose()
  }
})

onMounted(async () => {
  if (props.show) {
    rendered.value = true
    await nextTick()
    playOpen()
  }
})

onBeforeUnmount(() => {
  if (closeTimer) clearTimeout(closeTimer)
})

// ---- 内容 / 保存 ----
watch(() => props.show, (v) => {
  if (v) localContent.value = props.content
  copied.value = false
  if (!v) {
    saveFlashing.value = false
    errorFlashing.value = false
  }
})

watch(() => props.content, (v) => {
  if (props.show) localContent.value = v
})

watch(() => props.saving, (saving, previous) => {
  if (previous && !saving && props.show) {
    const success = localContent.value === props.content
    if (success) {
      if (saveFlashTimer) clearTimeout(saveFlashTimer)
      if (errorFlashTimer) {
        clearTimeout(errorFlashTimer)
        errorFlashTimer = null
      }
      errorFlashing.value = false
      saveFlashing.value = true
      saveFlashTimer = setTimeout(() => {
        saveFlashing.value = false
        saveFlashTimer = null
      }, 1100)
    } else {
      if (errorFlashTimer) clearTimeout(errorFlashTimer)
      if (saveFlashTimer) {
        clearTimeout(saveFlashTimer)
        saveFlashTimer = null
      }
      saveFlashing.value = false
      errorFlashing.value = true
      errorFlashTimer = setTimeout(() => {
        errorFlashing.value = false
        errorFlashTimer = null
      }, 1300)
    }
  }
})

// ---- JSON 格式化 / 树形 ----
const JSON_FORMAT_LIMIT = 2 * 1024 * 1024
const formatApplied = ref(false)
const treeMode = ref(false)
const treeData = ref(null)
const jsonError = ref(false)
const jsonTip = ref('')
const jsonTipTone = ref('ok')
let jsonErrorTimer = null
let jsonTipTimer = null

function flashJsonError(label) {
  jsonError.value = true
  if (jsonErrorTimer) clearTimeout(jsonErrorTimer)
  jsonErrorTimer = setTimeout(() => {
    jsonError.value = false
    jsonErrorTimer = null
  }, 1200)
  showTip(label, 'err')
}

function showTip(text, tone = 'ok') {
  jsonTipTone.value = tone
  jsonTip.value = text
  if (jsonTipTimer) clearTimeout(jsonTipTimer)
  jsonTipTimer = setTimeout(() => {
    jsonTip.value = ''
    jsonTipTimer = null
  }, 2200)
}

function resetJsonState() {
  formatApplied.value = false
  treeMode.value = false
  treeData.value = null
  jsonError.value = false
  jsonTip.value = ''
}

watch(() => props.show, (v) => {
  if (v) resetJsonState()
})

function toggleJson() {
  if (formatApplied.value) {
    localContent.value = props.content
    formatApplied.value = false
    if (treeMode.value) refreshTree(true)
    showTip(t('keyEditor.jsonRestored'))
    return
  }
  if (localContent.value.length > JSON_FORMAT_LIMIT) {
    flashJsonError(t('keyEditor.jsonTooLarge'))
    return
  }
  try {
    localContent.value = JSON.stringify(JSON.parse(localContent.value), null, 2)
    formatApplied.value = true
    if (treeMode.value) refreshTree(true)
    showTip(t('keyEditor.jsonFormatted'))
  } catch (e) {
    flashJsonError(t('keyEditor.jsonInvalid'))
  }
}

function refreshTree(silent = false) {
  try {
    treeData.value = JSON.parse(localContent.value)
    return true
  } catch (e) {
    if (!silent) flashJsonError(t('keyEditor.jsonInvalid'))
    treeMode.value = false
    return false
  }
}

function toggleTreeView() {
  if (treeMode.value) {
    treeMode.value = false
    return
  }
  if (refreshTree()) treeMode.value = true
}

function onClose() {
  if (closing.value) return
  emit('close')
}

function onSave() {
  emit('save', localContent.value)
}

async function copy() {
  const ok = await copyToClipboard(localContent.value)
  if (ok) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  }
}
</script>

<style scoped>
.expand-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  animation: overlayIn 170ms ease;
}
.expand-overlay.is-closing {
  animation: overlayOut 200ms ease forwards;
}
@keyframes overlayIn {
  from { background: rgba(0, 0, 0, 0); }
  to { background: rgba(0, 0, 0, 0.45); }
}
@keyframes overlayOut {
  from { background: rgba(0, 0, 0, 0.45); }
  to { background: rgba(0, 0, 0, 0); }
}
.expand-modal {
  background: white;
  border-radius: 12px;
  width: min(1160px, 94vw);
  height: min(88vh, 920px);
  max-height: 92vh;
  display: flex;
  flex-direction: column;
  border: 1px solid rgba(226, 232, 240, 0.84);
  box-shadow: 0 28px 56px rgba(15, 23, 42, 0.2), 0 10px 24px rgba(148, 163, 184, 0.08);
  will-change: transform, opacity;
}
.expand-overlay.theme-dark .expand-modal {
  background: rgba(15, 23, 42, 0.98);
  border-color: rgba(51, 65, 85, 0.9);
  box-shadow: 0 28px 56px rgba(2, 6, 23, 0.46), 0 10px 24px rgba(2, 6, 23, 0.22);
}
.expand-header {
  display: flex;
  align-items: center;
  gap: 10px;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  border-radius: 12px 12px 0 0;
  flex-shrink: 0;
}
.expand-overlay.theme-dark .expand-header {
  border-bottom-color: rgba(51, 65, 85, 0.96);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.995));
}
.expand-title {
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
  color: #1d4ed8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}
.expand-overlay.theme-dark .expand-title {
  color: #93c5fd;
}
.json-tip {
  flex-shrink: 0;
  font-size: 11px;
  line-height: 1;
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(220, 252, 231, 0.9);
  color: #15803d;
  border: 1px solid rgba(134, 239, 172, 0.8);
  animation: tipIn 160ms ease;
}
.json-tip.err {
  background: rgba(254, 226, 226, 0.92);
  color: #b91c1c;
  border-color: rgba(252, 165, 165, 0.9);
}
.expand-overlay.theme-dark .json-tip {
  background: rgba(6, 78, 59, 0.9);
  color: #6ee7b7;
  border-color: rgba(16, 185, 129, 0.45);
}
.expand-overlay.theme-dark .json-tip.err {
  background: rgba(76, 5, 25, 0.9);
  color: #fda4af;
  border-color: rgba(244, 63, 94, 0.4);
}
@keyframes tipIn {
  from { opacity: 0; transform: translateY(-3px); }
  to { opacity: 1; transform: translateY(0); }
}
.expand-header-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.btn-copy-modal,
.btn-save-modal,
.btn-close-modal,
.btn-json-modal {
  display: inline-flex; align-items: center; justify-content: center;
  height: 28px;
  min-height: 28px;
  padding: 0 8px;
  background: rgba(255, 255, 255, 0.94);
  color: #475569;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 6px;
  font-size: 11px;
  line-height: 1;
  font-weight: 600;
  cursor: pointer;
  min-width: 34px;
  box-sizing: border-box;
  transition: background 0.12s, border-color 0.12s, color 0.12s, transform 0.12s;
}
.btn-json-modal {
  font-size: 10.5px;
  padding: 0 10px;
  white-space: nowrap;
}
.btn-json-modal:disabled {
  opacity: 0.6;
  cursor: default;
}
.btn-json-modal:hover:not(:disabled) {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
  transform: translateY(-1px);
}
.btn-json-modal.json-flash-error {
  background: rgba(255, 241, 242, 0.98);
  color: #b91c1c;
  border-color: rgba(253, 164, 175, 0.9);
  animation: modalErrorFlashPulse 0.48s ease;
}
.expand-overlay.theme-dark .btn-json-modal {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.expand-overlay.theme-dark .btn-json-modal:hover:not(:disabled) {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
.expand-overlay.theme-dark .btn-json-modal.json-flash-error {
  background: rgba(76, 20, 27, 0.98);
  color: #ffe4e6;
  border-color: rgba(251, 113, 133, 0.42);
}
.btn-save-modal {
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  border-color: rgba(191, 219, 254, 0.96);
}
.btn-save-modal:hover { background: linear-gradient(180deg, #f8fbff, #eff6ff); border-color: #60a5fa; color: #1d4ed8; transform: translateY(-1px); }
.btn-save-modal.success-flash {
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.96), rgba(22, 163, 74, 0.96));
  color: #f0fdf4;
  border-color: rgba(22, 163, 74, 0.98);
  box-shadow: 0 10px 24px rgba(34, 197, 94, 0.2), 0 0 0 1px rgba(220, 252, 231, 0.22) inset;
  animation: modalSaveFlashPulse 0.42s ease;
}
.btn-save-modal.error-flash {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.96), rgba(220, 38, 38, 0.96));
  color: #fff7f7;
  border-color: rgba(220, 38, 38, 0.98);
  box-shadow: 0 10px 24px rgba(239, 68, 68, 0.18), 0 0 0 1px rgba(254, 202, 202, 0.18) inset;
  animation: modalErrorFlashPulse 0.48s ease;
}
.btn-copy-modal:hover { background: #f8fafc; border-color: #94a3b8; color: #1e293b; transform: translateY(-1px); }
.btn-copy-modal.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  transform: translateY(-1px);
  animation: copyPulse 0.26s ease;
}
.btn-close-modal {
  background: rgba(255, 255, 255, 0.94);
  color: #64748b;
}
.btn-close-modal:hover { color: #334155; border-color: #94a3b8; background: #f8fafc; transform: translateY(-1px); }
.expand-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
}
.expand-content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #1f2937;
}
.expand-textarea {
  width: 100%;
  flex: 1;
  min-height: 420px;
  resize: vertical;
  padding: 12px 14px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #1f2937;
  outline: none;
  box-sizing: border-box;
}
.expand-textarea:focus {
  border-color: rgba(96, 165, 250, 0.92);
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.32);
}
.expand-textarea:disabled {
  background: #f9fafb;
  color: #9ca3af;
}
.expand-overlay.theme-dark .btn-save-modal.success-flash {
  background: linear-gradient(135deg, rgba(9, 59, 44, 0.98), rgba(13, 78, 58, 0.96));
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.44);
  box-shadow: 0 12px 26px rgba(5, 150, 105, 0.22), 0 0 0 1px rgba(167, 243, 208, 0.08) inset;
}

.expand-overlay.theme-dark .btn-save-modal.error-flash {
  background: linear-gradient(135deg, rgba(76, 20, 27, 0.98), rgba(127, 29, 29, 0.96));
  color: #ffe4e6;
  border-color: rgba(251, 113, 133, 0.42);
  box-shadow: 0 12px 26px rgba(190, 24, 93, 0.18), 0 0 0 1px rgba(255, 228, 230, 0.06) inset;
}

.expand-overlay.theme-dark .btn-copy-modal {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.expand-overlay.theme-dark .btn-save-modal {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
}

.expand-overlay.theme-dark .btn-save-modal:hover {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.42);
  color: #dbeafe;
}

.expand-overlay.theme-dark .btn-copy-modal.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}

.expand-overlay.theme-dark .btn-copy-modal:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}

.expand-overlay.theme-dark .btn-close-modal {
  border-color: rgba(71, 85, 105, 0.96);
  color: #cbd5e1;
  background: rgba(15, 23, 42, 0.94);
}

.expand-overlay.theme-dark .btn-close-modal:hover {
  color: #e2e8f0;
  border-color: rgba(96, 165, 250, 0.34);
  background: rgba(30, 41, 59, 0.96);
}

.expand-overlay.theme-dark .expand-content {
  color: #e2e8f0;
}

.expand-overlay.theme-dark .expand-textarea {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}

.expand-overlay.theme-dark .expand-textarea:disabled {
  background: rgba(30, 41, 59, 0.9);
  color: #64748b;
}

@keyframes copyPulse {
  0% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-1px) scale(1.012); }
  100% { transform: translateY(-1px) scale(1); }
}

@keyframes modalSaveFlashPulse {
  0% { transform: translateY(0) scale(1); }
  48% { transform: translateY(-1px) scale(1.02); }
  100% { transform: translateY(0) scale(1); }
}

@keyframes modalErrorFlashPulse {
  0% { transform: translateY(0) scale(1); }
  38% { transform: translateY(-1px) scale(1.015); }
  72% { transform: translateY(0) scale(0.995); }
  100% { transform: translateY(0) scale(1); }
}
</style>
