<template>
  <div class="list-editor">
    <div class="toolbar">
      <button class="btn-add" @click="showAdd = !showAdd">+ 添加元素</button>
      <div class="search-bar">
        <input
          v-model="searchQuery"
          class="search-input"
          placeholder="搜索元素..."
          @keydown.enter="executeSearch"
        />
        <button class="btn-search" :disabled="isSearching" @click="executeSearch">
          {{ isSearching ? '…' : '搜索' }}
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search" @click="clearSearch">✕</button>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">搜索: {{ displayItems.length }}/{{ searchResults.length }}</template>
        <template v-else>{{ sourceItems.length }}/{{ rawItems.length }} 条</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <select v-model="pushDir"><option value="lpush">头部插入 (LPUSH)</option><option value="rpush">尾部插入 (RPUSH)</option></select>
      <input v-model="newValue" placeholder="value" @keydown.enter="addItem" />
      <button @click="addItem">添加</button>
      <button @click="showAdd = false">取消</button>
    </div>

    <!-- sort header -->
    <div class="list-header">
      <span class="sortable-col" @click="cycleSortOrder">
        值 <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
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
          <span v-if="item.length > 80" class="val-ellipsis" @click="openExpand(idx, item)">…展开</span>
        </span>
        <input
          v-else
          v-model="editValue"
          @blur="saveEdit(displayOriginalIndices[idx])"
          @keydown.enter="saveEdit(displayOriginalIndices[idx])"
          @keydown.esc="editingIdx = -1"
        />
        <div class="item-actions">
          <button class="btn-tiny" @click="copyItem(item, idx)">{{ copiedItem === item + idx ? '✓' : '复制' }}</button>
          <button class="btn-tiny" @click="openExpand(idx, item)">展开</button>
          <button
            v-if="searchResults === null && sortOrder === 'none'"
            class="btn-tiny"
            @click="startEdit(displayOriginalIndices[idx], item)"
          >编辑</button>
          <button class="btn-tiny danger" @click="removeItem(item, displayOriginalIndices[idx])">删除</button>
        </div>
      </div>
    </div>

    <!-- 显示更多 -->
    <div v-if="(searchResults !== null ? searchResults.length : sourceItems.length) > displayLimit" class="load-more">
      <button class="btn-load-more" @click="displayLimit += loadCount">
        显示更多（{{ displayLimit }}/{{ searchResults !== null ? searchResults.length : sourceItems.length }}）
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
import { lPush, rPush, lSet, lRem, searchValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()

const rawItems = ref([])      // 原始加载的 items（保留原始索引）
const showAdd = ref(false)
const pushDir = ref('rpush')
const newValue = ref('')
const editingIdx = ref(-1)
const editValue = ref('')
const msg = ref('')
const ok = ref(true)
const copiedItem = ref(null)

// 搜索状态（搜索结果是纯字符串数组，不含原始索引）
const searchQuery   = ref('')
const searchResults = ref(null)
const isSearching   = ref(false)

// 排序状态
const sortOrder = ref('none')
const sortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[sortOrder.value])
function cycleSortOrder() {
  sortOrder.value = { none: 'asc', asc: 'desc', desc: 'none' }[sortOrder.value]
  editingIdx.value = -1  // 切换排序时取消编辑
}

// expand modal
const expandShow = ref(false)
const expandTitle = ref('')
const expandContent = ref('')

const loadCount = computed(() => settingsStore.listLoadCount)
const displayLimit = ref(0)

watch(loadCount, (v) => { displayLimit.value = v }, { immediate: true })

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

const displayItems = computed(() =>
  sortedIndexed.value.slice(0, displayLimit.value).map(({ item }) => item)
)
const displayOriginalIndices = computed(() =>
  sortedIndexed.value.slice(0, displayLimit.value).map(({ origIdx }) => origIdx)
)

watch(() => props.keyValue, kv => {
  rawItems.value = [...(kv?.list_val || [])]
  searchQuery.value = ''
  searchResults.value = null
  sortOrder.value = 'none'
  displayLimit.value = loadCount.value
  msg.value = ''
  editingIdx.value = -1
}, { immediate: true })

async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  isSearching.value = true
  try {
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'list', pattern)
    searchResults.value = kv.list_val || []
    displayLimit.value = loadCount.value
    editingIdx.value = -1
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

function startEdit(idx, val) {
  if (idx === -1) return   // 搜索/排序模式下不可编辑
  editingIdx.value = idx
  editValue.value = val
}

