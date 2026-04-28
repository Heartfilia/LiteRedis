<template>
  <div class="key-editor">
    <!-- 未选中 key -->
    <div v-if="!selectedKey" class="empty-state">
      {{ t('keyEditor.selectKeyHint') }}
    </div>

    <!-- 加载中 -->
    <div v-else-if="keyValueLoading" class="empty-state loading-state">
      <div class="spinner"></div>
      <span>{{ t('keyEditor.loading') }}</span>
    </div>

    <!-- 加载错误 -->
    <div v-else-if="keyValueError" class="empty-state error-state">
      <div class="error-icon">⚠</div>
      <div class="error-text">{{ keyValueError }}</div>
      <button class="btn-retry" @click="refreshKey">{{ t('keyEditor.retry') }}</button>
    </div>

    <!-- key 详情 -->
    <template v-else-if="keyValue">
      <!-- 顶部元信息栏 -->
      <div class="key-header">
        <div class="key-meta">
          <!-- key 名点击即复制 -->
          <span
            class="key-name"
            :title="keyCopied ? t('keyEditor.copiedKeyName') : t('keyEditor.copyKeyName')"
            :class="{ copied: keyCopied }"
            @click="copyKeyName"
          >{{ keyValue.key }}</span>
          <span class="type-badge" :style="{ background: typeColor.bg, color: typeColor.text }">
            {{ typeColor.label }}
          </span>
          <span class="ttl-info">
            {{ t('keyEditor.ttl') }}:
            <span v-if="editingTTL">
              <input v-model.number="ttlInput" type="number" class="ttl-input" />
              <button class="btn-xs" @click="saveTTL">✓</button>
              <button class="btn-xs" @click="editingTTL = false">✕</button>
            </span>
            <span v-else class="ttl-val" @click="startTTLEdit">
              {{ displayTTL }}
            </span>
          </span>
        </div>
        <div class="key-actions">
          <button class="btn-sm" @click="startRename">{{ t('keyEditor.rename') }}</button>
          <button class="btn-sm" @click="refreshKey">{{ t('keyEditor.refresh') }}</button>
          <div class="delete-wrap">
            <button class="btn-sm danger" @click="confirmingDelete = true">{{ t('keyEditor.delete') }}</button>
            <div v-if="confirmingDelete" class="delete-popover">
              <div class="delete-popover-arrow"></div>
              <div class="delete-popover-content">
                <span class="delete-popover-text">{{ t('keyEditor.confirmDelete') }}</span>
                <div class="delete-popover-btns">
                  <button class="btn-xs btn-confirm-yes" @click="doDelete">✓</button>
                  <button class="btn-xs btn-confirm-no" @click="confirmingDelete = false">✕</button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 重命名输入 -->
      <div v-if="renamingKey" class="rename-bar">
        <input v-model="newKeyName" :placeholder="t('keyEditor.renameInput')" @keydown.enter="doRename" @keydown.esc="renamingKey = false" />
        <button class="btn-xs" @click="doRename">{{ t('keyEditor.renameConfirm') }}</button>
        <button class="btn-xs" @click="renamingKey = false">{{ t('keyEditor.renameCancel') }}</button>
        <span v-if="renameMsg" class="rename-msg">{{ renameMsg }}</span>
      </div>

      <!-- Value 编辑器（按类型分发） -->
      <div class="editor-body">
        <StringEditor v-if="keyValue.type === 'string'" :keyValue="keyValue" />
        <HashEditor   v-else-if="keyValue.type === 'hash'"   :keyValue="keyValue" />
        <ListEditor   v-else-if="keyValue.type === 'list'"   :keyValue="keyValue" />
        <SetEditor    v-else-if="keyValue.type === 'set'"    :keyValue="keyValue" />
        <ZSetEditor   v-else-if="keyValue.type === 'zset'"   :keyValue="keyValue" />
        <StreamEditor v-else-if="keyValue.type === 'stream'" :keyValue="keyValue" />
        <div v-else class="empty-state">{{ t('keyEditor.unsupportedType', { type: keyValue.type }) }}</div>
      </div>
    </template>

    <div v-else class="empty-state">
      <button class="btn-retry" @click="refreshKey">{{ t('keyEditor.reload') }}</button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { getTypeColor } from '../../utils/typeColors.js'
