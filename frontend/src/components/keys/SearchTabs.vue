<template>
  <div class="search-tabs" v-if="sessions.length > 0">
    <div class="tabs-bar">
      <div
        v-for="s in sessions"
        :key="s.id"
        :class="['tab', { active: activeSessionId === s.id }]"
        @click="workspaceStore.activeSessionId = s.id"
      >
        <span class="tab-label">{{ s.pattern }}</span>
        <button class="tab-close" @click.stop="workspaceStore.closeSession(s.id)">×</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'

const workspaceStore = useWorkspaceStore()
const sessions = computed(() => workspaceStore.searchSessions)
const activeSessionId = computed(() => workspaceStore.activeSessionId)
</script>

<style scoped>
.search-tabs {
  border-bottom: 1px solid #e0e0e0;
  background: #f5f5f5;
}
.tabs-bar {
  display: flex;
  overflow-x: auto;
  gap: 0;
}
.tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  font-size: 12px;
  color: #555;
  cursor: pointer;
  white-space: nowrap;
  border-right: 1px solid #e0e0e0;
  border-bottom: 2px solid transparent;
}
.tab:hover { background: #ebebeb; }
.tab.active {
  background: white;
  color: #4e9af1;
  border-bottom-color: #4e9af1;
  font-weight: 500;
}
.tab-label { max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
.tab-close {
  background: transparent;
  border: none;
  color: #999;
  font-size: 14px;
  cursor: pointer;
  padding: 0 2px;
  line-height: 1;
}
.tab-close:hover { color: #e53e3e; }
</style>
