<template>
  <div class="string-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <FloatingMessage :message="msg" :success="ok" />
    <div v-if="props.keyValue?.truncated" class="truncated-hint">⚠️ {{ t('keyEditor.stringTruncated') }}</div>
    <JsonViewer v-if="treeMode" :data="treeData" />
    <textarea v-else v-model="localVal" rows="10" :disabled="saving" />
    <div class="editor-actions">
      <button
        class="btn-action btn-json"
        :class="{ active: formatApplied }"
        :disabled="saving || (!formatApplied && !localVal.trim())"
        @click="toggleJson"
      >{{ formatApplied ? t('keyEditor.jsonRestore') : t('keyEditor.jsonFormat') }}</button>
      <button
        class="btn-action btn-json"
        :class="{ active: treeMode }"
        :disabled="saving || !localVal.trim()"
        @click="toggleTreeView"
      >
        {{ treeMode ? t('keyEditor.jsonTextView') : t('keyEditor.jsonTreeView') }}
      </button>
      <button class="btn-action" :class="{ copied }" @click="copyValue">{{ copied ? '✓ ' + t('keyEditor.copied') : t('keyEditor.copy') }}</button>
      <button
        :class="[isDirty ? 'btn-primary' : 'btn-action', { 'success-flash': saveFlashing, 'error-flash': errorFlashing }]"
        @click="save"
        :disabled="saving || !isDirty"
      >
        {{ saving ? t('keyEditor.saving') : t('keyEditor.save') }}
      </button>
      <button
        v-if="settingsStore.localApiEnabled && settingsStore.debugMode"
        class="btn-action btn-extract"
        :title="t('keyEditor.browserExtract')"
        :disabled="extracting"
        @click="handleBrowserExtract"
        @contextmenu.prevent
      >🌐</button>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { openBrowserExtract } from '../../services/browserExtract.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { createRequestGuard } from '../../utils/requestGuard.js'
import { setString } from '../../api/wails.js'
import FloatingMessage from '../common/FloatingMessage.vue'
import JsonViewer from '../common/JsonViewer.vue'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../../utils/connection.js'
import './editorShared.css'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
const requestGuard = createRequestGuard(() => ({
  connID: workspaceStore.activeConnID,
  db: workspaceStore.currentDB,
  key: props.keyValue?.key || null,
}))

const localVal = ref(props.keyValue?.string_val || '')
const saving = ref(false)
const msg = ref('')
const ok = ref(true)
const copied = ref(false)
const saveFlashing = ref(false)
const errorFlashing = ref(false)
const originalVal = ref(props.keyValue?.string_val || '')
const formatApplied = ref(false)
// 树形只读视图：只在内容能解析成 JSON 时可用
const treeMode = ref(false)
const treeData = ref(null)
const isDirty = computed(() => localVal.value !== originalVal.value)
let saveFlashTimer = null
let errorFlashTimer = null

function triggerSaveFlash() {
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
}

