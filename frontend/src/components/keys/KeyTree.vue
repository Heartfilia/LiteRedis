<template>
  <div class="key-tree" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <div v-if="!activeConnID" class="empty-state">
      {{ t('keyTree.selectConn') }}
    </div>
    <template v-else>
      <KeySearchBar />
      <!-- SearchTabs 只在单session模式显示 -->
      <SearchTabs v-if="!workspaceStore.keepPrevSearch && !showClusterEmptyHint" />

      <div v-if="favoriteKeys.length || recentKeyList.length" class="shortcut-block">
        <div v-if="favoriteKeys.length" class="shortcut-group">
          <div class="shortcut-title">★ {{ t('keyTree.favorites') }} · {{ favoriteKeys.length }}</div>
          <div
            v-for="item in favoriteKeys"
            :key="'fav-' + item"
            class="shortcut-row"
            :title="item"
            @click="workspaceStore.selectKey(item)"
          >
            <span class="shortcut-star active" @click.stop="workspaceStore.toggleFavorite(item)">★</span>
            <span class="shortcut-name">{{ item }}</span>
          </div>
        </div>
        <div v-if="recentKeyList.length" class="shortcut-group">
          <div class="shortcut-title">{{ t('keyTree.recent') }} · {{ recentKeyList.length }}</div>
          <div
            v-for="item in recentKeyList"
            :key="'recent-' + item"
            class="shortcut-row"
            :title="item"
            @click="workspaceStore.selectKey(item)"
          >
            <span
              class="shortcut-star"
              :class="{ active: favoriteKeySet.has(item) }"
              :title="t('keyTree.favoriteToggle')"
              @click.stop="workspaceStore.toggleFavorite(item)"
            >{{ favoriteKeySet.has(item) ? '★' : '☆' }}</span>
            <span class="shortcut-name">{{ item }}</span>
          </div>
        </div>
      </div>

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
                  :class="{ loading: sess.loading }"
                  :title="t('keyTree.loadMore')"
                  :disabled="sess.loading"
                  @click="workspaceStore.loadMoreKeys(sess.id)"
                >
                  <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
                    <path d="M4 6.2 8 10.2l4-4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                    <path d="M4 9.6 8 13.6l4-4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>

                  </svg>
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

      <div v-if="workspaceStore.selectionMode" class="batch-bar">
        <span class="batch-info">{{ t('keyTree.selectedCount').replace('{n}', workspaceStore.selectedKeys.length) }}</span>
        <span v-if="readOnly" class="batch-readonly">🔒</span>
        <button class="batch-btn" :disabled="readOnly || !workspaceStore.selectedKeys.length" @click.stop="openBatchDelete">
          {{ t('keyTree.batchDelete') }}
        </button>
        <button class="batch-btn" :disabled="readOnly || !workspaceStore.selectedKeys.length" @click.stop="openBatchTTL">
          {{ t('keyTree.batchTTL') }}
        </button>
        <button class="batch-btn" :disabled="readOnly || !workspaceStore.selectedKeys.length" @click.stop="openMoreMenu">
          ⋯
        </button>
      </div>

      <!-- DB 选择 + Key 统计 -->
      <div class="db-bar">
        <div class="db-bar-side db-bar-left">
          <div class="db-bar-left-inner" :class="{ 'cluster-align': activeConn?.is_cluster }">
            <template v-if="!activeConn?.is_cluster">
              <select :value="currentDB" @change="switchDB($event.target.value)" class="db-select">
                <option v-for="i in dbCount" :key="i-1" :value="i-1">{{ i-1 }}</option>
              </select>
            </template>
            <button
              v-if="!readOnly"
              class="tree-bulk-btn"
              type="button"
              :title="t('keyTree.importKeys')"
              @click="importKeys"
            >
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M12 4v9" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
                <path d="M8.5 9.5 12 13l3.5-3.5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M5 16.5v2A1.5 1.5 0 0 0 6.5 20h11a1.5 1.5 0 0 0 1.5-1.5v-2" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
              </svg>
            </button>
            <button
              class="tree-bulk-btn"
              :class="{ active: workspaceStore.selectionMode }"
              type="button"
              :title="t('keyTree.multiSelect')"
              @click="workspaceStore.toggleSelectionMode()"
            >
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <rect x="4" y="4" width="16" height="16" rx="3" stroke="currentColor" stroke-width="1.6"/>
                <path d="M8 12l2.5 2.5L16 9" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </button>
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
            :class="{ loading: session?.loading }"
            :title="t('keyTree.loadMore')"
            :disabled="session?.loading"
            @click="workspaceStore.loadMoreKeys(session.id)"
          >
            <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
                    <path d="M4 6.2 8 10.2l4-4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                    <path d="M4 9.6 8 13.6l4-4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>

                  </svg>
          </button>
        </div>
        <div class="db-bar-side db-bar-right">
          <span class="key-count">key:{{ totalKeys }}</span>
        </div>
      </div>
    </template>

    <Teleport to="body">
      <div v-if="moreMenu.visible" class="more-menu-backdrop" @mousedown.self="closeMoreMenu">
        <div
          class="more-menu"
          :class="'theme-' + (settingsStore.themeMode || 'light')"
          :style="{ left: moreMenu.x + 'px', top: moreMenu.y + 'px' }"
          @mousedown.stop
          @click.stop
        >
          <template v-if="moreMenu.mode === 'more'">
            <button class="more-item" @click="pickMore('rename')">{{ t('keyTree.batchRename') }}</button>
            <button class="more-item" @click="pickMore('json')">{{ t('keyTree.exportJson') }}</button>
            <button class="more-item" @click="pickMore('commands')">{{ t('keyTree.exportCommands') }}</button>
            <div class="more-divider" />
            <button class="more-item" @click="pickMore('copy')">
              {{ t('keyTree.copyTo') }}
            </button>
            <div class="more-divider" />
            <button class="more-item" @click="exitSelection">{{ t('keyTree.cancelSelection') }}</button>
          </template>
          <template v-else-if="moreMenu.mode === 'confirmDelete'">
            <div class="more-title">
              {{ t('keyTree.batchDeleteConfirm').replace('{n}', workspaceStore.selectedKeys.length) }}
            </div>
            <div class="more-actions">
              <button class="more-btn danger" @click="confirmBatchDelete">{{ t('keyTree.batchDelete') }}</button>
              <button class="more-btn ghost" @click="closeMoreMenu">{{ t('keyTree.cancelSelection') }}</button>
            </div>
          </template>
          <template v-else-if="moreMenu.mode === 'ttl'">
            <div class="more-title">{{ t('keyTree.batchTTLTitle') }}</div>
            <input
              v-model="ttlInput"
              class="more-input"
              inputmode="numeric"
              @keydown.enter="applyBatchTTL"
            />
            <div class="more-actions">
              <button class="more-btn" @click="applyBatchTTL">{{ t('keyTree.confirm') }}</button>
              <button class="more-btn ghost" @click="closeMoreMenu">{{ t('keyTree.cancelSelection') }}</button>
            </div>
          </template>
          <template v-else-if="moreMenu.mode === 'rename'">
            <div class="more-title">{{ t('keyTree.batchRenameTitle').replace('{prefix}', renameFrom) }}</div>
            <input
              ref="renameInputRef"
              v-model="renamePrefix"
              class="more-input"
              spellcheck="false"
              @keydown.enter="applyRename"
            />
            <div class="more-actions">
              <button class="more-btn" @click="applyRename">{{ t('keyTree.confirm') }}</button>
              <button class="more-btn ghost" @click="closeMoreMenu">{{ t('keyTree.cancelSelection') }}</button>
            </div>
          </template>
          <template v-else>
            <div class="more-title">{{ t('keyTree.copyToTarget') }}</div>
            <template v-if="!isCluster">
              <div class="more-subtitle">{{ t('keyTree.copyToDb') }}</div>
              <div v-if="dbGridMode === 'input'" class="db-input-row">
                <input
                  v-model="copyDbInput"
                  class="more-input"
                  :placeholder="t('keyTree.copyDbPlaceholder')"
                  @keydown.enter="copyToDBInput"
                />
                <button class="more-btn" @click="copyToDBInput">{{ t('keyTree.confirm') }}</button>
              </div>
              <div v-else class="db-grid" :class="{ dense: dbGridMode === 'dense' }">
                <button v-for="db in dbOptions" :key="'db-' + db" class="db-chip" @click="copyToDB(db)">
                  DB {{ db }}
                </button>
              </div>
            </template>
            <div class="more-divider" />
            <div class="more-subtitle">{{ t('keyTree.copyToConn') }}</div>
            <button
              v-for="target in copyTargets"
              :key="target.id"
              class="more-item"
              @click="copyTo(target)"
            >
              {{ target.name || t('sidebar.unnamed') }}
              <span class="target-db">[{{ targetDBs[target.id] ?? '?' }}]</span>
            </button>
            <div v-if="!copyTargets.length" class="more-empty">{{ t('keyTree.copyNoTarget') }}</div>
          </template>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import KeySearchBar from './KeySearchBar.vue'
