<template>
  <div class="string-editor">
    <textarea v-model="localVal" rows="10" :disabled="saving" />
    <div class="editor-actions">
      <button class="btn-action" @click="openExpand">⛶ 展开</button>
      <button class="btn-action" @click="copyValue">{{ copied ? '✓ 已复制' : '📋 复制' }}</button>
      <button class="btn-primary" @click="save" :disabled="saving">{{ saving ? '保存中...' : '保存' }}</button>
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
import { copyToClipboard } from '../../utils/clipboard.js'
import { setString } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()

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
    msg.value = result.success ? '保存成功' : (result.message || '保存失败')
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
textarea { flex: 1; resize: none; padding: 8px; border: 1px solid #ddd; border-radius: 4px; font-family: monospace; font-size: 13px; outline: none; }
textarea:focus { border-color: #4e9af1; }
.editor-actions { display: flex; justify-content: flex-end; gap: 6px; }
.btn-action {
  padding: 5px 14px;
  background: white;
  color: #555;
  border: 1px solid #ddd;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
}
.btn-action:hover { background: #f0f0f0; }
.btn-primary { padding: 5px 16px; background: #4e9af1; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 13px; }
.btn-primary:hover { background: #3a85e0; }
.btn-primary:disabled { background: #aaa; }
.msg { font-size: 12px; padding: 4px 8px; border-radius: 4px; }
.ok { background: #e8f5e9; color: #2e7d32; }
.err { background: #fce4ec; color: #b71c1c; }
</style>
