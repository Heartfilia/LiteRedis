<template>
  <div class="hash-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <FloatingMessage :message="msg" :success="ok" />
    <div class="toolbar">
      <button class="btn-add" :class="{ 'success-flash': addFlashing }" :title="t('keyEditor.addField')" @click="showAdd = !showAdd">+</button>
      <div class="search-bar" :class="{ 'search-bar-filter-active': calcSearchHighlight, 'search-bar-invalid': calcFilterHintVisible }">
        <input
          v-model="searchQuery"
          class="search-input"
          :placeholder="t('keyEditor.searchField')"
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
        <template v-if="searchResults !== null">{{ t('keyEditor.searchResult', { current: displayEntries.length, total: searchResults.length }) }}</template>
        <template v-else>{{ t('keyEditor.fieldsCount', { current: sourceEntries.length, total: totalFields }) }}</template>
      </span>
      <button class="btn-calc" :class="{ active: calcPanelOpen }" :title="t('keyEditor.calcTitle')" @click="toggleCalcPanel">
        <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
          <path
            d="M6.75 4.75h10.5A1.75 1.75 0 0 1 19 6.5v11a1.75 1.75 0 0 1-1.75 1.75H6.75A1.75 1.75 0 0 1 5 17.5v-11a1.75 1.75 0 0 1 1.75-1.75Z"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linejoin="round"
          />
          <path
            d="M8 8.25h8"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
          />
          <path
            d="M8.25 15.4 10.5 13.15l1.95 1.8 3.3-3.6"
            fill="none"
            stroke="currentColor"
            stroke-width="1.8"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
          <circle cx="8.25" cy="15.4" r="1" fill="currentColor" />
          <circle cx="10.5" cy="13.15" r="1" fill="currentColor" />
          <circle cx="12.45" cy="14.95" r="1" fill="currentColor" />
          <circle cx="15.75" cy="11.35" r="1" fill="currentColor" />
        </svg>
      </button>
    </div>

    <transition name="calc-panel">
        <div v-if="calcPanelOpen" class="calc-panel">
          <div class="calc-panel-header">
          <span class="calc-panel-title">{{ t('keyEditor.calcTitle') }}</span>
          <div class="calc-panel-actions">
            <button
              class="calc-filter-btn"
              :class="{ active: calcUseSearchFilter, disabled: !canUseCalcSearchFilter }"
              :title="canUseCalcSearchFilter ? t('keyEditor.calcFilterToggleHint') : t('keyEditor.calcFilterToggleDisabledHint')"
              @click="toggleCalcSearchFilter"
            >
              <svg viewBox="0 0 24 24" width="13" height="13" aria-hidden="true">
                <path
                  d="M4.75 6.25h14.5l-5.8 6.47v4.53l-2.9 1.5v-6.03z"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linejoin="round"
                />
              </svg>
            </button>
            <button class="calc-clear-btn" :title="t('keyEditor.calcClear')" @click="resetCalcPanelState">
              <svg viewBox="0 0 24 24" width="13" height="13" aria-hidden="true">
                <path
                  d="M9 5.75h6"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M7.25 7.25h9.5l-.7 10.05a1.5 1.5 0 0 1-1.5 1.4H9.45a1.5 1.5 0 0 1-1.5-1.4z"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <path
                  d="M10.25 10.25v5.25M13.75 10.25v5.25M8.25 7.25V6.5a.75.75 0 0 1 .75-.75h6a.75.75 0 0 1 .75.75v.75"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.7"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </button>
            <button class="calc-min-btn" :title="t('keyEditor.calcMinimize')" @click="closeCalcPanel">
              <svg viewBox="0 0 24 24" width="13" height="13" aria-hidden="true">
                <path
                  d="M6.5 12.5h11"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="1.9"
                  stroke-linecap="round"
                />
              </svg>
            </button>
          </div>
        </div>

        <div class="calc-panel-body">
          <div class="calc-config">
            <label class="calc-label">{{ t('keyEditor.calcRuleLabel') }}:</label>
            <input
              v-model="calcRule"
              class="calc-rule-input"
              :class="{ invalid: calcRuleInvalid }"
              type="text"
              :placeholder="t('keyEditor.calcRulePlaceholder')"
              @keydown.enter.prevent="runCalculation"
            />
          </div>
          <div class="calc-rule-hint">
            {{ t('keyEditor.calcRuleHint') }}
          </div>

          <div v-if="canAddCalcSubRule" class="calc-subrules">
            <div class="calc-subrules-header">
              <span class="calc-subrules-title">{{ t('keyEditor.calcSubRuleLabel') }}</span>
              <button class="calc-add-subrule" @click="addCalcSubRule">+ {{ t('keyEditor.calcAddSubRule') }}</button>
            </div>

            <div v-if="!calcSubRules.length" class="calc-subrules-empty">
              {{ t('keyEditor.calcSubRuleEmpty') }}
            </div>

            <div v-for="subRule in calcSubRules" :key="subRule.id" class="calc-subrule-row">
              <select v-model="subRule.group" class="calc-subrule-group">
                <option v-for="groupOption in calcGroupOptions" :key="groupOption.key" :value="groupOption.key">
                  {{ groupOption.label }}
                </option>
              </select>
              <select v-model="subRule.operator" class="calc-subrule-operator">
                <option value="==">==</option>
                <option value=">">></option>
                <option value=">=">>=</option>
                <option value="<"><</option>
                <option value="<="><=</option>
                <option value="!=">!=</option>
              </select>
              <input v-model="subRule.value" class="calc-subrule-value" type="text" />
              <button class="calc-remove-subrule" @click="removeCalcSubRule(subRule.id)">✕</button>
            </div>
          </div>

          <button class="calc-run-btn" :disabled="calculating" @click="runCalculation">
            {{ calculating ? t('keyEditor.calcRunning') : t('keyEditor.calcRun') }}
          </button>

          <div class="calc-log-wrap">
            <div class="calc-log-header">
              <div class="calc-log-header-left">
                <div class="calc-log-title">{{ t('keyEditor.calcLogTitle') }}</div>
                <select v-model="calcLogFilter" class="calc-log-filter">
                  <option value="all">{{ t('keyEditor.calcLogFilterAll') }}</option>
                  <option value="ok">{{ t('keyEditor.calcLogFilterOk') }}</option>
                  <option value="skip">{{ t('keyEditor.calcLogFilterSkip') }}</option>
                  <option value="err">{{ t('keyEditor.calcLogFilterErr') }}</option>
                </select>
              </div>
              <button class="calc-copy-log-btn" :class="{ copied: calcLogCopied }" @click="copyCalcLogs">
                {{ calcLogCopied ? t('keyEditor.calcCopiedLog') : t('keyEditor.calcCopyLog') }}
              </button>
            </div>
            <div class="calc-log-output">
              <template v-if="filteredCalcLogs.length">
                <div
                  v-for="log in filteredCalcLogs"
                  :key="log.id"
                  class="calc-log-line"
                  :class="[`tone-${log.tone}`, { spacer: log.tone === 'spacer' }]"
                >
                  {{ log.text }}
                </div>
              </template>
              <div v-else-if="calcLogs.length" class="calc-log-line tone-muted">
                {{ t('keyEditor.calcLogFilteredEmpty') }}
              </div>
              <div v-else class="calc-log-line tone-muted">{{ t('keyEditor.calcLogEmpty') }}</div>
            </div>
          </div>
        </div>
      </div>
    </transition>

    <!-- 添加新 field -->
    <div v-if="showAdd" class="add-row">
      <input v-model="newField" class="add-row-input" placeholder="field" @keydown.enter="addField" />
      <input v-model="newValue" class="add-row-input" placeholder="value" @keydown.enter="addField" />
      <button class="add-row-btn add-row-btn-primary" @click="addField">{{ t('keyEditor.add') }}</button>
      <button class="add-row-btn" @click="showAdd = false">{{ t('keyEditor.cancel') }}</button>
    </div>

    <!-- hash 表格 -->
    <div class="hash-table-wrap">
      <table class="hash-table">
        <colgroup>
          <col class="num-colgroup" />
          <col :style="fieldColumnStyle" />
          <col :style="valueColumnStyle" />
          <col class="action-colgroup" />
        </colgroup>
        <thead>
          <tr>
            <th class="num-col">#</th>
            <th class="sortable-col field-th" @click="cycleSortOrder" :style="fieldColumnStyle">
              <span class="th-content">Field <span class="sort-icon" :class="sortOrder">{{ sortIcon }}</span></span>
              <span class="col-resizer" @mousedown.stop="startResizeField" />
            </th>
            <th class="value-th" :style="valueColumnStyle">Value</th>
            <th class="action-th"><span class="action-th-label">{{ t('keyEditor.action') }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="([field, val], idx) in displayEntries" :key="field">
            <td class="num-cell">{{ idx + 1 }}</td>
            <td class="field-cell" :style="fieldColumnStyle">{{ field }}</td>
            <td class="value-cell" :style="valueColumnStyle">
              <span v-if="editingField !== field" class="value-text">
                <span class="val-preview">{{ val }}</span>
              </span>
              <input v-else v-model="editValue" class="value-edit-input" @keydown.enter="saveEdit(field)" @keydown.esc="cancelEdit()" />
            </td>
            <td class="action-cell">
              <div class="action-btns">
                <template v-if="editingField !== field">
                  <button class="btn-tiny" :class="{ copied: copiedField === field }" @click="copyVal(val, field)">{{ copiedField === field ? '✓' : t('keyEditor.copy') }}</button>
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
        v-if="(searchResults === null && hasMore) || (searchResults !== null && searchHasMore)"
        class="btn-load-more"
        :disabled="valueLoading"
        @click="loadMore"
      >
        {{ valueLoading ? '...' : t('keyTree.loadMore') }}
      </button>
      <span
        v-else-if="searchResults === null ? (!hasMore && totalFields > 0) : (!searchHasMore && searchResults.length > 0)"
        class="load-more-hint"
      >
        field:{{ searchResults === null ? totalFields : searchAllResults.length }}
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
import { hSet, hDel, searchValue, getValue } from '../../api/wails.js'
import ExpandModal from './ExpandModal.vue'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import FloatingMessage from '../common/FloatingMessage.vue'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../../utils/connection.js'
import './editorShared.css'

const INT32_MIN = -2147483648
const INT32_MAX = 2147483647

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

const rawHashVal = ref({})
const showAdd = ref(false)
const newField = ref('')
const newValue = ref('')
const editingField = ref(null)
const editValue = ref('')
const msg = ref('')
const ok = ref(true)
const copiedField = ref(null)
const addFlashing = ref(false)
const fieldWidth = ref(240)
const totalFieldCount = ref(0)
const calcPanelOpen = ref(false)
const calcRule = ref('')
const calcSubRules = ref([])
const calcLogs = ref([])
const calculating = ref(false)
const calcLogCopied = ref(false)
const calcLogFilter = ref('all')
const calcUseSearchFilter = ref(false)
const calcFilterHintVisible = ref(false)
let addFlashTimer = null
let calcSubRuleId = 0
let calcLogCopiedTimer = null
let calcFilterHintTimer = null

function triggerAddFlash() {
  if (addFlashTimer) clearTimeout(addFlashTimer)
  addFlashing.value = true
  addFlashTimer = setTimeout(() => {
    addFlashing.value = false
    addFlashTimer = null
  }, 1100)
}

function startResizeField(e) {
  const startX = e.clientX
  const startWidth = fieldWidth.value
  function onMove(ev) {
    const delta = ev.clientX - startX
    fieldWidth.value = Math.max(120, Math.min(520, startWidth + delta))
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
const searchAllResults = ref([])
const searchVisibleCount = ref(0)
const searchAppliedQuery = ref('')
const searchAppliedFuzzy = ref(false)
const searchNextCursor = ref(0)
const searchBackendHasMore = ref(false)
const isSearching   = ref(false)
const fuzzySearch   = ref(false)
const canToggleFuzzy = computed(() => searchQuery.value.trim().length > 0)
const searchPageSize = computed(() => Math.max(1, Number(settingsStore.hashLoadCount) || 20))

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

async function handleConnectionFailure(error, request = null) {
  if (request && !requestGuard.isCurrent(request)) return true
  if (!isConnectionErrorMessage(error)) return false
  await connectionsStore.handleConnectionFailure(request?.context.connID || workspaceStore.activeConnID, error)
  if (request && !requestGuard.isCurrent(request)) return true
  ok.value = false
  msg.value = formatConnectionLostMessage(error)
  return true
}

const fieldCount = computed(() => Object.keys(rawHashVal.value).length)
const totalFields = computed(() => totalFieldCount.value >= 0 ? totalFieldCount.value : fieldCount.value)
const fieldColumnStyle = computed(() => ({ width: `${fieldWidth.value}px` }))
const valueColumnStyle = computed(() => ({
  width: `calc(100% - ${fieldWidth.value}px - 36px - 170px)`,
  minWidth: '180px',
}))
const canAddCalcSubRule = computed(() => calcGroupOptions.value.length > 0)
const calcRuleInvalid = computed(() => {
  const ruleText = calcRule.value.trim()
  if (!ruleText) return false
  try {
    new RegExp(ruleText)
    return false
  } catch {
    return true
  }
})
const calcGroupOptions = computed(() => extractCalcGroups(calcRule.value))
const filteredCalcLogs = computed(() => {
  if (calcLogFilter.value === 'all') return calcLogs.value
  return calcLogs.value.filter(log => log.tone === calcLogFilter.value)
})
const visibleSearchResults = computed(() => searchAllResults.value.slice(0, searchVisibleCount.value))
const searchHasMore = computed(() =>
  searchResults.value !== null && (
    searchVisibleCount.value < searchAllResults.value.length || searchBackendHasMore.value
  )
)
const canUseCalcSearchFilter = computed(() => searchQuery.value.trim().length > 0)
const calcSearchHighlight = computed(() =>
  calcPanelOpen.value && calcUseSearchFilter.value && canUseCalcSearchFilter.value
)

// 数据源：搜索激活时用搜索结果，否则用全量
const sourceEntries = computed(() =>
  searchResults.value !== null
    ? visibleSearchResults.value
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

watch(canUseCalcSearchFilter, (enabled) => {
  if (!enabled && calcUseSearchFilter.value) {
    calcUseSearchFilter.value = false
  }
  if (enabled && !calcUseSearchFilter.value) {
    hideCalcFilterHint()
  }
})

watch(searchQuery, (value) => {
  if (String(value || '').trim() && !calcUseSearchFilter.value) {
    hideCalcFilterHint()
  }
})

watch(calcGroupOptions, (options) => {
  const fallback = options[0]?.key || ''
  calcSubRules.value = calcSubRules.value.map(rule => ({
    ...rule,
    group: options.some(option => option.key === rule.group) ? rule.group : fallback,
  }))
})

onBeforeUnmount(() => {
  requestGuard.invalidateAll()
  persistSearchState()
  hideCalcFilterHint()
})

watch(() => props.keyValue, (kv) => {
  requestGuard.invalidateAll()
  persistSearchState(lastKey.value)
  valueLoading.value = false
  isSearching.value = false
  calculating.value = false
  expandSaving.value = false

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
  searchAllResults.value = []
  searchVisibleCount.value = 0
  searchAppliedQuery.value = ''
  searchAppliedFuzzy.value = false
  searchNextCursor.value = 0
  searchBackendHasMore.value = false

  sortOrder.value = 'none'
  msg.value = ''
  resetCalcPanelState()
}, { immediate: true })

async function loadMore() {
  if (valueLoading.value || !props.keyValue?.key) return
  if (searchResults.value !== null) {
    if (searchVisibleCount.value < searchAllResults.value.length) {
      searchVisibleCount.value = Math.min(
        searchAllResults.value.length,
        searchVisibleCount.value + searchPageSize.value,
      )
      return
    }
    if (!searchBackendHasMore.value) return
    const request = requestGuard.begin('search')
    valueLoading.value = true
    try {
      const page = await fetchHashSearchPage(
        searchAppliedQuery.value,
        !searchAppliedFuzzy.value,
        searchNextCursor.value,
        request,
      )
      if (!page || !requestGuard.isCurrent(request)) return
      mergeHashSearchEntries(page.entries)
      searchNextCursor.value = page.nextCursor
      searchBackendHasMore.value = page.hasMore
      searchVisibleCount.value = Math.min(
        searchAllResults.value.length,
        searchVisibleCount.value + searchPageSize.value,
      )
    } catch (e) {
      if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
        ok.value = false
        msg.value = e.message || String(e)
      }
    } finally {
      if (requestGuard.isCurrent(request)) valueLoading.value = false
      requestGuard.finish(request)
    }
    return
  }
  const request = requestGuard.begin('load')
  valueLoading.value = true
  try {
    if (!hasMore.value) return
    const result = await getValue(request.context.connID, request.context.key, nextCursor.value, 0, '')
    if (!requestGuard.isCurrent(request)) return
    if (result.hash_val) {
      rawHashVal.value = { ...rawHashVal.value, ...result.hash_val }
    }
    hasMore.value = result.has_more || false
    nextCursor.value = result.next_cursor || 0
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

async function fetchHashSearchPage(pattern, exact, cursor = 0, request) {
  let pageCursor = cursor
  let hasMore = true
  const entries = []
  const seenCursors = new Set([cursor])

  while (hasMore && entries.length === 0) {
    const kv = await searchValue(
      request.context.connID,
      request.context.key,
      'hash',
      pattern,
      exact,
      pageCursor,
    )
    if (!requestGuard.isCurrent(request)) return null
    entries.push(...Object.entries(kv.hash_val || {}))
    const nextCursor = kv.next_cursor || 0
    hasMore = !!kv.has_more && nextCursor !== 0
    if (!hasMore || seenCursors.has(nextCursor)) {
      if (seenCursors.has(nextCursor) && nextCursor !== 0) hasMore = false
      pageCursor = nextCursor
      break
    }
    seenCursors.add(nextCursor)
    pageCursor = nextCursor
  }

  return { entries, nextCursor: pageCursor, hasMore }
}

function mergeHashSearchEntries(entries, replace = false) {
  const merged = new Map(replace ? [] : searchAllResults.value)
  for (const [field, value] of entries) {
    merged.set(field, value)
  }
  searchAllResults.value = [...merged.entries()]
  searchResults.value = searchAllResults.value
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
  const request = requestGuard.begin('search')
  isSearching.value = true
  searchResults.value = []
  searchAllResults.value = []
  searchVisibleCount.value = 0
  searchAppliedQuery.value = ''
  searchAppliedFuzzy.value = false
  searchNextCursor.value = 0
  searchBackendHasMore.value = false
  try {
    const exact = !fuzzySearch.value
    const page = await fetchHashSearchPage(pattern, exact, 0, request)
    if (!page || !requestGuard.isCurrent(request)) return
    mergeHashSearchEntries(page.entries, true)
    searchVisibleCount.value = Math.min(searchPageSize.value, searchAllResults.value.length)
    searchNextCursor.value = page.nextCursor
    searchBackendHasMore.value = page.hasMore
    searchAppliedQuery.value = pattern
    searchAppliedFuzzy.value = fuzzySearch.value
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
  searchAllResults.value = []
  searchVisibleCount.value = 0
  searchAppliedQuery.value = ''
  searchAppliedFuzzy.value = false
  searchNextCursor.value = 0
  searchBackendHasMore.value = false
  fuzzySearch.value = false
  if (props.keyValue?.key) {
  workspaceStore.setEditorSearchState(props.keyValue.key, 'hash', null)
  }
}

function toggleCalcSearchFilter() {
  if (!canUseCalcSearchFilter.value) {
    ok.value = false
    msg.value = t('keyEditor.calcFilterToggleDisabledHint')
    showCalcFilterHint()
    return
  }
  calcUseSearchFilter.value = !calcUseSearchFilter.value
  hideCalcFilterHint()
}

function showCalcFilterHint() {
  calcFilterHintVisible.value = true
  if (calcFilterHintTimer) clearTimeout(calcFilterHintTimer)
  calcFilterHintTimer = setTimeout(() => {
    hideCalcFilterHint()
  }, 2600)
}

function hideCalcFilterHint() {
  calcFilterHintVisible.value = false
  if (calcFilterHintTimer) {
    clearTimeout(calcFilterHintTimer)
    calcFilterHintTimer = null
  }
}

function toggleCalcPanel() {
  if (calcPanelOpen.value) {
    closeCalcPanel()
    return
  }
  calcPanelOpen.value = true
}

function addCalcSubRule() {
  calcSubRules.value.push({
    id: `sub-rule-${calcSubRuleId++}`,
    group: calcGroupOptions.value[0]?.key || '',
    operator: '==',
    value: '0',
  })
}

function removeCalcSubRule(id) {
  calcSubRules.value = calcSubRules.value.filter(rule => rule.id !== id)
}

function resetCalcPanelState() {
  calcRule.value = ''
  calcSubRules.value = []
  calcLogs.value = []
  calculating.value = false
  calcLogCopied.value = false
  calcLogFilter.value = 'all'
  calcUseSearchFilter.value = false
  hideCalcFilterHint()
  if (calcLogCopiedTimer) {
    clearTimeout(calcLogCopiedTimer)
    calcLogCopiedTimer = null
  }
}

function closeCalcPanel() {
  calcPanelOpen.value = false
  hideCalcFilterHint()
}

function pushCalcLog(text, tone = 'muted') {
  calcLogs.value.push({
    id: `calc-log-${Date.now()}-${calcLogs.value.length}`,
    text,
    tone,
  })
}

async function copyCalcLogs() {
  if (!calcLogs.value.length) return
  const text = calcLogs.value.map(log => log.text).join('\n')
  const copied = await copyToClipboard(text)
  if (!copied) return
  calcLogCopied.value = true
  if (calcLogCopiedTimer) clearTimeout(calcLogCopiedTimer)
  calcLogCopiedTimer = setTimeout(() => {
    calcLogCopied.value = false
    calcLogCopiedTimer = null
  }, 1400)
}

function extractCalcGroups(ruleText) {
  const source = String(ruleText || '')
  const groups = []
  let escaped = false

  for (let idx = 0; idx < source.length; idx += 1) {
    const ch = source[idx]
    if (escaped) {
      escaped = false
      continue
    }
    if (ch === '\\') {
      escaped = true
      continue
    }
    if (ch !== '(') continue

    const next = source[idx + 1]
    if (next === '?') continue

    let depth = 1
    let innerEscaped = false
    let end = idx + 1

    for (; end < source.length; end += 1) {
      const current = source[end]
      if (innerEscaped) {
        innerEscaped = false
        continue
      }
      if (current === '\\') {
        innerEscaped = true
        continue
      }
      if (current === '(') {
        depth += 1
        continue
      }
      if (current === ')') {
        depth -= 1
        if (depth === 0) break
      }
    }

    if (depth === 0) {
      const label = source.slice(idx, end + 1)
      groups.push({
        key: `group:${groups.length + 1}`,
        index: groups.length + 1,
        label,
      })
      idx = end
    }
  }

  return groups
}

function getCalcGroupMeta(groupRef) {
  return calcGroupOptions.value.find(option => option.key === groupRef) || null
}

function compareCalcValue(actual, operator, expected) {
  const actualText = String(actual ?? '')
  const expectedText = String(expected ?? '')
  const actualNumber = Number(actualText)
  const expectedNumber = Number(expectedText)
  const bothNumeric = Number.isFinite(actualNumber) && Number.isFinite(expectedNumber)
  const left = bothNumeric ? actualNumber : actualText
  const right = bothNumeric ? expectedNumber : expectedText

  switch (operator) {
    case '>':
      return left > right
    case '>=':
      return left >= right
    case '<':
      return left < right
    case '<=':
      return left <= right
    case '!=':
      return left !== right
    case '==':
    default:
      return left === right
  }
}

function evaluateCalcSubRules(field, match) {
  for (const subRule of calcSubRules.value) {
    const groupMeta = getCalcGroupMeta(subRule.group)
    const groupIndex = groupMeta?.index ?? -1
    if (groupIndex < 0 || groupIndex >= match.length) {
      return {
        ok: false,
        reason: t('keyEditor.calcGroupInvalid', { field, group: groupMeta?.label || subRule.group }),
      }
    }
    const actual = match[groupIndex]
    const actualNumber = Number(String(actual ?? ''))
    const expectedNumber = Number(String(subRule.value ?? ''))
    const isNumericComparison = Number.isFinite(actualNumber) && Number.isFinite(expectedNumber)
    if (!isNumericComparison && subRule.operator !== '==' && subRule.operator !== '!=') {
      return {
        ok: false,
        reason: t('keyEditor.calcGroupNonNumericOperator', {
          field,
          group: groupMeta?.label || subRule.group,
          actual,
        }),
      }
    }
    const passed = compareCalcValue(actual, subRule.operator, subRule.value)
    if (!passed) {
      return {
        ok: false,
        reason: t('keyEditor.calcGroupRuleFailed', {
          field,
          group: groupMeta?.label || subRule.group,
          actual,
          operator: subRule.operator,
          expected: subRule.value,
        }),
      }
    }
  }
  return { ok: true, reason: '' }
}

async function resolveCalculationEntries(request) {
  const pattern = searchQuery.value.trim()
  if (calcUseSearchFilter.value && pattern) {
    const matchesAppliedSearch =
      searchResults.value !== null &&
      searchAppliedQuery.value === pattern &&
      searchAppliedFuzzy.value === fuzzySearch.value

    if (matchesAppliedSearch && !searchBackendHasMore.value) {
      return searchAllResults.value
    }

    const exact = !fuzzySearch.value
    let cursor = matchesAppliedSearch ? searchNextCursor.value : 0
    let hasMore = matchesAppliedSearch ? searchBackendHasMore.value : true
    const merged = new Map(matchesAppliedSearch ? searchAllResults.value : [])

    while (hasMore) {
      const page = await fetchHashSearchPage(pattern, exact, cursor, request)
      if (!page || !requestGuard.isCurrent(request)) return null
      for (const [field, value] of page.entries) {
        merged.set(field, value)
      }
      cursor = page.nextCursor
      hasMore = page.hasMore
    }

    return [...merged.entries()]
  }

  if (calcUseSearchFilter.value && !pattern) {
    return Object.entries(rawHashVal.value)
  }

  if (hasMore.value) {
    let merged = { ...rawHashVal.value }
    let cursor = nextCursor.value
    let more = hasMore.value
    while (more) {
      const result = await getValue(request.context.connID, request.context.key, cursor, 0, '')
      if (!requestGuard.isCurrent(request)) return null
      merged = { ...merged, ...(result.hash_val || {}) }
      cursor = result.next_cursor || 0
      more = result.has_more || false
    }
    rawHashVal.value = merged
    nextCursor.value = cursor
    hasMore.value = false
    totalFieldCount.value = Object.keys(merged).length
  }

  return Object.entries(rawHashVal.value)
}

async function runCalculation() {
  const ruleText = calcRule.value.trim()
  if (!ruleText) {
    calcLogs.value = [{ id: 'calc-log-empty-rule', text: t('keyEditor.calcRuleRequired'), tone: 'err' }]
    return
  }

  let regex
  try {
    regex = new RegExp(ruleText)
  } catch (e) {
    calcLogs.value = [{
      id: 'calc-log-regex-error',
      text: t('keyEditor.calcRegexError', { message: e.message || String(e) }),
      tone: 'err',
    }]
    return
  }

  calculating.value = true
  calcLogs.value = []
  const request = requestGuard.begin('calculation')

  try {
    const entries = await resolveCalculationEntries(request)
    if (!entries || !requestGuard.isCurrent(request)) return
    let matchedCount = 0
    let total = 0

    for (const [field, val] of entries) {
      const match = field.match(regex)
      if (!match) continue

      matchedCount += 1

      if (calcSubRules.value.length) {
        const subRuleResult = evaluateCalcSubRules(field, match)
        if (!subRuleResult.ok) {
          pushCalcLog(subRuleResult.reason, 'skip')
          continue
        }
      }

      const numericValue = Number(val)
      if (!Number.isFinite(numericValue)) {
        pushCalcLog(t('keyEditor.calcNonNumericSkipped', { field, value: val }), 'skip')
        continue
      }
      if (numericValue < INT32_MIN || numericValue > INT32_MAX) {
        pushCalcLog(t('keyEditor.calcIntOverflowSkipped', { field, value: val }), 'skip')
        continue
      }

      total += numericValue
      pushCalcLog(`${field}: ${val}`, 'ok')
    }

    if (matchedCount === 0) {
      pushCalcLog(t('keyEditor.calcNoFieldMatched'), 'skip')
      return
    }

    pushCalcLog('', 'spacer')
    pushCalcLog(t('keyEditor.calcSummary', { total }), 'ok')
  } catch (e) {
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      calcLogs.value = [{
        id: 'calc-log-runtime-error',
        text: t('keyEditor.calcRunFailed', { message: e.message || String(e) }),
        tone: 'err',
      }]
    }
  } finally {
    if (requestGuard.isCurrent(request)) calculating.value = false
    requestGuard.finish(request)
  }
}

function startEdit(field, val) {
  editingField.value = field
  editValue.value = val
}

function cancelEdit() {
  editingField.value = null
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
  const request = requestGuard.begin(`write:modal:${field}`)
  expandSaving.value = true
  try {
    const result = await hSet(request.context.connID, request.context.key, field, newVal)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
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
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    if (requestGuard.isCurrent(request)) expandSaving.value = false
    requestGuard.finish(request)
  }
}

async function copyVal(val, field) {
  await copyToClipboard(val)
  copiedField.value = field
  setTimeout(() => { copiedField.value = null }, 1200)
}

async function saveEdit(field) {
  if (editingField.value !== field) return
  const nextValue = editValue.value
  const request = requestGuard.begin(`write:field:${field}`)
  editingField.value = null
  try {
    const result = await hSet(request.context.connID, request.context.key, field, nextValue)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.updated') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      rawHashVal.value = { ...rawHashVal.value, [field]: nextValue }
      if (searchResults.value !== null) {
        searchResults.value = searchResults.value.map(([f, v]) => f === field ? [field, nextValue] : [f, v])
      }
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

async function deleteField(field) {
  const request = requestGuard.begin(`write:delete:${field}`)
  try {
    const result = await hDel(request.context.connID, request.context.key, field)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
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
    if (!(await handleConnectionFailure(e, request)) && requestGuard.isCurrent(request)) {
      ok.value = false
      msg.value = e.message
    }
  } finally {
    requestGuard.finish(request)
  }
}

async function addField() {
  if (!newField.value.trim()) return
  const field = newField.value.trim()
  const value = newValue.value
  const request = requestGuard.begin(`write:add:${field}`)
  try {
    const existed = Object.prototype.hasOwnProperty.call(rawHashVal.value, field)
    const result = await hSet(request.context.connID, request.context.key, field, value)
    if (!requestGuard.isCurrent(request)) return
    if (!result.success && await handleConnectionFailure(result.message, request)) return
    ok.value = result.success
    msg.value = result.success ? t('keyEditor.added') : (result.message || t('keyEditor.saveFailed'))
    if (result.success) {
      rawHashVal.value = { ...rawHashVal.value, [field]: value }
      if (!existed) {
        totalFieldCount.value++
      }
      newField.value = ''; newValue.value = ''; showAdd.value = false
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
</script>

<style scoped>
.hash-editor { position: relative; display: flex; flex-direction: column; height: 100%; gap: 8px; }
.btn-add.success-flash {
  background: rgba(220, 252, 231, 0.96);
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 0 0 1px rgba(187, 247, 208, 0.7) inset, 0 8px 18px rgba(34, 197, 94, 0.14);
  animation: addSuccessPulse 0.42s ease;
}
.btn-calc {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  min-width: 28px;
  height: 28px;
  margin-left: 6px;
  padding: 0;
  border: 1px solid #d1d5db;
  border-radius: 5px;
  cursor: pointer;
  background: #fff;
  color: #475569;
  box-sizing: border-box;
  position: relative;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease, transform 0.16s ease, box-shadow 0.16s ease;
}
.btn-calc::after {
  content: '';
  position: absolute;
  left: 5px;
  right: 5px;
  bottom: -7px;
  height: 2px;
  border-radius: 999px;
  background: transparent;
  opacity: 0;
  transition: opacity 0.16s ease, background 0.16s ease;
}
.btn-calc:hover,
.btn-calc.active {
  background: #f8fafc;
  color: #1e293b;
  border-color: #94a3b8;
  transform: translateY(-1px);
}
.btn-calc.active {
  box-shadow: 0 6px 14px rgba(148, 163, 184, 0.18);
}
.btn-calc.active::after {
  opacity: 1;
  background: linear-gradient(90deg, rgba(148, 163, 184, 0.1), rgba(96, 165, 250, 0.72), rgba(148, 163, 184, 0.1));
}
.calc-panel {
  position: absolute;
  top: 36px;
  right: 0;
  z-index: 30;
  width: min(520px, calc(100% - 4px));
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.985), rgba(248, 250, 252, 0.97));
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.12), 0 2px 8px rgba(15, 23, 42, 0.06);
  backdrop-filter: blur(14px);
  overflow: hidden;
  transform-origin: top right;
}
.calc-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 10px 9px 12px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.96));
}
.calc-panel-title {
  font-size: 12px;
  font-weight: 700;
  color: #334155;
  letter-spacing: 0.01em;
  text-shadow: 0 1px 0 rgba(255, 255, 255, 0.7);
}
.calc-panel-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.calc-filter-btn,
.calc-clear-btn,
.calc-min-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 6px;
  background: #fff;
  color: #64748b;
  cursor: pointer;
}
.calc-filter-btn.active,
.calc-clear-btn:hover,
.calc-filter-btn:hover,
.calc-min-btn:hover {
  background: #f8fafc;
  color: #334155;
  border-color: #94a3b8;
}
.calc-filter-btn.disabled {
  opacity: 0.48;
  cursor: not-allowed;
  color: #94a3b8;
  border-color: rgba(203, 213, 225, 0.78);
  background: rgba(248, 250, 252, 0.72);
  box-shadow: none;
}
.calc-filter-btn.active {
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.98), rgba(219, 234, 254, 0.96));
  box-shadow: 0 0 0 1px rgba(191, 219, 254, 0.7) inset, 0 6px 14px rgba(147, 197, 253, 0.22);
}
.calc-filter-btn.active:hover {
  background: linear-gradient(180deg, rgba(219, 234, 254, 0.98), rgba(191, 219, 254, 0.96));
  color: #1e40af;
  border-color: #60a5fa;
}
.search-bar-filter-active {
  border-color: rgba(96, 165, 250, 0.92);
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.72);
}
.search-bar-invalid {
  border-color: rgba(245, 158, 11, 0.9);
  box-shadow: 0 0 0 3px rgba(253, 224, 71, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.72);
}
.hash-editor:not(.theme-dark) .search-bar.search-bar-filter-active {
  border-color: rgba(96, 165, 250, 0.92);
  box-shadow:
    0 0 0 3px rgba(191, 219, 254, 0.24),
    inset 0 1px 0 rgba(255, 255, 255, 0.84),
    0 1px 2px rgba(148, 163, 184, 0.08);
}
.hash-editor:not(.theme-dark) .search-bar.search-bar-invalid {
  border-color: rgba(245, 158, 11, 0.9);
  box-shadow:
    0 0 0 3px rgba(253, 224, 71, 0.22),
    inset 0 1px 0 rgba(255, 255, 255, 0.84),
    0 1px 2px rgba(148, 163, 184, 0.08);
}
.calc-panel-body {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 11px;
}
.calc-config {
  display: flex;
  align-items: center;
  gap: 7px;
}
.calc-rule-hint {
  margin-top: -1px;
  font-size: 11px;
  line-height: 1.45;
  color: #94a3b8;
}
.calc-label {
  flex: 0 0 auto;
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
}
.calc-rule-input,
.calc-subrule-group,
.calc-subrule-value,
.calc-subrule-operator {
  border: 1px solid #d1d5db;
  border-radius: 8px;
  background: #fff;
  color: #334155;
  font-size: 12px;
  outline: none;
}
.calc-subrule-group,
.calc-subrule-operator,
.calc-log-filter {
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  padding-right: 26px;
  background-image:
    linear-gradient(45deg, transparent 50%, currentColor 50%),
    linear-gradient(135deg, currentColor 50%, transparent 50%);
  background-position:
    calc(100% - 14px) calc(50% - 1px),
    calc(100% - 10px) calc(50% - 1px);
  background-size: 5px 5px, 5px 5px;
  background-repeat: no-repeat;
}
.calc-rule-input {
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 10px;
}
.calc-rule-input.invalid {
  border-color: rgba(248, 113, 113, 0.92);
  box-shadow: 0 0 0 2px rgba(254, 202, 202, 0.42);
}
.calc-subrules {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 9px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 10px;
  background: rgba(248, 250, 252, 0.78);
}
.calc-subrules-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.calc-subrules-title {
  font-size: 11px;
  font-weight: 700;
  color: #64748b;
  letter-spacing: 0.02em;
}
.calc-add-subrule,
.calc-remove-subrule,
.calc-run-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
}
.calc-add-subrule,
.calc-remove-subrule {
  height: 28px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #475569;
}
.calc-add-subrule:hover,
.calc-remove-subrule:hover {
  background: #f8fafc;
  color: #1e293b;
  border-color: #94a3b8;
}
.calc-subrules-empty {
  font-size: 12px;
  color: #94a3b8;
}
.calc-subrule-row {
  display: grid;
  grid-template-columns: 1.1fr 72px 1fr 36px;
  gap: 6px;
}
.calc-subrule-group,
.calc-subrule-operator,
.calc-subrule-value {
  height: 30px;
  padding: 0 8px;
}
.calc-subrule-group,
.calc-subrule-operator {
  font-weight: 600;
}
.calc-rule-input:focus,
.calc-subrule-group:focus,
.calc-subrule-operator:focus,
.calc-subrule-value:focus,
.calc-log-filter:focus {
  border-color: rgba(96, 165, 250, 0.92);
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.46);
}
.calc-run-btn {
  height: 32px;
  padding: 0 14px;
  border: 1px solid rgba(191, 219, 254, 0.92);
  background: linear-gradient(180deg, #ffffff, #f8fafc);
  color: #2563eb;
  align-self: flex-start;
}
.calc-run-btn:hover:not(:disabled) {
  background: linear-gradient(180deg, #f8fbff, #eff6ff);
  border-color: #60a5fa;
}
.calc-run-btn:disabled {
  color: #94a3b8;
  border-color: #d1d5db;
  background: #f8fafc;
  cursor: not-allowed;
}
.calc-log-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.calc-log-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.calc-log-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.calc-log-title {
  font-size: 11px;
  font-weight: 700;
  color: #64748b;
  letter-spacing: 0.02em;
}
.calc-log-filter {
  height: 28px;
  min-width: 76px;
  padding: 0 26px 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  background-color: #fff;
  color: #475569;
  font-size: 11px;
  font-weight: 600;
  outline: none;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease, box-shadow 0.16s ease;
}
.calc-copy-log-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 26px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  background: #fff;
  color: #475569;
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
}
.calc-copy-log-btn:hover {
  background: #f8fafc;
  color: #1e293b;
  border-color: #94a3b8;
}
.calc-copy-log-btn.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
}
.calc-log-output {
  min-height: 150px;
  max-height: 240px;
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 10px;
  background: rgba(248, 250, 252, 0.88);
  color: #334155;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace;
  font-size: 11.5px;
  line-height: 1.68;
  letter-spacing: 0.01em;
  word-break: break-word;
}
.calc-log-line {
  white-space: pre-wrap;
  word-break: break-word;
}
.calc-log-line + .calc-log-line {
  margin-top: 3px;
}
.calc-log-line.tone-ok {
  color: #15803d;
}
.calc-log-line.tone-skip {
  color: #b45309;
  opacity: 0.96;
}
.calc-log-line.tone-sum {
  color: #1d4ed8;
  font-weight: 700;
}
.calc-log-line.tone-err {
  color: #b91c1c;
}
.calc-log-line.tone-muted {
  color: #94a3b8;
  opacity: 0.9;
}
.calc-log-line.spacer {
  min-height: 8px;
}
.calc-panel-enter-active,
.calc-panel-leave-active {
  transition: opacity 0.18s ease, transform 0.18s cubic-bezier(0.22, 1, 0.36, 1);
}
.calc-panel-enter-from,
.calc-panel-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.982);
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
.hash-table-wrap { flex: 1; overflow-y: auto; overflow-x: hidden; }
.hash-table { width: 100%; border-collapse: collapse; font-size: 12px; table-layout: fixed; }
.num-colgroup { width: 36px; }
.action-colgroup { width: 170px; }
.hash-table thead { position: sticky; top: 0; z-index: 10; }
.hash-table th {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.96));
  padding: 7px 8px;
  text-align: left;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  font-weight: 700;
  color: #64748b;
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.hash-table td {
  padding: 6px 8px;
  border-bottom: 1px solid rgba(241, 245, 249, 0.96);
  vertical-align: middle;
}
.num-col { width: 36px; text-align: center; }
.num-cell {
  width: 36px;
  text-align: center;
  color: #cbd5e1;
  font-size: 10px;
  font-weight: 600;
}
.hash-table thead,
.toolbar,
.search-bar,
.btn-add,
.btn-search,
.btn-clear-search,
.fuzzy-check,
.count,
.add-row button,
.num-col,
.num-cell,
.field-th,
.action-th,
.action-cell,
.action-btns {
  user-select: none;
  -webkit-user-select: none;
}
.field-th { position: relative; min-width: 80px; }
.value-th { width: auto; }
.action-th {
  width: 170px;
  min-width: 170px;
  max-width: 170px;
  white-space: nowrap;
  text-align: center;
}
.hash-table th.action-th {
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
.field-cell {
  color: #1d4ed8;
  font-weight: 600;
  word-break: break-all;
  min-width: 0;
  user-select: text;
  -webkit-user-select: text;
}
.value-th,
.value-cell { min-width: 0; }
.value-text {
  display: block;
  min-width: 0;
  width: 100%;
}
.val-preview {
  font-family: monospace;
  font-size: 12px;
  color: #374151;
  display: block;
  width: 100%;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.value-cell input,
.value-edit-input {
  width: 100%;
  height: 28px;
  padding: 0 8px;
  border: 1px solid rgba(96, 165, 250, 0.92);
  border-radius: 6px;
  font-size: 12px;
  outline: none;
  color: #1e293b;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.3);
}
.action-th,
.action-cell {
  width: 170px;
  min-width: 170px;
  max-width: 170px;
  text-align: center;
  white-space: nowrap;
}
.action-btns {
  display: inline-flex;
  gap: 5px;
  justify-content: center;
  align-items: center;
}
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
.value-cell {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.78), rgba(248, 250, 252, 0.6));
  overflow: hidden;
}
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
.hash-editor.theme-dark {
  color: #e2e8f0;
}
.hash-editor.theme-dark .btn-add,
.hash-editor.theme-dark .add-row button,
.hash-editor.theme-dark .btn-search,
.hash-editor.theme-dark .btn-clear-search,
.hash-editor.theme-dark .btn-calc,
.hash-editor.theme-dark .calc-filter-btn,
.hash-editor.theme-dark .calc-clear-btn,
.hash-editor.theme-dark .calc-add-subrule,
.hash-editor.theme-dark .calc-remove-subrule,
.hash-editor.theme-dark .calc-min-btn {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.hash-editor.theme-dark .btn-add:hover,
.hash-editor.theme-dark .add-row button:hover,
.hash-editor.theme-dark .btn-search:hover,
.hash-editor.theme-dark .btn-clear-search:hover,
.hash-editor.theme-dark .btn-calc:hover,
.hash-editor.theme-dark .calc-filter-btn:hover,
.hash-editor.theme-dark .calc-clear-btn:hover,
.hash-editor.theme-dark .calc-filter-btn.active,
.hash-editor.theme-dark .btn-calc.active,
.hash-editor.theme-dark .calc-add-subrule:hover,
.hash-editor.theme-dark .calc-remove-subrule:hover,
.hash-editor.theme-dark .calc-min-btn:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: #60a5fa;
}
.hash-editor.theme-dark .calc-clear-btn {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.hash-editor.theme-dark .calc-clear-btn:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: #60a5fa;
}
.hash-editor.theme-dark .btn-calc.active {
  box-shadow: 0 8px 16px rgba(2, 6, 23, 0.3);
}
.hash-editor.theme-dark .calc-filter-btn.active {
  color: #93c5fd;
  border-color: rgba(96, 165, 250, 0.54);
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.28), rgba(30, 41, 59, 0.98));
  box-shadow: 0 0 0 1px rgba(96, 165, 250, 0.34) inset, 0 8px 16px rgba(30, 64, 175, 0.18);
}
.hash-editor.theme-dark .calc-filter-btn.disabled {
  opacity: 0.52;
  cursor: not-allowed;
  color: #64748b;
  border-color: rgba(51, 65, 85, 0.82);
  background: rgba(15, 23, 42, 0.72);
  box-shadow: none;
}
.hash-editor.theme-dark .search-bar-filter-active {
  border-color: rgba(96, 165, 250, 0.54);
  box-shadow: 0 0 0 3px rgba(30, 64, 175, 0.18), inset 0 1px 0 rgba(71, 85, 105, 0.28);
}
.hash-editor.theme-dark .search-bar-invalid {
  border-color: rgba(245, 158, 11, 0.76);
  box-shadow: 0 0 0 3px rgba(180, 83, 9, 0.16), inset 0 1px 0 rgba(71, 85, 105, 0.24);
}
.hash-editor.theme-dark .btn-calc.active::after {
  background: linear-gradient(90deg, rgba(96, 165, 250, 0), rgba(96, 165, 250, 0.88), rgba(96, 165, 250, 0));
}
.hash-editor.theme-dark .search-input,
.hash-editor.theme-dark .add-row-input,
.hash-editor.theme-dark .value-cell input,
.hash-editor.theme-dark .value-edit-input,
.hash-editor.theme-dark .calc-rule-input,
.hash-editor.theme-dark .calc-subrule-group,
.hash-editor.theme-dark .calc-subrule-value,
.hash-editor.theme-dark .calc-subrule-operator,
.hash-editor.theme-dark .calc-log-filter {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}
.hash-editor.theme-dark .calc-rule-input.invalid {
  border-color: rgba(251, 113, 133, 0.62);
  box-shadow: 0 0 0 2px rgba(127, 29, 29, 0.42);
}
.hash-editor.theme-dark .calc-rule-input:focus,
.hash-editor.theme-dark .calc-subrule-group:focus,
.hash-editor.theme-dark .calc-subrule-operator:focus,
.hash-editor.theme-dark .calc-subrule-value:focus,
.hash-editor.theme-dark .calc-log-filter:focus {
  border-color: rgba(96, 165, 250, 0.62);
  box-shadow: 0 0 0 3px rgba(30, 64, 175, 0.2);
}
.hash-editor.theme-dark .fuzzy-check {
  background: rgba(15, 23, 42, 0.7);
  color: #94a3b8;
  border-color: transparent;
}
.hash-editor.theme-dark .fuzzy-check:hover {
  background: rgba(30, 41, 59, 0.94);
  color: #cbd5e1;
  border-color: rgba(96, 165, 250, 0.28);
}
.hash-editor.theme-dark .fuzzy-check.active {
  background: rgba(30, 64, 175, 0.18);
  color: #93c5fd;
  border-color: rgba(96, 165, 250, 0.42);
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.2);
}
.hash-editor.theme-dark .fuzzy-check.disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}
.hash-editor.theme-dark .fuzzy-indicator {
  background: #475569;
}
.hash-editor.theme-dark .fuzzy-check.active .fuzzy-indicator {
  background: #60a5fa;
}
.hash-editor.theme-dark .count,
.hash-editor.theme-dark .load-more-hint {
  color: #94a3b8;
}
.hash-editor.theme-dark .hash-table th {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.96));
  color: #94a3b8;
  border-bottom-color: rgba(51, 65, 85, 0.92);
}
.hash-editor.theme-dark .hash-table td {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}
.hash-editor.theme-dark .num-cell,
.hash-editor.theme-dark .sort-icon {
  color: #475569;
}
.hash-editor.theme-dark .field-cell {
  color: #93c5fd;
}
.hash-editor.theme-dark .val-preview {
  color: #cbd5e1;
}
.hash-editor.theme-dark .value-cell {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.74), rgba(15, 23, 42, 0.58));
}
.hash-editor.theme-dark .sortable-col:hover {
  background: rgba(30, 41, 59, 0.92) !important;
}
.hash-editor.theme-dark .col-resizer {
  background: rgba(30, 41, 59, 0.94);
  border-left-color: rgba(51, 65, 85, 0.92);
  border-right-color: rgba(51, 65, 85, 0.92);
}
.hash-editor.theme-dark .load-more {
  min-height: var(--editor-footer-height);
  padding: 4px 12px;
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.98));
}
.hash-editor.theme-dark .btn-load-more {
  height: 24px;
  min-height: 24px;
  padding: 0 10px;
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.28);
}
.hash-editor.theme-dark .btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.48);
  box-shadow: 0 8px 18px rgba(2, 6, 23, 0.34);
}
.hash-editor.theme-dark .btn-load-more:disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}
.hash-editor.theme-dark .calc-panel {
  border-color: rgba(51, 65, 85, 0.96);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.988), rgba(17, 24, 39, 0.978));
  box-shadow: 0 18px 36px rgba(2, 6, 23, 0.34), 0 2px 8px rgba(2, 6, 23, 0.24);
}
.hash-editor.theme-dark .calc-panel-header {
  border-bottom-color: rgba(51, 65, 85, 0.92);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.96));
}
.hash-editor.theme-dark .calc-panel-title,
.hash-editor.theme-dark .calc-log-title,
.hash-editor.theme-dark .calc-subrules-title,
.hash-editor.theme-dark .calc-label {
  color: #cbd5e1;
}
.hash-editor.theme-dark .calc-panel-title {
  text-shadow: none;
}
.hash-editor.theme-dark .calc-rule-hint {
  color: #94a3b8;
}
.hash-editor.theme-dark .calc-subrules,
.hash-editor.theme-dark .calc-log-output {
  border-color: rgba(51, 65, 85, 0.92);
  background: rgba(15, 23, 42, 0.64);
}
.hash-editor.theme-dark .calc-subrules-empty,
.hash-editor.theme-dark .calc-log-output,
.hash-editor.theme-dark .calc-log-line.tone-muted {
  color: #cbd5e1;
}
.hash-editor.theme-dark .calc-log-line.tone-ok {
  color: #86efac;
}
.hash-editor.theme-dark .calc-log-line.tone-skip {
  color: #facc15;
  opacity: 0.96;
}
.hash-editor.theme-dark .calc-log-line.tone-sum {
  color: #bfdbfe;
}
.hash-editor.theme-dark .calc-log-line.tone-err {
  color: #fca5a5;
}
.hash-editor.theme-dark .calc-run-btn {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #93c5fd;
  border-color: rgba(71, 85, 105, 0.96);
}
.hash-editor.theme-dark .calc-run-btn:hover:not(:disabled) {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.2), rgba(30, 41, 59, 0.98));
  border-color: rgba(96, 165, 250, 0.48);
}
.hash-editor.theme-dark .calc-run-btn:disabled {
  background: rgba(15, 23, 42, 0.72);
  color: #475569;
  border-color: rgba(51, 65, 85, 0.82);
}
.hash-editor.theme-dark .calc-copy-log-btn {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.hash-editor.theme-dark .calc-copy-log-btn:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: #60a5fa;
}
.hash-editor.theme-dark .calc-copy-log-btn.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
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
