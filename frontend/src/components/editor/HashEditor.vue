<template>
  <div class="hash-editor">
    <FloatingMessage :message="msg" :success="ok" />
    <div class="toolbar">
      <button class="btn-add" @click="showAdd = !showAdd">+ {{ t('keyEditor.addField') }}</button>
      <div class="search-bar">
        <input
          v-model="searchQuery"
          class="search-input"
          :placeholder="t('keyEditor.searchField')"
          @keydown.enter="executeSearch"
        />
        <button class="btn-search" :disabled="isSearching" @click="executeSearch">
          {{ isSearching ? '…' : (searchResults !== null ? t('keyEditor.refresh') : t('keyTree.searchBtn')) }}
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search" @click="clearSearch">✕</button>
        <label class="fuzzy-check" title="模糊搜索需要内容自行带*">
          <input v-model="fuzzySearch" type="checkbox" />
          {{ t('keyEditor.fuzzy') }}
        </label>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayEntries.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.fieldsCount', { current: sourceEntries.length, total: totalFields }) }}</template>
      </span>
    </div>

    <!-- 添加新 field -->
    <div v-if="showAdd" class="add-row">
      <input v-model="newField" placeholder="field" @keydown.enter="addField" />
      <input v-model="newValue" placeholder="value" @keydown.enter="addField" />
      <button @click="addField">{{ t('keyEditor.add') }}</button>
      <button @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
    </div>

    <!-- hash 表格 -->
    <div class="hash-table-wrap">
      <table class="hash-table">
        <thead>
          <tr>
            <th class="num-col">#</th>
            <th class="sortable-col field-th" @click="cycleSortOrder" :style="fieldColumnStyle">
              <span class="th-content">Field <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span></span>
              <span class="col-resizer" @mousedown.stop="startResizeField" />
            </th>
            <th class="value-th">Value</th>
            <th class="action-th">{{ t('keyEditor.action') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="([field, val], idx) in displayEntries" :key="field">
            <td class="num-cell">{{ idx + 1 }}</td>
            <td class="field-cell" :style="fieldColumnStyle">{{ field }}</td>
            <td class="value-cell">
              <span v-if="editingField !== field" class="value-text">
                <span class="val-preview">{{ truncate(val) }}</span>
                <span v-if="val.length > 80" class="val-ellipsis" @click="openExpand(field, val)">…{{ t('keyEditor.expand') }}</span>
              </span>
              <input v-else v-model="editValue" @keydown.enter="saveEdit(field)" @keydown.esc="cancelEdit()" />
            </td>
            <td class="action-cell">
              <div class="action-btns">
                <template v-if="editingField !== field">
                  <button class="btn-tiny" @click="copyVal(val, field)">{{ copiedField === field ? '✓' : t('keyEditor.copy') }}</button>
                  <button class="btn-tiny" @click="openEdit(field, val)">{{ t('keyEditor.edit') }}</button>
                  <InlineDeleteConfirm
                    :label="t('keyEditor.delete')"
                    :confirm-text="t('keyEditor.confirmDelete')"
                    :reset-token="`${props.keyValue?.key || ''}:${field}`"
                    @confirm="deleteField(field)"
                  />
                </template>
                <template v-else>
                  <button class="btn-tiny btn-confirm-yes" @click="saveEdit(field)">✅</button>
                  <button class="btn-tiny btn-confirm-no" @click="cancelEdit()">❌</button>
                </template>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
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
      <span v-else-if="searchResults === null && !hasMore && totalFields > 0" class="load-more-hint">
        {{ t('keyEditor.allFieldsLoaded', { count: totalFields }) }}
      </span>
    </div>

    <ExpandModal :show="expandShow" :title="expandTitle" :content="expandContent" :editable="expandEditable" :saving="expandSaving" @close="expandShow = false" @save="saveFromModal" />
  </div>
</template>

<script setup>
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { hSet, hDel, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import FloatingMessage from '../common/FloatingMessage.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()

const rawHashVal = ref({})
const showAdd = ref(false)
const newField = ref('')
const newValue = ref('')
const editingField = ref(null)
const editValue = ref('')
const msg = ref('')
const ok = ref(true)
const copiedField = ref(null)
const fieldWidth = ref(240)
const totalFieldCount = ref(0)

function startResizeField(e) {
  const startX = e.clientX
  const startWidth = fieldWidth.value
  function onMove(ev) {
    const delta = ev.clientX - startX
    fieldWidth.value = Math.max(80, Math.min(400, startWidth + delta))
  }
  function onUp() {
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

// 搜索状态
const searchQuery   = ref('')
const searchResults = ref(null)   // null = 无搜索; array of entries = 搜索结果
const isSearching   = ref(false)
const fuzzySearch   = ref(false)

// 排序状态
const sortOrder = ref('none')   // 'none' | 'asc' | 'desc'
const sortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[sortOrder.value])
function cycleSortOrder() {
  sortOrder.value = { none: 'desc', desc: 'asc', asc: 'none' }[sortOrder.value]
}

// expand modal
const expandShow = ref(false)
const expandTitle = ref('')
const expandContent = ref('')
const expandEditable = ref(false)
const expandSaving = ref(false)
const editModalField = ref('')

// 服务端分页状态
const hasMore = ref(false)
const nextCursor = ref(0)
const valueLoading = ref(false)

const fieldCount = computed(() => Object.keys(rawHashVal.value).length)
const totalFields = computed(() => totalFieldCount.value >= 0 ? totalFieldCount.value : fieldCount.value)
const fieldColumnStyle = computed(() => ({ width: `min(${fieldWidth.value}px, 42%)` }))

// 数据源：搜索激活时用搜索结果，否则用全量
const sourceEntries = computed(() =>
  searchResults.value !== null
    ? searchResults.value
    : Object.entries(rawHashVal.value)
)

// 排序后
const sortedEntries = computed(() => {
  if (sortOrder.value === 'none') return sourceEntries.value
  const copy = [...sourceEntries.value]
  if (sortOrder.value === 'asc')  copy.sort(([a], [b]) => a.localeCompare(b))
  if (sortOrder.value === 'desc') copy.sort(([a], [b]) => b.localeCompare(a))
  return copy
})

// 直接显示所有已加载的数据（不再客户端分页）
const displayEntries = computed(() => sortedEntries.value)

const lastKey = ref('')
function persistSearchState(key = props.keyValue?.key || lastKey.value) {
  if (!key) return
  workspaceStore.setEditorSearchState(key, 'hash', {
    query: searchQuery.value,
    fuzzy: fuzzySearch.value,
  })
}

watch([searchQuery, fuzzySearch], () => {
  persistSearchState()
})

onBeforeUnmount(() => {
  persistSearchState()
})

watch(() => props.keyValue, (kv) => {
  persistSearchState(lastKey.value)

  rawHashVal.value = { ...(kv?.hash_val || {}) }
  hasMore.value = kv?.has_more || false
  nextCursor.value = kv?.next_cursor || 0
  totalFieldCount.value = kv?.total_count ?? Object.keys(kv?.hash_val || {}).length

  // 恢复或重置搜索状态
  if (kv?.key) {
    const cached = workspaceStore.getEditorSearchState(kv.key, 'hash')
    if (cached) {
      searchQuery.value = cached.query
      fuzzySearch.value = cached.fuzzy
    } else {
      searchQuery.value = ''
      fuzzySearch.value = false
    }
    lastKey.value = kv.key
  } else {
    searchQuery.value = ''
    fuzzySearch.value = false
    lastKey.value = ''
  }
  searchResults.value = null

  sortOrder.value = 'none'
  msg.value = ''
}, { immediate: true })

async function loadMore() {
  if (!hasMore.value || valueLoading.value || !props.keyValue?.key) return
  valueLoading.value = true
  try {
    const result = await getValue(workspaceStore.activeConnID, props.keyValue.key, nextCursor.value, 0, '')
    if (result.hash_val) {
      rawHashVal.value = { ...rawHashVal.value, ...result.hash_val }
    }
    hasMore.value = result.has_more || false
    nextCursor.value = result.next_cursor || 0
  } catch (e) {
    ok.value = false
    msg.value = e.message || String(e)
  } finally {
    valueLoading.value = false
  }
}

// 搜索
async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  if (fuzzySearch.value && !pattern.includes('*')) {
    ok.value = false
    msg.value = t('keyEditor.fuzzyRequireStar')
    return
  }
  isSearching.value = true
  try {
    const exact = !fuzzySearch.value
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'hash', pattern, exact)
    searchResults.value = Object.entries(kv.hash_val || {})
  } catch(e) { ok.value = false; msg.value = e.message }
  finally { isSearching.value = false }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = null
  fuzzySearch.value = false
  if (props.keyValue?.key) {
  workspaceStore.setEditorSearchState(props.keyValue.key, 'hash', null)
  }
}

function truncate(val, max = 80) {
  if (!val) return val
  return val.length > max ? val.slice(0, max) : val
}

function startEdit(field, val) {
  editingField.value = field
  editValue.value = val
}

function cancelEdit() {
  editingField.value = null
}

function openExpand(field, val) {
  expandTitle.value = field
  expandContent.value = val
  expandEditable.value = false
  expandShow.value = true
}

function openEdit(field, val) {
  expandTitle.value = field
  expandContent.value = val
  editModalField.value = field
  expandEditable.value = true
  expandShow.value = true
}

async function saveFromModal(newVal) {
  const field = editModalField.value
  if (!field) return
  expandSaving.value = true
  try {
    const result = await hSet(workspaceStore.activeConnID, props.keyValue.key, field, newVal)
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      rawHashVal.value = { ...rawHashVal.value, [field]: newVal }
      if (searchResults.value !== null) {
        searchResults.value = searchResults.value.map(([f, v]) => f === field ? [field, newVal] : [f, v])
      }
      expandShow.value = false
    }
  } catch (e) {
    ok.value = false
    msg.value = e.message
  } finally {
    expandSaving.value = false
  }
}

async function copyVal(val, field) {
  await copyToClipboard(val)
  copiedField.value = field
  setTimeout(() => { copiedField.value = null }, 1200)
}

async function saveEdit(field) {
  if (editingField.value !== field) return
  editingField.value = null
  try {
    const result = await hSet(workspaceStore.activeConnID, props.keyValue.key, field, editValue.value)
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      rawHashVal.value = { ...rawHashVal.value, [field]: editValue.value }
      if (searchResults.value !== null) {
        searchResults.value = searchResults.value.map(([f, v]) => f === field ? [field, editValue.value] : [f, v])
      }
    }
  } catch(e) {
    ok.value = false; msg.value = e.message
  }
}

