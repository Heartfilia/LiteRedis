<template>
  <div class="hash-editor">
    <div class="toolbar">
      <button class="btn-add" @click="showAdd = !showAdd">+ 添加 Field</button>
      <div class="search-bar">
        <input
          v-model="searchQuery"
          class="search-input"
          placeholder="搜索 field..."
          @keydown.enter="executeSearch"
        />
        <button class="btn-search" :disabled="isSearching" @click="executeSearch">
          {{ isSearching ? '…' : '搜索' }}
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search" @click="clearSearch">✕</button>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">搜索: {{ displayEntries.length }}/{{ searchResults.length }}</template>
        <template v-else>{{ sourceEntries.length }}/{{ fieldCount }} 个 field</template>
      </span>
    </div>

    <!-- 添加新 field -->
    <div v-if="showAdd" class="add-row">
      <input v-model="newField" placeholder="field" @keydown.enter="addField" />
      <input v-model="newValue" placeholder="value" @keydown.enter="addField" />
      <button @click="addField">添加</button>
      <button @click="showAdd = false">取消</button>
    </div>

    <!-- hash 表格 -->
    <div class="hash-table-wrap">
      <table class="hash-table">
        <thead>
          <tr>
            <th class="num-col">#</th>
            <th class="sortable-col" @click="cycleSortOrder">
              Field <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
            </th>
            <th>Value</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="([field, val], idx) in displayEntries" :key="field">
            <td class="num-cell">{{ idx + 1 }}</td>
            <td class="field-cell">{{ field }}</td>
            <td class="value-cell">
              <span v-if="editingField !== field" class="value-text" @dblclick="startEdit(field, val)">
                <span class="val-preview">{{ truncate(val) }}</span>
                <span v-if="val.length > 80" class="val-ellipsis" @click="openExpand(field, val)">…展开</span>
              </span>
              <input v-else v-model="editValue" @blur="saveEdit(field)" @keydown.enter="saveEdit(field)" @keydown.esc="editingField = null" />
            </td>
            <td class="action-cell">
              <div class="action-btns">
                <button class="btn-tiny" @click="copyVal(val, field)">{{ copiedField === field ? '✓' : '复制' }}</button>
                <button class="btn-tiny" @click="openExpand(field, val)">展开</button>
                <button class="btn-tiny" @click="startEdit(field, val)">编辑</button>
                <button class="btn-tiny danger" @click="deleteField(field)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 显示更多 -->
    <div v-if="(searchResults !== null ? searchResults.length : sourceEntries.length) > displayLimit" class="load-more">
      <button class="btn-load-more" @click="displayLimit += loadCount">
        显示更多（{{ displayLimit }}/{{ searchResults !== null ? searchResults.length : sourceEntries.length }}）
      </button>
    </div>

    <div v-if="msg" :class="['msg', ok ? 'ok' : 'err']">{{ msg }}</div>

    <ExpandModal :show="expandShow" :title="expandTitle" :content="expandContent" @close="expandShow = false" />
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { hSet, hDel, searchValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()

const rawHashVal = ref({})
const showAdd = ref(false)
const newField = ref('')
const newValue = ref('')
const editingField = ref(null)
const editValue = ref('')
const msg = ref('')
const ok = ref(true)
const copiedField = ref(null)

// 搜索状态
const searchQuery   = ref('')
const searchResults = ref(null)   // null = 无搜索; array of entries = 搜索结果
const isSearching   = ref(false)

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

const loadCount = computed(() => settingsStore.hashLoadCount)
const fieldCount = computed(() => Object.keys(rawHashVal.value).length)
const displayLimit = ref(0)

watch(loadCount, (v) => { displayLimit.value = v }, { immediate: true })

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

// 分页
const displayEntries = computed(() => sortedEntries.value.slice(0, displayLimit.value))

watch(() => props.keyValue, (kv) => {
  rawHashVal.value = { ...(kv?.hash_val || {}) }
  searchQuery.value = ''
  searchResults.value = null
  sortOrder.value = 'none'
  displayLimit.value = loadCount.value
  msg.value = ''
}, { immediate: true })

// 搜索
async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  isSearching.value = true
  try {
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'hash', pattern)
    searchResults.value = Object.entries(kv.hash_val || {})
    displayLimit.value = loadCount.value
  } catch(e) { ok.value = false; msg.value = e.message }
  finally { isSearching.value = false }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = null
  displayLimit.value = loadCount.value
}

