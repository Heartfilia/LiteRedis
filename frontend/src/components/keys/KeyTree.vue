<template>
  <div class="key-tree" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <div v-if="!activeConnID" class="empty-state">
      {{ t('keyTree.selectConn') }}
    </div>
    <template v-else>
      <KeySearchBar />
      <!-- SearchTabs 只在单session模式显示 -->
      <SearchTabs v-if="!workspaceStore.keepPrevSearch && !showClusterEmptyHint" />

      <div
        v-if="favoriteKeys.length || recentKeyList.length"
        class="shortcut-block"
        :class="{ 'is-collapsed': shortcutsCollapsed }"
      >
        <div class="shortcut-head">
          <span class="shortcut-summary">
            <span
              v-for="part in shortcutSummaryParts"
              :key="part.key"
              class="shortcut-summary-part"
            >{{ part.label }}</span>
          </span>
          <button
            type="button"
            class="shortcut-toggle"
            :class="{ 'is-collapsed': shortcutsCollapsed }"
            :title="shortcutsCollapsed ? t('keyTree.expandShortcut') : t('keyTree.collapseShortcut')"
            @click="shortcutsCollapsed = !shortcutsCollapsed"
          >
            <span class="shortcut-toggle-label">{{ t('keyTree.expandShortcut') }}</span>
            <svg viewBox="0 0 16 16" fill="none" aria-hidden="true" :class="{ flipped: !shortcutsCollapsed }">
              <path d="M4 6.2 8 10.2l4-4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </button>
        </div>
        <div class="shortcut-collapse" :class="{ 'is-open': !shortcutsCollapsed }">
        <div class="shortcut-collapse-inner">
        <div class="shortcut-body">
        <div v-if="favoriteKeys.length" class="shortcut-group">
          <div
            v-for="item in favoriteKeys"
            :key="'fav-' + item"
            v-kbd-row="{ tabindex: 0 }"
            :class="['shortcut-row', { active: workspaceStore.selectedKey === item }]"
            :title="item"
            @click="workspaceStore.selectKey(item)"
          >
            <span class="shortcut-star active" @click.stop="workspaceStore.toggleFavorite(item)">★</span>
            <span class="shortcut-name">{{ item }}</span>
          </div>
        </div>
        <div v-if="recentKeyList.length" class="shortcut-group">
          <div
            v-for="item in recentKeyList"
            :key="'recent-' + item"
            v-kbd-row="{ tabindex: 0 }"
            :class="['shortcut-row', { active: workspaceStore.selectedKey === item }]"
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
          <div v-else class="merged-scroll" tabindex="0" data-kb-container @keydown="onTreeKeydown">
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
                <KeyTreeRow
                  v-for="row in mergedRows(sess)"
                  :key="sess.id + '|' + row.node.fullPath"
                  :node="row.node"
                  :depth="row.depth"
                  :index="row.index"
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
          <div v-else ref="treeScrollRef" class="tree-scroll" tabindex="0" data-kb-container @keydown="onTreeKeydown">
            <div v-if="treeTopPad" class="tree-pad" :style="{ height: treeTopPad + 'px' }" aria-hidden="true" />
            <KeyTreeRow
              v-for="row in treeWindowRows"
              :key="row.node.fullPath"
              :node="row.node"
              :depth="row.depth"
              :index="row.index"
            />
            <div v-if="treeBottomPad" class="tree-pad" :style="{ height: treeBottomPad + 'px' }" aria-hidden="true" />
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
      <div ref="dbBarRef" class="db-bar" :class="{ 'bar-narrow': barNarrow }">
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
          class="more-menu pop-in-up"
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
import { computed, nextTick, onBeforeUnmount, onMounted, onUpdated, ref, watch } from 'vue'
import { moveRovingFocus, revealRow } from '../../utils/keyboard.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import KeySearchBar from './KeySearchBar.vue'
import SearchTabs from './SearchTabs.vue'
import KeyTreeRow from './KeyTreeRow.vue'
import { buildKeyTree, findKeyAncestors, flattenVisibleRows } from '../../utils/keyTree.js'
import { useRowVirtualizer } from '../../utils/virtualList.js'
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

