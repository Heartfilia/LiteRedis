<template>
  <div class="set-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
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
        <button
          class="btn-search icon-search-btn"
          :disabled="isSearching"
          :title="searchResults !== null ? t('keyEditor.refresh') : t('keyTree.searchBtn')"
          @click="executeSearch"
        >
          <span v-if="isSearching">…</span>
          <svg v-else-if="searchResults !== null" viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
            <path d="M19.25 10.25a7.35 7.35 0 00-13.38-3.1L4.75 8.72" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M4.7 4.55v4.2h4.2" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M4.75 13.75a7.35 7.35 0 0013.38 3.1l1.12-1.57" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M19.3 19.45v-4.2h-4.2" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
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
        <label class="fuzzy-check" :class="{ active: fuzzySearch, disabled: !canToggleFuzzy }" title="模糊搜索需要内容自行带*">
          <input v-model="fuzzySearch" type="checkbox" :disabled="!canToggleFuzzy" />
          <span class="fuzzy-indicator" aria-hidden="true" />
          {{ t('keyEditor.fuzzy') }}
        </label>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayMembers.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.membersCount', { current: sourceMembers.length, total: totalMembers }) }}</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <input v-model="newMember" class="add-row-input" placeholder="member" @keydown.enter="addMember" />
      <button class="add-row-btn add-row-btn-primary" @click="addMember">{{ t('keyEditor.add') }}</button>
      <button class="add-row-btn" @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
    </div>

    <!-- sort header -->
    <div class="set-header">
      <span class="sortable-col" @click="cycleSortOrder">
        {{ t('keyEditor.member') }} <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span>
      </span>
    </div>

    <div class="set-wrap">
      <div v-for="(m, idx) in displayMembers" :key="m" class="set-item">
        <span class="num-badge">{{ idx + 1 }}</span>
        <span class="member-val">
          <span class="val-preview">{{ m }}</span>
        </span>
        <div class="item-actions">
          <button class="btn-tiny" :class="{ copied: copiedMember === m + idx }" @click="copyMember(m, idx)">{{ copiedMember === m + idx ? '✓' : t('keyEditor.copy') }}</button>
          <button class="btn-tiny" @click="openEdit(idx, m)">{{ t('keyEditor.edit') }}</button>
          <InlineDeleteConfirm
            :label="t('keyEditor.delete')"
            :confirm-text="t('keyEditor.confirmDelete')"
            :reset-token="`${props.keyValue?.key || ''}:${m}:${idx}`"
            @confirm="removeMember(m)"
          />
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
        {{ valueLoading ? '...' : t('keyTree.loadMore') }}
      </button>
      <span v-else-if="searchResults === null && !hasMore && totalMembers > 0" class="load-more-hint">
        member:{{ totalMembers }}
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
import { sAdd, sRem, searchValue, getValue } from '../../api/wails.js'
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

const rawMembers = ref([])
const showAdd = ref(false)
const newMember = ref('')
const msg = ref('')
const ok = ref(true)
const copiedMember = ref(null)
const addFlashing = ref(false)
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

// 搜索状态
const searchQuery   = ref('')
const searchResults = ref(null)
const isSearching   = ref(false)
const fuzzySearch   = ref(false)
const canToggleFuzzy = computed(() => searchQuery.value.trim().length > 0)

function appendWildcardIfNeeded() {
  if (!searchQuery.value.includes('*')) {
    searchQuery.value = `${searchQuery.value}*`
  }
}

function removeAllWildcards() {
  if (searchQuery.value.includes('*')) {
    searchQuery.value = searchQuery.value.replaceAll('*', '')
  }
}

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
const expandEditable = ref(false)
const expandSaving = ref(false)
const editModalMember = ref('')

// 服务端分页状态
const hasMore = ref(false)
const nextCursor = ref(0)
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
  if (sortOrder.value === 'none') return sourceMembers.value
  const copy = [...sourceMembers.value]
  if (sortOrder.value === 'asc')  copy.sort((a, b) => a.localeCompare(b))
  if (sortOrder.value === 'desc') copy.sort((a, b) => b.localeCompare(a))
  return copy
})

// 直接显示所有已加载的数据
const displayMembers = computed(() => sortedMembers.value)

const lastKey = ref('')
function persistSearchState(key = props.keyValue?.key || lastKey.value) {
  if (!key) return
  workspaceStore.setEditorSearchState(key, 'set', {
    query: searchQuery.value,
    fuzzy: fuzzySearch.value,
  })
}

watch([searchQuery, fuzzySearch], () => {
  persistSearchState()
})

watch(fuzzySearch, (enabled) => {
  if (enabled) {
    appendWildcardIfNeeded()
    return
  }
  removeAllWildcards()
})

watch(canToggleFuzzy, (enabled) => {
  if (!enabled && fuzzySearch.value) {
    fuzzySearch.value = false
  }
})

