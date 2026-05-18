<template>
  <div class="key-search-bar">
    <div class="search-input-row" ref="inputRowRef">
      <div class="search-input-shell" ref="shellRef">
        <input
          v-model="pattern"
          type="text"
          :placeholder="isCluster ? t('keyTree.clusterSearchPlaceholder') : t('keyTree.searchPlaceholder')"
          @keydown.enter="onEnter"
          @keydown.down.prevent="onArrowDown"
          @keydown.up.prevent="onArrowUp"
          @keydown.esc="showHistory = false"
          @focus="onFocus"
          @blur="onBlur"
        />
        <button class="btn-search" @click="doSearch" :disabled="loading" :title="t('keyTree.searchBtn')">
          <span v-if="loading" class="search-loading">...</span>
          <svg v-else viewBox="0 0 20 20" width="14" height="14" aria-hidden="true">
            <path
              d="M8.5 3a5.5 5.5 0 104.03 9.24l3.11 3.11a1 1 0 001.41-1.41l-3.1-3.11A5.5 5.5 0 008.5 3zm0 2a3.5 3.5 0 110 7 3.5 3.5 0 010-7z"
              fill="currentColor"
            />
          </svg>
        </button>
      </div>
      <CreateKeyButton v-if="workspaceStore.activeConnID" />
    </div>
    <div class="search-options">
      <label v-if="!isCluster" class="keep-label">
        <input type="checkbox" v-model="keep" />
        {{ t('keyTree.keepPrev') }}
      </label>
      <div v-else class="cluster-hint">
        {{ t('keyTree.clusterSearchHint') }}
      </div>
    </div>

    <!-- 历史记录下拉（fixed 定位，避免被父容器 overflow:hidden 裁切） -->
    <div
      v-if="showHistory && filteredHistoryItems.length"
      class="history-dropdown"
      :style="dropdownStyle"
    >
      <template v-if="filteredPinnedHistory.length">
        <div class="history-section-title">{{ t('keyTree.pinnedHistory') }}</div>
        <div
          v-for="(item, idx) in filteredPinnedHistory"
          :key="`pinned:${item}`"
          :class="['history-item', { active: idx === activeIndex }]"
          @mousedown.prevent="selectHistory(item)"
          @mouseenter="activeIndex = idx"
        >
          <button
            class="history-pin-btn pinned"
            :title="t('keyTree.unpinHistory')"
            @mousedown.prevent.stop="togglePin(item)"
          >
            <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
              <path d="M10.8 1.5a.75.75 0 01.53 1.28L10.1 4v2.12l1.52 1.52a.75.75 0 01-.53 1.28H8.75v4.35a.75.75 0 01-1.28.53l-1.5-1.5a.75.75 0 01-.22-.53V8.92H3.41a.75.75 0 01-.53-1.28L4.4 6.12V4L3.17 2.78A.75.75 0 013.7 1.5h7.1z" fill="currentColor" />
            </svg>
          </button>
          <span class="history-item-text">{{ item }}</span>
        </div>
      </template>

      <template v-if="filteredPinnedHistory.length && filteredNormalHistory.length">
        <div class="history-section-divider" />
      </template>

      <template v-if="filteredNormalHistory.length">
        <div class="history-section-title">{{ t('keyTree.recentHistory') }}</div>
        <div
          v-for="(item, idx) in filteredNormalHistory"
          :key="`history:${item}`"
          :class="['history-item', { active: idx + filteredPinnedHistory.length === activeIndex }]"
          @mousedown.prevent="selectHistory(item)"
          @mouseenter="activeIndex = idx + filteredPinnedHistory.length"
        >
          <button
            class="history-pin-btn"
            :title="t('keyTree.pinHistory')"
            @mousedown.prevent.stop="togglePin(item)"
          >
            <svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
              <path d="M10.8 1.5a.75.75 0 01.53 1.28L10.1 4v2.12l1.52 1.52a.75.75 0 01-.53 1.28H8.75v4.35a.75.75 0 01-1.28.53l-1.5-1.5a.75.75 0 01-.22-.53V8.92H3.41a.75.75 0 01-.53-1.28L4.4 6.12V4L3.17 2.78A.75.75 0 013.7 1.5h7.1z" fill="currentColor" />
            </svg>
          </button>
          <span class="history-item-text">{{ item }}</span>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useI18n } from '../../i18n/index.js'