// —— 树窗口化 ——
// 展开状态在 store 里，按当前展开情况把树压平，只渲染可视区附近的行
const treeScrollRef = ref(null)
function expansionPredicate(node, depth) {
  return workspaceStore.isNodeExpanded(node.fullPath, depth)
}
const treeVisibleRows = computed(() =>
  flattenVisibleRows(session.value?.treeData || [], expansionPredicate)
)
const treeVirtualizer = useRowVirtualizer({
  containerRef: treeScrollRef,
  count: computed(() => treeVisibleRows.value.length),
  threshold: 150,
  overscan: 10,
})
const treeTopPad = computed(() => treeVirtualizer.topPad.value)
const treeBottomPad = computed(() => treeVirtualizer.bottomPad.value)
const treeWindowRows = computed(() => {
  const rows = treeVisibleRows.value
  const start = treeVirtualizer.start.value
  const end = treeVirtualizer.end.value
  const out = []
  for (let i = start; i < end; i++) {
    out.push({ ...rows[i], index: i })
  }
  return out
})

// 合并搜索模式不窗口化，但同样按展开状态压平
function mergedRows(source) {
  const rows = flattenVisibleRows(source?.treeData || [], expansionPredicate)
  return rows.map((row, index) => ({ ...row, index }))
}

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

// 底栏「加载更多」居中策略：宽度够时落在整条底栏中线；
// 面板太窄、左侧按钮组会越过中线时，改为并入左侧图标组（间距 2px）
const dbBarRef = ref(null)
const barNarrow = ref(false)
// 「收藏 / 最近访问」板块的折叠状态：不做持久化，切连接或切 DB 时恢复展开
const shortcutsCollapsed = ref(false)
// 顶栏摘要：收藏、最近访问各有多少条，谁有显示谁（展开和收起状态一致）
// 分成独立元素渲染，两段之间的间距由 CSS 控制，不依赖会被折叠的空格
const shortcutSummaryParts = computed(() => {
  const parts = []
  if (favoriteKeys.value.length) {
    parts.push({ key: 'favorites', label: t('keyTree.favorites') + ' · ' + favoriteKeys.value.length })
  }
  if (recentKeyList.value.length) {
    parts.push({ key: 'recent', label: t('keyTree.recent') + ' · ' + recentKeyList.value.length })
  }
  return parts
})
watch([activeConnID, () => workspaceStore.currentDB], () => {
  shortcutsCollapsed.value = false
})
let dbBarObserver = null

function syncDbBarNarrow() {
  const bar = dbBarRef.value
  if (!bar) return
  const inner = bar.querySelector('.db-bar-left-inner')
  const leftWidth = inner ? inner.getBoundingClientRect().width : 0
  // 居中需要：底栏宽度 >= 2 * 左侧组宽度 + 左右内边距 24 + 两处间距 16 + 按钮 22
  barNarrow.value = bar.clientWidth > 0 && bar.clientWidth < 2 * leftWidth + 62
}

let dbBarObservedEl = null
function ensureDbBarObserver() {
  const bar = dbBarRef.value
  if (!bar || typeof ResizeObserver === 'undefined' || dbBarObservedEl === bar) return
  if (dbBarObserver) dbBarObserver.disconnect()
  dbBarObserver = new ResizeObserver(syncDbBarNarrow)
  dbBarObserver.observe(bar)
  dbBarObservedEl = bar
}

function syncDbBarLayout() {
  ensureDbBarObserver()
  syncDbBarNarrow()
}

onMounted(syncDbBarLayout)
onUpdated(syncDbBarLayout)

// 树里用 ↑/↓ 在行之间移动焦点，Enter 打开；Tab 只停在树容器上一次
function onTreeKeydown(event) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
  // 合并搜索分组没有窗口化，直接按 DOM 顺序移动
  if (workspaceStore.keepPrevSearch) {
    moveRovingFocus(event, event.currentTarget, '.node-row')
    return
  }
  const rows = treeVisibleRows.value
  if (!rows.length) return
  event.preventDefault()
  const focusedRow = document.activeElement && document.activeElement.closest
    ? document.activeElement.closest('.tree-row')
    : null
  const current = focusedRow ? Number(focusedRow.dataset.rowIndex) : -1
  let target = current
  if (event.key === 'ArrowDown') target = current < 0 ? 0 : Math.min(rows.length - 1, current + 1)
  else if (event.key === 'ArrowUp') target = current <= 0 ? 0 : current - 1
  else if (event.key === 'Home') target = 0
  else target = rows.length - 1
  focusTreeRow(target)
}

// 窗口化后目标行可能还没渲染：先把滚动位置算到位，渲染出来再聚焦
async function focusTreeRow(index) {
  const container = treeScrollRef.value
  if (!container) return
  const rowHeight = measureTreeRowHeight(container)
  const top = index * rowHeight
  const bottom = top + rowHeight
  if (top < container.scrollTop) {
    container.scrollTop = top
  } else if (bottom > container.scrollTop + container.clientHeight) {
    container.scrollTop = bottom - container.clientHeight
  }
  // 目标行可能还在窗口外，先把滚动位置同步给窗口，再等 DOM 渲染
  treeVirtualizer.syncScroll()
  await nextTick()
  const row = container.querySelector(`.tree-row[data-row-index="${index}"] .node-row`)
  if (!row) return
  try {
    row.focus({ preventScroll: true })
  } catch (e) {
    row.focus()
  }
  revealRow(row)
}