function truncate(val, max = 80) {
  if (!val) return val
  return val.length > max ? val.slice(0, max) : val
}

function startEdit(field, val) {
  editingField.value = field
  editValue.value = val
}

function openExpand(field, val) {
  expandTitle.value = field
  expandContent.value = val
  expandShow.value = true
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
    msg.value = result.success ? '已更新' : (result.message || '失败')
    if (result.success) rawHashVal.value[field] = editValue.value
  } catch(e) {
    ok.value = false; msg.value = e.message
  }
}

async function deleteField(field) {
  try {
    const result = await hDel(workspaceStore.activeConnID, props.keyValue.key, field)
    ok.value = result.success
    msg.value = result.success ? '已删除' : (result.message || '失败')
    if (result.success) {
      delete rawHashVal.value[field]
      // 同步从搜索结果中移除
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
    const result = await hSet(workspaceStore.activeConnID, props.keyValue.key, newField.value, newValue.value)
    ok.value = result.success
    msg.value = result.success ? '已添加' : (result.message || '失败')
    if (result.success) {
      rawHashVal.value[newField.value] = newValue.value
      newField.value = ''; newValue.value = ''; showAdd.value = false
    }
  } catch(e) {
    ok.value = false; msg.value = e.message
  }
}
</script>

<style scoped>
.hash-editor { display: flex; flex-direction: column; height: 100%; gap: 8px; }
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
.add-row input { flex: 1; padding: 4px 8px; border: 1px solid #d1d5db; border-radius: 5px; font-size: 12px; outline: none; }
.add-row input:focus { border-color: #3b82f6; }
.add-row button { padding: 4px 10px; border: 1px solid #d1d5db; border-radius: 5px; cursor: pointer; font-size: 12px; background: #fff; color: #374151; }
.add-row button:hover { background: #f3f4f6; }
.hash-table-wrap { flex: 1; overflow-y: auto; }
.hash-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.hash-table th { background: #f9fafb; padding: 6px 8px; text-align: left; border-bottom: 1px solid #e5e7eb; font-weight: 600; color: #6b7280; font-size: 11px; text-transform: uppercase; letter-spacing: 0.4px; }
.hash-table td { padding: 5px 8px; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
.num-col { width: 36px; text-align: center; }
.num-cell { width: 36px; text-align: center; color: #d1d5db; font-size: 11px; }
.field-cell { color: #1d4ed8; font-weight: 500; max-width: 150px; word-break: break-all; }
.value-cell { max-width: 300px; }
.value-text { cursor: pointer; display: flex; align-items: baseline; gap: 2px; flex-wrap: wrap; }
.val-preview { font-family: monospace; font-size: 12px; word-break: break-all; color: #374151; }
.val-ellipsis { font-size: 11px; color: #3b82f6; cursor: pointer; white-space: nowrap; flex-shrink: 0; }
.val-ellipsis:hover { text-decoration: underline; }
.value-cell input { width: 100%; padding: 3px 6px; border: 1px solid #3b82f6; border-radius: 4px; font-size: 12px; outline: none; }
.action-cell { text-align: right; white-space: nowrap; }
.action-btns { display: inline-flex; gap: 4px; justify-content: flex-end; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { background: #f3f4f6 !important; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 10px; color: #d1d5db; }
.sort-icon.asc, .sort-icon.desc { color: #3b82f6; font-weight: bold; }
.load-more { display: flex; justify-content: center; padding: 6px 0; flex-shrink: 0; }
.msg { font-size: 12px; padding: 5px 10px; border-radius: 6px; }
.ok { background: #f0fdf4; color: #166534; }
.err { background: #fff1f2; color: #991b1b; }
</style>
