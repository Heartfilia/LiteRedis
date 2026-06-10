<template>
  <div class="list-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <FloatingMessage :message="msg" :success="ok" />
    <div class="toolbar">
      <button class="btn-add" :class="{ 'success-flash': addFlashing }" :title="t('keyEditor.addElement')" @click="showAdd = !showAdd">+</button>
      <div class="search-bar">
        <input
          v-model="searchQuery"
          class="search-input"
          :placeholder="t('keyEditor.searchElement')"
          @keydown.enter="executeSearch"
        />
        <button class="btn-search" :disabled="isSearching" @click="executeSearch">
          {{ isSearching ? '…' : t('keyTree.searchBtn') }}
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search" @click="clearSearch">✕</button>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayItems.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.itemsCount', { current: sourceItems.length, total: totalItems }) }}</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <select v-model="pushDir" class="add-row-input"><option value="lpush">{{ t('keyEditor.lpush') }}</option><option value="rpush">{{ t('keyEditor.rpush') }}</option></select>
      <input v-model="newValue" class="add-row-input" placeholder="value" @keydown.enter="addItem" />
      <button class="add-row-btn add-row-btn-primary" @click="addItem">{{ t('keyEditor.add') }}</button>
      <button class="add-row-btn" @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
    </div>

    <!-- sort header -->
    <div class="list-header">
      <span class="sortable-col" @click="cycleSortOrder">
        {{ t('keyEditor.value') }} <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
      </span>
    </div>

    <div class="list-wrap">
      <div v-for="(item, idx) in displayItems" :key="idx" class="list-item">
        <span class="idx-badge">
          {{ (searchResults !== null || sortOrder !== 'none') ? '—' : (idx + 1) }}
        </span>
        <span
          v-if="editingIdx !== displayOriginalIndices[idx]"
          class="item-val"
          @dblclick="searchResults === null && sortOrder === 'none' && startEdit(displayOriginalIndices[idx], item)"
        >
          <span class="val-preview">{{ item }}</span>
        </span>
        <input
          v-else
          v-model="editValue"
          @blur="saveEdit(displayOriginalIndices[idx])"
          @keydown.enter="saveEdit(displayOriginalIndices[idx])"
          @keydown.esc="editingIdx = -1"
        />
        <div class="item-actions">
          <template v-if="editingIdx !== displayOriginalIndices[idx]">
            <button class="btn-tiny" :class="{ copied: copiedItem === item + idx }" @click="copyItem(item, idx)">{{ copiedItem === item + idx ? '✓' : t('keyEditor.copy') }}</button>
            <button v-if="searchResults === null && sortOrder === 'none'" class="btn-tiny" @click="openEdit(displayOriginalIndices[idx], item)">{{ t('keyEditor.edit') }}</button>
            <InlineDeleteConfirm
              :label="t('keyEditor.delete')"
              :confirm-text="t('keyEditor.confirmDelete')"
              :reset-token="`${props.keyValue?.key || ''}:${idx}:${item}`"
              @confirm="removeItem(item, displayOriginalIndices[idx])"
            />
          </template>
          <template v-else>
            <button class="btn-tiny btn-confirm-yes" @click="saveEdit(displayOriginalIndices[idx])">✅</button>
            <button class="btn-tiny btn-confirm-no" @click="cancelEdit()">❌</button>
          </template>
        </div>
      </div>
    </div>

    <!-- 加载更多 -->
    <div class="load-more">
      <button
        v-if="searchResults === null && hasMore"
        class="btn-load-more"
        :disabled="valueLoading"
        @click="loadMore"
      >
        {{ valueLoading ? t('keyEditor.loading') : t('keyTree.loadMore') }}
      </button>
      <span v-else-if="searchResults === null && !hasMore && totalItems > 0" class="load-more-hint">
        {{ t('keyEditor.allItemsLoaded', { count: totalItems }) }}
      </span>
    </div>

    <ExpandModal :show="expandShow" :title="expandTitle" :content="expandContent" :editable="expandEditable" :saving="expandSaving" @close="expandShow = false" @save="saveFromModal" />
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { lPush, rPush, lSet, lRem, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import FloatingMessage from '../common/FloatingMessage.vue'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../../utils/connection.js'
import './editorShared.css'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()

const rawItems = ref([])      // 原始加载的 items（保留原始索引）
const showAdd = ref(false)
const pushDir = ref('rpush')
const newValue = ref('')
const editingIdx = ref(-1)
const editValue = ref('')
const msg = ref('')
const ok = ref(true)
const copiedItem = ref(null)
const addFlashing = ref(false)
const totalItemCount = ref(0)
let addFlashTimer = null

function triggerAddFlash() {
  if (addFlashTimer) clearTimeout(addFlashTimer)
  addFlashing.value = true
  addFlashTimer = setTimeout(() => {
    addFlashing.value = false
    addFlashTimer = null
  }, 1100)
}

// 搜索状态（搜索结果是纯字符串数组，不含原始索引）
const searchQuery   = ref('')
const searchResults = ref(null)
const isSearching   = ref(false)

// 排序状态
const sortOrder = ref('none')
const sortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[sortOrder.value])
function cycleSortOrder() {
  sortOrder.value = { none: 'desc', desc: 'asc', asc: 'none' }[sortOrder.value]
  editingIdx.value = -1  // 切换排序时取消编辑
}