import CreateKeyButton from './CreateKeyButton.vue'

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const connectionsStore = useConnectionsStore()

const pattern = ref('')
const keep = ref(workspaceStore.keepPrevSearch)
const loading = ref(false)
const showHistory = ref(false)
const activeIndex = ref(-1)
const inputRowRef = ref(null)
const shellRef = ref(null)
const dropdownStyle = ref({})
let blurTimer = null
const activeConn = computed(() => connectionsStore.connections.find(c => c.id === workspaceStore.activeConnID))
const isCluster = computed(() => !!activeConn.value?.is_cluster)

const filteredPinnedHistory = computed(() => {
  const id = workspaceStore.activeConnID
  if (!id) return []
  const entry = workspaceStore.getConnSearchHistory(id)
  const maxCount = settingsStore.loaded ? settingsStore.searchHistoryLimit : 10
  const term = pattern.value.trim()
  const list = entry.pinned || []
  if (!term || term === '*') return list.slice(0, maxCount)
  return list.filter(h => h.toLowerCase().includes(term.toLowerCase())).slice(0, maxCount)
})

const filteredNormalHistory = computed(() => {
  const id = workspaceStore.activeConnID
  if (!id) return []
  const entry = workspaceStore.getConnSearchHistory(id)
  const maxCount = settingsStore.loaded ? settingsStore.searchHistoryLimit : 10
  const term = pattern.value.trim()
  const list = entry.history || []
  if (!term || term === '*') return list.slice(0, maxCount)
  return list.filter(h => h.toLowerCase().includes(term.toLowerCase())).slice(0, maxCount)
})

const filteredHistoryItems = computed(() => [
  ...filteredPinnedHistory.value,
  ...filteredNormalHistory.value,
])

function updateDropdownPosition() {
  const rect = shellRef.value?.getBoundingClientRect()
  if (!rect) return
  dropdownStyle.value = {
    top: `${rect.bottom}px`,
    left: `${rect.left}px`,
    minWidth: `${rect.width}px`,
  }
}

watch(keep, val => {
  if (isCluster.value) {
    workspaceStore.keepPrevSearch = true
    return
  }
  workspaceStore.keepPrevSearch = val
})

watch(isCluster, (val) => {
  if (val) {
    keep.value = true
    workspaceStore.keepPrevSearch = true
  }
}, { immediate: true })

watch(() => workspaceStore.activeConnID, () => {
  pattern.value = ''
  keep.value = isCluster.value ? true : workspaceStore.keepPrevSearch
  loading.value = false
  showHistory.value = false
  activeIndex.value = -1
})
watch(() => workspaceStore.currentDB, () => {
  showHistory.value = false
  activeIndex.value = -1
})

function togglePin(item) {
  const id = workspaceStore.activeConnID
  if (!id) return
  workspaceStore.togglePinnedSearchHistory(id, item)
  nextTick(updateDropdownPosition)
}

function onFocus() {
  if (blurTimer) {
    clearTimeout(blurTimer)
    blurTimer = null
  }
  const id = workspaceStore.activeConnID
  if (id) {
    const entry = workspaceStore.getConnSearchHistory(id)
    if (!entry.pinned.length && !entry.history.length) return
    showHistory.value = true
    activeIndex.value = -1
    nextTick(updateDropdownPosition)
  }
}

function onBlur() {
  if (blurTimer) clearTimeout(blurTimer)
  blurTimer = setTimeout(() => {
    showHistory.value = false
    activeIndex.value = -1
    blurTimer = null
  }, 120)
}

function onArrowDown() {
  if (!showHistory.value || !filteredHistoryItems.value.length) return
  activeIndex.value = (activeIndex.value + 1) % filteredHistoryItems.value.length
}

