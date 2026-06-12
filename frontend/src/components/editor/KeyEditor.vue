<template>
  <div class="key-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
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
          <button
            ref="ttlTriggerRef"
            class="ttl-chip"
            :class="ttlChipClass"
            type="button"
            @click="toggleTTLEdit"
          >
            TTL
          </button>
        </div>
        <div class="key-actions">
          <button class="btn-tiny icon-btn top-action-btn" :title="t('keyEditor.rename')" @click="startRename">
            <svg viewBox="0 0 24 24" width="15" height="15" aria-hidden="true">
              <path
                d="M4.75 19.25l4.05-.88L18.35 8.82a2.14 2.14 0 000-3.03l-.14-.14a2.14 2.14 0 00-3.03 0L5.63 15.2l-.88 4.05z"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M13.75 7.08l3.17 3.17"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
              />
            </svg>
          </button>
          <button class="btn-tiny icon-btn top-action-btn" :title="t('keyEditor.refresh')" @click="refreshKey">
            <svg viewBox="0 0 24 24" width="15" height="15" aria-hidden="true">
              <path
                d="M19.25 10.25a7.35 7.35 0 00-13.38-3.1L4.75 8.72"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M4.7 4.55v4.2h4.2"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M4.75 13.75a7.35 7.35 0 0013.38 3.1l1.12-1.57"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
              <path
                d="M19.3 19.45v-4.2h-4.2"
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <InlineDeleteConfirm
            class="header-delete-confirm header-action-delete"
            :label="t('keyEditor.delete')"
            :confirm-text="t('keyEditor.confirmDelete')"
            :reset-token="selectedKey || ''"
            :icon-only="true"
            @confirm="doDelete"
          />
        </div>
      </div>

      <Teleport to="body">
        <div
          v-if="editingTTL"
          ref="ttlPopoverRef"
          class="ttl-popover"
          :class="`theme-${settingsStore.themeMode || 'light'}`"
          :style="ttlPopoverStyle"
        >
          <div class="ttl-popover-title">TTL</div>
          <div class="ttl-popover-row">
            <input
              v-model.number="ttlInput"
              type="number"
              class="ttl-input"
              @keydown.enter.prevent="saveTTL"
              @keydown.esc.prevent="closeTTLEdit"
            />
            <button class="btn-xs ttl-popover-btn" @click="saveTTL">✓</button>
            <button class="btn-xs ttl-popover-btn" @click="closeTTLEdit">✕</button>
          </div>
        </div>
      </Teleport>

      <!-- 重命名输入 -->
      <div v-if="renamingKey" class="rename-bar">
        <input v-model="newKeyName" :placeholder="t('keyEditor.renameInput')" @keydown.enter="doRename" @keydown.esc="cancelRename" />
        <button class="btn-xs" :disabled="!canConfirmRename" @click="doRename">{{ t('keyEditor.renameConfirm') }}</button>
        <button class="btn-xs" @click="cancelRename">{{ t('keyEditor.renameCancel') }}</button>
        <span v-if="renameMsg" class="rename-msg">{{ renameMsg }}</span>
      </div>

      <!-- Value 编辑器（按类型分发） -->
      <div class="editor-body">
        <div v-if="keyValueRefreshing && keyValue.type === 'list'" class="refresh-overlay">
          <div class="refresh-overlay-chip">
            <div class="spinner refresh-spinner"></div>
            <span>{{ t('keyEditor.loading') }}</span>
          </div>
        </div>
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
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick, Teleport } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { getKeyInfo } from '../../api/wails.js'
import { getTypeColor } from '../../utils/typeColors.js'
import StringEditor from './StringEditor.vue'
import HashEditor from './HashEditor.vue'
import ListEditor from './ListEditor.vue'
import SetEditor from './SetEditor.vue'
import ZSetEditor from './ZSetEditor.vue'
import StreamEditor from './StreamEditor.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import { isConnectionErrorMessage } from '../../utils/connection.js'