import StringEditor from './StringEditor.vue'
import HashEditor from './HashEditor.vue'
import ListEditor from './ListEditor.vue'
import SetEditor from './SetEditor.vue'
import ZSetEditor from './ZSetEditor.vue'
import StreamEditor from './StreamEditor.vue'

const { t } = useI18n()

const workspaceStore = useWorkspaceStore()
const selectedKey = computed(() => workspaceStore.selectedKey)
const keyValue = computed(() => workspaceStore.keyValue)
const keyValueLoading = computed(() => workspaceStore.keyValueLoading)
const keyValueError = computed(() => workspaceStore.keyValueError)
const typeColor = computed(() => getTypeColor(keyValue.value?.type))

// 切换 key 时重置删除确认状态
watch(selectedKey, () => { confirmingDelete.value = false })

// 复制 key 名（点击 key-name 触发）
const keyCopied = ref(false)
async function copyKeyName() {
  const k = keyValue.value?.key || selectedKey.value || ''
  if (!k) return
  await copyToClipboard(k)
  keyCopied.value = true
  setTimeout(() => { keyCopied.value = false }, 1500)
}

// TTL 编辑
const editingTTL = ref(false)
const ttlInput = ref(0)
function startTTLEdit() {
  ttlInput.value = keyValue.value?.ttl ?? -1
  editingTTL.value = true
}
async function saveTTL() {
  await workspaceStore.updateTTL(ttlInput.value)
  editingTTL.value = false
}

// 重命名
const renamingKey = ref(false)
const newKeyName = ref('')
const renameMsg = ref('')
function startRename() {
  newKeyName.value = selectedKey.value
  renamingKey.value = true
  renameMsg.value = ''
}
async function doRename() {
  if (!newKeyName.value.trim()) return
  const result = await workspaceStore.renameCurrentKey(newKeyName.value.trim())
  if (result?.success) {
    renamingKey.value = false
  } else {
    renameMsg.value = result?.message || t('keyEditor.renameFailed')
  }
}

// 刷新
async function refreshKey() {
  confirmingDelete.value = false
  if (selectedKey.value) {
    await workspaceStore.selectKey(selectedKey.value)
  }
}

// 删除（二次确认）
const confirmingDelete = ref(false)
async function doDelete() {
  confirmingDelete.value = false
  await workspaceStore.deleteCurrentKey()
}

// TTL 自动更新
const liveTTL = ref(null)
let ttlTimer = null

const displayTTL = computed(() => {
  if (keyValue.value?.ttl === -1) return t('keyEditor.permanent')
  const ttl = liveTTL.value !== null ? liveTTL.value : keyValue.value?.ttl
  if (ttl === null || ttl === undefined) return '—'
  return ttl + 's'
})

watch(() => keyValue.value?.ttl, (ttl) => {
  liveTTL.value = ttl ?? null
  if (ttlTimer) { clearInterval(ttlTimer); ttlTimer = null }
  if (typeof ttl === 'number' && ttl > 0) {
    ttlTimer = setInterval(() => {
      if (liveTTL.value > 0) {
        liveTTL.value--
      } else {
        clearInterval(ttlTimer)
        ttlTimer = null
      }
    }, 1000)
  }
}, { immediate: true })

onBeforeUnmount(() => {
  if (ttlTimer) { clearInterval(ttlTimer); ttlTimer = null }
})
</script>