function onArrowUp() {
  if (!showHistory.value || !filteredHistoryItems.value.length) return
  activeIndex.value = (activeIndex.value - 1 + filteredHistoryItems.value.length) % filteredHistoryItems.value.length
}

function onEnter() {
  if (showHistory.value && activeIndex.value >= 0 && filteredHistoryItems.value[activeIndex.value]) {
    selectHistory(filteredHistoryItems.value[activeIndex.value])
  } else {
    doSearch()
  }
}

function selectHistory(item) {
  pattern.value = item
  showHistory.value = false
  activeIndex.value = -1
  doSearch()
}

async function doSearch() {
  if (loading.value) return
  loading.value = true
  try {
    const p = pattern.value.trim()
    if (isCluster.value) {
      if (!p) return
      await workspaceStore.searchExact(p)
      return
    }
    const normalized = p || '*'
    if (normalized !== '*' && !normalized.includes('*') && !normalized.includes('?') && !normalized.includes('[')) {
      await workspaceStore.searchExact(normalized)
    } else {
      await workspaceStore.search(normalized)
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.key-search-bar {
  padding: 8px 10px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
}
.search-input-row {
  display: flex;
  gap: 6px;
  position: relative;
  align-items: center;
  min-width: 0;
}
.search-input-shell {
  display: flex;
  align-items: stretch;
  flex: 1;
  min-width: 0;
  min-height: 30px;
  height: 30px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background: #fff;
  overflow: hidden;
  transition: border-color 0.15s, box-shadow 0.15s;
}
.search-input-shell input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0 10px;
  border: none;
  font-size: 12px;
  line-height: 1;
  outline: none;
  color: #1f2937;
  box-sizing: border-box;
  background: transparent;
}
.search-input-shell:focus-within {
  border-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59,130,246,.12);
}
.btn-search {
  display: inline-flex; align-items: center; justify-content: center;
  width: 34px;
  min-width: 34px;
  height: 100%;
  padding: 0;
  background: #f8fafc;
  color: #3b82f6;
  border: none;
  border-left: 1px solid #e5e7eb;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  transition: color 0.15s, background 0.15s, border-color 0.15s;
}
.btn-search:hover:not(:disabled) { color: #2563eb; background: #eff6ff; border-color: #bfdbfe; }
.btn-search:disabled { color: #93c5fd; cursor: not-allowed; }
.search-loading {
  font-size: 11px;
  letter-spacing: 0.5px;
}
.search-options { margin-top: 5px; }
.keep-label {
  font-size: 12px;
  color: #6b7280;
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}
.cluster-hint {
  font-size: 12px;
  color: #b45309;
  line-height: 1.4;
}

.history-dropdown {
  position: fixed;
  background: white;
  border: 1px solid #d1d5db;
  border-radius: 0 6px 6px 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 10000;
  max-height: 240px;
  overflow-y: auto;
  width: max-content;
}
.history-section-title {
  padding: 7px 10px 5px;
  font-size: 10px;
  line-height: 1;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #94a3b8;
  background: #f8fafc;
}
.history-section-divider {
  height: 1px;
  margin: 2px 0;
  background: #e5e7eb;
}
.history-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 12px;
  color: #374151;
  cursor: pointer;
  white-space: nowrap;
  min-width: 0;
}
.history-item-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.history-pin-btn {
  width: 18px;
  height: 18px;
  min-width: 18px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #cbd5e1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: color 0.15s, background 0.15s;
}
.history-pin-btn:hover {
  color: #64748b;
  background: #e2e8f0;
}
.history-pin-btn.pinned {
  color: #2563eb;
}
.history-item:hover,
.history-item.active {
  background: #eff6ff;
  color: #2563eb;
}
.history-item:hover .history-pin-btn,
.history-item.active .history-pin-btn {
  color: #64748b;
}
.history-item:hover .history-pin-btn.pinned,
.history-item.active .history-pin-btn.pinned {
  color: #2563eb;
}
</style>
