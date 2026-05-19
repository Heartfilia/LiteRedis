<template>
  <div class="string-editor">
    <FloatingMessage :message="msg" :success="ok" />
    <textarea v-model="localVal" rows="10" :disabled="saving" />
    <div class="editor-actions">
      <button class="btn-action" @click="copyValue">{{ copied ? '✓ ' + t('keyEditor.copied') : t('keyEditor.copy') }}</button>
      <button
        :class="isDirty ? 'btn-primary' : 'btn-action'"
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
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { setString } from '../../api/wails.js'
import FloatingMessage from '../common/FloatingMessage.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const { t } = useI18n()

const localVal = ref(props.keyValue?.string_val || '')
const saving = ref(false)
const msg = ref('')
const ok = ref(true)
const copied = ref(false)
const originalVal = ref(props.keyValue?.string_val || '')
const isDirty = computed(() => localVal.value !== originalVal.value)

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
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.saveSuccess') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      originalVal.value = localVal.value
    }
  } catch(e) {
    ok.value = false
    msg.value = e.message || String(e)
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
  gap: 8px;
  height: 100%;
  min-height: 0;
}
textarea {
  flex: 1; resize: none; padding: 10px 12px;
  border: 1px solid #d1d5db; border-radius: 6px;
  font-family: monospace; font-size: 13px; outline: none;
  line-height: 1.6; color: #1f2937;
  transition: border-color 0.15s;
  min-height: 0;
}
textarea:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
  padding: 0 0 8px;
  flex-shrink: 0;
}
.btn-action {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 5px 13px;
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
.btn-action:disabled {
  cursor: default;
  opacity: 0.72;
}
.btn-primary {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 5px 16px;
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
</style>