async function deleteField(field) {
  try {
    const result = await hDel(workspaceStore.activeConnID, props.keyValue.key, field)
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      const next = { ...rawHashVal.value }
      delete next[field]
      rawHashVal.value = next
      totalFieldCount.value = Math.max(0, totalFieldCount.value - 1)
      if (searchResults.value !== null) {
        searchResults.value = searchResults.value.filter(([f]) => f !== field)
      }
    }
  } catch(e) {
    ok.value = false; msg.value = e.message
  }
}

async function addField() {
  if (!newField.value.trim()) return
  try {
    const field = newField.value.trim()
    const existed = Object.prototype.hasOwnProperty.call(rawHashVal.value, field)
    const result = await hSet(workspaceStore.activeConnID, props.keyValue.key, field, newValue.value)
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      rawHashVal.value = { ...rawHashVal.value, [field]: newValue.value }
      if (!existed) {
        totalFieldCount.value++
      }
      newField.value = ''; newValue.value = ''; showAdd.value = false
    }
  } catch(e) {
    ok.value = false; msg.value = e.message
  }
}
</script>

<style scoped>
.hash-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.search-bar { display: flex; align-items: center; }
.search-input {
  width: 130px; padding: 3px 8px;
  border: 1px solid #d1d5db; border-right: none;
  border-radius: 5px 0 0 5px; font-size: 12px; outline: none; color: #333;
}
.search-input:focus { border-color: #3b82f6; }
.fuzzy-check { display: flex; align-items: center; gap: 3px; font-size: 12px; color: #6b7280; cursor: pointer; margin-left: 4px; white-space: nowrap; }
.fuzzy-check input { cursor: pointer; }
.count { font-size: 12px; color: #9ca3af; margin-left: auto; white-space: nowrap; }
.add-row { display: flex; gap: 6px; padding: 6px; background: #f9fafb; border-radius: 6px; border: 1px solid #e5e7eb; }
.add-row input { flex: 1; padding: 4px 8px; border: 1px solid #d1d5db; border-radius: 5px; font-size: 12px; outline: none; }
.add-row input:focus { border-color: #3b82f6; }
.add-row button { padding: 4px 10px; border: 1px solid #d1d5db; border-radius: 5px; cursor: pointer; font-size: 12px; background: #fff; color: #374151; }
.add-row button:hover { background: #f3f4f6; }
.hash-table-wrap { flex: 1; overflow-y: auto; overflow-x: hidden; }
.hash-table { width: 100%; border-collapse: collapse; font-size: 12px; table-layout: fixed; }
.hash-table thead { position: sticky; top: 0; z-index: 10; }
.hash-table th { background: #f9fafb; padding: 6px 8px; text-align: left; border-bottom: 1px solid #e5e7eb; font-weight: 600; color: #6b7280; font-size: 11px; text-transform: uppercase; letter-spacing: 0.4px; }
.hash-table td { padding: 5px 8px; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
.num-col { width: 36px; text-align: center; }
.num-cell { width: 36px; text-align: center; color: #d1d5db; font-size: 11px; }
.field-th { position: relative; min-width: 80px; }
.value-th { width: auto; }
.action-th { width: 1px; white-space: nowrap; }
.th-content { display: inline-block; }
.col-resizer {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 7px;
  cursor: col-resize;
  background: #f3f4f6;
  border-left: 1px solid #e5e7eb;
  border-right: 1px solid #e5e7eb;
  transition: background 0.15s, border-color 0.15s;
  z-index: 5;
}
.col-resizer:hover { background: #3b82f6; border-color: #3b82f6; }
.field-cell { color: #1d4ed8; font-weight: 500; word-break: break-all; min-width: 0; }
.value-th,
.value-cell { min-width: 0; }
.value-text { cursor: pointer; display: flex; align-items: baseline; gap: 2px; flex-wrap: wrap; }
.val-preview {
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
  color: #374151;
  display: inline-block;
  max-width: 100%;
}
.val-ellipsis { font-size: 11px; color: #3b82f6; cursor: pointer; white-space: nowrap; flex-shrink: 0; }
.val-ellipsis:hover { text-decoration: underline; }
.value-cell input { width: 100%; padding: 3px 6px; border: 1px solid #3b82f6; border-radius: 4px; font-size: 12px; outline: none; }
.action-th,
.action-cell {
  width: 170px;
  min-width: 170px;
  max-width: 170px;
  text-align: center;
  white-space: nowrap;
}
.action-btns { display: inline-flex; gap: 4px; justify-content: center; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { background: #f3f4f6 !important; }
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
.value-cell {
  background: #f8fafc;
  overflow: hidden;
}

</style>
