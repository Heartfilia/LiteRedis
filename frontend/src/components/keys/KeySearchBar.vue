<template>
  <div class="key-search-bar">
    <div class="search-input-row" ref="inputRowRef">
      <div class="search-input-shell" ref="shellRef">
        <input
          v-model="pattern"
          type="text"
          :placeholder="t('keyTree.searchPlaceholder')"
          @keydown.enter="onEnter"
          @keydown.down.prevent="onArrowDown"
          @keydown.up.prevent="onArrowUp"
          @keydown.esc="showHistory = false"
          @focus="onFocus"
          @blur="onBlur"
        />
        <button class="btn-search" @click="doSearch" :disabled="loading">
          {{ loading ? '...' : t('keyTree.searchBtn') }}
        </button>
      </div>
      <CreateKeyButton v-if="workspaceStore.activeConnID" />
    </div>
    <div class="search-options">
      <label class="keep-label">
        <input type="checkbox" v-model="keep" />
        {{ t('keyTree.keepPrev') }}
      </label>
    </div>

    <!-- 历史记录下拉（fixed 定位，避免被父容器 overflow:hidden 裁切） -->
    <div
      v-if="showHistory && filteredHistory.length"
      class="history-dropdown"
      :style="dropdownStyle"
    >
      <div
        v-for="(item, idx) in filteredHistory"
        :key="item"
        :class="['history-item', { active: idx === activeIndex }]"
        @mousedown.prevent="selectHistory(item)"
        @mouseenter="activeIndex = idx"
      >
        {{ item }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'
import CreateKeyButton from './CreateKeyButton.vue'

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const pattern = ref('*')
const keep = ref(workspaceStore.keepPrevSearch)
const loading = ref(false)
const showHistory = ref(false)
const activeIndex = ref(-1)
const inputRowRef = ref(null)
const shellRef = ref(null)
const dropdownStyle = ref({})

const filteredHistory = computed(() => {
  const id = workspaceStore.activeConnID
  if (!id) return []
  const list = workspaceStore.connSearchHistory[id] || []
  const term = pattern.value.trim()
  if (!term || term === '*') return list.slice(0, 10)
  return list.filter(h => h.toLowerCase().includes(term.toLowerCase())).slice(0, 10)
})

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
  workspaceStore.keepPrevSearch = val
})

watch(() => workspaceStore.activeConnID, () => {
  pattern.value = '*'
  keep.value = workspaceStore.keepPrevSearch
  loading.value = false
  showHistory.value = false
  activeIndex.value = -1
})
watch(() => workspaceStore.currentDB, () => {
  showHistory.value = false
  activeIndex.value = -1
})

function onFocus() {
  const id = workspaceStore.activeConnID
  if (id && (workspaceStore.connSearchHistory[id] || []).length > 0) {
    showHistory.value = true
    activeIndex.value = -1
    nextTick(updateDropdownPosition)
  }
}

function onBlur() {
  showHistory.value = false
  activeIndex.value = -1
}

function onArrowDown() {
  if (!showHistory.value || !filteredHistory.value.length) return
  activeIndex.value = (activeIndex.value + 1) % filteredHistory.value.length
}

function onArrowUp() {
  if (!showHistory.value || !filteredHistory.value.length) return
  activeIndex.value = (activeIndex.value - 1 + filteredHistory.value.length) % filteredHistory.value.length
}

function onEnter() {
  if (showHistory.value && activeIndex.value >= 0 && filteredHistory.value[activeIndex.value]) {
    selectHistory(filteredHistory.value[activeIndex.value])
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
    const p = pattern.value.trim() || '*'
    if (p !== '*' && !p.includes('*') && !p.includes('?') && !p.includes('[')) {
      await workspaceStore.searchExact(p)
    } else {
      await workspaceStore.search(p)
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
  position: relative;
  flex: 1;
  min-width: 0;
}
.search-input-shell input {
  width: 100%;
  padding: 5px 66px 5px 10px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 12px;
  outline: none;
  color: #1f2937;
  transition: border-color 0.15s;
  box-sizing: border-box;
}
.search-input-shell input:focus { border-color: #3b82f6; }
.btn-search {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  display: inline-flex; align-items: center; justify-content: center;
  height: 24px;
  padding: 0 10px;
  background: transparent;
  color: #3b82f6;
  border: none;
  border-left: 1px solid #e5e7eb;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  transition: color 0.15s;
}
.btn-search:hover:not(:disabled) { color: #2563eb; }
.btn-search:disabled { color: #93c5fd; cursor: not-allowed; }
.search-options { margin-top: 5px; }
.keep-label {
  font-size: 12px;
  color: #6b7280;
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}

.history-dropdown {
  position: fixed;
  background: white;
  border: 1px solid #d1d5db;
  border-top: none;
  border-radius: 0 0 6px 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 10000;
  max-height: 240px;
  overflow-y: auto;
  width: max-content;
}
.history-item {
  padding: 6px 10px;
  font-size: 12px;
  color: #374151;
  cursor: pointer;
  white-space: nowrap;
}
.history-item:hover,
.history-item.active {
  background: #eff6ff;
  color: #2563eb;
}
</style>
