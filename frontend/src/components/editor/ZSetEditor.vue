<template>
  <div class="zset-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <FloatingMessage :message="msg" :success="ok" />
    <div class="toolbar">
      <button class="btn-add" :class="{ 'success-flash': addFlashing }" :title="t('keyEditor.addMember')" @click="showAdd = !showAdd">+</button>
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
        <template v-else>{{ t('keyEditor.membersCount', { current: sourceMembers.length, total: totalMembers }) }}</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <input v-model="newMember" placeholder="member" @keydown.enter="addMember" />
      <input v-model.number="newScore" type="number" step="any" placeholder="score" @keydown.enter="addMember" />
      <button @click="addMember">{{ t('keyEditor.add') }}</button>
      <button @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
    </div>
    <div class="zset-wrap">
      <table class="zset-table">
        <thead>
          <tr>
            <th class="num-col">#</th>
            <th class="sortable-col score-th" :style="{ width: scoreWidth + 'px' }" @click="cycleScoreSort">
              <span class="th-content">Score <span class="sort-icon" :class="scoreSortOrder">{{ scoreSortIcon }}</span></span>
              <span class="col-resizer" @mousedown.stop="startResizeScore" />
            </th>
            <th class="sortable-col member-th" @click="cycleMemberSort">
              Member <span class="sort-icon" :class="memberSortOrder">{{ memberSortIcon }}</span>
            </th>
            <th class="action-th"><span class="action-th-label">{{ t('keyEditor.action') }}</span></th>
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
              <span class="val-preview">{{ m.member }}</span>
            </td>
            <td class="action-cell">
              <div class="action-btns">
                <template v-if="editingMember !== m.member">
                  <button class="btn-tiny" :class="{ copied: copiedMember === m.member }" @click="copyMember(m.member)">
                    {{ copiedMember === m.member ? '✓' : t('keyEditor.copy') }}
                  </button>
                  <button class="btn-tiny" @click="openEdit(m)">{{ t('keyEditor.edit') }}</button>
                  <InlineDeleteConfirm
                    :label="t('keyEditor.delete')"
                    :confirm-text="t('keyEditor.confirmDelete')"
                    :reset-token="`${props.keyValue?.key || ''}:${m.member}`"
                    @confirm="removeMember(m.member)"
                  />
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
      <span v-else-if="searchResults === null && !hasMore && totalMembers > 0" class="load-more-hint">
        {{ t('keyEditor.allMembersLoaded', { count: totalMembers }) }}
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
import { zAdd, zRem, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import FloatingMessage from '../common/FloatingMessage.vue'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../../utils/connection.js'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
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
const addFlashing = ref(false)
const scoreWidth = ref(180)
const totalMemberCount = ref(0)
let addFlashTimer = null

function triggerAddFlash() {
  if (addFlashTimer) clearTimeout(addFlashTimer)
  addFlashing.value = true
  addFlashTimer = setTimeout(() => {
    addFlashing.value = false
    addFlashTimer = null
  }, 1100)
}

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

// 排序状态
const activeSort = ref('score')
const memberSortOrder = ref('asc')
const scoreSortOrder = ref('asc')
const memberSortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[activeSort.value === 'member' ? memberSortOrder.value : 'none'])
const scoreSortIcon = computed(() => ({ none: '⇅', asc: '↑', desc: '↓' })[activeSort.value === 'score' ? scoreSortOrder.value : 'none'])

function cycleMemberSort() {
  activeSort.value = 'member'
  memberSortOrder.value = memberSortOrder.value === 'asc' ? 'desc' : 'asc'
}