onBeforeUnmount(() => {
  persistSearchState()
})

watch(() => props.keyValue, (kv) => {
  persistSearchState(lastKey.value)

  rawMembers.value = [...(kv?.set_val || [])]
  hasMore.value = kv?.has_more || false
  nextCursor.value = kv?.next_cursor || 0
  totalMemberCount.value = kv?.total_count ?? rawMembers.value.length

  if (kv?.key) {
    const cached = workspaceStore.getEditorSearchState(kv.key, 'set')
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
    if (result.set_val) {
      rawMembers.value.push(...result.set_val)
    }
    hasMore.value = result.has_more || false
    nextCursor.value = result.next_cursor || 0
  } catch (e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message || String(e)
    }
  } finally {
    valueLoading.value = false
  }
}

function replaceLocalMember(oldMember, newMember) {
  const hadOld = rawMembers.value.includes(oldMember)
  const hadNew = rawMembers.value.includes(newMember)
  rawMembers.value = [
    ...rawMembers.value.filter(item => item !== oldMember && item !== newMember),
    newMember,
  ]
  if (hadOld && hadNew && oldMember !== newMember) {
    totalMemberCount.value = Math.max(0, totalMemberCount.value - 1)
  }
  if (searchResults.value !== null) {
    const nextSearch = searchResults.value.filter(item => item !== oldMember && item !== newMember)
    nextSearch.push(newMember)
    searchResults.value = nextSearch
  }
}

function removeLocalMember(member) {
  const rawIdx = rawMembers.value.findIndex(item => item === member)
  if (rawIdx !== -1) {
    const nextRaw = [...rawMembers.value]
    nextRaw.splice(rawIdx, 1)
    rawMembers.value = nextRaw
    totalMemberCount.value = Math.max(0, totalMemberCount.value - 1)
  }
  if (searchResults.value !== null) {
    const searchIdx = searchResults.value.findIndex(item => item === member)
    if (searchIdx !== -1) {
      const nextSearch = [...searchResults.value]
      nextSearch.splice(searchIdx, 1)
      searchResults.value = nextSearch
    }
  }
}

function addLocalMember(member) {
  if (rawMembers.value.includes(member)) return
  rawMembers.value = [...rawMembers.value, member]
  totalMemberCount.value++
}

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
    const kv = await searchValue(workspaceStore.activeConnID, props.keyValue.key, 'set', pattern, exact)
    searchResults.value = kv.set_val || []
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
  fuzzySearch.value = false
  if (props.keyValue?.key) {
  workspaceStore.setEditorSearchState(props.keyValue.key, 'set', null)
  }
}

function openEdit(idx, val) {
  expandTitle.value = `member[${idx + 1}]`
  expandContent.value = val
  editModalMember.value = val
  expandEditable.value = true
  expandShow.value = true
}

