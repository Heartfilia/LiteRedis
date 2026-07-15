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
        <button class="btn-search icon-search-btn" :disabled="isSearching" :title="t('keyTree.searchBtn')" @click="executeSearch">
          <span v-if="isSearching">…</span>
          <svg v-else viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
            <circle cx="11" cy="11" r="5.5" fill="none" stroke="currentColor" stroke-width="1.8" />
            <path d="M15.2 15.2 19 19" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
          </svg>
        </button>
        <button v-if="searchResults !== null" class="btn-clear-search icon-search-btn" :title="t('keyEditor.cancel')" @click="clearSearch">
          <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
            <path d="M6.7 6.7l10.6 10.6M17.3 6.7L6.7 17.3" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" />
          </svg>
        </button>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayMembers.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.membersCount', { current: sourceMembers.length, total: totalMembers }) }}</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <input v-model="newMember" class="add-row-input" placeholder="member" @keydown.enter="addMember" />
      <input v-model.number="newScore" class="add-row-input" type="number" step="any" placeholder="score" @keydown.enter="addMember" />
      <button class="add-row-btn add-row-btn-primary" @click="addMember">{{ t('keyEditor.add') }}</button>
      <button class="add-row-btn" @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
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
        v-if="(searchResults === null && hasMore) || (searchResults !== null && searchHasMore)"
        class="btn-load-more"
        :disabled="valueLoading"
        @click="loadMore"
      >
        {{ valueLoading ? '...' : t('keyTree.loadMore') }}
      </button>
      <span
        v-else-if="searchResults === null ? (!hasMore && totalMembers > 0) : (!searchHasMore && searchResults.length > 0)"
        class="load-more-hint"
      >
        member:{{ searchResults === null ? totalMembers : searchResults.length }}
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
import { createRequestGuard } from '../../utils/requestGuard.js'
import { zAdd, zRem, renameZSetMember, searchValue, getValue } from '../../api/wails.js'
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
const requestGuard = createRequestGuard(() => ({
  connID: workspaceStore.activeConnID,
  db: workspaceStore.currentDB,
  key: props.keyValue?.key || null,
}))

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
const searchAppliedQuery = ref('')
const searchNextCursor = ref(0)
const searchHasMore = ref(false)
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

async function handleConnectionFailure(error, request = null) {
  if (request && !requestGuard.isCurrent(request)) return true
  if (!isConnectionErrorMessage(error)) return false
  await connectionsStore.handleConnectionFailure(request?.context.connID || workspaceStore.activeConnID, error)
  if (request && !requestGuard.isCurrent(request)) return true
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
  requestGuard.invalidateAll()
  persistSearchState()
})

watch(() => props.keyValue, (kv) => {
  requestGuard.invalidateAll()
  persistSearchState(lastKey.value)
  valueLoading.value = false
  isSearching.value = false
  expandSaving.value = false

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
  searchAppliedQuery.value = ''
  searchNextCursor.value = 0
  searchHasMore.value = false

  activeSort.value = 'score'
  memberSortOrder.value = 'asc'
  scoreSortOrder.value = 'asc'
  msg.value = ''
}, { immediate: true })

async function loadMore() {
  if (valueLoading.value || !props.keyValue?.key) return
  if (searchResults.value !== null && !searchHasMore.value) return
  if (searchResults.value === null && !hasMore.value) return
  const request = requestGuard.begin(searchResults.value !== null ? 'search' : 'load')
  const requestOrder = scoreRequestOrder.value
  valueLoading.value = true
  try {
    if (searchResults.value !== null) {
      const result = await searchValue(
        request.context.connID,
        request.context.key,
        'zset',
        searchAppliedQuery.value,
        false,
        searchNextCursor.value,
      )
      if (!requestGuard.isCurrent(request)) return
      mergeZSetSearchMembers(result.zset_val || [])
      searchNextCursor.value = result.next_cursor || 0
      searchHasMore.value = !!result.has_more && searchNextCursor.value !== 0
      return
    }
    const result = await getValue(request.context.connID, request.context.key, 0, nextOffset.value, requestOrder)
    if (!requestGuard.isCurrent(request)) return
    if (result.zset_val) {
      rawMembers.value.push(...result.zset_val)
    }
    hasMore.value = result.has_more || false
    nextOffset.value = result.next_offset || 0
  } catch (e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message || String(e)
    }
  } finally {
    if (requestGuard.isCurrent(request)) valueLoading.value = false
    requestGuard.finish(request)
  }
}

const scoreRequestOrder = computed(() => scoreSortOrder.value === 'desc' ? 'desc' : 'asc')