async function cycleScoreSort() {
  activeSort.value = 'score'
  scoreSortOrder.value = scoreSortOrder.value === 'asc' ? 'desc' : 'asc'
  if (searchResults.value === null) {
    await reloadByScoreSort()
  }
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

async function handleConnectionFailure(error) {
  if (!isConnectionErrorMessage(error)) return false
  await connectionsStore.handleConnectionFailure(workspaceStore.activeConnID, error)
  ok.value = false
  msg.value = formatConnectionLostMessage(error)
  return true
}
const totalMembers = computed(() => totalMemberCount.value >= 0 ? totalMemberCount.value : rawMembers.value.length)

const sourceMembers = computed(() =>
  searchResults.value !== null ? searchResults.value : rawMembers.value
)

const sortedMembers = computed(() => {
  if (activeSort.value === 'score' && searchResults.value !== null) {
    const copy = [...sourceMembers.value]
    if (scoreSortOrder.value === 'asc') copy.sort((a, b) => a.score - b.score)
    if (scoreSortOrder.value === 'desc') copy.sort((a, b) => b.score - a.score)
    return copy
  }
  if (activeSort.value !== 'member') return sourceMembers.value
  const copy = [...sourceMembers.value]
  if (memberSortOrder.value === 'asc')  copy.sort((a, b) => a.member.localeCompare(b.member))
  if (memberSortOrder.value === 'desc') copy.sort((a, b) => b.member.localeCompare(a.member))
  return copy
})

// 直接显示所有已加载的数据
const displayMembers = computed(() => sortedMembers.value)

const lastKey = ref('')
function persistSearchState(key = props.keyValue?.key || lastKey.value) {
  if (!key) return
  workspaceStore.setEditorSearchState(key, 'zset', {
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

  rawMembers.value = [...(kv?.zset_val || [])]
  hasMore.value = kv?.has_more || false
  nextOffset.value = kv?.next_offset || 0
  totalMemberCount.value = kv?.total_count ?? rawMembers.value.length

  if (kv?.key) {
    const cached = workspaceStore.getEditorSearchState(kv.key, 'zset')
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

  activeSort.value = 'score'
  memberSortOrder.value = 'asc'
  scoreSortOrder.value = 'asc'
  msg.value = ''
}, { immediate: true })

async function loadMore() {
  if (!hasMore.value || valueLoading.value || !props.keyValue?.key) return
  valueLoading.value = true
  try {
    const result = await getValue(workspaceStore.activeConnID, props.keyValue.key, 0, nextOffset.value, scoreRequestOrder.value)
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

const scoreRequestOrder = computed(() => scoreSortOrder.value === 'desc' ? 'desc' : 'asc')

async function reloadByScoreSort() {
  if (!props.keyValue?.key) return
  valueLoading.value = true
  try {
    const result = await getValue(workspaceStore.activeConnID, props.keyValue.key, 0, 0, scoreRequestOrder.value)
    rawMembers.value = [...(result.zset_val || [])]
    hasMore.value = result.has_more || false
    nextOffset.value = result.next_offset || 0
    msg.value = ''
  } catch (e) {
    ok.value = false
    msg.value = e.message || String(e)
  } finally {
    valueLoading.value = false
  }
}

function sortRawMembersByScore() {
  if (searchResults.value !== null || activeSort.value !== 'score') return
  rawMembers.value = [...rawMembers.value].sort((a, b) =>
    scoreSortOrder.value === 'asc' ? a.score - b.score : b.score - a.score
  )
}

function upsertLocalMember(member, score) {
  const nextRaw = [...rawMembers.value]
  const rawIdx = nextRaw.findIndex(item => item.member === member)
  if (rawIdx === -1) {
    nextRaw.push({ member, score })
    totalMemberCount.value++
  } else {
    nextRaw[rawIdx] = { member, score }
  }
  rawMembers.value = nextRaw
  if (searchResults.value !== null) {
    const nextSearch = [...searchResults.value]
    const searchIdx = nextSearch.findIndex(item => item.member === member)
    if (searchIdx !== -1) {
      nextSearch[searchIdx] = { member, score }
      searchResults.value = nextSearch
    }
  }
  sortRawMembersByScore()
}

function renameLocalMember(oldMember, newMember, score) {
  const hadOld = rawMembers.value.some(item => item.member === oldMember)
  const hadNew = rawMembers.value.some(item => item.member === newMember)
  rawMembers.value = [
    ...rawMembers.value.filter(item => item.member !== oldMember && item.member !== newMember),
    { member: newMember, score },
  ]
  if (hadOld && hadNew && oldMember !== newMember) {
    totalMemberCount.value = Math.max(0, totalMemberCount.value - 1)
  }
  sortRawMembersByScore()
  if (searchResults.value !== null) {
    const nextSearch = searchResults.value.filter(item => item.member !== oldMember && item.member !== newMember)
    nextSearch.push({ member: newMember, score })
    searchResults.value = nextSearch
  }
}

function removeLocalMember(member) {
  const rawIdx = rawMembers.value.findIndex(item => item.member === member)
  if (rawIdx !== -1) {
    const nextRaw = [...rawMembers.value]
    nextRaw.splice(rawIdx, 1)
    rawMembers.value = nextRaw
    totalMemberCount.value = Math.max(0, totalMemberCount.value - 1)
  }
  if (searchResults.value !== null) {
    searchResults.value = searchResults.value.filter(item => item.member !== member)
  }
}

async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  isSearching.value = true
  try {
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'zset', pattern, false)
    searchResults.value = kv.zset_val || []
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
  workspaceStore.setEditorSearchState(props.keyValue.key, 'zset', null)
  }
}

function startEdit(m) { editingMember.value = m.member; editScore.value = m.score }
function cancelEdit() { editingMember.value = null }

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
    if (!result.success && await handleConnectionFailure(result.message)) return
    if (!result.success) {
      ok.value = false
      msg.value = result.message || t('keyEditor.deleteOldFailed')
      return
    }
    result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, newMember, score)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      renameLocalMember(oldMember, newMember, score)
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

async function saveEdit(member) {
  if (editingMember.value !== member) return
  editingMember.value = null
  try {
    const result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, member, editScore.value)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) upsertLocalMember(member, editScore.value)
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
}

async function addMember() {
  if (!newMember.value.trim()) return
  try {
    const result = await zAdd(workspaceStore.activeConnID, props.keyValue.key, newMember.value, newScore.value)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      upsertLocalMember(newMember.value, newScore.value)
      newMember.value = ''
      newScore.value = 0
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

async function removeMember(member) {
  try {
    const result = await zRem(workspaceStore.activeConnID, props.keyValue.key, member)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) removeLocalMember(member)
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
}

async function copyMember(member) {
  await copyToClipboard(member)
  copiedMember.value = member
  setTimeout(() => { copiedMember.value = null }, 1200)
}
</script>

<style scoped>
.zset-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
.toolbar { display: flex; align-items: center; gap: 2px; flex-wrap: wrap; }
.btn-add {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  min-width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid #d1d5db;
  border-radius: 5px;
  cursor: pointer;
  background: #fff;
  color: #374151;
  font-size: 18px;
  font-weight: 800;
  line-height: 1;
  box-sizing: border-box;
}
.btn-add:hover { background: #f3f4f6; }
.btn-add.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: addSuccessPulse 0.42s ease;
}
.search-bar { display: flex; align-items: center; min-height: 28px; }
.search-input {
  width: 130px; height: 28px; min-height: 28px; padding: 0 8px;
  border: 1px solid #d1d5db; border-right: none;
  border-radius: 5px 0 0 5px; font-size: 12px; outline: none; color: #333;
  line-height: 28px; box-sizing: border-box;
}
.search-input:focus { border-color: #3b82f6; }
.count { font-size: 12px; color: #9ca3af; margin-left: auto; white-space: nowrap; }
.add-row { display: flex; gap: 6px; padding: 6px; background: #f9fafb; border-radius: 6px; border: 1px solid #e5e7eb; }
.add-row input { flex: 1; padding: 4px 8px; border: 1px solid #d1d5db; border-radius: 5px; font-size: 12px; outline: none; }
.add-row input:focus { border-color: #3b82f6; }
.add-row button { padding: 4px 10px; border: 1px solid #d1d5db; border-radius: 5px; cursor: pointer; font-size: 12px; background: #fff; color: #374151; }
.add-row button:hover { background: #f3f4f6; }
.zset-wrap { flex: 1; overflow-y: auto; }
.zset-table { width: 100%; border-collapse: collapse; font-size: 12px; table-layout: fixed; }
.zset-table thead { position: sticky; top: 0; z-index: 10; }
.zset-table th { background: #f9fafb; padding: 6px 8px; text-align: left; border-bottom: 1px solid #e5e7eb; font-weight: 600; color: #6b7280; font-size: 11px; text-transform: uppercase; letter-spacing: 0.4px; }
.zset-table td { padding: 5px 8px; border-bottom: 1px solid #f3f4f6; vertical-align: middle; }
.num-col { width: 36px; text-align: center; }
.num-cell { width: 36px; text-align: center; color: #d1d5db; font-size: 11px; }
.zset-table thead,
.toolbar,
.search-bar,
.btn-add,
.btn-search,
.btn-clear-search,
.count,
.add-row button,
.num-col,
.num-cell,
.score-th,
.score-cell,
.score-text,
.action-th,
.action-cell,
.action-btns {
  user-select: none;
  -webkit-user-select: none;
}
.score-th { position: relative; min-width: 60px; }
.member-th { width: auto; }
.action-th {
  width: 170px;
  min-width: 170px;
  max-width: 170px;
  white-space: nowrap;
  text-align: center;
}
.zset-table th.action-th {
  text-align: center !important;
}
.action-th-label {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  text-align: center;
}
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
.score-cell {
  color: #d97706;
  font-weight: 600;
  min-width: 160px;
}
.score-text { cursor: pointer; }
.score-cell input { width: 80px; padding: 3px 6px; border: 1px solid #3b82f6; border-radius: 4px; font-size: 12px; outline: none; }
.member-th,
.member-cell {
  min-width: 0;
}
.member-cell {
  font-family: monospace;
  overflow: hidden;
}
.val-preview {
  color: #374151;
  font-size: 12px;
  display: block;
  min-width: 0;
  width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.action-th,
.action-cell {
  width: 170px;
  min-width: 170px;
  max-width: 170px;
  text-align: center;
  white-space: nowrap;
}
.member-th { user-select: none; -webkit-user-select: none; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { background: #f3f4f6 !important; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 10px; color: #d1d5db; }
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
.action-btns { display: inline-flex; gap: 4px; justify-content: center; }
.btn-tiny {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 26px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.96);
  color: #475569;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  box-sizing: border-box;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease, transform 0.16s ease;
}

.btn-tiny:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
  transform: translateY(-1px);
}
.btn-tiny.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  animation: copyPulse 0.26s ease;
}

.btn-confirm-yes {
  color: #16a34a;
  border-color: #16a34a;
}

.btn-confirm-yes:hover {
  background: #16a34a;
  color: #fff;
}

.btn-confirm-no {
  color: #dc2626;
  border-color: #dc2626;
}

.btn-confirm-no:hover {
  background: #dc2626;
  color: #fff;
}
:global(.app-layout.theme-dark) .zset-editor {
  color: #e2e8f0;
}
:global(.app-layout.theme-dark) .zset-editor .btn-add,
:global(.app-layout.theme-dark) .zset-editor .add-row button,
:global(.app-layout.theme-dark) .zset-editor .btn-search,
:global(.app-layout.theme-dark) .zset-editor .btn-clear-search {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
:global(.app-layout.theme-dark) .zset-editor .btn-add:hover,
:global(.app-layout.theme-dark) .zset-editor .add-row button:hover,
:global(.app-layout.theme-dark) .zset-editor .btn-search:hover,
:global(.app-layout.theme-dark) .zset-editor .btn-clear-search:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: #60a5fa;
}
:global(.app-layout.theme-dark) .zset-editor .search-input,
:global(.app-layout.theme-dark) .zset-editor .add-row input,
:global(.app-layout.theme-dark) .zset-editor .score-cell input {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}
:global(.app-layout.theme-dark) .zset-editor .count,
:global(.app-layout.theme-dark) .zset-editor .load-more-hint {
  color: #94a3b8;
}
:global(.app-layout.theme-dark) .zset-editor .add-row {
  background: rgba(30, 41, 59, 0.72);
  border-color: rgba(51, 65, 85, 0.92);
}
:global(.app-layout.theme-dark) .zset-editor .zset-table th {
  background: rgba(30, 41, 59, 0.92);
  color: #94a3b8;
  border-bottom-color: rgba(51, 65, 85, 0.92);
}
:global(.app-layout.theme-dark) .zset-editor .zset-table td {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}
:global(.app-layout.theme-dark) .zset-editor .num-cell,
:global(.app-layout.theme-dark) .zset-editor .sort-icon {
  color: #475569;
}
:global(.app-layout.theme-dark) .zset-editor .score-cell {
  color: #fbbf24;
}
:global(.app-layout.theme-dark) .zset-editor .val-preview {
  color: #cbd5e1;
}
:global(.app-layout.theme-dark) .zset-editor .sortable-col:hover {
  background: rgba(30, 41, 59, 0.92) !important;
}
:global(.app-layout.theme-dark) .zset-editor .col-resizer {
  background: rgba(30, 41, 59, 0.94);
  border-left-color: rgba(51, 65, 85, 0.92);
  border-right-color: rgba(51, 65, 85, 0.92);
}
:global(.app-layout.theme-dark) .zset-editor .load-more {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.98));
}
:global(.app-layout.theme-dark) .zset-editor .btn-load-more {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.28);
}
:global(.app-layout.theme-dark) .zset-editor .btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.48);
  box-shadow: 0 8px 18px rgba(2, 6, 23, 0.34);
}
:global(.app-layout.theme-dark) .zset-editor .btn-load-more:disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}
.zset-editor.theme-dark {
  color: #e2e8f0;
}

.zset-editor.theme-dark .btn-add,
.zset-editor.theme-dark .add-row button,
.zset-editor.theme-dark .btn-search,
.zset-editor.theme-dark .btn-clear-search {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.zset-editor.theme-dark .btn-add:hover,
.zset-editor.theme-dark .add-row button:hover,
.zset-editor.theme-dark .btn-search:hover,
.zset-editor.theme-dark .btn-clear-search:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: #60a5fa;
}

.zset-editor.theme-dark .search-input,
.zset-editor.theme-dark .add-row input,
.zset-editor.theme-dark .score-cell input {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}

.zset-editor.theme-dark .count,
.zset-editor.theme-dark .load-more-hint {
  color: #94a3b8;
}

.zset-editor.theme-dark .add-row {
  background: rgba(30, 41, 59, 0.72);
  border-color: rgba(51, 65, 85, 0.92);
}

.zset-editor.theme-dark .zset-table th {
  background: rgba(30, 41, 59, 0.92);
  color: #94a3b8;
  border-bottom-color: rgba(51, 65, 85, 0.92);
}

.zset-editor.theme-dark .zset-table td {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}

.zset-editor.theme-dark .num-cell,
.zset-editor.theme-dark .sort-icon {
  color: #475569;
}

.zset-editor.theme-dark .score-cell {
  color: #fbbf24;
}

.zset-editor.theme-dark .val-preview {
  color: #cbd5e1;
}

.zset-editor.theme-dark .sortable-col:hover {
  background: rgba(30, 41, 59, 0.92) !important;
}

.zset-editor.theme-dark .col-resizer {
  background: rgba(30, 41, 59, 0.94);
  border-left-color: rgba(51, 65, 85, 0.92);
  border-right-color: rgba(51, 65, 85, 0.92);
}

.zset-editor.theme-dark .load-more {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.98));
}

.zset-editor.theme-dark .btn-load-more {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.28);
}

.zset-editor.theme-dark .btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.48);
  box-shadow: 0 8px 18px rgba(2, 6, 23, 0.34);
}

.zset-editor.theme-dark .btn-load-more:disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}

.zset-editor.theme-dark .btn-tiny {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.zset-editor.theme-dark .btn-tiny.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}

.zset-editor.theme-dark .btn-tiny:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: rgba(96, 165, 250, 0.34);
}

.zset-editor.theme-dark .btn-add.success-flash {
  background: rgba(9, 59, 44, 0.94);
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.5);
  box-shadow: 0 0 0 1px rgba(167, 243, 208, 0.08) inset, 0 10px 22px rgba(5, 150, 105, 0.22);
}

.zset-editor.theme-dark .action-btns :deep(.delete-wrap > .btn-tiny),
.zset-editor.theme-dark .action-btns :deep(.delete-wrap > .btn-tiny:hover) {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.zset-editor.theme-dark .action-btns :deep(.delete-wrap > .btn-tiny:hover) {
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
