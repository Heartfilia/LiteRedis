<template>
  <div class="key-tree" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <div v-if="!activeConnID" class="empty-state">
      {{ t('keyTree.selectConn') }}
    </div>
    <template v-else>
      <KeySearchBar />
      <!-- SearchTabs 只在单session模式显示 -->
      <SearchTabs v-if="!workspaceStore.keepPrevSearch && !showClusterEmptyHint" />

      <div class="tree-content">
        <div v-if="showClusterEmptyHint" class="empty-state cluster-empty-state">
          <div class="cluster-empty-title">{{ t('keyTree.clusterEmptyTitle') }}</div>
          <div class="cluster-empty-text">{{ t('keyTree.clusterEmptyHint') }}</div>
        </div>
        <!-- 合并模式：keepPrevSearch = true -->
        <template v-else-if="workspaceStore.keepPrevSearch">
          <div v-if="displaySessions.length === 0" class="empty-state">{{ t('keyTree.searchHint') }}</div>
          <div v-else class="merged-scroll">
            <div v-for="sess in displaySessions" :key="sess.id" class="search-section">
              <div class="search-section-header">
                <span class="section-pattern">🔍 {{ sess.pattern }}</span>
                <span v-if="sess.loading" class="section-status">{{ t('keyTree.loading') }}</span>
                <span v-else class="section-status">{{ sess.keys?.length ?? 0 }}</span>
                <button class="section-close" @click="workspaceStore.removeSession(sess.id)">✕</button>
              </div>
              <div v-if="sess.loading" class="section-tip">{{ t('keyTree.loading') }}</div>
              <div v-else-if="!sess.treeData?.length" class="section-tip">{{ t('keyTree.noKeys') }}</div>
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
                  {{ sess.loading ? '...' : t('keyTree.loadMore') }}
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- 单session模式：keepPrevSearch = false -->
        <template v-else>
          <div v-if="session?.loading" class="loading">{{ t('keyTree.loading') }}</div>
          <div v-else-if="!session" class="empty-state">{{ t('keyTree.searchHint') }}</div>
          <div v-else-if="session.treeData?.length === 0" class="empty-state">{{ t('keyTree.noKeys') }}</div>
          <div v-else class="tree-scroll">
            <KeyTreeNode
              v-for="node in session.treeData"
              :key="node.fullPath"
              :node="node"
              :depth="0"
            />
          </div>
        </template>
      </div>

      <!-- DB 选择 + Key 统计 -->
      <div class="db-bar">
        <div class="db-bar-side db-bar-left">
          <div class="db-bar-left-inner" :class="{ 'cluster-align': activeConn?.is_cluster }">
            <template v-if="!activeConn?.is_cluster">
              <select :value="currentDB" @change="switchDB($event.target.value)" class="db-select">
                <option v-for="i in 16" :key="i-1" :value="i-1">{{ i-1 }}</option>
              </select>
            </template>
            <div v-if="showTreeExpandControls" class="tree-bulk-controls" aria-label="tree bulk controls">
              <button
                class="tree-bulk-btn"
                type="button"
                :title="t('keyTree.collapseAll')"
                @click="collapseVisibleTree"
              >
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M6 15.75h12" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
                  <path d="M8.25 12.75 12 9l3.75 3.75" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
              <button
                class="tree-bulk-btn"
                type="button"
                :title="t('keyTree.expandAll')"
                @click="expandVisibleTree"
              >
                <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path d="M6 8.25h12" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
                  <path d="M8.25 11.25 12 15l3.75-3.75" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
            </div>
          </div>
        </div>
        <div class="db-bar-center">
          <button
            v-if="showBottomLoadMore"
            class="btn-load-more"
            :disabled="session?.loading"
            @click="workspaceStore.loadMoreKeys(session.id)"
          >
            {{ session?.loading ? '...' : t('keyTree.loadMore') }}
          </button>
        </div>
        <div class="db-bar-side db-bar-right">
          <span class="key-count">key:{{ totalKeys }}</span>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import KeySearchBar from './KeySearchBar.vue'
