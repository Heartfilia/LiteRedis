<template>
  <div class="string-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <FloatingMessage :message="msg" :success="ok" />
    <textarea v-model="localVal" rows="10" :disabled="saving" />
    <div class="editor-actions">
      <button class="btn-action" :class="{ copied }" @click="copyValue">{{ copied ? '✓ ' + t('keyEditor.copied') : t('keyEditor.copy') }}</button>
      <button
        :class="[isDirty ? 'btn-primary' : 'btn-action', { 'success-flash': saveFlashing }]"
        @click="save"
        :disabled="saving || !isDirty"
      >
        {{ saving ? t('keyEditor.saving') : t('keyEditor.save') }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { setString } from '../../api/wails.js'
import FloatingMessage from '../common/FloatingMessage.vue'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../../utils/connection.js'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()

const localVal = ref(props.keyValue?.string_val || '')
const saving = ref(false)
const msg = ref('')
const ok = ref(true)
const copied = ref(false)
const saveFlashing = ref(false)
const originalVal = ref(props.keyValue?.string_val || '')
const isDirty = computed(() => localVal.value !== originalVal.value)
let saveFlashTimer = null

function triggerSaveFlash() {
  if (saveFlashTimer) clearTimeout(saveFlashTimer)
  saveFlashing.value = true
  saveFlashTimer = setTimeout(() => {
    saveFlashing.value = false
    saveFlashTimer = null
  }, 1100)
}

async function handleConnectionFailure(error) {
  if (!isConnectionErrorMessage(error)) return false
  await connectionsStore.handleConnectionFailure(workspaceStore.activeConnID, error)
  ok.value = false
  msg.value = formatConnectionLostMessage(error)
  return true
}

watch(() => props.keyValue, (kv) => {
  localVal.value = kv?.string_val || ''
  originalVal.value = kv?.string_val || ''
  msg.value = ''
})

async function copyValue() {
  const result = await copyToClipboard(localVal.value)
  if (result) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  }
}

async function save() {
  saving.value = true
  msg.value = ''
  try {
    const result = await setString(workspaceStore.activeConnID, props.keyValue.key, localVal.value, props.keyValue.ttl)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.saveSuccess') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      originalVal.value = localVal.value
      triggerSaveFlash()
    }
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message || String(e)
    }
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.string-editor {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  height: 100%;
  min-height: 0;
}
:global(.app-layout.theme-dark) .string-editor {
  color: #e2e8f0;
}
textarea {
  flex: 1; resize: none; padding: 9px 11px;
  border: 1px solid #d1d5db; border-radius: 6px;
  font-family: monospace; font-size: 13px; outline: none;
  line-height: 1.6; color: #1f2937;
  transition: border-color 0.15s;
  min-height: 0;
}
:global(.app-layout.theme-dark) .string-editor textarea {
  background: rgba(11, 18, 32, 0.985);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.04);
}
textarea:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.editor-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 5px;
  min-height: 36px;
  padding: 0 12px;
  border-top: 1px solid #e8e8e8;
  margin: -1px -12px 0;
  flex-shrink: 0;
}
:global(.app-layout.theme-dark) .string-editor .editor-actions {
  border-top-color: rgba(51, 65, 85, 0.92);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.995));
}
.btn-action {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 24px;
  padding: 0 9px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
  white-space: nowrap;
}
.btn-action:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-action.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  transform: translateY(-1px);
  animation: copyPulse 0.26s ease;
}
:global(.app-layout.theme-dark) .string-editor .btn-action {
  background: rgba(11, 18, 32, 0.98);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
:global(.app-layout.theme-dark) .string-editor .btn-action.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}
:global(.app-layout.theme-dark) .string-editor .btn-action:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
.btn-action:disabled {
  cursor: default;
  opacity: 0.72;
}
.btn-primary {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 24px;
  padding: 0 10px;
  background: #3b82f6;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s;
  white-space: nowrap;
}
.btn-primary:hover { background: #2563eb; }
.btn-primary:disabled { background: #93c5fd; cursor: not-allowed; }
.btn-primary.success-flash,
.btn-action.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: successFlashPulse 0.42s ease;
}
:global(.app-layout.theme-dark) .string-editor .btn-primary {
  box-shadow: 0 8px 18px rgba(30, 64, 175, 0.22);
}
:global(.app-layout.theme-dark) .string-editor .btn-primary.success-flash,
:global(.app-layout.theme-dark) .string-editor .btn-action.success-flash {
  background: rgba(9, 59, 44, 0.94);
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.5);
  box-shadow: 0 0 0 1px rgba(167, 243, 208, 0.08) inset, 0 10px 22px rgba(5, 150, 105, 0.22);
}
:global(.app-layout.theme-dark) .string-editor .btn-primary:hover {
  background: #2563eb;
  box-shadow: 0 10px 22px rgba(30, 64, 175, 0.28);
}
.string-editor.theme-dark {
  color: #e2e8f0;
}

.string-editor.theme-dark textarea {
  background: rgba(11, 18, 32, 0.985);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: inset 0 1px 0 rgba(148, 163, 184, 0.04);
}

.string-editor.theme-dark .editor-actions {
  border-top-color: rgba(51, 65, 85, 0.92);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.995));
}

.string-editor.theme-dark .btn-action {
  background: rgba(11, 18, 32, 0.98);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.string-editor.theme-dark .btn-action.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
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

.string-editor.theme-dark .btn-action:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}

.string-editor.theme-dark .btn-primary {
  box-shadow: 0 8px 18px rgba(30, 64, 175, 0.22);
}

.string-editor.theme-dark .btn-primary:hover {
  background: #2563eb;
  box-shadow: 0 10px 22px rgba(30, 64, 175, 0.28);
}

@keyframes copyPulse {
  0% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-1px) scale(1.012); }
  100% { transform: translateY(-1px) scale(1); }
}
</style>