const { t } = useI18n()

const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
const settingsStore = useSettingsStore()
const selectedKey = computed(() => workspaceStore.selectedKey)
const keyValue = computed(() => workspaceStore.keyValue)
const keyValueLoading = computed(() => workspaceStore.keyValueLoading)
const keyValueRefreshing = computed(() => workspaceStore.keyValueRefreshing)
const keyValueError = computed(() => workspaceStore.keyValueError)
const typeColor = computed(() => getTypeColor(keyValue.value?.type))
const ttlTriggerRef = ref(null)
const ttlPopoverRef = ref(null)
const ttlPopoverStyle = ref({})

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
  ttlInput.value = currentTTLValue()
  editingTTL.value = true
  nextTick(updateTTLPopoverPosition)
}
function closeTTLEdit() {
  editingTTL.value = false
}
function toggleTTLEdit() {
  if (editingTTL.value) {
    closeTTLEdit()
    return
  }
  startTTLEdit()
}
async function saveTTL() {
  await workspaceStore.updateTTL(ttlInput.value)
  closeTTLEdit()
}

// 重命名
const renamingKey = ref(false)
const newKeyName = ref('')
const renameMsg = ref('')
const canConfirmRename = computed(() => {
  const next = newKeyName.value.trim()
  return !!next && next !== (selectedKey.value || '')
})
function startRename() {
  newKeyName.value = selectedKey.value
  renamingKey.value = true
  renameMsg.value = ''
}
function cancelRename() {
  renamingKey.value = false
  newKeyName.value = ''
  renameMsg.value = ''
}
async function doRename() {
  if (!canConfirmRename.value) return
  const result = await workspaceStore.renameCurrentKey(newKeyName.value.trim())
  if (result?.success) {
    cancelRename()
  } else {
    renameMsg.value = result?.message || t('keyEditor.renameFailed')
  }
}

// 刷新
async function refreshKey() {
  if (!selectedKey.value) return
  const preserveCurrentValue = keyValue.value?.type === 'list'
  if (preserveCurrentValue) {
    try {
      const info = await getKeyInfo(workspaceStore.activeConnID, selectedKey.value)
      if (workspaceStore.keyValue && workspaceStore.keyValue.key === selectedKey.value) {
        workspaceStore.keyValue = {
          ...workspaceStore.keyValue,
          ttl: info.ttl,
          total_count: typeof info.count === 'number' ? info.count : workspaceStore.keyValue.total_count,
        }
      }
    } catch (e) {}
  }
  await workspaceStore.selectKey(selectedKey.value, { preserveCurrentValue })
  if (keyValueError.value && isConnectionErrorMessage(keyValueError.value)) {
    await connectionsStore.handleConnectionFailure(workspaceStore.activeConnID, keyValueError.value)
  }
}

async function doDelete() {
  await workspaceStore.deleteCurrentKey()
}

// TTL 自动更新
const liveTTL = ref(null)
let ttlTimer = null

const ttlChipClass = computed(() => {
  const ttl = liveTTL.value !== null ? liveTTL.value : keyValue.value?.ttl
  return {
    active: typeof ttl === 'number' && ttl >= 0,
    permanent: ttl === -1,
    missing: ttl === -2 || ttl === null || ttl === undefined,
  }
})

function currentTTLValue() {
  const ttl = liveTTL.value !== null ? liveTTL.value : keyValue.value?.ttl
  if (ttl === null || ttl === undefined) return -2
  return ttl
}

function updateTTLPopoverPosition() {
  const rect = ttlTriggerRef.value?.getBoundingClientRect()
  if (!rect) return
  ttlPopoverStyle.value = {
    top: `${rect.bottom + 8}px`,
    left: `${rect.left}px`,
  }
}