import SearchTabs from './SearchTabs.vue'
import KeyTreeNode from './KeyTreeNode.vue'
import { buildKeyTree } from '../../utils/keyTree.js'

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()
const connectionsStore = useConnectionsStore()
const settingsStore = useSettingsStore()
const activeConnID = computed(() => workspaceStore.activeConnID)
const currentDB = computed(() => workspaceStore.currentDB)
const totalKeys = computed(() => workspaceStore.totalKeys)
const keyDisplayMode = computed(() => settingsStore.keyDisplayMode || 'tree')
const session = computed(() => mapSessionForDisplay(workspaceStore.activeSession))
const displaySessions = computed(() => workspaceStore.displaySessions.map(mapSessionForDisplay))
const activeConn = computed(() => connectionsStore.connections.find(c => c.id === activeConnID.value))
const clusterExactOnly = computed(() => !!activeConn.value?.is_cluster && !activeConn.value?.allow_cluster_scan)
const showClusterEmptyHint = computed(() =>
  clusterExactOnly.value &&
  !session.value &&
  displaySessions.value.length === 0
)
const visibleTreeData = computed(() => {
  if (workspaceStore.keepPrevSearch) {
    return displaySessions.value.flatMap(item => item?.treeData || [])
  }
  return session.value?.treeData || []
})
const showTreeExpandControls = computed(() =>
  keyDisplayMode.value === 'tree' &&
  visibleTreeData.value.length > 0 &&
  !showClusterEmptyHint.value
)
const showBottomLoadMore = computed(() =>
  !workspaceStore.keepPrevSearch &&
  !!session.value?.hasMore
)

async function switchDB(db) {
  await workspaceStore.switchDB(parseInt(db))
  await workspaceStore.search('*')
}

function buildFlatTree(keys = []) {
  return keys.map(key => ({
    label: key.name,
    fullPath: key.name,
    isLeaf: true,
    keyType: key.type,
    ttl: key.ttl,
    children: [],
    count: 1,
  }))
}

function mapSessionForDisplay(source) {
  if (!source) return source
  const keys = source.keys || []
  return {
    ...source,
    treeData: keyDisplayMode.value === 'flat' ? buildFlatTree(keys) : buildKeyTree(keys),
  }
}

function expandVisibleTree() {
  workspaceStore.setVisibleTreeExpanded(visibleTreeData.value, true)
}

function collapseVisibleTree() {
  workspaceStore.setVisibleTreeExpanded(visibleTreeData.value, false)
}
</script>

<style scoped>
.key-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  color: #0f172a;
  background:
    linear-gradient(180deg, rgba(248, 251, 255, 0.92), rgba(255, 255, 255, 0.98) 16%, #ffffff 42%);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}
.key-tree.theme-dark {
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.12), transparent 24%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.92), rgba(15, 23, 42, 0.96) 20%, rgba(2, 6, 23, 0.98) 100%);
}
.key-tree.theme-dark {
  color: #e2e8f0;
  background:
    radial-gradient(circle at top right, rgba(37, 99, 235, 0.12), transparent 24%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.92), rgba(15, 23, 42, 0.96) 20%, rgba(2, 6, 23, 0.98) 100%);
}
.tree-content {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.tree-scroll,
.merged-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 8px 8px 10px;
}
.loading,
.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: #64748b;
  font-size: 13px;
  padding: 40px;
  text-align: center;
}
.key-tree.theme-dark .loading,
.key-tree.theme-dark .empty-state {
  color: #94a3b8;
}
.cluster-empty-state {
  gap: 10px;
  margin: 12px;
  border: 1px dashed rgba(191, 219, 254, 0.92);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.88), rgba(248, 250, 252, 0.92));
}
.key-tree.theme-dark .cluster-empty-state {
  border-color: rgba(59, 130, 246, 0.34);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.94), rgba(15, 23, 42, 0.92));
}
.cluster-empty-title {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
}
.key-tree.theme-dark .cluster-empty-title {
  color: #e2e8f0;
}
.cluster-empty-text {
  font-size: 12px;
  color: #64748b;
  max-width: 320px;
  line-height: 1.7;
}
.key-tree.theme-dark .cluster-empty-text {
  color: #94a3b8;
}