function measureTreeRowHeight(container) {
  const row = container.querySelector('[data-vrow]')
  const height = row ? row.getBoundingClientRect().height : 0
  return height > 4 ? height : 32
}

// 从命令面板 / 收藏 / 最近访问选中 key 时，展开父级目录并把它滚到可视区
async function revealSelectedKey(key) {
  if (!key) return
  if (workspaceStore.keepPrevSearch) return
  const ancestors = findKeyAncestors(session.value?.treeData || [], key)
  if (!ancestors) return
  for (const path of ancestors) {
    workspaceStore.setNodeExpanded(path, true)
  }
  await nextTick()
  const index = treeVisibleRows.value.findIndex((row) => row.node.isLeaf && row.node.fullPath === key)
  if (index < 0) return
  scrollTreeToIndex(index)
}

function scrollTreeToIndex(index) {
  const container = treeScrollRef.value
  if (!container) return
  // 已经在可视区里就不用动，避免点自己的行时轻微跳一下
  const rendered = container.querySelector(`.tree-row[data-row-index="${index}"]`)
  if (rendered) {
    const box = container.getBoundingClientRect()
    const rect = rendered.getBoundingClientRect()
    if (rect.top >= box.top && rect.bottom <= box.bottom) return
  }
  const rowHeight = measureTreeRowHeight(container)
  const top = index * rowHeight
  const bottom = top + rowHeight
  if (top < container.scrollTop) {
    container.scrollTop = top
  } else if (bottom > container.scrollTop + container.clientHeight) {
    container.scrollTop = bottom - container.clientHeight
  }
  treeVirtualizer.syncScroll()
}