import SearchTabs from './SearchTabs.vue'
import KeyTreeNode from './KeyTreeNode.vue'
import { buildKeyTree } from '../../utils/keyTree.js'
import { exportKeysToFile, copyKeysToConnection, copyKeysToDatabase, getDatabases, getConnectionDB, importKeysFromFile } from '../../api/wails.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'

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
const favoriteKeys = computed(() => workspaceStore.favoriteKeys)
const recentKeyList = computed(() => workspaceStore.recentKeyList)
const favoriteKeySet = computed(() => workspaceStore.favoriteKeySet)
const readOnly = computed(() => connectionsStore.isReadOnly(activeConnID.value))

// 批量操作「更多」菜单：改前缀 / 导出 / 复制到其他连接
const moreMenu = ref({ visible: false, x: 0, y: 0, mode: 'more' })
const renamePrefix = ref('')
const renameFrom = ref('')
const renameInputRef = ref(null)
const ttlInput = ref('3600')
// 目标实例当前所在 DB，展示成 name[db]
const targetDBs = ref({})
// 只列出已连接的其他实例，未连接的没法直接写数据
const copyTargets = computed(() =>
  connectionsStore.connections.filter(
    item => item.id !== activeConnID.value && connectionsStore.isConnected(item.id)
  )
)
const isCluster = computed(() => !!activeConn.value?.is_cluster)
// 真实 DB 数量：读不到 CONFIG 时后端回退 16
const dbCount = ref(16)
const copyDbInput = ref('')
// 同实例跨 DB 复制：列出除当前库以外的 DB
const dbOptions = computed(() => {
  const current = Number(workspaceStore.currentDB) || 0
  const total = Math.max(1, dbCount.value || 16)
  return Array.from({ length: total }, (_, index) => index).filter(index => index !== current)
})
const dbGridMode = computed(() => {
  if (dbOptions.value.length <= 20) return 'compact'
  if (dbOptions.value.length <= 64) return 'dense'
  return 'input'
})