// expand modal
const expandShow = ref(false)
const expandTitle = ref('')
const expandContent = ref('')
const expandEditable = ref(false)
const expandSaving = ref(false)
const editModalIdx = ref(-1)

// 服务端分页状态
const hasMore = ref(false)
const nextOffset = ref(0)
const valueLoading = ref(false)

async function handleConnectionFailure(error) {
  if (!isConnectionErrorMessage(error)) return false
  await connectionsStore.handleConnectionFailure(workspaceStore.activeConnID, error)
  ok.value = false
  msg.value = formatConnectionLostMessage(error)
  return true
}
const totalItems = computed(() => totalItemCount.value >= 0 ? totalItemCount.value : rawItems.value.length)

// 数据源（搜索激活时不保留原始索引）
const sourceItems = computed(() =>
  searchResults.value !== null ? searchResults.value : rawItems.value
)

// 排序处理（带原始索引，仅当无搜索时保留索引）
const sortedIndexed = computed(() => {
  if (searchResults.value !== null) {
    // 搜索模式：无原始索引
    const items = [...sourceItems.value]
    if (sortOrder.value === 'asc')  items.sort((a, b) => a.localeCompare(b))
    if (sortOrder.value === 'desc') items.sort((a, b) => b.localeCompare(a))
    return items.map(item => ({ item, origIdx: -1 }))
  }
  // 非搜索模式：保留原始索引用于 LSet / LRem
  const indexed = rawItems.value.map((item, i) => ({ item, origIdx: i }))
  if (sortOrder.value === 'asc')  indexed.sort((a, b) => a.item.localeCompare(b.item))
  if (sortOrder.value === 'desc') indexed.sort((a, b) => b.item.localeCompare(a.item))
  return indexed
})

// 直接显示所有已加载的数据
const displayItems = computed(() =>
  sortedIndexed.value.map(({ item }) => item)
)
const displayOriginalIndices = computed(() =>
  sortedIndexed.value.map(({ origIdx }) => origIdx)
)

const lastKey = ref('')
function persistSearchState(key = props.keyValue?.key || lastKey.value) {
  if (!key) return
  workspaceStore.setEditorSearchState(key, 'list', {
    query: searchQuery.value,
  })
}

watch(searchQuery, () => {
  persistSearchState()
})

onBeforeUnmount(() => {
  persistSearchState()
})

watch(() => props.keyValue, (kv) => {
  persistSearchState(lastKey.value)

  rawItems.value = [...(kv?.list_val || [])]
  hasMore.value = kv?.has_more || false
  nextOffset.value = kv?.next_offset || 0
  totalItemCount.value = kv?.total_count ?? rawItems.value.length

  if (kv?.key) {
    const cached = workspaceStore.getEditorSearchState(kv.key, 'list')
    if (cached) {
      searchQuery.value = cached.query
    } else {
      searchQuery.value = ''
    }
    lastKey.value = kv.key
  } else {
    searchQuery.value = ''
    lastKey.value = ''
  }
  searchResults.value = null

  sortOrder.value = 'none'
  msg.value = ''
  editingIdx.value = -1
}, { immediate: true })

async function loadMore() {
  if (!hasMore.value || valueLoading.value || !props.keyValue?.key) return
  valueLoading.value = true
  try {
    const result = await getValue(workspaceStore.activeConnID, props.keyValue.key, 0, nextOffset.value, '')
    if (result.list_val) {
      rawItems.value.push(...result.list_val)
    }
    hasMore.value = result.has_more || false
    nextOffset.value = result.next_offset || 0
  } catch (e) {
    ok.value = false
    msg.value = e.message || String(e)
  } finally {
    valueLoading.value = false
  }
}

function replaceLocalItem(idx, newVal) {
  if (idx < 0 || idx >= rawItems.value.length) return
  const nextRaw = [...rawItems.value]
  nextRaw[idx] = newVal
  rawItems.value = nextRaw
}