watch(() => workspaceStore.selectedKey, (key) => {
  revealSelectedKey(key)
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
  if (dbBarObserver) {
    dbBarObserver.disconnect()
    dbBarObserver = null
    dbBarObservedEl = null
  }
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
.loading::before {
  content: '';
  width: 16px;
  height: 16px;
  margin-bottom: 8px;
  border-radius: 50%;
  border: 2px solid rgba(148, 163, 184, 0.35);
  border-top-color: #60a5fa;
  animation: loadingSpin 720ms linear infinite;
}
@keyframes loadingSpin {
  to { transform: rotate(360deg); }
}
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
  border-radius: var(--radius-lg);
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
  border-radius: var(--radius-pill);
  transition: background var(--motion-base) ease, color var(--motion-base) ease, transform var(--motion-base) ease;
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

/* 加载更多：与右侧 key 内容面板底部按钮保持一致 */
.btn-load-more {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  min-width: 22px;
  height: 22px;
  aspect-ratio: 1 / 1;
  box-sizing: border-box;
  padding: 0;
  background: transparent;
  color: #64748b;
  border: 1px solid rgba(203, 213, 225, 0.95);
  border-radius: var(--radius-sm);
  cursor: pointer;
  flex: 0 0 auto;
  transition: color var(--motion-base) ease, background var(--motion-base) ease, border-color var(--motion-base) ease;
}
.btn-load-more svg {
  width: 12px;
  height: 12px;
  display: block;
}
.btn-load-more:hover:not(:disabled) {
  background: #eff6ff;
  color: #2563eb;
  border-color: #93c5fd;
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
  color: #9ca3af;
  border-color: #d1d5db;
  cursor: not-allowed;
  background: #f9fafb;
  box-shadow: none;
}
.key-tree.theme-dark .btn-load-more {
  background: transparent;
  color: #94a3b8;
  border-color: rgba(71, 85, 105, 0.95);
  box-shadow: none;
}
.key-tree.theme-dark .btn-load-more:hover:not(:disabled) {
  background: rgba(30, 58, 95, 0.6);
  color: #bfdbfe;
  border-color: rgba(59, 130, 246, 0.7);
}
.load-more-hint {
  font-size: 12px;
  color: #9ca3af;
}

.db-bar {
  display: flex;
  align-items: center;
  gap: 8px;
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
  flex: 1 1 0;
  min-width: min-content;
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
  flex: 0 0 auto;
  height: 100%;
}
.db-bar-right {
  justify-content: flex-end;
  min-width: 0;
}
/* 窄面板：左侧按钮组会越过中线，加载更多改为并入左侧图标组，不再单独居中 */
.db-bar.bar-narrow .db-bar-left {
  flex: 0 0 auto;
}
.db-bar.bar-narrow .db-bar-center {
  margin-left: -6px;
}
.db-bar.bar-narrow .db-bar-right {
  flex: 1 1 auto;
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
  transition: border-color var(--motion-base) ease, box-shadow var(--motion-base) ease;
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
  border-radius: var(--radius-sm);
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  flex-shrink: 0;
  transition: background var(--motion-base) ease, color var(--motion-base) ease, opacity var(--motion-base) ease;
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
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 8px 6px;
  border-bottom: 1px dashed rgba(203, 213, 225, 0.9);
  margin-bottom: 4px;
}
/* 只让内容区滚动，右上角的收起按钮始终留在原位 */
.shortcut-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 148px;
  overflow-y: auto;
}
/* 收起态不改 padding，避免展开/收起时高度额外跳一下 */
/* 顶栏：收起时左侧显示统计标题，右侧是「展开 ⌄」；展开时只有右上角一个 ⌃ */
.shortcut-head {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 16px;
  flex-shrink: 0;
}
/* 顶栏摘要：展开/收起都显示「收藏 · N   最近访问 · M」 */
.shortcut-summary {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.key-tree.theme-dark .shortcut-summary {
  color: #94a3b8;
}
/* 收藏与最近访问两段之间的间隔 */
.shortcut-summary-part + .shortcut-summary-part {
  margin-left: 6px;
}
/* 收藏与最近访问之间用一条细虚线分开（标题已经合并到顶栏） */
.shortcut-group + .shortcut-group {
  margin-top: 3px;
  padding-top: 3px;
  border-top: 1px dashed rgba(203, 213, 225, 0.85);
}
.key-tree.theme-dark .shortcut-group + .shortcut-group {
  border-top-color: rgba(51, 65, 85, 0.9);
}
.shortcut-toggle {
  margin-left: auto;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 0;
  height: 16px;
  padding: 0 4px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  transition: color var(--motion-fast) var(--ease-out), background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out);
}
/* 「展开」文字常驻 DOM，用宽度+透明度补间，避免切换瞬间整块按钮跳一下 */
.shortcut-toggle-label {
  flex: 0 0 auto;
  white-space: nowrap;
  max-width: 0;
  margin-right: 0;
  opacity: 0;
  overflow: hidden;
  transition: max-width var(--motion-base) var(--ease-out), margin-right var(--motion-base) var(--ease-out), opacity var(--motion-base) var(--ease-out);
}
.shortcut-toggle.is-collapsed .shortcut-toggle-label {
  max-width: 48px;
  margin-right: 3px;
  opacity: 1;
}
.shortcut-toggle svg {
  width: 12px;
  height: 12px;
  display: block;
  flex: 0 0 auto;
  transition: transform var(--motion-fast) var(--ease-out);
}
/* 展开态箭头翻转朝上，收起态朝下 */
.shortcut-toggle svg.flipped {
  transform: rotate(180deg);
}
.shortcut-toggle:hover {
  color: #475569;
  background: rgba(241, 245, 249, 0.95);
  border-color: rgba(203, 213, 225, 0.9);
}
.shortcut-toggle.is-collapsed {
  color: #2563eb;
}
.shortcut-toggle.is-collapsed:hover {
  color: #1d4ed8;
  background: rgba(239, 246, 255, 0.95);
  border-color: rgba(191, 219, 254, 0.9);
}
.key-tree.theme-dark .shortcut-toggle:hover {
  color: #e2e8f0;
  background: rgba(30, 41, 59, 0.95);
  border-color: rgba(71, 85, 105, 0.95);
}
.key-tree.theme-dark .shortcut-toggle.is-collapsed {
  color: #93c5fd;
}
.key-tree.theme-dark .shortcut-toggle.is-collapsed:hover {
  color: #dbeafe;
  background: rgba(30, 64, 175, 0.3);
  border-color: rgba(96, 165, 250, 0.5);
}
/* 展开/收起用 grid-template-rows 补间，内容高度自适应且不会突跳 */
.shortcut-collapse {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows var(--motion-base) var(--ease-out);
}
.shortcut-collapse.is-open {
  grid-template-rows: 1fr;
}
.shortcut-collapse-inner {
  overflow: hidden;
  min-height: 0;
}
/* 当前正在查看的 key 在列表里高亮 */
.shortcut-row.active {
  background: rgba(219, 234, 254, 0.78);
  color: #1d4ed8;
}
.key-tree.theme-dark .shortcut-row.active {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
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
  border-radius: var(--radius-sm);
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
  border-radius: var(--radius-md);
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
  border-radius: var(--radius-md);
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
  border-radius: var(--radius-sm);
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
  border-radius: var(--radius-md);
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
