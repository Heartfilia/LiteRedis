<template>
  <div class="set-editor">
    <FloatingMessage :message="msg" :success="ok" />
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
        <label class="fuzzy-check" title="模糊搜索需要内容自行带*">
          <input v-model="fuzzySearch" type="checkbox" />
          {{ t('keyEditor.fuzzy') }}
        </label>
      </div>
      <span class="count">
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayMembers.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.membersCount', { current: sourceMembers.length, total: totalMembers }) }}</template>
      </span>
    </div>
    <div v-if="showAdd" class="add-row">
      <input v-model="newMember" placeholder="member" @keydown.enter="addMember" />
      <button @click="addMember">{{ t('keyEditor.add') }}</button>
      <button @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
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
          <span class="val-preview">{{ truncate(m) }}</span>
          <span v-if="m.length > 80" class="val-ellipsis" @click="openExpand(idx, m)">…{{ t('keyEditor.expand') }}</span>
        </span>
        <div class="item-actions">
          <button class="btn-tiny" @click="copyMember(m, idx)">{{ copiedMember === m + idx ? '✓' : t('keyEditor.copy') }}</button>
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
import { ref, computed, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { sAdd, sRem, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import FloatingMessage from '../common/FloatingMessage.vue'

const props = defineProps({ keyValue: Object })
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()

const rawMembers = ref([])
const showAdd = ref(false)
const newMember = ref('')
const msg = ref('')
const ok = ref(true)
const copiedMember = ref(null)
const totalMemberCount = ref(0)

// 搜索状态
const searchQuery   = ref('')
const searchResults = ref(null)
const isSearching   = ref(false)
const fuzzySearch   = ref(false)

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

watch(() => props.keyValue, kv => {
  rawMembers.value = [...(kv?.set_val || [])]
  hasMore.value = kv?.has_more || false
  nextCursor.value = kv?.next_cursor || 0
  totalMemberCount.value = kv?.total_count ?? rawMembers.value.length
  searchQuery.value = ''
  searchResults.value = null
  fuzzySearch.value = false
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
    ok.value = false
    msg.value = e.message || String(e)
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

function openExpand(idx, val) {
  expandTitle.value = `member[${idx + 1}]`
  expandContent.value = val
  expandEditable.value = false
  expandShow.value = true
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
    if (!result.success) {
      ok.value = false
      msg.value = result.message || t('keyEditor.deleteOldFailed')
      return
    }
    result = await sAdd(workspaceStore.activeConnID, props.keyValue.key, newVal)
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      replaceLocalMember(oldMember, newVal)
      expandShow.value = false
    }
  } catch (e) {
    ok.value = false
    msg.value = e.message
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
    ok.value = result.success; msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      addLocalMember(newMember.value)
      newMember.value = ''
      showAdd.value = false
    }
  } catch(e) { ok.value = false; msg.value = e.message }
}

async function removeMember(m) {
  try {
    const result = await sRem(workspaceStore.activeConnID, props.keyValue.key, m)
    ok.value = result.success; msg.value = result.success ? t('keyEditor.deleted') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) removeLocalMember(m)
  } catch(e) { ok.value = false; msg.value = e.message }
}
</script>

<style scoped>
.set-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
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
