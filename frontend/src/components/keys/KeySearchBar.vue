<template>
  <div class="key-search-bar">
    <div class="search-input-row">
      <input
        v-model="pattern"
        type="text"
        placeholder="搜索 key，支持 * 通配符，如 user:*"
        @keydown.enter="doSearch"
      />
      <button class="btn-search" @click="doSearch" :disabled="loading">
        {{ loading ? '...' : '搜索' }}
      </button>
    </div>
    <div class="search-options">
      <label class="keep-label">
        <input type="checkbox" v-model="keep" />
        保留上次搜索
      </label>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'

const workspaceStore = useWorkspaceStore()

const pattern = ref('*')
const keep = ref(workspaceStore.keepPrevSearch)
const loading = ref(false)

watch(keep, val => {
  workspaceStore.keepPrevSearch = val
})

// 切换连接或切换 DB 时，重置搜索框为 *
watch(() => workspaceStore.activeConnID, () => {
  pattern.value = '*'
  loading.value = false
})
watch(() => workspaceStore.currentDB, () => {
  pattern.value = '*'
  loading.value = false
})

async function doSearch() {
  if (loading.value) return
  loading.value = true
  try {
    const p = pattern.value.trim() || '*'
    // 没有通配符且不为空：构造只含这一个 key 的搜索 session，并直接选中展示 value
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
</style>
