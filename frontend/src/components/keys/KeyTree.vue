<template>
  <div class="key-tree">
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
                  {{ sess.loading ? t('keyTree.loading') : t('keyTree.loadMore') }}
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
            <div class="tree-load-more">
              <button
                v-if="session.hasMore"
                class="btn-load-more"
                :disabled="session.loading"
                @click="workspaceStore.loadMoreKeys(session.id)"
              >
                {{ session.loading ? t('keyTree.loading') : t('keyTree.loadMore') }}
              </button>
            </div>
          </div>
        </template>
      </div>

      <!-- DB 选择 + Key 统计 -->
      <div class="db-bar">
        <template v-if="!activeConn?.is_cluster">
          <label class="db-label">{{ t('keyTree.db') }}</label>
          <select :value="currentDB" @change="switchDB($event.target.value)" class="db-select">
            <option v-for="i in 16" :key="i-1" :value="i-1">{{ i-1 }}</option>
          </select>
        </template>
        <span class="key-count">{{ t('keyTree.totalKeys', { count: totalKeys }) }}</span>
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
const showClusterEmptyHint = computed(() =>
  !!activeConn.value?.is_cluster &&
  !session.value &&
  displaySessions.value.length === 0
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
</script>

<style scoped>
.key-tree {
  display: flex;
  flex-direction: column;
  height: 100%;
  background:
    linear-gradient(180deg, rgba(248, 251, 255, 0.92), rgba(255, 255, 255, 0.98) 16%, #ffffff 42%);
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
.cluster-empty-state {
  gap: 10px;
  margin: 12px;
  border: 1px dashed rgba(191, 219, 254, 0.92);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.88), rgba(248, 250, 252, 0.92));
}
.cluster-empty-title {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
}
.cluster-empty-text {
  font-size: 12px;
  color: #64748b;
  max-width: 320px;
  line-height: 1.7;
}

.search-section {
  margin-bottom: 10px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 10px 18px rgba(148, 163, 184, 0.08);
  overflow: hidden;
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
.section-tip {
  padding: 16px 18px;
  color: #94a3b8;
  font-size: 12px;
}

.section-load-more,
.tree-load-more {
  display: flex;
  justify-content: center;
  padding: 12px 0 4px;
  flex-shrink: 0;
}

.btn-load-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 30px;
  padding: 0 18px;
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  border: 1px solid rgba(147, 197, 253, 0.9);
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  box-shadow: 0 8px 16px rgba(191, 219, 254, 0.26);
  transition: transform 0.16s ease, background 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}
.btn-load-more:hover:not(:disabled) {
  background: linear-gradient(180deg, #f8fbff, #eff6ff);
  border-color: #60a5fa;
  box-shadow: 0 12px 20px rgba(147, 197, 253, 0.28);
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
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 5px 14px;
  border-top: 1px solid rgba(226, 232, 240, 0.95);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.95), rgba(241, 245, 249, 0.95));
  flex-shrink: 0;
  box-sizing: border-box;
  backdrop-filter: blur(8px);
}
.db-label {
  font-size: 11px;
  color: #64748b;
  font-weight: 600;
  white-space: nowrap;
}
.db-select {
  min-height: 28px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 10px;
  font-size: 12px;
  outline: none;
  background: rgba(255, 255, 255, 0.96);
  color: #1e293b;
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}
.db-select:focus {
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.14);
}
.key-count {
  font-size: 12px;
  color: #64748b;
  font-weight: 500;
  margin-left: auto;
}
</style>