<style scoped>
.key-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: white;
}
.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #9ca3af;
  font-size: 13px;
  gap: 10px;
}
.loading-state { color: #6b7280; }
.spinner {
  width: 24px;
  height: 24px;
  border: 3px solid #e5e7eb;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
.error-state { color: #991b1b; }
.error-icon { font-size: 28px; }
.error-text {
  max-width: 400px;
  text-align: center;
  font-size: 12px;
  font-family: monospace;
  background: #fff1f2;
  padding: 8px 14px;
  border-radius: 6px;
  word-break: break-all;
  color: #991b1b;
}
.btn-retry {
  padding: 5px 16px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  background: white;
  color: #374151;
  transition: background 0.12s;
}
.btn-retry:hover { background: #f3f4f6; }
.key-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  flex-shrink: 0;
}
.key-meta { display: flex; align-items: center; gap: 8px; min-width: 0; flex: 1; }
.key-name {
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
  color: #1d4ed8;
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
  border-radius: 4px;
  padding: 2px 6px;
  transition: background 0.15s, color 0.15s;
}
.key-name:hover { background: #dbeafe; }
.key-name.copied { background: #dcfce7; color: #16a34a; }
.type-badge {
  font-size: 10px;
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 600;
  flex-shrink: 0;
  letter-spacing: 0.3px;
}
.ttl-info { font-size: 12px; color: #6b7280; display: flex; align-items: center; gap: 4px; }
.ttl-val { color: #d97706; cursor: pointer; text-decoration: underline dotted; font-weight: 500; }
.ttl-input { width: 70px; padding: 2px 6px; border: 1px solid #3b82f6; border-radius: 4px; font-size: 12px; outline: none; }
.key-actions { display: flex; gap: 6px; flex-shrink: 0; align-items: center; }
.btn-sm {
  padding: 4px 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  background: white;
  color: #374151;
  font-weight: 500;
  transition: background 0.12s, border-color 0.12s;
}
.btn-sm:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-sm.danger { color: #dc2626; border-color: #fca5a5; }
.btn-sm.danger:hover { background: #dc2626; color: #fff; border-color: #dc2626; }
.btn-sm.danger-confirm { background: #dc2626; color: white; border-color: #dc2626; }
.btn-sm.danger-confirm:hover { background: #b91c1c; border-color: #b91c1c; }
.delete-wrap { position: relative; display: inline-flex; }
.delete-popover {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 100;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.12);
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}
.delete-popover-arrow {
  position: absolute;
  top: -5px;
  right: 10px;
  width: 10px;
  height: 10px;
  background: white;
  border-left: 1px solid #e5e7eb;
  border-top: 1px solid #e5e7eb;
  transform: rotate(45deg);
}
.delete-popover-content {
  display: flex;
  align-items: center;
  gap: 8px;
}
.delete-popover-text {
  font-size: 12px;
  color: #dc2626;
  font-weight: 500;
}
.delete-popover-btns {
  display: flex;
  gap: 4px;
}
.btn-confirm-yes {
  color: #16a34a;
  border-color: #16a34a;
}
.btn-confirm-yes:hover { background: #16a34a; color: white; }
.btn-confirm-no {
  color: #dc2626;
  border-color: #dc2626;
}
.btn-confirm-no:hover { background: #dc2626; color: white; }
.btn-xs {
  padding: 2px 8px;
  border: 1px solid #d1d5db;
  border-radius: 4px;
  cursor: pointer;
  font-size: 11px;
  background: white;
  color: #374151;
  transition: background 0.12s;
}
.btn-xs:hover { background: #f3f4f6; }
.rename-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  background: #fffbeb;
  border-bottom: 1px solid #fde68a;
  flex-shrink: 0;
}
.rename-bar input {
  flex: 1;
  max-width: 300px;
  padding: 4px 8px;
  border: 1px solid #d1d5db;
  border-radius: 5px;
  font-size: 12px;
  font-family: monospace;
  outline: none;
}
.rename-bar input:focus { border-color: #3b82f6; }
.rename-msg { font-size: 12px; color: #991b1b; }
.editor-body {
  flex: 1;
  overflow: hidden;
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
}
</style>
