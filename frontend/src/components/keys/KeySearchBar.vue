<template>
  <div class="key-search-bar">
    <div class="search-input-row" ref="inputRowRef">
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

      <!-- 历史记录下拉 -->
      <div v-if="showHistory && filteredHistory.length" class="history-dropdown">
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
    <div class="search-options">
      <label class="keep-label">
        <input type="checkbox" v-model="keep" />
        {{ t('keyTree.keepPrev') }}
      </label>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const pattern = ref('*')
const keep = ref(workspaceStore.keepPrevSearch)
const loading = ref(false)
const showHistory = ref(false)
const activeIndex = ref(-1)
const inputRowRef = ref(null)

const filteredHistory = computed(() => {
  const id = workspaceStore.activeConnID
  if (!id) return []
  const list = workspaceStore.connSearchHistory[id] || []
  const term = pattern.value.trim()
  if (!term || term === '*') return list.slice(0, 10)
  return list.filter(h => h.toLowerCase().includes(term.toLowerCase())).slice(0, 10)
})

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
  gap: 0;
  position: relative;
}
.search-input-row input {
  flex: 1;
  padding: 5px 10px;
  border: 1px solid #d1d5db;
  border-right: none;
  border-radius: 6px 0 0 6px;
  font-size: 12px;
  outline: none;
  color: #1f2937;
  transition: border-color 0.15s;
}
.search-input-row input:focus { border-color: #3b82f6; }
.btn-search {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 5px 14px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 0 6px 6px 0;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  transition: background 0.15s;
}
.btn-search:hover:not(:disabled) { background: #2563eb; }
.btn-search:disabled { background: #93c5fd; cursor: not-allowed; }
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
  position: absolute;
  top: 100%;
  left: 0;
  right: 36px;
  background: white;
  border: 1px solid #d1d5db;
  border-top: none;
  border-radius: 0 0 6px 6px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 100;
  max-height: 240px;
  overflow-y: auto;
}
.history-item {
  padding: 6px 10px;
  font-size: 12px;
  color: #374151;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.history-item:hover,
.history-item.active {
  background: #eff6ff;
  color: #2563eb;
}
</style>
