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
    await workspaceStore.search(pattern.value)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.key-search-bar {
  padding: 8px 12px;
  border-bottom: 1px solid #eee;
  background: #fafafa;
}
.search-input-row {
  display: flex;
  gap: 6px;
}
.search-input-row input {
  flex: 1;
  padding: 5px 8px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
}
.search-input-row input:focus { border-color: #4e9af1; }
.btn-search {
  padding: 5px 14px;
  background: #4e9af1;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 13px;
  white-space: nowrap;
}
.btn-search:hover { background: #3a85e0; }
.btn-search:disabled { background: #aaa; cursor: default; }
.search-options { margin-top: 5px; }
.keep-label {
  font-size: 12px;
  color: #666;
  display: flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}
</style>
