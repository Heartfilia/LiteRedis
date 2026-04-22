<template>
  <div class="set-editor">
    <div class="toolbar">
      <button class="btn-add" @click="showAdd = !showAdd">+ 添加成员</button>
      <div class="search-bar">
        <input
          v-model="searchQuery"
          class="search-input"
          placeholder="搜索成员..."
          @keydown.enter="executeSearch"
        />
        <button class="btn-search" :disabled="isSearching" @click="executeSearch">
          {{ isSearching ? '…' : '搜索' }}
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search" @click="clearSearch">✕</button>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">搜索: {{ displayMembers.length }}/{{ searchResults.length }}</template>
        <template v-else>{{ sourceMembers.length }}/{{ rawMembers.length }} 个</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <input v-model="newMember" placeholder="member" @keydown.enter="addMember" />
      <button @click="addMember">添加</button>
      <button @click="showAdd = false">取消</button>
    </div>

    <!-- sort header -->
    <div class="set-header">
      <span class="sortable-col" @click="cycleSortOrder">
        成员 <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
      </span>
    </div>

    <div class="set-wrap">
      <div v-for="(m, idx) in displayMembers" :key="m" class="set-item">
        <span class="num-badge">{{ idx + 1 }}</span>
        <span class="member-val">
          <span class="val-preview">{{ truncate(m) }}</span>
          <span v-if="m.length > 80" class="val-ellipsis" @click="openExpand(idx, m)">…展开</span>
        </span>
        <div class="item-actions">
          <button class="btn-tiny" @click="copyMember(m, idx)">{{ copiedMember === m + idx ? '✓' : '复制' }}</button>
          <button class="btn-tiny" @click="openExpand(idx, m)">展开</button>
          <button class="btn-tiny danger" @click="removeMember(m)">删除</button>
        </div>
      </div>
    </div>

    <!-- 显示更多 -->
    <div v-if="(searchResults !== null ? searchResults.length : sourceMembers.length) > displayLimit" class="load-more">
      <button class="btn-load-more" @click="displayLimit += loadCount">
        显示更多（{{ displayLimit }}/{{ searchResults !== null ? searchResults.length : sourceMembers.length }}）
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
import { sAdd, sRem, searchValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()

const rawMembers = ref([])
const showAdd = ref(false)
const newMember = ref('')
const msg = ref('')
const ok = ref(true)
const copiedMember = ref(null)

// 搜索状态
const searchQuery   = ref('')
const searchResults = ref(null)
const isSearching   = ref(false)

// 排序状态
const sortOrder = ref('none')
const sortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[sortOrder.value])
function cycleSortOrder() {
  sortOrder.value = { none: 'desc', desc: 'asc', asc: 'none' }[sortOrder.value]
}

// expand modal
const expandShow = ref(false)
const expandTitle = ref('')
const expandContent = ref('')

const loadCount = computed(() => settingsStore.setLoadCount)
const displayLimit = ref(0)

watch(loadCount, (v) => { displayLimit.value = v }, { immediate: true })

const sourceMembers = computed(() =>
  searchResults.value !== null ? searchResults.value : rawMembers.value
)

const sortedMembers = computed(() => {
  if (sortOrder.value === 'none') return sourceMembers.value
  const copy = [...sourceMembers.value]
  if (sortOrder.value === 'asc')  copy.sort((a, b) => a.localeCompare(b))
  if (sortOrder.value === 'desc') copy.sort((a, b) => b.localeCompare(a))
  return copy
})

const displayMembers = computed(() => sortedMembers.value.slice(0, displayLimit.value))

watch(() => props.keyValue, kv => {
  rawMembers.value = [...(kv?.set_val || [])]
  searchQuery.value = ''
  searchResults.value = null
  sortOrder.value = 'none'
  displayLimit.value = loadCount.value
  msg.value = ''
}, { immediate: true })

async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  isSearching.value = true
  try {
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'set', pattern)
    searchResults.value = kv.set_val || []
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

function openExpand(idx, val) {
  expandTitle.value = `member[${idx + 1}]`
  expandContent.value = val
  expandShow.value = true
}

async function copyMember(m, idx) {
  await copyToClipboard(m)
  copiedMember.value = m + idx
  setTimeout(() => { copiedMember.value = null }, 1200)
}

async function addMember() {
  if (!newMember.value.trim()) return
  try {
    const result = await sAdd(workspaceStore.activeConnID, props.keyValue.key, newMember.value)
    ok.value = result.success; msg.value = result.success ? '已添加' : (result.message || '失败')
    if (result.success) { rawMembers.value.push(newMember.value); newMember.value = ''; showAdd.value = false }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function removeMember(m) {
  try {
    const result = await sRem(workspaceStore.activeConnID, props.keyValue.key, m)
    ok.value = result.success; msg.value = result.success ? '已删除' : (result.message || '失败')
    if (result.success) {
      rawMembers.value = rawMembers.value.filter(i => i !== m)
      if (searchResults.value !== null) {
        searchResults.value = searchResults.value.filter(i => i !== m)
      }
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}
</script>

<style scoped>
.set-editor { display: flex; flex-direction: column; height: 100%; gap: 8px; }
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
.set-header {
  display: flex; align-items: center; padding: 5px 8px;
  background: #f9fafb; border-bottom: 1px solid #e5e7eb;
  font-size: 11px; font-weight: 600; color: #6b7280;
  text-transform: uppercase; letter-spacing: 0.4px; flex-shrink: 0;
}
.set-wrap { flex: 1; overflow-y: auto; }
.set-item { display: flex; align-items: center; gap: 6px; padding: 5px 8px; border-bottom: 1px solid #f3f4f6; font-size: 12px; }
.set-item:hover { background: #f9fafb; }
.num-badge { background: #faf5ff; color: #7c3aed; padding: 1px 6px; border-radius: 4px; font-size: 11px; flex-shrink: 0; min-width: 28px; text-align: center; font-weight: 500; }
.member-val { font-family: monospace; flex: 1; display: flex; align-items: baseline; gap: 2px; flex-wrap: wrap; }
.val-preview { word-break: break-all; color: #374151; font-size: 12px; }
.val-ellipsis { font-size: 11px; color: #3b82f6; cursor: pointer; white-space: nowrap; }
.val-ellipsis:hover { text-decoration: underline; }
.item-actions { display: flex; gap: 4px; flex-shrink: 0; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { color: #374151; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 10px; color: #d1d5db; }
.sort-icon.asc, .sort-icon.desc { color: #3b82f6; font-weight: bold; }
.load-more { display: flex; justify-content: center; padding: 6px 0; flex-shrink: 0; }
.msg { font-size: 12px; padding: 5px 10px; border-radius: 6px; }
.ok { background: #f0fdf4; color: #166534; }
.err { background: #fff1f2; color: #991b1b; }
</style>