async function loadDatabaseCount() {
  if (!activeConnID.value) return
  try {
    const count = await getDatabases(activeConnID.value)
    if (count > 0) dbCount.value = count
  } catch (e) {
    dbCount.value = 16
  }
}

onMounted(loadDatabaseCount)
watch(activeConnID, () => {
  dbCount.value = 16
  loadDatabaseCount()
})

function openMoreMenu(event) {
  if (readOnly.value || !workspaceStore.selectedKeys.length) return
  const rect = event.currentTarget.getBoundingClientRect()
  moreMenu.value = { visible: true, x: rect.left, y: rect.top - 8, mode: 'more' }
}

function closeMoreMenu() {
  moreMenu.value.visible = false
}

function exitSelection() {
  closeMoreMenu()
  workspaceStore.toggleSelectionMode()
}

async function loadTargetDBs() {
  const map = {}
  await Promise.all(
    copyTargets.value.map(async (target) => {
      try {
        map[target.id] = await getConnectionDB(target.id)
      } catch (e) {
        // 读不到就显示 ?，不影响复制
      }
    })
  )
  targetDBs.value = map
}

async function pickMore(kind) {
  if (kind === 'copy') {
    moreMenu.value = { ...moreMenu.value, mode: 'copy' }
    loadTargetDBs()
    return
  }
  if (kind === 'rename') {
    const keys = [...workspaceStore.selectedKeys]
    renameFrom.value = commonKeyPrefix(keys)
    if (!renameFrom.value) {
      closeMoreMenu()
      connectionsStore.showGlobalToast(t('keyTree.batchRenameInvalid'), false)
      return
    }
    renamePrefix.value = renameFrom.value
    moreMenu.value = { ...moreMenu.value, mode: 'rename' }
    await nextTick()
    renameInputRef.value?.focus()
    renameInputRef.value?.select()
    return
  }
  closeMoreMenu()
  await exportSelected(kind === 'commands' ? 'commands' : 'json')
}

