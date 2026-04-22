<template>
  <div class="zset-editor">
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
      <input v-model.number="newScore" type="number" step="any" placeholder="score" @keydown.enter="addMember" />
      <button @click="addMember">添加</button>
      <button @click="showAdd = false">取消</button>
    </div>
    <div class="zset-wrap">
      <table class="zset-table">
        <thead>
          <tr>
            <th class="num-col">#</th>
            <th>Score</th>
            <th class="sortable-col" @click="cycleSortOrder">
              Member <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
            </th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(m, idx) in displayMembers" :key="m.member">
            <td class="num-cell">{{ idx + 1 }}</td>
            <td class="score-cell">
              <span v-if="editingMember !== m.member">{{ m.score }}</span>
              <input v-else v-model.number="editScore" type="number" step="any"
                @blur="saveEdit(m.member)" @keydown.enter="saveEdit(m.member)" @keydown.esc="editingMember = null" />
            </td>
            <td class="member-cell">
              <span class="val-preview">{{ truncate(m.member) }}</span>
              <span v-if="m.member.length > 80" class="val-ellipsis" @click="openExpand(m.member)">…展开</span>
            </td>
            <td class="action-cell">
              <button class="btn-tiny" @click="copyMember(m.member)">
                {{ copiedMember === m.member ? '✓' : '复制' }}
              </button>
              <button class="btn-tiny" @click="startEdit(m)">编辑分数</button>
              <button class="btn-tiny danger" @click="removeMember(m.member)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
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
import { zAdd, zRem, searchValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()

const rawMembers = ref([])
const showAdd = ref(false)
const newMember = ref('')
const newScore = ref(0)
const editingMember = ref(null)
const editScore = ref(0)
const msg = ref('')
const ok = ref(true)
const copiedMember = ref(null)

// 搜索状态
const searchQuery   = ref('')
const searchResults = ref(null)   // null = 无搜索; array of {member,score} = 搜索结果
const isSearching   = ref(false)

// 排序状态（按 member 名称）
const sortOrder = ref('none')
const sortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[sortOrder.value])
function cycleSortOrder() {
  sortOrder.value = { none: 'asc', asc: 'desc', desc: 'none' }[sortOrder.value]
}

// expand modal
const expandShow = ref(false)
const expandTitle = ref('')
const expandContent = ref('')

const loadCount = computed(() => settingsStore.zsetLoadCount)
const displayLimit = ref(0)

watch(loadCount, (v) => { displayLimit.value = v }, { immediate: true })

const sourceMembers = computed(() =>
  searchResults.value !== null ? searchResults.value : rawMembers.value
)

const sortedMembers = computed(() => {
  if (sortOrder.value === 'none') return sourceMembers.value
  const copy = [...sourceMembers.value]
  if (sortOrder.value === 'asc')  copy.sort((a, b) => a.member.localeCompare(b.member))
  if (sortOrder.value === 'desc') copy.sort((a, b) => b.member.localeCompare(a.member))
  return copy
})

const displayMembers = computed(() => sortedMembers.value.slice(0, displayLimit.value))

watch(() => props.keyValue, kv => {
  rawMembers.value = [...(kv?.zset_val || [])]
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
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'zset', pattern)
    searchResults.value = kv.zset_val || []
    displayLimit.value = loadCount.value
  } catch(e) { ok.value = false; msg.value = e.message }
  finally { isSearching.value = false }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = null
  displayLimit.value = loadCount.value
}

function startEdit(m) { editingMember.value = m.member; editScore.value = m.score }

function truncate(val, max = 80) {
  if (!val) return val
  return val.length > max ? val.slice(0, max) : val
}

function openExpand(member) {
  expandTitle.value = 'member'
  expandContent.value = member
  expandShow.value = true
}

async function saveEdit(member) {
  if (editingMember.value !== member) return
  editingMember.value = null
  try {
    const result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, member, editScore.value)
    ok.value = result.success; msg.value = result.success ? '已更新' : (result.message || '失败')
    if (result.success) {
      const idx = rawMembers.value.findIndex(m => m.member === member)
      if (idx !== -1) rawMembers.value[idx].score = editScore.value
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function addMember() {
  if (!newMember.value.trim()) return
  try {
    const result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, newMember.value, newScore.value)
    ok.value = result.success; msg.value = result.success ? '已添加' : (result.message || '失败')
    if (result.success) {
      rawMembers.value.push({ member: newMember.value, score: newScore.value })
      newMember.value = ''; newScore.value = 0; showAdd.value = false
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function removeMember(member) {
  try {
    const result = await zRem(workspaceStore.activeConnID, props.keyValue.key, member)
    ok.value = result.success; msg.value = result.success ? '已删除' : (result.message || '失败')
    if (result.success) {
      rawMembers.value = rawMembers.value.filter(m => m.member !== member)
      if (searchResults.value !== null) {
        searchResults.value = searchResults.value.filter(m => m.member !== member)
      }
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function copyMember(member) {
  await copyToClipboard(member)
  copiedMember.value = member
  setTimeout(() => { copiedMember.value = null }, 1200)
}
</script>

<style scoped>
.zset-editor { display: flex; flex-direction: column; height: 100%; gap: 8px; }
.toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.btn-add { padding: 4px 10px; background: #F44336; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 12px; flex-shrink: 0; }
.btn-add:hover { background: #c62828; }
.search-bar { display: flex; align-items: center; }
.search-input {
  width: 130px; padding: 3px 8px;
  border: 1px solid #ddd; border-right: none;
  border-radius: 4px 0 0 4px; font-size: 12px; outline: none; color: #333;
}
.search-input:focus { border-color: #F44336; }
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
.add-row input { flex: 1; padding: 4px 6px; border: 1px solid #ddd; border-radius: 3px; font-size: 12px; }
.add-row button { padding: 4px 10px; border: 1px solid #ddd; border-radius: 3px; cursor: pointer; font-size: 12px; }
.zset-wrap { flex: 1; overflow-y: auto; }
.zset-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.zset-table th { background: #f5f5f5; padding: 5px 8px; text-align: left; border-bottom: 1px solid #e0e0e0; font-weight: 500; color: #555; }
.zset-table td { padding: 4px 8px; border-bottom: 1px solid #f5f5f5; }
.num-col { width: 36px; text-align: center; color: #bbb; font-weight: 400; }
.num-cell { width: 36px; text-align: center; color: #bbb; font-size: 11px; }
.score-cell { color: #e65100; font-weight: 500; width: 100px; }
.score-cell input { width: 80px; padding: 2px 4px; border: 1px solid #4e9af1; border-radius: 2px; font-size: 12px; }
.member-cell { font-family: monospace; }
.val-preview { word-break: break-all; color: #333; font-size: 12px; }
.val-ellipsis { font-size: 11px; color: #4e9af1; cursor: pointer; white-space: nowrap; margin-left: 2px; }
.val-ellipsis:hover { text-decoration: underline; }
.action-cell { white-space: nowrap; }
.btn-tiny { padding: 2px 6px; border: 1px solid #ddd; border-radius: 3px; cursor: pointer; font-size: 11px; background: white; margin-right: 3px; }
.btn-tiny:hover { background: #f0f0f0; }
.btn-tiny.danger:hover { background: #e53e3e; color: white; border-color: #e53e3e; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { background: #ebebeb !important; }
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