function handleTTLOutsidePointer(event) {
  if (!editingTTL.value) return
  const target = event.target
  if (ttlTriggerRef.value?.contains(target)) return
  if (ttlPopoverRef.value?.contains(target)) return
  closeTTLEdit()
}

function handleTTLViewportChange() {
  if (editingTTL.value) {
    updateTTLPopoverPosition()
  }
}

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

watch(selectedKey, () => {
  cancelRename()
})

onBeforeUnmount(() => {
  if (ttlTimer) { clearInterval(ttlTimer); ttlTimer = null }
})

onMounted(() => {
  document.addEventListener('pointerdown', handleTTLOutsidePointer, true)
  window.addEventListener('resize', handleTTLViewportChange)
  window.addEventListener('scroll', handleTTLViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleTTLOutsidePointer, true)
  window.removeEventListener('resize', handleTTLViewportChange)
  window.removeEventListener('scroll', handleTTLViewportChange, true)
})
</script>

<style scoped>
.key-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
  background:
    radial-gradient(circle at top right, rgba(219, 234, 254, 0.24), transparent 28%),
    linear-gradient(180deg, rgba(248, 250, 252, 0.4), rgba(255, 255, 255, 0.94) 14%, #ffffff 40%);
}
.key-editor.theme-dark {
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.08), transparent 18%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.99), rgba(11, 18, 32, 0.995) 26%, rgba(2, 6, 23, 0.995) 52%);
}
.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #9ca3af;
  font-size: 13px;
  gap: 12px;
  padding: 24px;
  text-align: center;
}
.loading-state { color: #6b7280; }
.key-editor.theme-dark .loading-state {
  color: #94a3b8;
}
.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid #e5e7eb;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
.error-state { color: #991b1b; }
.key-editor.theme-dark .error-state {
  color: #fca5a5;
}
.error-icon { font-size: 28px; }
.error-text {
  max-width: 520px;
  text-align: center;
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  background: linear-gradient(180deg, rgba(255, 241, 242, 0.98), rgba(254, 226, 226, 0.98));
  padding: 10px 14px;
  border-radius: 10px;
  border: 1px solid rgba(254, 202, 202, 0.9);
  word-break: break-all;
  color: #991b1b;
  box-shadow: 0 10px 24px rgba(248, 113, 113, 0.08);
}
.btn-retry {
  min-height: 32px;
  padding: 0 16px;
  border: 1px solid rgba(191, 219, 254, 0.92);
  border-radius: 10px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  box-shadow: 0 10px 20px rgba(191, 219, 254, 0.18);
  transition: background 0.16s ease, transform 0.16s ease, box-shadow 0.16s ease;
}
.btn-retry:hover {
  background: linear-gradient(180deg, #f8fbff, #eff6ff);
  transform: translateY(-1px);
  box-shadow: 0 12px 22px rgba(191, 219, 254, 0.24);
}
.key-editor.theme-dark .error-text {
  background: linear-gradient(180deg, rgba(127, 29, 29, 0.34), rgba(69, 10, 10, 0.28));
  border-color: rgba(248, 113, 113, 0.24);
  color: #fecaca;
  box-shadow: 0 14px 28px rgba(2, 6, 23, 0.18);
}
.key-editor.theme-dark .btn-retry {
  border-color: rgba(71, 85, 105, 0.96);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  box-shadow: 0 10px 20px rgba(2, 6, 23, 0.26);
}
.key-editor.theme-dark .btn-retry:hover {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.18), rgba(30, 41, 59, 0.98));
  box-shadow: 0 12px 24px rgba(2, 6, 23, 0.34);
}
.key-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 7px 10px;
  padding: 9px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.95);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(255, 255, 255, 0.92));
  flex-shrink: 0;
  backdrop-filter: blur(10px);
}
.key-editor.theme-dark .key-header {
  border-bottom-color: rgba(51, 65, 85, 0.95);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.995));
  backdrop-filter: none;
}
.key-meta { display: flex; align-items: center; gap: 7px; min-width: 0; flex: 1 1 420px; flex-wrap: wrap; }
.key-name {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 13px;
  font-weight: 600;
  color: #1e40af;
  max-width: min(100%, 720px);
  white-space: normal;
  word-break: break-all;
  overflow-wrap: anywhere;
  line-height: 1.45;
  cursor: pointer;
  border-radius: 8px;
  padding: 3px 9px;
  background: rgba(239, 246, 255, 0.75);
  box-shadow: inset 0 0 0 1px rgba(191, 219, 254, 0.82);
  transition: background 0.14s ease, color 0.14s ease, box-shadow 0.14s ease, transform 0.14s ease;
}
.key-editor.theme-dark .key-name {
  color: #93c5fd;
  background: rgba(30, 41, 59, 0.92);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.28);
}
.key-name:hover {
  background: rgba(219, 234, 254, 0.92);
  box-shadow: inset 0 0 0 1px rgba(147, 197, 253, 0.9);
}
.key-editor.theme-dark .key-name:hover {
  background: rgba(30, 64, 175, 0.2);
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.5);
}
.key-name.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.96), 0 0 0 1px rgba(191, 219, 254, 0.52);
  transform: translateY(-1px);
  animation: keyCopyPulse 0.26s ease;
}
.key-editor.theme-dark .key-name.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  box-shadow: inset 0 0 0 1px rgba(147, 197, 253, 0.72), 0 0 14px rgba(59, 130, 246, 0.22);
}
.key-name.copied {
  background: rgba(220, 252, 231, 0.95);
  color: #15803d;
  box-shadow: inset 0 0 0 1px rgba(134, 239, 172, 0.92);
}
.type-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 22px;
  height: 22px;
  font-size: 10px;
  padding: 0 8px;
  border-radius: 999px;
  font-weight: 600;
  flex-shrink: 0;
  letter-spacing: 0.04em;
  line-height: 1;
  box-sizing: border-box;
}
.ttl-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 22px;
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(255, 255, 255, 0.88);
  color: #94a3b8;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.04em;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.16s ease, color 0.16s ease, border-color 0.16s ease, transform 0.16s ease;
}
.key-editor.theme-dark .ttl-chip {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(11, 18, 32, 0.98);
  color: #94a3b8;
}
.ttl-chip.active {
  color: #c2410c;
  background: rgba(255, 247, 237, 0.96);
  border-color: rgba(253, 186, 116, 0.92);
}
.ttl-chip.permanent {
  color: #94a3b8;
}
.ttl-chip.missing {
  color: #b91c1c;
  background: rgba(255, 241, 242, 0.9);
  border-color: rgba(254, 202, 202, 0.9);
}
.ttl-chip:hover {
  transform: translateY(-1px);
  border-color: rgba(148, 163, 184, 0.96);
}
.key-editor.theme-dark .ttl-chip.active {
  color: #fdba74;
  background: rgba(124, 45, 18, 0.28);
  border-color: rgba(251, 146, 60, 0.34);
}
.key-editor.theme-dark .ttl-chip.missing {
  color: #fda4af;
  background: rgba(127, 29, 29, 0.24);
  border-color: rgba(248, 113, 113, 0.28);
}
.ttl-popover {
  position: fixed;
  z-index: 10020;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 164px;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 18px 36px rgba(148, 163, 184, 0.22);
  backdrop-filter: blur(10px);
}
.ttl-popover.theme-dark {
  border-color: rgba(51, 65, 85, 0.96);
  background: rgba(15, 23, 42, 0.98);
  box-shadow: 0 18px 36px rgba(2, 6, 23, 0.42);
}
.ttl-popover-title {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #64748b;
}
.ttl-popover.theme-dark .ttl-popover-title {
  color: #94a3b8;
}
.ttl-popover-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.ttl-input {
  width: 76px;
  height: 28px;
  padding: 0 9px;
  border: 1px solid #60a5fa;
  border-radius: 6px;
  font-size: 12px;
  outline: none;
  background: rgba(255, 255, 255, 0.95);
}
.ttl-popover.theme-dark .ttl-input {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(59, 130, 246, 0.82);
}
.ttl-popover-btn {
  min-width: 28px;
  padding: 0;
}
.key-actions {
  display: flex;
  gap: 5px;
  flex-shrink: 0;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.btn-tiny,
.btn-sm {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 30px;
  height: 30px;
  width: 56px;
  min-width: 56px;
  padding: 0;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1;
  background: rgba(255, 255, 255, 0.96);
  color: #475569;
  font-weight: 600;
  box-sizing: border-box;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease, transform 0.16s ease;
}
.key-editor.theme-dark .btn-tiny,
.key-editor.theme-dark .btn-sm,
.key-editor.theme-dark .btn-xs {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(11, 18, 32, 0.98);
  color: #cbd5e1;
}
.key-editor.theme-dark .btn-tiny:hover,
.key-editor.theme-dark .btn-sm:hover,
.key-editor.theme-dark .btn-xs:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(96, 165, 250, 0.34);
  color: #e2e8f0;
}
.icon-btn {
  font-size: 0;
}
.key-actions .top-action-btn,
:deep(.key-actions .header-action-delete),
:deep(.key-actions .header-action-delete > .btn-tiny) {
  width: 32px;
  min-width: 32px;
  height: 30px;
  min-height: 30px;
  padding: 0;
  border-radius: 10px;
  flex: 0 0 32px;
}
.key-actions .top-action-btn,
:deep(.key-actions .header-action-delete > .btn-tiny) {
  border-color: rgba(203, 213, 225, 0.92);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.94));
  color: #64748b;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), inset 0 1px 0 rgba(255, 255, 255, 0.9);
}
.key-actions .top-action-btn:hover,
:deep(.key-actions .header-action-delete > .btn-tiny:hover) {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(241, 245, 249, 0.96));
  border-color: rgba(148, 163, 184, 0.96);
  color: #334155;
  transform: translateY(-1px);
}
.key-editor.theme-dark .key-actions .top-action-btn,
.key-editor.theme-dark :deep(.key-actions .header-action-delete > .btn-tiny) {
  border-color: rgba(71, 85, 105, 0.92);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.96));
  color: #cbd5e1;
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.24), inset 0 1px 0 rgba(148, 163, 184, 0.04);
}
.key-editor.theme-dark .key-actions .top-action-btn:hover,
.key-editor.theme-dark :deep(.key-actions .header-action-delete > .btn-tiny:hover) {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(30, 64, 175, 0.16));
  border-color: rgba(96, 165, 250, 0.32);
  color: #f8fafc;
}
:deep(.key-actions .header-action-delete > .btn-tiny.danger-confirm) {
  background: linear-gradient(180deg, #ef4444, #dc2626);
  border-color: #dc2626;
  color: #fff;
}
:deep(.key-actions .header-action-delete > .btn-tiny.danger) {
  border-color: rgba(203, 213, 225, 0.92);
  color: #64748b;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), inset 0 0 0 1px rgba(255, 255, 255, 0.18);
}
.key-editor.theme-dark :deep(.key-actions .header-action-delete > .btn-tiny.danger) {
  border-color: rgba(71, 85, 105, 0.92);
  color: #cbd5e1;
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.24), inset 0 0 0 1px rgba(148, 163, 184, 0.04);
}
.key-actions .top-action-btn svg,
:deep(.key-actions .header-action-delete > .btn-tiny.icon-only svg) {
  width: 15px;
  height: 15px;
  display: block;
  overflow: visible;
}
.btn-tiny:hover,
.btn-sm:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
  transform: translateY(-1px);
}
.btn-sm.danger,
.btn-tiny.danger { color: #dc2626; border-color: #fca5a5; }
.btn-sm.danger:hover,
.btn-tiny.danger:hover { background: #dc2626; color: #fff; border-color: #dc2626; }
.btn-confirm-yes {
  color: #16a34a;
  border-color: rgba(34, 197, 94, 0.82);
  background: rgba(240, 253, 244, 0.96);
}
.btn-confirm-yes:hover { background: #16a34a; color: white; }
.btn-confirm-no {
  color: #dc2626;
  border-color: rgba(248, 113, 113, 0.82);
  background: rgba(254, 242, 242, 0.96);
}
.btn-confirm-no:hover { background: #dc2626; color: white; }
.btn-xs {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 26px;
  height: 26px;
  padding: 0 9px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  cursor: pointer;
  font-size: 11px;
  line-height: 1;
  background: rgba(255, 255, 255, 0.96);
  color: #475569;
  box-sizing: border-box;
  transition: background 0.16s ease, border-color 0.16s ease, transform 0.16s ease;
}
.btn-xs:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  transform: translateY(-1px);
}
.rename-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 18px;
  background: linear-gradient(180deg, rgba(255, 251, 235, 0.96), rgba(254, 249, 195, 0.72));
  border-bottom: 1px solid rgba(253, 230, 138, 0.96);
  flex-shrink: 0;
}
.key-editor.theme-dark .rename-bar {
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.995));
  border-bottom-color: rgba(71, 85, 105, 0.96);
}
.rename-bar input {
  flex: 1;
  max-width: 300px;
  padding: 6px 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  outline: none;
  background: rgba(255, 255, 255, 0.95);
}
.rename-bar input:focus {
  border-color: rgba(96, 165, 250, 0.92);
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.32);
}
.key-editor.theme-dark .rename-bar input {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}
.rename-msg { font-size: 12px; color: #991b1b; }
.key-editor.theme-dark .rename-msg {
  color: #fca5a5;
}
.key-header,
.key-meta,
.key-actions,
.type-badge,
.ttl-chip,
.ttl-popover-title,
.btn-tiny,
.btn-sm,
.btn-xs,
.btn-retry,
.rename-bar,
.rename-msg,
.spinner,
.error-icon {
  user-select: none;
  -webkit-user-select: none;
}
.editor-body {
  flex: 1;
  overflow: hidden;
  padding: 14px 12px 0;
  display: flex;
  flex-direction: column;
  position: relative;
}
.refresh-overlay {
  position: absolute;
  inset: 14px 12px 0;
  z-index: 8;
  pointer-events: none;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.5), rgba(255, 255, 255, 0.18) 26%, rgba(255, 255, 255, 0.04) 56%, rgba(255, 255, 255, 0));
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 12px;
}
.refresh-overlay-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid rgba(191, 219, 254, 0.92);
  background: rgba(255, 255, 255, 0.94);
  color: #2563eb;
  box-shadow: 0 10px 22px rgba(191, 219, 254, 0.22);
  font-size: 11px;
  font-weight: 600;
  backdrop-filter: blur(8px);
}
.refresh-spinner {
  width: 14px;
  height: 14px;
  border-width: 2px;
  flex-shrink: 0;
}
.key-editor.theme-dark .refresh-overlay {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.48), rgba(15, 23, 42, 0.16) 26%, rgba(15, 23, 42, 0.04) 56%, rgba(15, 23, 42, 0));
}
.key-editor.theme-dark .refresh-overlay-chip {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.92);
  color: #93c5fd;
  box-shadow: 0 12px 24px rgba(2, 6, 23, 0.34);
  backdrop-filter: none;
}
@keyframes keyCopyPulse {
  0% {
    transform: translateY(0) scale(1);
  }
  50% {
    transform: translateY(-1px) scale(1.012);
  }
  100% {
    transform: translateY(-1px) scale(1);
  }
}

</style>