function triggerErrorFlash() {
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

async function handleConnectionFailure(error, request = null) {
  if (request && !requestGuard.isCurrent(request)) return true
  if (!isConnectionErrorMessage(error)) return false
  await connectionsStore.handleConnectionFailure(request?.context.connID || workspaceStore.activeConnID, error)
  if (request && !requestGuard.isCurrent(request)) return true
  ok.value = false
  msg.value = formatConnectionLostMessage(error)
  triggerErrorFlash()
  return true
}

watch(() => props.keyValue, (kv) => {
  requestGuard.invalidateAll()
  saving.value = false
  localVal.value = kv?.string_val || ''
  originalVal.value = kv?.string_val || ''
  formatApplied.value = false
  treeMode.value = false
  treeData.value = null
  msg.value = ''
}, { immediate: true })

onBeforeUnmount(() => {
  requestGuard.invalidateAll()
  if (saveFlashTimer) clearTimeout(saveFlashTimer)
  if (errorFlashTimer) clearTimeout(errorFlashTimer)
})

const JSON_FORMAT_LIMIT = 2 * 1024 * 1024

function formatJson() {
  if (localVal.value.length > JSON_FORMAT_LIMIT) {
    ok.value = false
    msg.value = t('keyEditor.jsonTooLarge')
    setTimeout(() => { msg.value = '' }, 3000)
    return
  }
  try {
    const parsed = JSON.parse(localVal.value)
    localVal.value = JSON.stringify(parsed, null, 2)
    formatApplied.value = true
    ok.value = true
    msg.value = t('keyEditor.jsonFormatted')
    setTimeout(() => { msg.value = '' }, 1800)
  } catch (e) {
    ok.value = false
    msg.value = t('keyEditor.jsonInvalid')
    setTimeout(() => { msg.value = '' }, 2600)
  }
}

function toggleTreeView() {
  if (treeMode.value) {
    treeMode.value = false
    return
  }
  try {
    treeData.value = JSON.parse(localVal.value)
    treeMode.value = true
  } catch (e) {
    ok.value = false
    msg.value = t('keyEditor.jsonInvalid')
    setTimeout(() => { msg.value = '' }, 2600)
  }
}

// 恢复成该 key 在 Redis 中的原始 value（未保存的改动会被丢弃）
function restoreJson() {
  localVal.value = originalVal.value
  formatApplied.value = false
  ok.value = true
  msg.value = t('keyEditor.jsonRestored')
  setTimeout(() => { msg.value = '' }, 1800)
}

function toggleJson() {
  if (formatApplied.value) {
    restoreJson()
    return
  }
  formatJson()
}

async function copyValue() {
  const result = await copyToClipboard(localVal.value)
  if (result) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  }
}

const extracting = ref(false)
async function handleBrowserExtract() {
  extracting.value = true
  try {
    const opened = await openBrowserExtract({
      key: props.keyValue?.key,
      type: 'string',
      position: '0',
      data: { string_val: originalVal.value },
      totalCount: 1,
      hasMore: false,
    })
    ok.value = opened
    msg.value = opened ? t('keyEditor.extractOpened') : t('keyEditor.extractFailed')
    setTimeout(() => { msg.value = '' }, 3000)
  } finally {
    extracting.value = false
  }
}

async function save() {
  const request = requestGuard.begin('save')
  const value = localVal.value
  saving.value = true
  msg.value = ''
  try {
    const result = await setString(request.context.connID, request.context.key, value)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.saveSuccess') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      originalVal.value = value
      // 保存后格式化内容已成为“原始值”，按钮回到「格式化」
      formatApplied.value = false
      triggerSaveFlash()
    } else {
      triggerErrorFlash()
    }
  } catch(e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message || String(e)
      triggerErrorFlash()
    }
  } finally {
    if (requestGuard.isCurrent(request)) saving.value = false
    requestGuard.finish(request)
  }
}
</script>