function removeFirstMatching(list, val) {
  const idx = list.findIndex(item => item === val)
  if (idx === -1) return { next: list, removed: false }
  const next = [...list]
  next.splice(idx, 1)
  return { next, removed: true }
}

function removeLocalItem(val) {
  const rawResult = removeFirstMatching(rawItems.value, val)
  if (rawResult.removed) {
    rawItems.value = rawResult.next
    totalItemCount.value = Math.max(0, totalItemCount.value - 1)
  }
  if (searchResults.value !== null) {
    const searchResult = removeFirstMatching(searchResults.value, val)
    if (searchResult.removed) {
      searchResults.value = searchResult.next
    }
  }
}

function addLocalItem(val) {
  rawItems.value = pushDir.value === 'lpush'
    ? [val, ...rawItems.value]
    : [...rawItems.value, val]
  totalItemCount.value++
}

async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  isSearching.value = true
  try {
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'list', pattern, false)
    searchResults.value = kv.list_val || []
    editingIdx.value = -1
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
  finally { isSearching.value = false }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = null
  if (props.keyValue?.key) {
  workspaceStore.setEditorSearchState(props.keyValue.key, 'list', null)
  }
}

function startEdit(idx, val) {
  if (idx === -1) return   // 搜索/排序模式下不可编辑
  editingIdx.value = idx
  editValue.value = val
}
function cancelEdit() { editingIdx.value = -1 }

function openEdit(idx, val) {
  if (idx === -1) return
  expandTitle.value = `item[${idx + 1}]`
  expandContent.value = val
  editModalIdx.value = idx
  expandEditable.value = true
  expandShow.value = true
}

async function saveFromModal(newVal) {
  const idx = editModalIdx.value
  if (idx === -1) return
  expandSaving.value = true
  try {
    const result = await lSet(workspaceStore.activeConnID, props.keyValue.key, idx, newVal)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      replaceLocalItem(idx, newVal)
      expandShow.value = false
    }
  } catch (e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    expandSaving.value = false
  }
}

async function copyItem(item, idx) {
  await copyToClipboard(item)
  copiedItem.value = item + idx
  setTimeout(() => { copiedItem.value = null }, 1200)
}

async function saveEdit(idx) {
  if (editingIdx.value !== idx || idx === -1) return
  editingIdx.value = -1
  try {
    const result = await lSet(workspaceStore.activeConnID, props.keyValue.key, idx, editValue.value)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) replaceLocalItem(idx, editValue.value)
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
}

async function removeItem(val, origIdx) {
  try {
    const result = await lRem(workspaceStore.activeConnID, props.keyValue.key, 1, val)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) removeLocalItem(val)
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
}

async function addItem() {
  if (!newValue.value.trim()) return
  try {
    const fn = pushDir.value === 'lpush' ? lPush : rPush
    const result = await fn(workspaceStore.activeConnID, props.keyValue.key, newValue.value)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      addLocalItem(newValue.value)
      newValue.value = ''
      showAdd.value = false
      triggerAddFlash()
    }
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
}
</script>