async function reloadByScoreSort() {
  if (!props.keyValue?.key) return
  const request = requestGuard.begin('load')
  const requestOrder = scoreRequestOrder.value
  valueLoading.value = true
  try {
    const result = await getValue(request.context.connID, request.context.key, 0, 0, requestOrder)
    if (!requestGuard.isCurrent(request)) return
    rawMembers.value = [...(result.zset_val || [])]
    hasMore.value = result.has_more || false
    nextOffset.value = result.next_offset || 0
    msg.value = ''
  } catch (e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message || String(e)
    }
  } finally {
    if (requestGuard.isCurrent(request)) valueLoading.value = false
    requestGuard.finish(request)
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

function mergeZSetSearchMembers(members, replace = false) {
  const merged = new Map()
  if (!replace) {
    for (const member of searchResults.value || []) {
      merged.set(member.member, member)
    }
  }
  for (const member of members) {
    merged.set(member.member, member)
  }
  searchResults.value = [...merged.values()]
}

async function executeSearch() {
  const pattern = searchQuery.value.trim()
  if (!pattern) { clearSearch(); return }
  const request = requestGuard.begin('search')
  isSearching.value = true
  searchResults.value = []
  searchAppliedQuery.value = ''
  searchNextCursor.value = 0
  searchHasMore.value = false
  try {
    const kv = await searchValue(request.context.connID, request.context.key, 'zset', pattern, false)
    if (!requestGuard.isCurrent(request)) return
    mergeZSetSearchMembers(kv.zset_val || [], true)
    searchAppliedQuery.value = pattern
    searchNextCursor.value = kv.next_cursor || 0
    searchHasMore.value = !!kv.has_more && searchNextCursor.value !== 0
  } catch(e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  }
  finally {
    if (requestGuard.isCurrent(request)) isSearching.value = false
    requestGuard.finish(request)
  }
}

function clearSearch() {
  requestGuard.invalidate('search')
  searchQuery.value = ''
  searchResults.value = null
  searchAppliedQuery.value = ''
  searchNextCursor.value = 0
  searchHasMore.value = false
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
  const request = requestGuard.begin(`write:rename:${oldMember}`)
  expandSaving.value = true
  try {
    const result = await renameZSetMember(request.context.connID, request.context.key, oldMember, newMember)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      renameLocalMember(oldMember, newMember, score)
      expandShow.value = false
    }
  } catch (e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    if (requestGuard.isCurrent(request)) expandSaving.value = false
    requestGuard.finish(request)
  }
}

async function saveEdit(member) {
  if (editingMember.value !== member) return
  const score = editScore.value
  const request = requestGuard.begin(`write:score:${member}`)
  editingMember.value = null
  try {
    const result = await zAdd(request.context.connID, request.context.key, member, score)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) upsertLocalMember(member, score)
  } catch(e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    requestGuard.finish(request)
  }
}

async function addMember() {
  if (!newMember.value.trim()) return
  const member = newMember.value
  const score = newScore.value
  const request = requestGuard.begin('write:add')
  try {
    const result = await zAdd(request.context.connID, request.context.key, member, score)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      upsertLocalMember(member, score)
      newMember.value = ''
      newScore.value = 0
      showAdd.value = false
      triggerAddFlash()
    }
  } catch(e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    requestGuard.finish(request)
  }
}

async function removeMember(member) {
  const request = requestGuard.begin(`write:remove:${member}`)
  try {
    const result = await zRem(request.context.connID, request.context.key, member)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) removeLocalMember(member)
  } catch(e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    requestGuard.finish(request)
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
.btn-add.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: addSuccessPulse 0.42s ease;
}
.zset-wrap { flex: 1; overflow-y: auto; }
.zset-table { width: 100%; border-collapse: collapse; font-size: 12px; table-layout: fixed; }
.zset-table thead { position: sticky; top: 0; z-index: 10; }
.zset-table th { background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.96)); padding: 7px 8px; text-align: left; border-bottom: 1px solid rgba(226, 232, 240, 0.96); font-weight: 700; color: #64748b; font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.08em; }
.zset-table td { padding: 6px 8px; border-bottom: 1px solid rgba(241, 245, 249, 0.96); vertical-align: middle; }
.num-col { width: 36px; text-align: center; }
.num-cell { width: 36px; text-align: center; color: #cbd5e1; font-size: 10px; font-weight: 600; }
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
.score-cell input { width: 80px; height: 28px; padding: 0 8px; border: 1px solid rgba(96, 165, 250, 0.92); border-radius: 6px; font-size: 12px; outline: none; color: #1e293b; background: rgba(255, 255, 255, 0.96); box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.3); }
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
.sortable-col:hover { background: rgba(241, 245, 249, 0.98) !important; }
.sort-icon { display: inline-block; margin-left: 4px; font-size: 10px; color: #cbd5e1; }
.sort-icon.asc, .sort-icon.desc { color: #3b82f6; font-weight: bold; }
.load-more {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: var(--editor-footer-height);
  padding: 4px 12px;
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
  height: 24px;
  min-height: 24px;
  padding: 0 10px;
  background: linear-gradient(180deg, #ffffff, #f8fafc);
  color: #3b82f6;
  border: 1px solid rgba(191, 219, 254, 0.92);
  border-radius: 20px;
  font-size: 10px;
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
  font-size: 11px;
  color: #9ca3af;
  line-height: 1;
}
.action-btns { display: inline-flex; gap: 5px; justify-content: center; align-items: center; }
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
.zset-editor.theme-dark {
  color: #e2e8f0;
}
.zset-editor.theme-dark .search-input,
.zset-editor.theme-dark .add-row-input,
.zset-editor.theme-dark .score-cell input {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}
.zset-editor.theme-dark .count,
.zset-editor.theme-dark .load-more-hint {
  color: #94a3b8;
}
.zset-editor.theme-dark .zset-table th {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.96));
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