<style scoped>
.string-editor {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 7px;
  height: 100%;
  min-height: 0;
  background: transparent;
}
.string-editor.theme-dark {
  color: #e2e8f0;
}
.truncated-hint {
  font-size: 12px;
  color: #b45309;
  background: rgba(251, 191, 36, 0.12);
  border: 1px solid rgba(251, 191, 36, 0.35);
  border-radius: 6px;
  padding: 5px 10px;
}
textarea {
  flex: 1; resize: none; padding: 10px 12px;
  border: 1px solid rgba(203, 213, 225, 0.96); border-radius: 8px;
  font-family: monospace; font-size: 13px; outline: none;
  line-height: 1.6; color: #1f2937;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.94));
  transition: border-color 0.15s, box-shadow 0.15s, background 0.15s;
  min-height: 0;
}
.string-editor.theme-dark textarea {
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.985), rgba(11, 18, 32, 0.995));
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.04);
}
textarea:focus { border-color: rgba(96, 165, 250, 0.92); box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.32); background: #fff; }
.string-editor.theme-dark textarea:focus {
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 1));
  box-shadow: 0 0 0 3px rgba(30, 64, 175, 0.2);
}
.editor-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 6px;
  min-height: var(--editor-footer-height);
  padding: 0 12px;
  border-top: 1px solid rgba(226, 232, 240, 0.95);
  margin: -1px -12px 0;
  flex-shrink: 0;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(241, 245, 249, 0.98));
}
.string-editor.theme-dark .editor-actions {
  border-top-color: rgba(51, 65, 85, 0.92);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.995), rgba(8, 15, 29, 1));
}
.btn-action {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 24px;
  padding: 0 9px;
  background: rgba(255, 255, 255, 0.94);
  color: #475569;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, color 0.12s, transform 0.12s;
  white-space: nowrap;
}
.btn-action:hover { background: #f8fafc; border-color: #94a3b8; color: #1e293b; transform: translateY(-1px); }
.btn-action.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  transform: translateY(-1px);
  animation: copyPulse 0.26s ease;
}
.string-editor.theme-dark .btn-action {
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.98), rgba(8, 15, 29, 0.98));
  color: #dbe4f0;
  border-color: rgba(71, 85, 105, 0.96);
}
.string-editor.theme-dark .btn-action.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}
.string-editor.theme-dark .btn-action:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
/* 格式化 / 树形：触发状态高亮 */
.btn-json.active {
  background: rgba(219, 234, 254, 0.98);
  border-color: rgba(96, 165, 250, 0.92);
  color: #1d4ed8;
  box-shadow: 0 0 0 1px rgba(191, 219, 254, 0.7) inset;
}
.btn-json.active:hover {
  background: rgba(219, 234, 254, 1);
  border-color: rgba(59, 130, 246, 0.95);
  color: #1e40af;
}
.string-editor.theme-dark .btn-json.active {
  background: rgba(30, 64, 175, 0.34);
  border-color: rgba(96, 165, 250, 0.56);
  color: #dbeafe;
  box-shadow: 0 0 0 1px rgba(96, 165, 250, 0.24) inset;
}
.btn-action:disabled {
  cursor: default;
  opacity: 0.72;
}
.btn-primary {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 24px;
  padding: 0 10px;
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  border: 1px solid rgba(191, 219, 254, 0.96);
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s, color 0.15s, transform 0.15s;
  white-space: nowrap;
}
.btn-primary:hover { background: linear-gradient(180deg, #f8fbff, #eff6ff); border-color: #60a5fa; color: #1d4ed8; transform: translateY(-1px); }
.btn-primary:disabled { background: #f8fafc; color: #94a3b8; border-color: #d1d5db; cursor: not-allowed; }
.btn-primary.success-flash,
.btn-action.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: successFlashPulse 0.42s ease;
}
.btn-primary.error-flash,
.btn-action.error-flash {
  background: rgba(255, 241, 242, 0.98);
  color: #991b1b;
  border-color: rgba(253, 164, 175, 0.9);
  box-shadow: 0 0 0 1px rgba(254, 202, 202, 0.78) inset, 0 8px 18px rgba(239, 68, 68, 0.14);
  animation: errorFlashPulse 0.48s ease;
}
.string-editor.theme-dark .btn-primary {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(10, 17, 30, 0.98));
  color: #dbeafe;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 8px 18px rgba(30, 64, 175, 0.22);
}
.string-editor.theme-dark .btn-primary.success-flash,
.string-editor.theme-dark .btn-action.success-flash {
  background: rgba(9, 59, 44, 0.94);
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.5);
  box-shadow: 0 0 0 1px rgba(167, 243, 208, 0.08) inset, 0 10px 22px rgba(5, 150, 105, 0.22);
}
.string-editor.theme-dark .btn-primary.error-flash,
.string-editor.theme-dark .btn-action.error-flash {
  background: rgba(76, 20, 27, 0.94);
  color: #ffe4e6;
  border-color: rgba(251, 113, 133, 0.42);
  box-shadow: 0 0 0 1px rgba(255, 228, 230, 0.06) inset, 0 10px 22px rgba(190, 24, 93, 0.18);
}
.string-editor.theme-dark .btn-primary:hover {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  box-shadow: 0 10px 22px rgba(30, 64, 175, 0.28);
}
@keyframes successFlashPulse {
  0% {
    transform: translateY(0) scale(1);
  }

  48% {
    transform: translateY(-1px) scale(1.02);
  }

  100% {
    transform: translateY(0) scale(1);
  }
}

@keyframes errorFlashPulse {
  0% {
    transform: translateY(0) scale(1);
  }

  38% {
    transform: translateY(-1px) scale(1.015);
  }

  72% {
    transform: translateY(0) scale(0.995);
  }
  100% {
    transform: translateY(0) scale(1);
  }
}

@keyframes copyPulse {
  0% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-1px) scale(1.012); }
  100% { transform: translateY(-1px) scale(1); }
}
</style>
