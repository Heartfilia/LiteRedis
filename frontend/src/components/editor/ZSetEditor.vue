<template>
  <div class="zset-editor">
    <div class="toolbar">
      <button class="btn-add" @click="showAdd = !showAdd">+ {{ t('keyEditor.addMember') }}</button>
      <div class="search-bar">
        <input
          v-model="searchQuery"
          class="search-input"
          :placeholder="t('keyEditor.searchMember')"
          @keydown.enter="executeSearch"
        />
        <button class="btn-search" :disabled="isSearching" @click="executeSearch">
          {{ isSearching ? '…' : t('keyTree.searchBtn') }}
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search" @click="clearSearch">✕</button>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayMembers.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.membersCount', { current: sourceMembers.length, total: rawMembers.length }) }}</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <input v-model="newMember" placeholder="member" @keydown.enter="addMember" />
      <input v-model.number="newScore" type="number" step="any" placeholder="score" @keydown.enter="addMember" />
      <button @click="addMember">{{ t('keyEditor.add') }}</button>
      <button @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
    </div>
    <div v-if="msg" :class="['msg-bar', ok ? 'ok' : 'err']">{{ msg }}</div>
    <div class="zset-wrap">
      <table class="zset-table">
        <thead>
          <tr>
            <th class="num-col">#</th>
            <th class="score-th" :style="{ width: scoreWidth + 'px' }">
              <span class="th-content">Score</span>
              <span class="col-resizer" @mousedown.stop="startResizeScore" />
            </th>
            <th class="sortable-col member-th" @click="cycleSortOrder">
              Member <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
            </th>
            <th class="action-th">{{ t('keyEditor.action') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(m, idx) in displayMembers" :key="m.member">
            <td class="num-cell">{{ idx + 1 }}</td>
            <td class="score-cell" :style="{ width: scoreWidth + 'px' }">
              <span v-if="editingMember !== m.member" class="score-text" @click="startEdit(m)">{{ m.score }}</span>
              <input v-else v-model.number="editScore" type="number" step="any"
                @blur="saveEdit(m.member)" @keydown.enter="saveEdit(m.member)" @keydown.esc="editingMember = null" />
            </td>
            <td class="member-cell">
              <span class="val-preview">{{ truncate(m.member) }}</span>
              <span v-if="m.member.length > 80" class="val-ellipsis" @click="openExpand(m.member)">…{{ t('keyEditor.expand') }}</span>
            </td>
            <td class="action-cell">
              <div class="action-btns">
                <template v-if="editingMember !== m.member">
                  <button class="btn-tiny" @click="copyMember(m.member)">
                    {{ copiedMember === m.member ? '✓' : t('keyEditor.copy') }}
                  </button>
                  <button class="btn-tiny" @click="openEdit(m)">{{ t('keyEditor.edit') }}</button>
                  <button class="btn-tiny danger" @click="removeMember(m.member)">{{ t('keyEditor.delete') }}</button>
                </template>
                <template v-else>
                  <button class="btn-tiny btn-confirm-yes" @click="saveEdit(m.member)">✅</button>
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
      <span v-else-if="searchResults === null && !hasMore && rawMembers.length > 0" class="load-more-hint">
        {{ t('keyEditor.allMembersLoaded', { count: rawMembers.length }) }}
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
import { zAdd, zRem, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()

const rawMembers = ref([])
const showAdd = ref(false)
const newMember = ref('')
const newScore = ref(0)
const editingMember = ref(null)
const editScore = ref(0)
const msg = ref('')
const ok = ref(true)
const copiedMember = ref(null)
const scoreWidth = ref(100)

function startResizeScore(e) {
  const startX = e.clientX
  const startWidth = scoreWidth.value
  function onMove(ev) {
    const delta = ev.clientX - startX
    scoreWidth.value = Math.max(60, Math.min(300, startWidth + delta))
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
const searchResults = ref(null)
const isSearching   = ref(false)

// 排序状态（按 member 名称）
const sortOrder = ref('none')
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
const editModalMember = ref('')
const editModalScore = ref(0)

// 服务端分页状态
const hasMore = ref(false)
const nextOffset = ref(0)
const valueLoading = ref(false)

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

// 直接显示所有已加载的数据
const displayMembers = computed(() => sortedMembers.value)

watch(() => props.keyValue, kv => {
  rawMembers.value = [...(kv?.zset_val || [])]
  hasMore.value = kv?.has_more || false
  nextOffset.value = kv?.next_offset || 0
  searchQuery.value = ''
  searchResults.value = null
  sortOrder.value = 'none'
  msg.value = ''
}, { immediate: true })

async function loadMore() {
  if (!hasMore.value || valueLoading.value || !props.keyValue?.key) return
  valueLoading.value = true
  try {
    const result = await getValue(workspaceStore.activeConnID, props.keyValue.key, 0, nextOffset.value)
    if (result.zset_val) {
      rawMembers.value.push(...result.zset_val)
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

async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  isSearching.value = true
  try {
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'zset', pattern)
    searchResults.value = kv.zset_val || []
  } catch(e) { ok.value = false; msg.value = e.message }
  finally { isSearching.value = false }
}

function clearSearch() {
  searchQuery.value = ''
  searchResults.value = null
}

function startEdit(m) { editingMember.value = m.member; editScore.value = m.score }
function cancelEdit() { editingMember.value = null }

function truncate(val, max = 80) {
  if (!val) return val
  return val.length > max ? val.slice(0, max) : val
}

function openExpand(member) {
  expandTitle.value = 'member'
  expandContent.value = member
  expandEditable.value = false
  expandShow.value = true
}

function openEdit(m) {
  expandTitle.value = 'member'
  expandContent.value = m.member
  editModalMember.value = m.member
  editModalScore.value = m.score
  expandEditable.value = true
  expandShow.value = true
}

async function saveFromModal(newMember) {
  const oldMember = editModalMember.value
  const score = editModalScore.value
  if (!oldMember) return
  expandSaving.value = true
  try {
    let result = await zRem(workspaceStore.activeConnID, props.keyValue.key, oldMember)
    if (!result.success) {
      ok.value = false
      msg.value = result.message || t('keyEditor.deleteOldFailed')
      return
    }
    result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, newMember, score)
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      const idx = rawMembers.value.findIndex(m => m.member === oldMember)
      if (idx !== -1) rawMembers.value[idx] = { member: newMember, score }
      if (searchResults.value !== null) {
        const sidx = searchResults.value.findIndex(m => m.member === oldMember)
        if (sidx !== -1) searchResults.value[sidx] = { member: newMember, score }
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

async function saveEdit(member) {
  if (editingMember.value !== member) return
  editingMember.value = null
  try {
    const result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, member, editScore.value)
    ok.value = result.success; msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
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
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      rawMembers.value.push({ member: newMember.value, score: newScore.value })
      newMember.value = ''; newScore.value = 0; showAdd.value = false
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function removeMember(member) {
  try {
    const result = await zRem(workspaceStore.activeConnID, props.keyValue.key, member)
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
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
.zset-wrap { flex: 1; overflow-y: auto; }
.zset-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.zset-table th { background: #f9fafb; padding: 6px 8px; text-align: left; border-bottom: 1px solid #e5e7eb; font-weight: 600; color: #6b7280; font-size: 11px; text-transform: uppercase; letter-spacing: 0.4px; }
.zset-table td { padding: 5px 8px; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
.num-col { width: 36px; text-align: center; }
.num-cell { width: 36px; text-align: center; color: #d1d5db; font-size: 11px; }
.score-th { position: relative; min-width: 60px; }
.member-th { width: auto; }
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
.score-cell { color: #d97706; font-weight: 600; }
.score-text { cursor: pointer; }
.score-cell input { width: 80px; padding: 3px 6px; border: 1px solid #3b82f6; border-radius: 4px; font-size: 12px; outline: none; }
.member-cell { font-family: monospace; }
.val-preview { word-break: break-all; color: #374151; font-size: 12px; }
.val-ellipsis { font-size: 11px; color: #3b82f6; cursor: pointer; white-space: nowrap; margin-left: 2px; }
.val-ellipsis:hover { text-decoration: underline; }
.action-cell { white-space: nowrap; }
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
.msg { font-size: 12px; padding: 5px 10px; border-radius: 6px; }
.ok { background: #f0fdf4; color: #166534; }
.err { background: #fff1f2; color: #991b1b; }

.msg-bar {
  text-align: center;
  font-size: 12px;
  padding: 5px 10px;
  border-radius: 6px;
  margin: 0 6px;
}
.msg-bar.ok { background: #f0fdf4; color: #166534; }
.msg-bar.err { background: #fff1f2; color: #991b1b; }

.action-btns { display: inline-flex; gap: 4px; justify-content: flex-end; }

.btn-confirm-yes { color: #16a34a; border-color: #16a34a; }
.btn-confirm-yes:hover { background: #16a34a; color: white; }
.btn-confirm-no { color: #dc2626; border-color: #dc2626; }
.btn-confirm-no:hover { background: #dc2626; color: white; }
</style>