async function saveFromModal(newVal) {
  const oldMember = editModalMember.value
  if (!oldMember) return
  expandSaving.value = true
  try {
    let result = await sRem(workspaceStore.activeConnID, props.keyValue.key, oldMember)
    if (!result.success && await handleConnectionFailure(result.message)) return
    if (!result.success) {
      ok.value = false
      msg.value = result.message || t('keyEditor.deleteOldFailed')
      return
    }
    result = await sAdd(workspaceStore.activeConnID, props.keyValue.key, newVal)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      replaceLocalMember(oldMember, newVal)
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

async function copyMember(m, idx) {
  await copyToClipboard(m)
  copiedMember.value = m + idx
  setTimeout(() => { copiedMember.value = null }, 1200)
}

async function addMember() {
  if (!newMember.value.trim()) return
  try {
    const result = await sAdd(workspaceStore.activeConnID, props.keyValue.key, newMember.value)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      addLocalMember(newMember.value)
      newMember.value = ''
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

async function removeMember(m) {
  try {
    const result = await sRem(workspaceStore.activeConnID, props.keyValue.key, m)
    if (!result.success && await handleConnectionFailure(result.message)) return
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) removeLocalMember(m)
  } catch(e) {
    if (!(await handleConnectionFailure(e))) {
      ok.value = false
      msg.value = e.message
    }
  }
}
</script>

<style scoped>
.set-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
.btn-add.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: addSuccessPulse 0.42s ease;
}
.fuzzy-check {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 24px;
  margin-left: 4px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.84);
  color: #94a3b8;
  cursor: pointer;
  white-space: nowrap;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  transition: border-color 0.18s ease, background-color 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
}
.fuzzy-check:hover {
  border-color: rgba(203, 213, 225, 0.96);
  background: rgba(248, 250, 252, 0.96);
  color: #64748b;
}
.fuzzy-check.active {
  border-color: rgba(191, 219, 254, 0.96);
  background: rgba(248, 251, 255, 0.98);
  color: #2563eb;
  box-shadow: inset 0 0 0 1px rgba(191, 219, 254, 0.42);
}
.fuzzy-check.disabled {
  opacity: 0.48;
  cursor: not-allowed;
  background: #f8fafc;
  color: #cbd5e1;
  border-color: #e5e7eb;
  box-shadow: none;
}
.fuzzy-check input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.fuzzy-indicator {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: #d4dbe5;
  transition: background-color 0.18s ease, transform 0.18s ease;
}
.fuzzy-check.active .fuzzy-indicator {
  background: #3b82f6;
  transform: scale(1.08);
}
.fuzzy-check.disabled .fuzzy-indicator {
  background: #e2e8f0;
  transform: none;
}
.set-header {
  display: flex; align-items: center; padding: 7px 8px;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.96));
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  font-size: 10.5px; font-weight: 700; color: #64748b;
  text-transform: uppercase; letter-spacing: 0.08em; flex-shrink: 0;
}
.set-wrap { flex: 1; overflow-y: auto; }
.set-item { display: flex; align-items: center; gap: 6px; padding: 6px 8px; border-bottom: 1px solid rgba(241, 245, 249, 0.96); font-size: 12px; }
.set-item:hover { background: rgba(248, 250, 252, 0.9); }
.num-badge { background: rgba(245, 243, 255, 0.96); color: #7c3aed; padding: 1px 6px; border-radius: 4px; font-size: 10px; flex-shrink: 0; min-width: 28px; text-align: center; font-weight: 600; }
.set-header,
.toolbar,
.search-bar,
.btn-add,
.btn-search,
.btn-clear-search,
.fuzzy-check,
.count,
.add-row button,
.num-badge,
.item-actions {
  user-select: none;
  -webkit-user-select: none;
}
.member-val {
  font-family: monospace;
  flex: 1;
  min-width: 0;
  display: block;
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
.item-actions { display: flex; gap: 5px; flex-shrink: 0; }
.sortable-col { cursor: pointer; user-select: none; }
.sortable-col:hover { color: #374151; }
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
.btn-tiny.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  animation: copyPulse 0.26s ease;
}
.set-editor.theme-dark {
  color: #e2e8f0;
}
.set-editor.theme-dark .fuzzy-check {
  background: rgba(15, 23, 42, 0.7);
  color: #94a3b8;
  border-color: transparent;
}
.set-editor.theme-dark .fuzzy-check:hover {
  background: rgba(30, 41, 59, 0.94);
  color: #cbd5e1;
  border-color: rgba(96, 165, 250, 0.28);
}
.set-editor.theme-dark .fuzzy-check.active {
  background: rgba(30, 64, 175, 0.18);
  color: #93c5fd;
  border-color: rgba(96, 165, 250, 0.42);
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.2);
}
.set-editor.theme-dark .fuzzy-check.disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}
.set-editor.theme-dark .fuzzy-indicator {
  background: #475569;
}
.set-editor.theme-dark .fuzzy-check.active .fuzzy-indicator {
  background: #60a5fa;
}
.set-editor.theme-dark .count,
.set-editor.theme-dark .load-more-hint {
  color: #94a3b8;
}
.set-editor.theme-dark .add-row,
.set-editor.theme-dark .set-header {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.92), rgba(15, 23, 42, 0.88));
  border-color: rgba(51, 65, 85, 0.92);
  color: #94a3b8;
  box-shadow: inset 0 1px 0 rgba(71, 85, 105, 0.24);
}
.set-editor.theme-dark .set-item {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}
.set-editor.theme-dark .set-item:hover {
  background: rgba(30, 41, 59, 0.66);
}
.set-editor.theme-dark .num-badge {
  background: rgba(55, 48, 163, 0.28);
  color: #c4b5fd;
}
.set-editor.theme-dark .val-preview {
  color: #cbd5e1;
}
.set-editor.theme-dark .sortable-col:hover {
  color: #e2e8f0;
}
.set-editor.theme-dark .load-more {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.98));
}
.set-editor.theme-dark .btn-load-more {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.28);
}

.set-editor.theme-dark .btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.48);
  box-shadow: 0 8px 18px rgba(2, 6, 23, 0.34);
}

.set-editor.theme-dark .btn-load-more:disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}

.set-editor.theme-dark .btn-tiny {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.set-editor.theme-dark .btn-tiny.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}

.set-editor.theme-dark .btn-tiny:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: rgba(96, 165, 250, 0.34);
}

.set-editor.theme-dark .btn-add.success-flash {
  background: rgba(9, 59, 44, 0.94);
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.5);
  box-shadow: 0 0 0 1px rgba(167, 243, 208, 0.08) inset, 0 10px 22px rgba(5, 150, 105, 0.22);
}

.set-editor.theme-dark .item-actions :deep(.delete-wrap > .btn-tiny),
.set-editor.theme-dark .item-actions :deep(.delete-wrap > .btn-tiny:hover) {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.set-editor.theme-dark .item-actions :deep(.delete-wrap > .btn-tiny:hover) {
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
