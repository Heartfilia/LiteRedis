<template>
  <div class="key-tree">
    <div v-if="!activeConnID" class="empty-state">
      请在左侧选择一个连接
    </div>
    <template v-else>
      <KeySearchBar />
      <!-- SearchTabs 只在单session模式显示 -->
      <SearchTabs v-if="!workspaceStore.keepPrevSearch" />

      <div class="tree-content">
        <!-- 合并模式：keepPrevSearch = true -->
        <template v-if="workspaceStore.keepPrevSearch">
          <div v-if="displaySessions.length === 0" class="empty-state">输入关键词搜索 key</div>
          <div v-else class="merged-scroll">
            <div v-for="sess in displaySessions" :key="sess.id" class="search-section">
              <div class="search-section-header">
                <span class="section-pattern">🔍 {{ sess.pattern }}</span>
                <span v-if="sess.loading" class="section-status">加载中...</span>
                <span v-else class="section-status">{{ sess.keys?.length ?? 0 }} 个</span>
                <button class="section-close" @click="workspaceStore.removeSession(sess.id)">✕</button>
              </div>
              <div v-if="sess.loading" class="section-tip">加载中...</div>
              <div v-else-if="!sess.treeData?.length" class="section-tip">未找到匹配的 key</div>
              <div v-else>
                <KeyTreeNode
                  v-for="node in sess.treeData"
                  :key="node.fullPath"
                  :node="node"
                  :depth="0"
                />
              </div>
              <div class="section-load-more">
                <button
                  v-if="sess.hasMore"
                  class="btn-load-more"
                  :disabled="sess.loading"
                  @click="workspaceStore.loadMoreKeys(sess.id)"
                >
                  {{ sess.loading ? '加载中...' : '加载更多' }}
                </button>
                <span v-else-if="sess.keys?.length > 0" class="load-more-hint">已加载全部 {{ sess.keys.length }} 个 key</span>
              </div>
            </div>
          </div>
        </template>

        <!-- 单session模式：keepPrevSearch = false -->
        <template v-else>
          <div v-if="session?.loading" class="loading">加载中...</div>
          <div v-else-if="!session" class="empty-state">输入关键词搜索 key</div>
          <div v-else-if="session.treeData?.length === 0" class="empty-state">未找到匹配的 key</div>
          <div v-else class="tree-scroll">
            <KeyTreeNode
              v-for="node in session.treeData"
              :key="node.fullPath"
              :node="node"
              :depth="0"
            />
            <div class="tree-load-more">
              <button
                v-if="session.hasMore"
                class="btn-load-more"
                :disabled="session.loading"
                @click="workspaceStore.loadMoreKeys(session.id)"
              >
                {{ session.loading ? '加载中...' : '加载更多' }}
              </button>
              <span v-else class="load-more-hint">已加载全部 {{ session.keys.length }} 个 key</span>
            </div>
          </div>
        </template>
      </div>

      <!-- DB 选择 + Key 统计 -->
      <div class="db-bar">
        <template v-if="!activeConn?.is_cluster">
          <label class="db-label">DB</label>
          <select :value="currentDB" @change="switchDB($event.target.value)" class="db-select">
            <option v-for="i in 16" :key="i-1" :value="i-1">{{ i-1 }}</option>
          </select>
        </template>
        <span class="key-count">共 {{ totalKeys }} 个 Key</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import KeySearchBar from './KeySearchBar.vue'
import SearchTabs from './SearchTabs.vue'
import KeyTreeNode from './KeyTreeNode.vue'

const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
const activeConnID = computed(() => workspaceStore.activeConnID)
const currentDB = computed(() => workspaceStore.currentDB)
const totalKeys = computed(() => workspaceStore.totalKeys)
const session = computed(() => workspaceStore.activeSession)
const displaySessions = computed(() => workspaceStore.displaySessions)
const activeConn = computed(() => connectionsStore.connections.find(c => c.id === activeConnID.value))

async function switchDB(db) {
  await workspaceStore.switchDB(parseInt(db))
  await workspaceStore.search('*')
}
</script>

<style scoped>
.key-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: white;
  border-right: 1px solid #e0e0e0;
}
.tree-content { flex: 1; overflow: hidden; display: flex; flex-direction: column; }
.tree-scroll { flex: 1; overflow-y: auto; padding: 4px; }
.loading, .empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #999;
  font-size: 13px;
  padding: 40px;
}

/* 合并模式 */
.merged-scroll { flex: 1; overflow-y: auto; }
.search-section { border-bottom: 1px solid #f0f0f0; }
.search-section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  background: #f5f7fa;
  font-size: 11px;
  font-weight: 600;
  color: #6b7280;
  border-bottom: 1px solid #e8e8e8;
  position: sticky;
  top: 0;
  z-index: 1;
}
.section-pattern { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.section-status { font-size: 10px; color: #aaa; font-weight: 400; flex-shrink: 0; }
.section-close {
  background: transparent;
  border: none;
  color: #bbb;
  cursor: pointer;
  font-size: 12px;
  padding: 0 2px;
  flex-shrink: 0;
  line-height: 1;
}
.section-close:hover { color: #e53e3e; }
.section-tip { padding: 10px 16px; color: #bbb; font-size: 12px; }

.section-load-more,
.tree-load-more {
  display: flex;
  justify-content: center;
  padding: 8px 0;
  flex-shrink: 0;
}

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

.db-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 12px;
  border-top: 1px solid #e8e8e8;
  background: #fafafa;
  flex-shrink: 0;
}
.db-label {
  font-size: 11px;
  color: #888;
  font-weight: 500;
  white-space: nowrap;
}
.db-select {
  padding: 2px 6px;
  border: 1px solid #ddd;
  border-radius: 4px;
  font-size: 12px;
  outline: none;
  background: white;
  color: #333;
  cursor: pointer;
}
.db-select:focus { border-color: #4e9af1; }
.key-count {
  font-size: 11px;
  color: #aaa;
  margin-left: auto;
}
</style>