async function applyRename() {
  const nextPrefix = String(renamePrefix.value || '').trim()
  if (!nextPrefix) {
    connectionsStore.showGlobalToast(t('keyTree.batchRenameInvalid'), false)
    return
  }
  if (nextPrefix === renameFrom.value) {
    connectionsStore.showGlobalToast(t('keyTree.batchRenameSame'), false)
    return
  }
  closeMoreMenu()
  const result = await workspaceStore.batchRenameSelectedPrefix(renameFrom.value, nextPrefix)
  if (result?.success) {
    connectionsStore.showGlobalToast(t('keyTree.batchRenameOk').replace('{n}', result.affected), true)
  } else {
    connectionsStore.showGlobalToast(result?.message || t('keyTree.batchRenameFailed'), false)
  }
}

async function copyToDBInput() {
  const db = Number.parseInt(String(copyDbInput.value || '').trim(), 10)
  if (!Number.isInteger(db) || db < 0 || db >= dbCount.value) {
    connectionsStore.showGlobalToast(t('keyTree.copyDbInvalid'), false)
    return
  }
  copyDbInput.value = ''
  await copyToDB(db)
}

async function copyToDB(db) {
  closeMoreMenu()
  const keys = [...workspaceStore.selectedKeys]
  if (!keys.length) return
  try {
    const result = await copyKeysToDatabase(workspaceStore.activeConnID, keys, db, false)
    connectionsStore.showGlobalToast(result?.message || t('keyTree.copyFailed'), !!result?.success)
  } catch (e) {
    connectionsStore.showGlobalToast(e?.message || String(e), false)
  }
}

async function exportSelected(format) {
  const keys = [...workspaceStore.selectedKeys]
  if (!keys.length) return
  try {
    const path = await exportKeysToFile(workspaceStore.activeConnID, keys, format)
    if (!path) return
    connectionsStore.showGlobalToast(t('keyTree.exportOk').replace('{path}', path), true)
  } catch (e) {
    connectionsStore.showGlobalToast(e?.message || String(e), false)
  }
}

async function copyTo(target) {
  closeMoreMenu()
  const keys = [...workspaceStore.selectedKeys]
  if (!keys.length) return
  try {
    const result = await copyKeysToConnection(workspaceStore.activeConnID, target.id, keys, false)
    if (result?.success) {
      // 目标连接的缓存里没有新数据，切过去时重新加载
      workspaceStore.invalidateConnState(target.id)
    }
    connectionsStore.showGlobalToast(result?.message || t('keyTree.copyFailed'), !!result?.success)
  } catch (e) {
    connectionsStore.showGlobalToast(e?.message || String(e), false)
  }
}