.search-section {
  margin-bottom: 10px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 10px 18px rgba(148, 163, 184, 0.08);
  overflow: hidden;
}
.key-tree.theme-dark .search-section {
  border-color: rgba(51, 65, 85, 0.94);
  background: rgba(15, 23, 42, 0.84);
  box-shadow: 0 14px 28px rgba(2, 6, 23, 0.28);
}
.search-section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(241, 245, 249, 0.96));
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  position: sticky;
  top: 0;
  z-index: 2;
  backdrop-filter: blur(8px);
}
.key-tree.theme-dark .search-section-header {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.96));
  color: #cbd5e1;
  border-bottom-color: rgba(51, 65, 85, 0.94);
}
.section-pattern { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.section-status {
  font-size: 10px;
  color: #94a3b8;
  font-weight: 500;
  flex-shrink: 0;
}
.section-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  background: rgba(255, 255, 255, 0.72);
  border: none;
  color: #94a3b8;
  cursor: pointer;
  font-size: 12px;
  padding: 0;
  flex-shrink: 0;
  line-height: 1;
  border-radius: 999px;
  transition: background 0.16s ease, color 0.16s ease, transform 0.16s ease;
}
.section-close:hover {
  color: #dc2626;
  background: #fee2e2;
  transform: scale(1.04);
}
.key-tree.theme-dark .section-close {
  background: rgba(15, 23, 42, 0.94);
  color: #94a3b8;
  box-shadow: inset 0 0 0 1px rgba(51, 65, 85, 0.9);
}
.key-tree.theme-dark .section-close:hover {
  color: #fca5a5;
  background: rgba(127, 29, 29, 0.56);
  box-shadow: inset 0 0 0 1px rgba(248, 113, 113, 0.28);
}
.section-tip {
  padding: 16px 18px;
  color: #94a3b8;
  font-size: 12px;
}
.key-tree.theme-dark .section-tip {
  color: #94a3b8;
}

.section-load-more {
  display: flex;
  justify-content: center;
  min-height: 26px;
  padding: 4px 10px 8px;
  align-items: center;
  flex-shrink: 0;
}

.btn-load-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 24px;
  height: 24px;
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
  background: #f8fafc;
  box-shadow: none;
}
.load-more-hint {
  font-size: 12px;
  color: #9ca3af;
}

.db-bar {
  display: grid;
  grid-template-columns: minmax(0, auto) minmax(0, 1fr) auto;
  align-items: center;
  min-height: 34px;
  height: 34px;
  padding: 3px 12px;
  border-top: 1px solid rgba(226, 232, 240, 0.95);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.95), rgba(241, 245, 249, 0.95));
  flex-shrink: 0;
  box-sizing: border-box;
  backdrop-filter: blur(8px);
}
.key-tree.theme-dark .db-bar {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.98), rgba(11, 18, 32, 0.995));
  backdrop-filter: none;
}
.db-bar-side {
  display: flex;
  align-items: center;
  min-width: 0;
}
.db-bar-left {
  justify-content: flex-start;
  overflow: hidden;
}
.db-bar-left-inner {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  max-width: 100%;
}
.db-bar-left-inner.cluster-align {
  justify-content: flex-start;
}
.db-bar-center {
  display: flex;
  align-items: center;
  justify-content: center;
  min-width: 0;
  height: 100%;
  padding: 0 6px;
  overflow: hidden;
}
.db-bar-right {
  justify-content: flex-end;
  min-width: fit-content;
  padding-left: 6px;
}
.db-select {
  width: 46px;
  min-width: 46px;
  min-height: 24px;
  padding: 0 6px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 10px;
  font-size: 12px;
  outline: none;
  background: rgba(255, 255, 255, 0.96);
  color: #1e293b;
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}
.key-tree.theme-dark .db-select {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.92);
  color: #e2e8f0;
}
.db-select:focus {
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.14);
}
.tree-bulk-controls {
  display: inline-flex;
  align-items: center;
  gap: 1px;
  min-width: 0;
  flex-shrink: 0;
}
.tree-bulk-btn {
  width: 18px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  flex-shrink: 0;
  transition: background 0.16s ease, color 0.16s ease, opacity 0.16s ease;
  opacity: 0.78;
}
.tree-bulk-btn svg {
  width: 14px;
  height: 14px;
  display: block;
}
.tree-bulk-btn:hover {
  color: #2563eb;
  background: rgba(239, 246, 255, 0.72);
  opacity: 1;
}
.tree-bulk-btn:active {
  opacity: 1;
}
.key-tree.theme-dark .tree-bulk-btn {
  color: #94a3b8;
  opacity: 0.84;
}
.key-tree.theme-dark .tree-bulk-btn:hover {
  color: #dbeafe;
  background: rgba(30, 41, 59, 0.9);
  opacity: 1;
}
.key-count {
  font-size: 11px;
  color: #64748b;
  font-weight: 500;
  text-align: right;
  white-space: nowrap;
}
.key-tree.theme-dark .key-count {
  color: #94a3b8;
}
</style>