function openExpand(idx, val) {
  expandTitle.value = `item[${idx + 1}]`
  expandContent.value = val
  expandShow.value = true
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
    ok.value = result.success; msg.value = result.success ? '已更新' : (result.message || '失败')
    if (result.success) rawItems.value[idx] = editValue.value
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function removeItem(val, origIdx) {
  try {
    const result = await lRem(workspaceStore.activeConnID, props.keyValue.key, 1, val)
    ok.value = result.success; msg.value = result.success ? '已删除' : (result.message || '失败')
    if (result.success) {
      if (origIdx !== -1) {
        rawItems.value.splice(origIdx, 1)
      } else {
        // 搜索模式：从搜索结果和 rawItems 各移除一个
        if (searchResults.value !== null) {
          const si = searchResults.value.indexOf(val)
          if (si !== -1) searchResults.value.splice(si, 1)
        }
        const ri = rawItems.value.indexOf(val)
        if (ri !== -1) rawItems.value.splice(ri, 1)
      }
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function addItem() {
  if (!newValue.value.trim()) return
  try {
    const fn = pushDir.value === 'lpush' ? lPush : rPush
    const result = await fn(workspaceStore.activeConnID, props.keyValue.key, newValue.value)
    ok.value = result.success; msg.value = result.success ? '已添加' : (result.message || '失败')
    if (result.success) {
      if (pushDir.value === 'lpush') rawItems.value.unshift(newValue.value)
      else rawItems.value.push(newValue.value)
      newValue.value = ''; showAdd.value = false
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}
</script>

<style scoped>
.list-editor { display: flex; flex-direction: column; height: 100%; gap: 8px; }
.toolbar { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.btn-add { padding: 4px 10px; background: #4e9af1; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 12px; flex-shrink: 0; }
.btn-add:hover { background: #3a85e0; }
.search-bar { display: flex; align-items: center; }
.search-input {
  width: 130px; padding: 3px 8px;
  border: 1px solid #ddd; border-right: none;
  border-radius: 4px 0 0 4px; font-size: 12px; outline: none; color: #333;
}
.search-input:focus { border-color: #4e9af1; }
.btn-search {
  padding: 3px 8px; cursor: pointer; font-size: 12px;
  border: 1px solid #ddd; border-radius: 0 4px 4px 0;
  background: #f5f5f5; color: #555; white-space: nowrap;
}
.btn-search:hover:not(:disabled) { background: #e8e8e8; }
.btn-search:disabled { opacity: 0.6; cursor: default; }
.btn-clear-search {
  padding: 3px 6px; margin-left: 4px; border: 1px solid #ddd; border-radius: 4px;
  cursor: pointer; font-size: 11px; background: #fff3e0; color: #e65100;
}
.btn-clear-search:hover { background: #ffe0b2; }
.count { font-size: 12px; color: #999; margin-left: auto; white-space: nowrap; }
.add-row { display: flex; gap: 6px; padding: 6px; background: #f9f9f9; border-radius: 4px; }
.add-row input, .add-row select { flex: 1; padding: 4px 6px; border: 1px solid #ddd; border-radius: 3px; font-size: 12px; }
.add-row button { padding: 4px 10px; border: 1px solid #ddd; border-radius: 3px; cursor: pointer; font-size: 12px; }
.list-header {
  display: flex;
  align-items: center;
  padding: 4px 8px;
  background: #f5f5f5;
  border-bottom: 1px solid #e0e0e0;
  font-size: 12px;
  font-weight: 500;
  color: #555;
  flex-shrink: 0;
}
.list-wrap { flex: 1; overflow-y: auto; }
.list-item { display: flex; align-items: center; gap: 6px; padding: 4px 6px; border-bottom: 1px solid #f5f5f5; font-size: 12px; }
.list-item:hover { background: #fafafa; }
.idx-badge { background: #e3f2fd; color: #1565c0; padding: 1px 5px; border-radius: 3px; font-size: 11px; flex-shrink: 0; min-width: 28px; text-align: center; }
.item-val { flex: 1; font-family: monospace; display: flex; align-items: baseline; gap: 2px; flex-wrap: wrap; cursor: pointer; }
.val-preview { word-break: break-all; color: #333; font-size: 12px; }
.val-ellipsis { font-size: 11px; color: #4e9af1; cursor: pointer; white-space: nowrap; }
.val-ellipsis:hover { text-decoration: underline; }
.list-item input { flex: 1; padding: 2px 4px; border: 1px solid #4e9af1; border-radius: 2px; font-size: 12px; }
.item-actions { display: flex; gap: 3px; flex-shrink: 0; }
.btn-tiny { padding: 2px 6px; border: 1px solid #ddd; border-radius: 3px; cursor: pointer; font-size: 11px; background: white; }
.btn-tiny:hover { background: #f0f0f0; }
.btn-tiny.danger:hover { background: #e53e3e; color: white; border-color: #e53e3e; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { color: #333; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 11px; color: #bbb; }
.sort-icon.asc, .sort-icon.desc { color: #4e9af1; font-weight: bold; }
.load-more { display: flex; justify-content: center; padding: 4px 0; flex-shrink: 0; }
.btn-load-more {
  padding: 4px 16px; border: 1px solid #ddd; border-radius: 4px;
  cursor: pointer; font-size: 12px; background: white; color: #555;
}
.btn-load-more:hover { background: #f0f0f0; }
.msg { font-size: 12px; padding: 4px 8px; border-radius: 4px; }
.ok { background: #e8f5e9; color: #2e7d32; }
.err { background: #fce4ec; color: #b71c1c; }
</style>