// 菜单打开期间接管 Esc
let disposeMenuEscape = null
watch(() => moreMenu.value.visible, (visible) => {
  if (visible) {
    disposeMenuEscape = registerEscapeHandler(closeMoreMenu)
    return
  }
  if (disposeMenuEscape) {
    disposeMenuEscape()
    disposeMenuEscape = null
  }
})

onBeforeUnmount(() => {
  if (disposeMenuEscape) disposeMenuEscape()
  document.removeEventListener('click', closeMoreMenu)
})
document.addEventListener('click', closeMoreMenu)
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

// 删除 / TTL 都走菜单内的二次确认，不用浏览器原生 confirm/prompt（WebView 里不可用）
function openBatchDelete(event) {
  if (!workspaceStore.selectedKeys.length) return
  const rect = event.currentTarget.getBoundingClientRect()
  moreMenu.value = { visible: true, x: rect.left, y: rect.top - 8, mode: 'confirmDelete' }
}

function openBatchTTL(event) {
  if (!workspaceStore.selectedKeys.length) return
  const rect = event.currentTarget.getBoundingClientRect()
  ttlInput.value = '3600'
  moreMenu.value = { visible: true, x: rect.left, y: rect.top - 8, mode: 'ttl' }
}

async function confirmBatchDelete() {
  const count = workspaceStore.selectedKeys.length
  closeMoreMenu()
  if (!count) return
  const result = await workspaceStore.batchDeleteSelected()
  if (result?.success) {
    connectionsStore.showGlobalToast(t('keyTree.batchDeleteOk').replace('{n}', result.affected), true)
  } else {
    connectionsStore.showGlobalToast(result?.message || t('keyTree.batchDelete'), false)
  }
}

async function applyBatchTTL() {
  const ttl = Number(String(ttlInput.value || '').trim())
  if (!Number.isFinite(ttl) || ttl === 0 || ttl < -1) {
    connectionsStore.showGlobalToast(t('keyTree.batchTTLInvalid'), false)
    return
  }
  closeMoreMenu()
  const result = await workspaceStore.batchSetTTLSelected(Math.trunc(ttl))
  if (result?.success) {
    connectionsStore.showGlobalToast(t('keyTree.batchTTLOk').replace('{n}', result.affected), true)
  } else {
    connectionsStore.showGlobalToast(result?.message || t('keyTree.batchTTL'), false)
  }
}

// 选中 key 的公共前缀（按最后一个 : 截断，保证 key 名不被吃掉）
function commonKeyPrefix(keys) {
  if (!keys.length) return ''
  let prefix = keys[0]
  for (const key of keys.slice(1)) {
    let i = 0
    while (i < prefix.length && i < key.length && prefix[i] === key[i]) i++
    prefix = prefix.slice(0, i)
    if (!prefix) break
  }
  const idx = prefix.lastIndexOf(':')
  return idx >= 0 ? prefix.slice(0, idx + 1) : prefix
}

async function importKeys() {
  if (readOnly.value || !workspaceStore.activeConnID) return
  try {
    const result = await importKeysFromFile(workspaceStore.activeConnID, false)
    if (result?.message) {
      connectionsStore.showGlobalToast(result.message, !!result.success)
    }
    if (result?.affected > 0) {
      // 导入完成后刷新当前列表，新 key 才会出现
      const pattern = workspaceStore.activeSession?.pattern || '*'
      await workspaceStore.search(pattern)
    }
  } catch (e) {
    connectionsStore.showGlobalToast(e?.message || String(e), false)
  }
}

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