<style scoped>
.list-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
.btn-add.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: addSuccessPulse 0.42s ease;
}
.list-header {
  display: flex; align-items: center; padding: 7px 8px;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.96));
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  font-size: 10.5px; font-weight: 700; color: #64748b;
  text-transform: uppercase; letter-spacing: 0.08em; flex-shrink: 0;
}
.list-wrap { flex: 1; overflow-y: auto; }
.list-item { display: flex; align-items: center; gap: 6px; padding: 6px 6px; border-bottom: 1px solid rgba(241, 245, 249, 0.96); font-size: 12px; }
.list-item:hover { background: rgba(248, 250, 252, 0.9); }
.idx-badge { background: rgba(239, 246, 255, 0.96); color: #1d4ed8; padding: 1px 6px; border-radius: 4px; font-size: 10px; flex-shrink: 0; min-width: 28px; text-align: center; font-weight: 600; }
.list-header,
.toolbar,
.search-bar,
.btn-add,
.btn-search,
.btn-clear-search,
.count,
.add-row button,
.idx-badge,
.item-actions {
  user-select: none;
  -webkit-user-select: none;
}
.item-val {
  flex: 1;
  min-width: 0;
  font-family: monospace;
  display: block;
  cursor: pointer;
}
.val-preview {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #374151;
  font-size: 12px;
}
.list-item input { flex: 1; height: 28px; padding: 0 8px; border: 1px solid rgba(96, 165, 250, 0.92); border-radius: 6px; font-size: 12px; outline: none; color: #1e293b; background: rgba(255, 255, 255, 0.96); box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.3); }
.item-actions { display: flex; gap: 5px; flex-shrink: 0; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { color: #374151; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 10px; color: #cbd5e1; }
.sort-icon.asc, .sort-icon.desc { color: #3b82f6; font-weight: bold; }
.load-more {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding: 5px 12px;
  margin: -8px -12px 0;
  border-top: 1px solid rgba(226, 232, 240, 0.95);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.95), rgba(241, 245, 249, 0.95));
  flex-shrink: 0;
  box-sizing: border-box;
}
.btn-load-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 28px;
  padding: 0 14px;
  background: linear-gradient(180deg, #ffffff, #f8fafc);
  color: #3b82f6;
  border: 1px solid rgba(191, 219, 254, 0.92);
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 4px 10px rgba(191, 219, 254, 0.18);
  transition: transform 0.16s ease, background 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}
.btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, #f8fbff, #f1f5f9);
  border-color: #60a5fa;
  box-shadow: 0 6px 14px rgba(147, 197, 253, 0.2);
  transform: translateY(-1px);
}
.btn-load-more:disabled {
  color: #9ca3af;
  border-color: #d1d5db;
  cursor: not-allowed;
  background: #f9fafb;
  box-shadow: none;
}
.load-more-hint {
  font-size: 12px;
  color: #9ca3af;
  line-height: 1;
}
.btn-tiny.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  animation: copyPulse 0.26s ease;
}

.btn-confirm-yes {
  color: #16a34a;
  border-color: rgba(34, 197, 94, 0.82);
  background: rgba(240, 253, 244, 0.96);
}

.btn-confirm-yes:hover {
  background: #16a34a;
  color: #fff;
}

.btn-confirm-no {
  color: #dc2626;
  border-color: rgba(248, 113, 113, 0.82);
  background: rgba(254, 242, 242, 0.96);
}

.btn-confirm-no:hover {
  background: #dc2626;
  color: #fff;
}
.list-editor.theme-dark {
  color: #e2e8f0;
}
.list-editor.theme-dark .search-input,
.list-editor.theme-dark .add-row-input,
.list-editor.theme-dark .list-item input {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}
.list-editor.theme-dark .count,
.list-editor.theme-dark .load-more-hint {
  color: #94a3b8;
}
.list-editor.theme-dark .add-row,
.list-editor.theme-dark .list-header {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.92), rgba(15, 23, 42, 0.88));
  border-color: rgba(51, 65, 85, 0.92);
  color: #94a3b8;
  box-shadow: inset 0 1px 0 rgba(71, 85, 105, 0.24);
}
.list-editor.theme-dark .list-item {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}
.list-editor.theme-dark .list-item:hover {
  background: rgba(30, 41, 59, 0.66);
}
.list-editor.theme-dark .idx-badge {
  background: rgba(30, 64, 175, 0.22);
  color: #93c5fd;
}
.list-editor.theme-dark .val-preview {
  color: #cbd5e1;
}
.list-editor.theme-dark .sortable-col:hover {
  color: #e2e8f0;
}
.list-editor.theme-dark .load-more {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.98));
}
.list-editor.theme-dark .btn-load-more {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.28);
}

.list-editor.theme-dark .btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.48);
  box-shadow: 0 8px 18px rgba(2, 6, 23, 0.34);
}

.list-editor.theme-dark .btn-load-more:disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}

.list-editor.theme-dark .btn-tiny {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.list-editor.theme-dark .btn-tiny.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}

.list-editor.theme-dark .btn-tiny:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: rgba(96, 165, 250, 0.34);
}

.list-editor.theme-dark .btn-add.success-flash {
  background: rgba(9, 59, 44, 0.94);
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.5);
  box-shadow: 0 0 0 1px rgba(167, 243, 208, 0.08) inset, 0 10px 22px rgba(5, 150, 105, 0.22);
}

.list-editor.theme-dark .item-actions :deep(.delete-wrap > .btn-tiny),
.list-editor.theme-dark .item-actions :deep(.delete-wrap > .btn-tiny:hover) {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.list-editor.theme-dark .item-actions :deep(.delete-wrap > .btn-tiny:hover) {
  background: rgba(30, 41, 59, 0.96);
  color: #f8fafc;
  border-color: rgba(96, 165, 250, 0.34);
}

@keyframes copyPulse {
  0% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-1px) scale(1.012); }
  100% { transform: translateY(-1px) scale(1); }
}

@keyframes addSuccessPulse {
  0% { transform: translateY(0) scale(1); }
  48% { transform: translateY(-1px) scale(1.03); }
  100% { transform: translateY(0) scale(1); }
}

</style>
