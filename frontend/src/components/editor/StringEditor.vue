<template>
  <div class="string-editor">
    <textarea v-model="localVal" rows="10" :disabled="saving" />
    <div class="editor-actions">
      <button class="btn-action" @click="openExpand">⛶ {{ t('keyEditor.expand') }}</button>
      <button class="btn-action" @click="copyValue">{{ copied ? '✓ ' + t('keyEditor.copied') : '📋 ' + t('keyEditor.copy') }}</button>
      <button class="btn-primary" @click="save" :disabled="saving">{{ saving ? t('keyEditor.saving') : t('keyEditor.save') }}</button>
    </div>
    <div v-if="msg" :class="['msg', ok ? 'ok' : 'err']">{{ msg }}</div>

    <ExpandModal
      :show="expandShow"
      :title="props.keyValue?.key || 'string'"
      :content="localVal"
      @close="expandShow = false"
    />
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { setString } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const { t } = useI18n()

const localVal = ref(props.keyValue?.string_val || '')
const saving = ref(false)
const msg = ref('')
const ok = ref(true)
const copied = ref(false)
const expandShow = ref(false)

watch(() => props.keyValue, (kv) => {
  localVal.value = kv?.string_val || ''
  msg.value = ''
})

function openExpand() {
  expandShow.value = true
}

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
  } catch(e) {
    ok.value = false
    msg.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.string-editor { display: flex; flex-direction: column; gap: 8px; height: 100%; }
textarea {
  flex: 1; resize: none; padding: 10px 12px;
  border: 1px solid #d1d5db; border-radius: 6px;
  font-family: monospace; font-size: 13px; outline: none;
  line-height: 1.6; color: #1f2937;
  transition: border-color 0.15s;
}
textarea:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.editor-actions { display: flex; justify-content: flex-end; gap: 6px; }
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
.msg { font-size: 12px; padding: 5px 10px; border-radius: 6px; }
.ok { background: #f0fdf4; color: #166534; }
.err { background: #fff1f2; color: #991b1b; }
</style>