/* 加载更多：与同栏图标按钮一致的方形小按钮 */
.btn-load-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  min-width: 18px;
  height: 18px;
  padding: 0;
  background: transparent;
  color: #94a3b8;
  border: 1px solid rgba(203, 213, 225, 0.95);
  border-radius: 5px;
  cursor: pointer;
  transition: color 0.16s ease, background 0.16s ease, border-color 0.16s ease;
}
.btn-load-more svg {
  width: 11px;
  height: 11px;
  display: block;
}
.btn-load-more:hover:not(:disabled) {
  color: #2563eb;
  border-color: #93c5fd;
  background: rgba(239, 246, 255, 0.9);
}
.btn-load-more.loading svg {
  opacity: 0.4;
  animation: load-more-pulse 1s ease-in-out infinite;
}
@keyframes load-more-pulse {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(2px); }
}
.btn-load-more:disabled {
  color: #cbd5e1;
  border-color: rgba(226, 232, 240, 0.9);
  cursor: not-allowed;
  background: transparent;
}
.key-tree.theme-dark .btn-load-more {
  color: #94a3b8;
  border-color: rgba(71, 85, 105, 0.9);
}
.key-tree.theme-dark .btn-load-more:hover:not(:disabled) {
  color: #bfdbfe;
  border-color: rgba(59, 130, 246, 0.7);
  background: rgba(30, 58, 95, 0.6);
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
.tree-bulk-btn.active {
  background: rgba(96, 165, 250, 0.18);
  color: #2563eb;
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
.batch-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 7px 10px;
  margin: 0 8px 6px;
  border-radius: 10px;
  border: 1px solid rgba(147, 197, 253, 0.6);
  background: rgba(239, 246, 255, 0.9);
  font-size: 12px;
  color: #1e40af;
}
.shortcut-block {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 8px 6px;
  border-bottom: 1px dashed rgba(203, 213, 225, 0.9);
  margin-bottom: 4px;
  max-height: 168px;
  overflow-y: auto;
}
.shortcut-title {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #94a3b8;
  padding: 4px 4px 2px;
  text-transform: uppercase;
}
.shortcut-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 6px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  color: #334155;
  min-width: 0;
}
.shortcut-row:hover { background: rgba(239, 246, 255, 0.9); }
.shortcut-star {
  flex: 0 0 12px;
  font-size: 11px;
  color: #cbd5e1;
  line-height: 1;
}
.shortcut-star.active { color: #f59e0b; }
.shortcut-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.key-tree.theme-dark .shortcut-block { border-bottom-color: rgba(51, 65, 85, 0.9); }
.key-tree.theme-dark .shortcut-row { color: #cbd5e1; }
.key-tree.theme-dark .shortcut-row:hover { background: rgba(30, 41, 59, 0.9); }
.batch-info { flex: 1; min-width: 0; font-weight: 600; }
.batch-readonly { font-size: 12px; }
.more-menu-backdrop {
  position: fixed;
  inset: 0;
  z-index: 2400;
  background: transparent;
}
.more-menu {
  position: fixed;
  transform: translateY(-100%);
  min-width: 176px;
  padding: 5px 0;
  border-radius: 10px;
  border: 1px solid rgba(214, 224, 236, 0.9);
  background: #fff;
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.18);
}
.more-menu.theme-dark {
  border-color: rgba(71, 85, 105, 0.92);
  background: rgba(15, 23, 42, 0.98);
}
.more-item {
  display: block;
  width: 100%;
  padding: 7px 14px;
  border: none;
  background: transparent;
  color: #334155;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}
.more-item:hover:not(:disabled) { background: rgba(239, 246, 255, 0.9); }
.more-item:disabled { opacity: 0.45; cursor: default; }
.more-menu.theme-dark .more-item { color: #cbd5e1; }
.more-menu.theme-dark .more-item:hover:not(:disabled) { background: rgba(30, 41, 59, 0.9); }
.more-divider { height: 1px; margin: 4px 0; background: rgba(226, 232, 240, 0.9); }
.more-menu.theme-dark .more-divider { background: rgba(51, 65, 85, 0.9); }
.more-title {
  padding: 4px 14px;
  font-size: 11px;
  color: #94a3b8;
}
.more-subtitle {
  padding: 6px 14px 2px;
  font-size: 11px;
  color: #94a3b8;
}
.more-input {
  display: block;
  width: calc(100% - 28px);
  margin: 6px 14px;
  height: 28px;
  padding: 0 8px;
  border-radius: 7px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #1e293b;
  font-size: 12px;
  outline: none;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.more-input:focus { border-color: #60a5fa; box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.18); }
.more-actions { display: flex; gap: 8px; padding: 2px 14px 8px; }
.more-btn.danger {
  border-color: rgba(248, 113, 113, 0.9);
  background: rgba(254, 226, 226, 0.9);
  color: #b91c1c;
}
.more-menu.theme-dark .more-btn.danger {
  border-color: rgba(185, 28, 28, 0.7);
  background: rgba(127, 29, 29, 0.5);
  color: #fecaca;
}
.more-btn {
  height: 26px;
  padding: 0 12px;
  border-radius: 7px;
  border: 1px solid rgba(147, 197, 253, 0.9);
  background: rgba(219, 234, 254, 0.8);
  color: #1d4ed8;
  font-size: 12px;
  cursor: pointer;
}
.more-btn.ghost {
  border-color: rgba(203, 213, 225, 0.9);
  background: transparent;
  color: #64748b;
}
.db-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 4px;
  padding: 4px 12px 6px;
}
.db-grid.dense {
  grid-template-columns: repeat(6, 1fr);
  max-height: 168px;
  overflow-y: auto;
}
.db-input-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px 8px;
}
.db-input-row .more-input {
  width: auto;
  flex: 1;
  margin: 0;
}
.db-chip {
  height: 24px;
  border-radius: 6px;
  border: 1px solid rgba(203, 213, 225, 0.9);
  background: #fff;
  color: #334155;
  font-size: 11px;
  cursor: pointer;
}
.db-chip:hover { border-color: #93c5fd; color: #1d4ed8; background: rgba(239, 246, 255, 0.9); }
.more-menu.theme-dark .more-input {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.9);
  color: #e2e8f0;
}
.target-db {
  margin-left: 4px;
  color: #94a3b8;
  font-variant-numeric: tabular-nums;
}
.more-menu.theme-dark .target-db { color: #64748b; }
.more-menu.theme-dark .db-chip {
  border-color: rgba(71, 85, 105, 0.9);
  background: rgba(30, 41, 59, 0.9);
  color: #cbd5e1;
}
.more-menu.theme-dark .db-chip:hover { border-color: rgba(59, 130, 246, 0.8); color: #bfdbfe; }
.more-menu.theme-dark .more-btn {
  border-color: rgba(59, 130, 246, 0.6);
  background: rgba(30, 58, 95, 0.7);
  color: #bfdbfe;
}
.more-menu.theme-dark .more-btn.ghost {
  border-color: rgba(71, 85, 105, 0.9);
  background: transparent;
  color: #94a3b8;
}
.more-empty {
  padding: 8px 14px;
  font-size: 11px;
  color: #94a3b8;
}
.batch-btn {
  height: 24px;
  padding: 0 10px;
  border-radius: 7px;
  border: 1px solid rgba(147, 197, 253, 0.8);
  background: #fff;
  color: #1d4ed8;
  font-size: 12px;
  cursor: pointer;
}
.batch-btn:hover:not(:disabled) { background: rgba(219, 234, 254, 0.9); }
.batch-btn:disabled { opacity: 0.5; cursor: default; }
.batch-btn.ghost { background: transparent; color: #64748b; border-color: rgba(203, 213, 225, 0.9); }
.key-tree.theme-dark .batch-bar {
  border-color: rgba(59, 130, 246, 0.5);
  background: rgba(30, 58, 95, 0.55);
  color: #bfdbfe;
}
.key-tree.theme-dark .batch-btn {
  border-color: rgba(59, 130, 246, 0.55);
  background: rgba(15, 23, 42, 0.8);
  color: #bfdbfe;
}
.key-tree.theme-dark .batch-btn.ghost { color: #94a3b8; border-color: rgba(71, 85, 105, 0.9); }
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
