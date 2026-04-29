<template>
  <div class="list-editor">
    <FloatingMessage :message="msg" :success="ok" />
    <div class="toolbar">
      <button class="btn-add" @click="showAdd = !showAdd">+ {{ t('keyEditor.addElement') }}</button>
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
      <select v-model="pushDir"><option value="lpush">{{ t('keyEditor.lpush') }}</option><option value="rpush">{{ t('keyEditor.rpush') }}</option></select>
      <input v-model="newValue" placeholder="value" @keydown.enter="addItem" />
      <button @click="addItem">{{ t('keyEditor.add') }}</button>
      <button @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
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
          <span class="val-preview">{{ truncate(item) }}</span>
          <span v-if="item.length > 80" class="val-ellipsis" @click="openExpand(idx, item)">…{{ t('keyEditor.expand') }}</span>
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
            <button class="btn-tiny" @click="copyItem(item, idx)">{{ copiedItem === item + idx ? '✓' : t('keyEditor.copy') }}</button>
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
import { ref, computed, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { lPush, rPush, lSet, lRem, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import FloatingMessage from '../common/FloatingMessage.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
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
const totalItemCount = ref(0)

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

watch(() => props.keyValue, kv => {
  rawItems.value = [...(kv?.list_val || [])]
  hasMore.value = kv?.has_more || false
  nextOffset.value = kv?.next_offset || 0
  totalItemCount.value = kv?.total_count ?? rawItems.value.length
  searchQuery.value = ''
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
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'list', pattern)
    searchResults.value = kv.list_val || []
    editingIdx.value = -1
  } catch(e) { ok.value = false; msg.value = e.message }
  finally { isSearching.value = false }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = null
}

function truncate(val, max = 80) {
  if (!val) return val
  return val.length > max ? val.slice(0, max) : val
}

function startEdit(idx, val) {
  if (idx === -1) return   // 搜索/排序模式下不可编辑
  editingIdx.value = idx
  editValue.value = val
}
function cancelEdit() { editingIdx.value = -1 }

function openExpand(idx, val) {
  expandTitle.value = `item[${idx + 1}]`
  expandContent.value = val
  expandEditable.value = false
  expandShow.value = true
}

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
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      replaceLocalItem(idx, newVal)
      expandShow.value = false
    }
  } catch (e) {
    ok.value = false
    msg.value = e.message
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
    ok.value = result.success; msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) replaceLocalItem(idx, editValue.value)
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function removeItem(val, origIdx) {
  try {
    const result = await lRem(workspaceStore.activeConnID, props.keyValue.key, 1, val)
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) removeLocalItem(val)
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function addItem() {
  if (!newValue.value.trim()) return
  try {
    const fn = pushDir.value === 'lpush' ? lPush : rPush
    const result = await fn(workspaceStore.activeConnID, props.keyValue.key, newValue.value)
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      addLocalItem(newValue.value)
      newValue.value = ''
      showAdd.value = false
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}
</script>

<style scoped>
.list-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.search-bar { display: flex; align-items: center; }
.search-input {
  width: 130px; padding: 3px 8px;
  border: 1px solid #d1d5db; border-right: none;
  border-radius: 5px 0 0 5px; font-size: 12px; outline: none; color: #333;
}
.search-input:focus { border-color: #3b82f6; }
.count { font-size: 12px; color: #9ca3af; margin-left: auto; white-space: nowrap; }
.add-row { display: flex; gap: 6px; padding: 6px; background: #f9fafb; border-radius: 6px; border: 1px solid #e5e7eb; }
.add-row input, .add-row select { flex: 1; padding: 4px 8px; border: 1px solid #d1d5db; border-radius: 5px; font-size: 12px; outline: none; }
.add-row input:focus, .add-row select:focus { border-color: #3b82f6; }
.add-row button { padding: 4px 10px; border: 1px solid #d1d5db; border-radius: 5px; cursor: pointer; font-size: 12px; background: #fff; color: #374151; }
.add-row button:hover { background: #f3f4f6; }
.list-header {
  display: flex; align-items: center; padding: 5px 8px;
  background: #f9fafb; border-bottom: 1px solid #e5e7eb;
  font-size: 11px; font-weight: 600; color: #6b7280;
  text-transform: uppercase; letter-spacing: 0.4px; flex-shrink: 0;
}
.list-wrap { flex: 1; overflow-y: auto; }
.list-item { display: flex; align-items: center; gap: 6px; padding: 5px 6px; border-bottom: 1px solid #f3f4f6; font-size: 12px; }
.list-item:hover { background: #f9fafb; }
.idx-badge { background: #eff6ff; color: #1d4ed8; padding: 1px 6px; border-radius: 4px; font-size: 11px; flex-shrink: 0; min-width: 28px; text-align: center; font-weight: 500; }
.item-val { flex: 1; font-family: monospace; display: flex; align-items: baseline; gap: 2px; flex-wrap: wrap; cursor: pointer; }
.val-preview { word-break: break-all; color: #374151; font-size: 12px; }
.val-ellipsis { font-size: 11px; color: #3b82f6; cursor: pointer; white-space: nowrap; }
.val-ellipsis:hover { text-decoration: underline; }
.list-item input { flex: 1; padding: 3px 6px; border: 1px solid #3b82f6; border-radius: 4px; font-size: 12px; outline: none; }
.item-actions { display: flex; gap: 4px; flex-shrink: 0; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { color: #374151; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 10px; color: #d1d5db; }
.sort-icon.asc, .sort-icon.desc { color: #3b82f6; font-weight: bold; }
.load-more { display: flex; justify-content: center; padding: 6px 0; flex-shrink: 0; }
.btn-load-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 5px 20px;
  background: #fff;
  color: #3b82f6;
  border: 1px solid #3b82f6;
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-load-more:hover:not(:disabled) {
  background: #eff6ff;
}
.btn-load-more:disabled {
  color: #9ca3af;
  border-color: #d1d5db;
  cursor: not-allowed;
  background: #f9fafb;
}
.load-more-hint {
  font-size: 12px;
  color: #9ca3af;
}

</style>
