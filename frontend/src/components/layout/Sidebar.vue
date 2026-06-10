<template>
  <div
    class="sidebar"
    :class="{
      collapsed: sidebarCollapsed,
      'theme-dark': resolvedThemeMode === 'dark',
      'theme-light': resolvedThemeMode !== 'dark',
    }"
  >
    <div class="sidebar-layers">
      <!-- 折叠状态：图标列表 + 底部展开按钮 -->
      <div class="sidebar-layer sidebar-collapsed-layer" :class="{ visible: sidebarCollapsed }">
        <div class="sidebar-collapsed-bar">
          <div class="collapsed-icons">
            <div
              v-for="conn in connectedConnections"
              :key="conn.id"
              class="collapsed-conn-icon"
              :class="{ active: activeConnID === conn.id }"
              :style="{ background: connColor(conn) }"
              :title="conn.name || conn.host"
              @click="handleConnect(conn)"
            >
              {{ connInitial(conn) }}
            </div>
          </div>
          <button class="btn-expand-bottom" :title="t('sidebar.expand')" @click="sidebarCollapsed = false">
            <span class="btn-expand-glyph" aria-hidden="true">▶</span>
          </button>
        </div>
      </div>

      <div class="sidebar-layer sidebar-expanded-layer" :class="{ visible: !sidebarCollapsed }">
      <!-- 连接列表（按分组） -->
      <div class="conn-list">
        <!-- 未分组连接 -->
        <div
          class="ungrouped-zone"
          :class="{
            'drag-over': isGroupDropPreview(''),
            'cross-group-preview': isCrossGroupDropPreview(''),
          }"
          @dragover.prevent="onDragOverGroup('', $event)"
          @dragleave="onDragLeaveContainer"
          @drop.prevent="onDropToGroup('', '')"
        >
        <div
          v-if="isGroupDropPreview('')"
          class="drop-group-hint"
          :class="{ 'cross-group-preview': isCrossGroupDropPreview('') }"
        >{{ groupDropHint('') }}</div>
        <div
          v-for="conn in (groupedConnections[''] || [])"
          :key="conn.id"
          :class="['conn-item', {
            active: activeConnID === conn.id,
            connecting: connectionsStore.isConnecting(conn.id),
            dragging: dragId === conn.id,
            'drop-before': isItemDropPreview(conn.id, 'before'),
            'drop-after': isItemDropPreview(conn.id, 'after'),
            'cross-group-preview': isItemCrossGroupPreview(conn.id),
          }]"
          draggable="true"
          @dragstart="onDragStart(conn.id)"
          @dragend="onDragEnd"
          @dragover.prevent="onDragOverItem('', conn.id, $event)"
          @drop.prevent="onDropToGroup('', conn.id)"
          @contextmenu.prevent="showCtxMenu($event, conn)"
          @mouseleave="cancelDelete()"
        >
          <div v-if="isItemDropPreview(conn.id, 'before')" class="drop-indicator before" />
          <div v-if="isItemDropPreview(conn.id, 'after')" class="drop-indicator after" />
          <div class="conn-main" @click="handleConnect(conn)">
            <span class="conn-avatar" :style="{ background: connColor(conn) }">{{ connInitial(conn) }}</span>
            <span :class="['conn-dot', connectionStateClass(conn.id)]" />
            <span class="conn-name">
              {{ conn.name || t('sidebar.unnamed') }}
              <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-inline">{{ t('sidebar.connecting') }}</span>
            </span>
          </div>
          <div class="conn-actions">
            <button
              v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
              class="btn-tiny btn-conn-action btn-disconnect"
              :title="t('sidebar.disconnect')"
              @click.stop="disconnectConn(conn.id)"
            >⊘</button>
            <button
              v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id) && !conn.is_cluster"
              class="btn-tiny btn-conn-action btn-overview"
              :title="t('sidebar.connectionOverview')"
              @click.stop="openOverview(conn)"
            >⌘</button>
            <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-spinner" />
            <button class="btn-tiny btn-conn-action" :title="t('sidebar.edit')" @click.stop="openEdit(conn)">✎</button>
            <InlineDeleteConfirm
              class="sidebar-delete-confirm"
              label="✕"
              :confirm-text="t('sidebar.confirmDelete')"
              :reset-token="`${conn.id}:${activeConnID || ''}`"
              :icon-only="true"
              @confirm="removeConnection(conn.id)"
            />
          </div>
        </div>
        </div>

        <!-- 命名分组 -->
        <div v-for="(conns, group) in namedGroups" :key="group" class="group-block">
          <div
            class="group-header"
            @click="toggleGroup(group)"
            :class="{
              'drag-over': isGroupDropPreview(group),
              'cross-group-preview': isCrossGroupDropPreview(group),
            }"
            @dragover.prevent="onDragOverGroup(group, $event)"
            @dragleave="onDragLeaveContainer"
            @drop.prevent="onDropToGroup(group, '')"
          >
            <span class="group-arrow">{{ collapsed[group] ? '▶' : '▼' }}</span>
            <span class="group-name">{{ group }}</span>
            <span class="group-count">{{ conns.length }}</span>
            <span
              v-if="isGroupDropPreview(group)"
              class="group-drop-hint"
              :class="{ 'cross-group-preview': isCrossGroupDropPreview(group) }"
            >{{ groupDropHint(group) }}</span>
          </div>
          <div v-if="!collapsed[group]">
            <div
              v-for="conn in conns"
              :key="conn.id"
              :class="['conn-item', 'grouped', {
                active: activeConnID === conn.id,
                connecting: connectionsStore.isConnecting(conn.id),
                dragging: dragId === conn.id,
                'drop-before': isItemDropPreview(conn.id, 'before'),
                'drop-after': isItemDropPreview(conn.id, 'after'),
                'cross-group-preview': isItemCrossGroupPreview(conn.id),
              }]"
              draggable="true"
              @dragstart="onDragStart(conn.id)"
              @dragend="onDragEnd"
              @dragover.prevent="onDragOverItem(group, conn.id, $event)"
              @drop.prevent="onDropToGroup(group, conn.id)"
              @contextmenu.prevent="showCtxMenu($event, conn)"
              @mouseleave="cancelDelete()"
            >
              <div v-if="isItemDropPreview(conn.id, 'before')" class="drop-indicator before" />
              <div v-if="isItemDropPreview(conn.id, 'after')" class="drop-indicator after" />
              <div class="conn-main" @click="handleConnect(conn)">
                <span class="conn-avatar" :style="{ background: connColor(conn) }">{{ connInitial(conn) }}</span>
                <span :class="['conn-dot', connectionStateClass(conn.id)]" />
                <span class="conn-name">
                  {{ conn.name || t('sidebar.unnamed') }}
                  <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-inline">{{ t('sidebar.connecting') }}</span>
                </span>
              </div>
              <div class="conn-actions">
                <button
                  v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
                  class="btn-tiny btn-conn-action btn-disconnect"
                  :title="t('sidebar.disconnect')"
                  @click.stop="disconnectConn(conn.id)"
                >⊘</button>
                <button
                  v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id) && !conn.is_cluster"
                  class="btn-tiny btn-conn-action btn-overview"
                  :title="t('sidebar.connectionOverview')"
                  @click.stop="openOverview(conn)"
                >⌘</button>
                <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-spinner" />
                <button class="btn-tiny btn-conn-action" :title="t('sidebar.edit')" @click.stop="openEdit(conn)">✎</button>
                <InlineDeleteConfirm
                  class="sidebar-delete-confirm"
                  label="✕"
                  :confirm-text="t('sidebar.confirmDelete')"
                  :reset-token="`${conn.id}:${activeConnID || ''}`"
                  :icon-only="true"
                  @confirm="removeConnection(conn.id)"
                />
              </div>
            </div>
          </div>
        </div>

        <div v-if="connectionsStore.connections.length === 0" class="empty-hint">
          {{ t('sidebar.emptyHint') }}
        </div>
      </div>

      <!-- 底部操作区 -->
      <div class="sidebar-footer">
        <div class="sidebar-links">
          <a class="sidebar-link-icon" title="GitHub" @click="openGitHub()">
            <span class="sidebar-link-glyph" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16" aria-hidden="true">
                <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"/>
              </svg>
            </span>
          </a>
          <a class="sidebar-link-icon sidebar-link-json" :title="t('sidebar.jsonParser')" @click="openJSONParser()">
            <span class="sidebar-link-glyph" aria-hidden="true">{}</span>
          </a>
        </div>
        <div class="footer-actions">
          <button
            class="btn-icon btn-theme"
            :title="settingsStore.themeMode === 'dark' ? '切换浅色主题' : '切换暗色主题'"
            @click="settingsStore.toggleTheme()"
          >
            <span class="btn-icon-glyph" aria-hidden="true">{{ settingsStore.themeMode === 'dark' ? '☾' : '☀' }}</span>
          </button>
          <button class="btn-icon" :title="t('sidebar.manageConn')" @click="openConnManager()"><span class="btn-icon-glyph" aria-hidden="true">＋</span></button>
          <button class="btn-icon btn-settings" :title="t('sidebar.settings')" @click="openSettings()"><span class="btn-icon-glyph" aria-hidden="true">⚙</span></button>
          <button
            class="btn-icon btn-collapse"
            :title="t('sidebar.collapse')"
            :disabled="!hasConnections"
            @click="hasConnections && (sidebarCollapsed = true)"
          ><span class="btn-icon-glyph" aria-hidden="true">◀</span></button>
        </div>
      </div>
      </div>
    </div>

    <!-- 右键菜单 -->
    <div
      v-if="ctxMenu.visible"
      class="ctx-menu"
      :style="{ top: ctxMenu.y + 'px', left: ctxMenu.x + 'px' }"
      @click.stop
    >
      <div class="ctx-item" @click="openEdit(ctxMenu.conn); ctxMenu.visible = false">✎ {{ t('sidebar.edit') }}</div>
      <div class="ctx-divider" />
      <div class="ctx-item ctx-danger" @click="removeConnection(ctxMenu.conn.id); ctxMenu.visible = false">✕ {{ t('sidebar.delete') }}</div>
    </div>

    <Teleport to="body">
      <Transition name="toast">
        <div v-if="toastMsg" class="sidebar-toast" :class="toastOk ? 'ok' : 'err'">{{ toastMsg }}</div>
      </Transition>
    </Teleport>

    <ConnectionOverviewModal
      v-if="overviewConn"
      :conn-id="overviewConn.id"
      :connection-name="overviewConn.name || t('sidebar.unnamed')"
      @close="overviewConn = null"
    />
  </div>
</template>

<script setup>
import { computed, ref, inject, onMounted, onBeforeUnmount, watch } from 'vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime.js'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import ConnectionOverviewModal from '../connections/ConnectionOverviewModal.vue'
import { formatDebugMessage } from '../../utils/debug.js'

const props = defineProps({
  themeMode: {
    type: String,
    default: 'light',
  },
})

const { t } = useI18n()

const connectionsStore = useConnectionsStore()
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const resolvedThemeMode = computed(() => props.themeMode || settingsStore.themeMode || 'light')

const openConnManager = inject('openConnManager')
const openSettings = inject('openSettings')

const activeConnID = computed(() => workspaceStore.activeConnID)
const activeConn = computed(() => connectionsStore.connections.find(c => c.id === activeConnID.value))

const groupedConnections = computed(() => connectionsStore.groupedConnections)
const namedGroups = computed(() => {
  const g = { ...groupedConnections.value }
  delete g['']
  return g
})

// 已连接的连接列表（用于折叠栏图标）
const connectedConnections = computed(() =>
  connectionsStore.connections.filter(c => connectionsStore.isConnected(c.id))
)
const hasConnections = computed(() => connectionsStore.connections.length > 0)
const overviewConn = ref(null)

function connectionStateClass(id) {
  if (connectionsStore.isConnecting(id)) return 'connecting'
  return connectionsStore.isConnected(id) ? 'connected' : 'disconnected'
}

// 连接图标颜色（淡雅色板，按 id 哈希取色）
const AVATAR_COLORS = [
  '#5c7f9e', '#6e8c6a', '#8a6a7a', '#7a6e8a', '#8a7a5a',
  '#5a7a7a', '#7a5a6a', '#6a7a5a', '#7a6a5a', '#5a6a7a',
  '#6a5a8a', '#7a8a6a',
]

function connColor(conn) {
  const customColor = (conn.iconColor || conn.icon_color || '').trim()
  if (/^#[0-9a-fA-F]{6}$/.test(customColor)) return customColor
  let hash = 0
  const s = conn.id || conn.name || conn.host || ''
  for (let i = 0; i < s.length; i++) hash = (hash * 31 + s.charCodeAt(i)) >>> 0
  return AVATAR_COLORS[hash % AVATAR_COLORS.length]
}

function connInitial(conn) {
  const s = conn.name || conn.host || '?'
  return s[0].toUpperCase()
}

// 折叠状态
const sidebarCollapsed = ref(false)
const toastMsg = ref('')
const toastOk = ref(false)
let toastTimer = null
let dragExpandTimer = null

watch(hasConnections, (value) => {
  if (!value) {
    sidebarCollapsed.value = false
  }
}, { immediate: true })

// 右键菜单
const ctxMenu = ref({ visible: false, x: 0, y: 0, conn: null })
function showCtxMenu(e, conn) {
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, conn }
}
function closeCtxMenu() { ctxMenu.value.visible = false }
onMounted(() => document.addEventListener('click', closeCtxMenu))
onBeforeUnmount(() => {
  document.removeEventListener('click', closeCtxMenu)
  if (toastTimer) clearTimeout(toastTimer)
  if (dragExpandTimer) clearTimeout(dragExpandTimer)
})

// 编辑连接（打开 ConnectionManager）
function openEdit(conn) {
  openConnManager(conn)
}

function openOverview(conn) {
  overviewConn.value = conn
}

function openGitHub() {
  BrowserOpenURL('https://github.com/Heartfilia/LiteRedis')
}

function openJSONParser() {
  BrowserOpenURL('https://json.litetools.top')
}

function showToast(message, ok = false) {
  toastMsg.value = message
  toastOk.value = ok
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toastMsg.value = ''
    toastTimer = null
  }, 3200)
}

function formatError(prefix, message) {
  return `${prefix}: ${formatDebugMessage(message, 'Unknown error')}`
}

// 分组折叠状态
const collapsed = ref({})
function toggleGroup(group) {
  collapsed.value[group] = !collapsed.value[group]
}

// 拖拽
const dragId = ref(null)
const dragOverGroup = ref(null)
const dragPreview = ref({ group: null, targetId: '', placement: 'before', mode: null })
function onDragStart(connId) {
  dragId.value = connId
  dragPreview.value = { group: null, targetId: '', placement: 'before', mode: null }
}
function onDragOverItem(group, targetId, event) {
  if (!dragId.value || dragId.value === targetId) return
  const rect = event.currentTarget.getBoundingClientRect()
  const placement = event.clientY - rect.top > rect.height / 2 ? 'after' : 'before'
  dragOverGroup.value = null
  dragPreview.value = { group, targetId, placement, mode: 'item' }
}
function onDragOverGroup(group) {
  if (!dragId.value) return
  if (dragExpandTimer) {
    clearTimeout(dragExpandTimer)
    dragExpandTimer = null
  }
  if (group && collapsed.value[group]) {
    dragExpandTimer = setTimeout(() => {
      collapsed.value[group] = false
      dragExpandTimer = null
    }, 420)
  }
  dragOverGroup.value = group
  dragPreview.value = { group, targetId: '', placement: 'after', mode: 'group' }
}
function onDragLeaveContainer(event) {
  if (event.currentTarget?.contains(event.relatedTarget)) return
  if (dragExpandTimer) {
    clearTimeout(dragExpandTimer)
    dragExpandTimer = null
  }
  dragOverGroup.value = null
  if (dragPreview.value.mode === 'group') {
    dragPreview.value = { group: null, targetId: '', placement: 'before', mode: null }
  }
}
function onDragEnd() {
  if (dragExpandTimer) {
    clearTimeout(dragExpandTimer)
    dragExpandTimer = null
  }
  dragOverGroup.value = null
  dragId.value = null
  dragPreview.value = { group: null, targetId: '', placement: 'before', mode: null }
}
function isItemDropPreview(targetId, placement) {
  return dragPreview.value.mode === 'item' &&
    dragPreview.value.targetId === targetId &&
    dragPreview.value.placement === placement
}
function isGroupDropPreview(group) {
  return dragPreview.value.mode === 'group' && dragPreview.value.group === group
}
function draggedConnectionGroup() {
  return connectionsStore.connections.find(conn => conn.id === dragId.value)?.group || ''
}
function isCrossGroupDropPreview(group) {
  return isGroupDropPreview(group) && draggedConnectionGroup() !== group
}
function isItemCrossGroupPreview(targetId) {
  if (dragPreview.value.mode !== 'item' || dragPreview.value.targetId !== targetId) return false
  const targetGroup = connectionsStore.connections.find(conn => conn.id === targetId)?.group || ''
  return draggedConnectionGroup() !== targetGroup
}
function groupDropHint(group) {
  return group ? `${t('connManager.moveToGroup')} ${group}` : t('sidebar.moveToUngrouped')
}
async function onDropToGroup(targetGroup, targetId = '') {
  const placement = dragPreview.value.targetId === targetId ? dragPreview.value.placement : 'before'
  dragOverGroup.value = null
  if (!dragId.value) return
  if (dragId.value !== targetId) {
    await connectionsStore.moveConnection(dragId.value, targetId, targetGroup, placement)
  }
  onDragEnd()
}

async function handleConnect(conn) {
  if (connectionsStore.isConnecting(conn.id)) return
  if (connectionsStore.isConnected(conn.id)) {
    // 已连接：重新激活，恢复该连接的历史状态（如有）
    const restored = workspaceStore.setActiveConn(conn.id, conn.name, conn.db || 0)
    if (restored) {
      // 恢复历史状态时不阻塞界面，总数后台刷新
      workspaceStore.fetchTotalKeys()
    } else {
      // Cluster 模式不默认扫 key，避免模糊查询
      await workspaceStore.switchDB(conn.db || 0)
      if (!conn.is_cluster) {
        await workspaceStore.search('*')
      }
      workspaceStore.fetchTotalKeys()
    }
    return
  }
  const result = await connectionsStore.connect(conn.id)
  if (result?.message === 'connecting') return
  if (result.success) {
    workspaceStore.setActiveConn(conn.id, conn.name, result.init_db || 0)
    if (!conn.is_cluster) {
      await workspaceStore.search('*')
    }
    workspaceStore.fetchTotalKeys()
  } else {
    showToast(formatError(t('sidebar.connectFailed'), result.message))
  }
}

async function removeConnection(id) {
  ctxMenu.value.visible = false
  const result = await connectionsStore.remove(id)
  if (result?.success) {
    if (activeConnID.value === id) {
      workspaceStore.setActiveConn(null, '')
    }
    delete workspaceStore.connStates[id]
    workspaceStore.clearSearchHistory(id)
  } else {
    showToast(formatError(t('sidebar.deleteFailed'), result?.message))
  }
}

// 断开连接
async function disconnectConn(id) {
  await connectionsStore.disconnect(id)
  if (activeConnID.value === id) {
    workspaceStore.setActiveConn(null, '')
  }
  delete workspaceStore.connStates[id]
}
</script>

<style scoped>
.sidebar {
  width: 200px;
  min-width: 200px;
  margin: var(--app-shell-gap) 0 var(--app-shell-gap) var(--app-shell-gap);
  border-radius: var(--app-panel-radius);
  border: 1px solid var(--app-panel-border);
  background: var(--app-sidebar-bg);
  box-shadow: 0 18px 38px rgba(148, 163, 184, 0.16);
  backdrop-filter: blur(14px);
  color: var(--app-muted);
  display: flex;
  flex-direction: column;
  height: calc(100vh - (var(--app-shell-gap) * 2));
  overflow: hidden;
  position: relative;
  transition: width 0.2s ease, min-width 0.2s ease;
}
.sidebar.theme-dark {
  backdrop-filter: none;
  box-shadow: 0 18px 38px rgba(2, 6, 23, 0.32);
}
.sidebar.collapsed {
  width: 44px;
  min-width: 44px;
}
.sidebar-layers {
  position: relative;
  flex: 1;
  min-height: 0;
}
.sidebar-layer {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.22s ease, transform 0.22s ease;
  will-change: opacity, transform;
}
.sidebar-collapsed-layer {
  transform: translateX(-10px);
}
.sidebar-expanded-layer {
  transform: translateX(10px);
}
.sidebar-layer.visible {
  opacity: 1;
  transform: translateX(0);
  pointer-events: auto;
}

/* ===== 折叠栏 ===== */
.sidebar-collapsed-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
  padding: 8px 0 8px;
}
.collapsed-icons {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 4px 0;
  width: 100%;
}
.collapsed-icons::-webkit-scrollbar { display: none; }
.collapsed-conn-icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  flex-shrink: 0;
  transition: opacity 0.15s, box-shadow 0.15s;
  opacity: 0.72;
  user-select: none;
}
.collapsed-conn-icon:hover { opacity: 1; }
.collapsed-conn-icon.active {
  opacity: 1;
  box-shadow: 0 0 0 2px rgba(255, 255, 255, 0.96), 0 0 0 4px rgba(96, 165, 250, 0.32);
}
.sidebar.theme-dark .collapsed-conn-icon.active {
  box-shadow: 0 0 0 2px rgba(15, 23, 42, 0.98), 0 0 0 4px rgba(96, 165, 250, 0.34);
}
.btn-expand-bottom {
  background: rgba(255, 255, 255, 0.9);
  color: #64748b;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  width: 28px;
  height: 28px;
  font-size: 11px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 6px;
  box-shadow: 0 8px 16px rgba(148, 163, 184, 0.14);
}
.btn-expand-glyph {
  width: 12px;
  height: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  font-size: 10px;
  transform: translateX(0.5px);
}
.btn-expand-bottom:hover {
  background: #eff6ff;
  color: #2563eb;
  border-color: #93c5fd;
}
.sidebar.theme-dark .btn-expand-bottom {
  background: rgba(15, 23, 42, 0.92);
  color: #94a3b8;
  border-color: rgba(71, 85, 105, 0.96);
  box-shadow: 0 12px 22px rgba(2, 6, 23, 0.34);
}
.sidebar.theme-dark .btn-expand-bottom:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: rgba(96, 165, 250, 0.42);
}

/* ===== 展开状态 footer ===== */
.sidebar-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
  min-height: 36px;
  padding: 4px 12px 2px;
  box-sizing: border-box;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.65), rgba(241, 245, 249, 0.94));
  position: relative;
}
.sidebar-footer::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 2px;
  height: 1px;
  background: var(--app-panel-border);
}
.sidebar.theme-dark .sidebar-footer {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.98), rgba(11, 18, 32, 0.995));
}
.sidebar.theme-dark .sidebar-footer::before {
  background: rgba(51, 65, 85, 0.92);
}
.sidebar-links {
  display: flex;
  align-items: center;
  gap: 6px;
}
.sidebar-link-icon {
  color: #64748b;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  min-width: 22px;
  min-height: 22px;
  border-radius: 6px;
  transition: color 0.15s, background 0.15s;
  text-decoration: none;
  box-sizing: border-box;
  overflow: hidden;
}
.sidebar-link-glyph {
  width: 16px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  flex-shrink: 0;
  transform-origin: center center;
  transition: transform 0.16s ease;
}
.sidebar-link-glyph svg {
  display: block;
  width: 16px;
  height: 16px;
}
.sidebar-link-icon:hover {
  color: #2563eb;
  background: rgba(219, 234, 254, 0.76);
}
.sidebar-link-icon:hover .sidebar-link-glyph {
  transform: scale(1.08);
}
.sidebar.theme-dark .sidebar-link-icon {
  color: #9fb0c7;
  background: transparent;
  border: 1px solid transparent;
}
.sidebar.theme-dark .sidebar-link-icon:hover,
.sidebar.theme-dark .sidebar-link-icon:hover:focus,
.sidebar.theme-dark .sidebar-link-icon:hover:focus-visible,
.sidebar.theme-dark .sidebar-link-icon:hover:active {
  color: #e2e8f0;
  background: rgba(30, 41, 59, 0.98);
  border-color: rgba(51, 65, 85, 0.96);
}
.sidebar-link-icon:focus,
.sidebar-link-icon:focus-visible,
.sidebar-link-icon:active {
  color: #64748b;
  background: transparent;
  outline: none;
  box-shadow: none;
}
.sidebar-link-icon:hover:focus,
.sidebar-link-icon:hover:focus-visible,
.sidebar-link-icon:hover:active {
  color: #2563eb;
  background: rgba(219, 234, 254, 0.76);
}
.sidebar-link-json {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: -0.02em;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
}
.footer-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.btn-icon {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(241, 245, 249, 0.94));
  color: #64748b;
  border: 1px solid rgba(203, 213, 225, 0.94);
  border-radius: 8px;
  width: 24px; height: 24px;
  min-width: 24px; min-height: 24px;
  font-size: 14px; line-height: 1;
  cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  box-sizing: border-box;
  overflow: hidden;
  box-shadow: 0 6px 14px rgba(148, 163, 184, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.85);
  transition: background 0.15s, color 0.15s, border-color 0.15s, box-shadow 0.15s;
}
.btn-icon:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(239, 246, 255, 0.96));
  color: #2563eb;
  border-color: rgba(147, 197, 253, 0.92);
  box-shadow: 0 8px 18px rgba(191, 219, 254, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.92);
}
.btn-icon-glyph {
  width: 14px;
  height: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  transform-origin: center center;
  transition: transform 0.16s ease;
}
.btn-icon:hover .btn-icon-glyph {
  transform: scale(1.08);
}
.btn-settings {
  background: linear-gradient(180deg, rgba(250, 252, 255, 0.98), rgba(241, 245, 249, 0.94));
  color: #5b6b82;
}
.btn-settings:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(238, 246, 255, 0.98));
  color: #2563eb;
}
.btn-theme {
  background: linear-gradient(180deg, rgba(255, 252, 240, 0.98), rgba(254, 249, 195, 0.92));
  color: #ca8a04;
  box-shadow: 0 8px 18px rgba(250, 204, 21, 0.18), inset 0 1px 0 rgba(255, 255, 255, 0.9);
}
.btn-theme:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(254, 240, 138, 0.98));
  color: #a16207;
  box-shadow: 0 10px 22px rgba(250, 204, 21, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.96);
}
.btn-collapse {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(226, 232, 240, 0.92));
  color: #64748b;
}
.btn-collapse .btn-icon-glyph { font-size: 10px; }
.btn-collapse:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(241, 245, 249, 0.96));
  color: #334155;
}
.btn-icon:disabled {
  cursor: default;
  opacity: 0.46;
  color: #a8b4c5;
  border-color: rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.82), rgba(241, 245, 249, 0.78));
  box-shadow: none;
}
.btn-icon:disabled .btn-icon-glyph {
  transform: none !important;
}
.btn-icon:disabled:hover {
  color: #a8b4c5;
  border-color: rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.82), rgba(241, 245, 249, 0.78));
  box-shadow: none;
}
.sidebar.theme-dark .btn-icon {
  background: transparent;
  color: #a8b6c9;
  border-color: transparent;
  box-shadow: none;
}
.sidebar.theme-dark .btn-icon:hover {
  background: #1e293b;
  color: #e2e8f0;
  border-color: rgba(96, 165, 250, 0.38);
  box-shadow: none;
}
.sidebar.theme-dark .btn-settings {
  color: #cbd5e1;
}
.sidebar.theme-dark .btn-theme {
  background: transparent;
  color: #fbbf24;
  box-shadow: none;
}
.sidebar.theme-dark .btn-theme:hover {
  background: #243047;
  color: #fde68a;
  box-shadow: none;
}
.sidebar.theme-dark .btn-theme .btn-icon-glyph {
  text-shadow: 0 0 10px rgba(251, 191, 36, 0.22);
}
.sidebar.theme-dark .btn-collapse {
  color: #cbd5e1;
}
.sidebar.theme-dark .btn-icon:disabled,
.sidebar.theme-dark .btn-icon:disabled:hover {
  color: #475569;
  border-color: transparent;
  background: transparent;
}

/* ===== 连接列表 ===== */
.conn-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
  user-select: none;
  -webkit-user-select: none;
}
.conn-item {
  display: flex;
  align-items: center;
  padding: 5px 10px;
  cursor: pointer;
  border-radius: 8px;
  margin: 2px 8px;
  position: relative;
  overflow: visible;
  border: 1px solid transparent;
  transition: background 0.14s, border-color 0.14s, box-shadow 0.14s;
}
.conn-item.drop-before,
.conn-item.drop-after {
  background: rgba(78, 154, 241, 0.08);
}
.conn-item.cross-group-preview.drop-before,
.conn-item.cross-group-preview.drop-after {
  background: rgba(56, 189, 148, 0.12);
}
.conn-item.dragging {
  opacity: 0.42;
  transform: scale(0.985);
  background: rgba(78, 154, 241, 0.12);
  box-shadow: inset 0 0 0 1px rgba(120, 185, 255, 0.18);
}
.conn-item.dragging .conn-actions {
  display: none;
}
.drop-indicator {
  position: absolute;
  left: 10px;
  right: 10px;
  height: 0;
  pointer-events: none;
  z-index: 3;
}
.drop-indicator::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  height: 2px;
  background: #69b1ff;
  border-radius: 999px;
  box-shadow: 0 0 0 1px rgba(30, 42, 58, 0.45), 0 0 10px rgba(105, 177, 255, 0.35);
}
.conn-item.cross-group-preview .drop-indicator::before {
  background: #34d399;
  box-shadow: 0 0 0 1px rgba(16, 24, 39, 0.38), 0 0 10px rgba(52, 211, 153, 0.35);
}
.drop-indicator::after {
  content: '';
  position: absolute;
  left: -1px;
  top: -3px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #69b1ff;
  box-shadow: 0 0 0 2px #1e2a3a, 0 0 10px rgba(105, 177, 255, 0.45);
}
.conn-item.cross-group-preview .drop-indicator::after {
  background: #34d399;
  box-shadow: 0 0 0 2px #1e2a3a, 0 0 10px rgba(52, 211, 153, 0.45);
}
.drop-indicator.before { top: -1px; }
.drop-indicator.after { bottom: -1px; }
.drop-indicator.after::after { top: -3px; }
.conn-item.grouped { margin-left: 14px; margin-right: 6px; }
.conn-item:hover {
  background: rgba(255, 255, 255, 0.82);
  border-color: rgba(147, 197, 253, 0.32);
  box-shadow: 0 10px 18px rgba(191, 219, 254, 0.18);
}
.conn-item.active {
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.96), rgba(219, 234, 254, 0.92));
  border-color: rgba(96, 165, 250, 0.38);
  box-shadow: 0 10px 22px rgba(147, 197, 253, 0.22);
}
.sidebar.theme-dark .conn-item:hover {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(71, 85, 105, 0.9);
  box-shadow: 0 12px 20px rgba(2, 6, 23, 0.24);
}
.sidebar.theme-dark .conn-item.active {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(30, 64, 175, 0.14));
  border-color: rgba(96, 165, 250, 0.34);
  box-shadow: 0 14px 24px rgba(2, 6, 23, 0.3);
}
.conn-item.connecting::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(110deg, transparent 0%, rgba(96, 165, 250, 0.08) 35%, rgba(96, 165, 250, 0.18) 50%, rgba(96, 165, 250, 0.08) 65%, transparent 100%);
  transform: translateX(-100%);
  animation: connectingSweep 1.3s linear infinite;
  pointer-events: none;
}
.conn-main { flex: 1; display: flex; align-items: center; gap: 6px; min-width: 0; }

/* 连接图标（方形 avatar） */
.conn-avatar {
  width: 20px;
  height: 20px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 11px;
  font-weight: 700;
  flex-shrink: 0;
  letter-spacing: 0;
  user-select: none;
}
.conn-item.connecting .conn-avatar {
  animation: avatarFloat 0.9s ease-in-out infinite alternate;
}

.conn-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.conn-dot.connected { background: #4CAF50; }
.conn-dot.connecting {
  width: 8px;
  height: 8px;
  background: #60a5fa;
  box-shadow: 0 0 0 0 rgba(96, 165, 250, 0.6);
  animation: dotPulse 1.2s ease-out infinite;
}
.conn-dot.disconnected { background: #9e9e9e; }
.conn-name { font-size: 13px; color: #334155; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex: 1; min-width: 0; }
.sidebar.theme-dark .conn-dot.disconnected {
  background: #475569;
}
.sidebar.theme-dark .conn-name {
  color: #f1f5f9;
}
.sidebar.theme-dark .conn-item.active .conn-name {
  color: #ffffff;
}
.sidebar.theme-dark .conn-host,
.sidebar.theme-dark .group-header,
.sidebar.theme-dark .group-count,
.sidebar.theme-dark .empty-hint {
  color: #a8b6c9;
}
.connecting-inline {
  margin-left: 6px;
  font-size: 10px;
  color: #3b82f6;
  font-weight: 500;
}
.conn-actions { display: none; gap: 2px; }
.conn-item:hover .conn-actions { display: flex; }
.conn-item.connecting .conn-actions { display: flex; }
.btn-conn-action,
:deep(.sidebar-delete-confirm),
:deep(.sidebar-delete-confirm > .btn-tiny) {
  width: 22px;
  min-width: 22px;
  height: 22px;
  min-height: 22px;
  padding: 0;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 22px;
  line-height: 1;
}
.btn-conn-action {
  font-size: 11px;
}
.connecting-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid rgba(191, 219, 254, 0.35);
  border-top-color: #93c5fd;
  border-radius: 50%;
  animation: spin 0.75s linear infinite;
  flex-shrink: 0;
}
.btn-tiny {
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(203, 213, 225, 0.96);
  color: #64748b;
  border-radius: 6px;
  padding: 1px 5px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease;
}
.btn-tiny:hover {
  background: #eff6ff;
  border-color: #93c5fd;
  color: #2563eb;
}
.sidebar.theme-dark .btn-tiny {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(71, 85, 105, 0.96);
  color: #94a3b8;
}
.sidebar.theme-dark .btn-tiny:hover {
  background: rgba(30, 64, 175, 0.16);
  border-color: rgba(96, 165, 250, 0.38);
  color: #e2e8f0;
}
.btn-tiny.danger:hover { background: #e53e3e; border-color: #e53e3e; color: white; }
:deep(.sidebar-delete-confirm .btn-tiny.danger) {
  border-color: rgba(203, 213, 225, 0.96);
  color: #64748b;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), inset 0 0 0 1px rgba(255, 255, 255, 0.18);
}
.sidebar.theme-dark :deep(.sidebar-delete-confirm .btn-tiny.danger) {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(71, 85, 105, 0.96);
  color: #94a3b8;
  box-shadow: 0 6px 14px rgba(2, 6, 23, 0.24), inset 0 0 0 1px rgba(148, 163, 184, 0.04);
}
:deep(.sidebar-delete-confirm > .btn-tiny.icon-only svg) {
  width: 11px;
  height: 11px;
  display: block;
}
:deep(.sidebar-delete-confirm .btn-tiny.danger:hover) {
  background: rgba(254, 226, 226, 0.86);
  border-color: #f87171;
  color: #dc2626;
}
:deep(.sidebar-delete-confirm .btn-tiny.danger-confirm) {
  background: #b91c1c;
  border-color: #ef4444;
  color: #fff;
}
:deep(.sidebar-delete-confirm .delete-popover) {
  border-color: #334155;
  background: #0f172a;
  box-shadow: 0 12px 28px rgba(2, 6, 23, 0.42);
}
:deep(.sidebar-delete-confirm .delete-popover-arrow) {
  background: #0f172a;
  border-left-color: #334155;
  border-top-color: #334155;
}
:deep(.sidebar-delete-confirm .delete-popover-text) {
  color: #fecaca;
}
:deep(.sidebar-delete-confirm .btn-confirm-yes) {
  color: #86efac;
  border-color: #22c55e;
}
:deep(.sidebar-delete-confirm .btn-confirm-yes:hover) {
  background: #16a34a;
  color: #fff;
}
:deep(.sidebar-delete-confirm .btn-confirm-no) {
  color: #fda4af;
  border-color: #fb7185;
}
:deep(.sidebar-delete-confirm .btn-confirm-no:hover) {
  background: #e11d48;
  color: #fff;
}
.btn-disconnect { color: #f59e0b; border-color: #f59e0b; }
.btn-disconnect:hover { background: rgba(245,158,11,0.2); color: #d97706; }
.btn-overview {
  color: #0f766e;
  border-color: rgba(94, 234, 212, 0.92);
}
.btn-overview:hover {
  background: rgba(204, 251, 241, 0.84);
  color: #0f766e;
  border-color: rgba(45, 212, 191, 0.96);
}
.btn-confirm-yes { color: #16a34a; border-color: #16a34a; }
.btn-confirm-yes:hover { background: #16a34a; color: white; }
.btn-confirm-no { color: #dc2626; border-color: #dc2626; }
.btn-confirm-no:hover { background: #dc2626; color: white; }
.empty-hint { text-align: center; color: #94a3b8; font-size: 12px; padding: 20px; }
.sidebar.theme-dark .empty-hint {
  color: #64748b;
}

/* 未分组拖拽区域 */
.ungrouped-zone {
  position: relative;
  border-radius: 4px;
  transition: background 0.15s;
  padding: 1px 0;
}
.ungrouped-zone.drag-over { background: rgba(219, 234, 254, 0.68); }
.ungrouped-zone.cross-group-preview.drag-over { background: rgba(209, 250, 229, 0.74); }
.drop-group-hint,
.group-drop-hint {
  margin-left: auto;
  font-size: 10px;
  color: #8ec5ff;
  background: rgba(78, 154, 241, 0.16);
  border: 1px solid rgba(78, 154, 241, 0.35);
  border-radius: 999px;
  padding: 1px 6px;
  line-height: 1.5;
}
.drop-group-hint.cross-group-preview,
.group-drop-hint.cross-group-preview {
  color: #b7f7df;
  background: rgba(52, 211, 153, 0.18);
  border-color: rgba(52, 211, 153, 0.34);
}
.drop-group-hint {
  display: inline-flex;
  align-items: center;
  margin: 2px 6px 4px 14px;
}
.group-block { margin: 2px 0; }
.group-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 12px;
  cursor: pointer;
  color: #64748b;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-radius: 8px;
  margin: 1px 8px;
  user-select: none;
  transition: background 0.15s, border-color 0.15s;
  border: 1px solid transparent;
}
.group-header:hover {
  background: rgba(255, 255, 255, 0.74);
  border-color: rgba(147, 197, 253, 0.24);
}
.sidebar.theme-dark .group-header {
  color: #94a3b8;
}
.sidebar.theme-dark .group-header:hover {
  background: rgba(30, 41, 59, 0.88);
  border-color: rgba(71, 85, 105, 0.82);
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
@keyframes connectingSweep {
  to { transform: translateX(100%); }
}
@keyframes dotPulse {
  0% { box-shadow: 0 0 0 0 rgba(96, 165, 250, 0.6); }
  100% { box-shadow: 0 0 0 8px rgba(96, 165, 250, 0); }
}
@keyframes avatarFloat {
  from { transform: translateY(0); }
  to { transform: translateY(-1px); }
}
.group-header.drag-over { background: rgba(219, 234, 254, 0.72); border: 1px dashed #60a5fa; }
.group-header.cross-group-preview.drag-over {
  background: rgba(209, 250, 229, 0.8);
  border-color: #34d399;
}
.group-arrow { font-size: 9px; flex-shrink: 0; }
.group-name { flex: 1; }
.group-count {
  background: rgba(255, 255, 255, 0.86);
  color: #64748b;
  font-size: 10px;
  padding: 0 5px;
  border-radius: 10px;
  font-weight: 400;
  box-shadow: inset 0 0 0 1px rgba(226, 232, 240, 0.92);
}
.sidebar.theme-dark .group-count {
  background: rgba(30, 41, 59, 0.94);
  color: #cbd5e1;
  box-shadow: inset 0 0 0 1px rgba(51, 65, 85, 0.9);
}

/* ===== 右键菜单 ===== */
.ctx-menu {
  position: fixed;
  z-index: 2000;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  box-shadow: 0 6px 20px rgba(0,0,0,0.2);
  min-width: 120px;
  padding: 4px 0;
}
.ctx-item {
  padding: 7px 14px;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  background: transparent;
  color: #333;
}
.ctx-item:hover { background: #f0f0f0; }
.ctx-danger { color: #e53e3e; }
.ctx-danger:hover { background: #fff5f5; color: #e53e3e; }
.ctx-divider { height: 1px; background: #eee; margin: 3px 0; }
.sidebar.theme-dark .ctx-menu {
  background: rgba(15, 23, 42, 0.98);
  border-color: rgba(71, 85, 105, 0.92);
  box-shadow: 0 18px 40px rgba(2, 6, 23, 0.48);
}
.sidebar.theme-dark .ctx-item {
  color: #e2e8f0;
}
.sidebar.theme-dark .ctx-item:hover {
  background: rgba(30, 41, 59, 0.92);
}
.sidebar.theme-dark .ctx-danger {
  color: #fca5a5;
}
.sidebar.theme-dark .ctx-danger:hover {
  background: rgba(127, 29, 29, 0.56);
  color: #fecaca;
}
.sidebar.theme-dark .ctx-divider {
  background: rgba(51, 65, 85, 0.88);
}

.sidebar-toast {
  position: fixed;
  top: 18px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 5000;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 12px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.18);
  border: 1px solid transparent;
  max-width: min(520px, calc(100vw - 24px));
  word-break: break-word;
}
.sidebar-toast.ok { background: #f0fdf4; color: #166534; border-color: #bbf7d0; }
.sidebar-toast.err { background: #fff1f2; color: #991b1b; border-color: #fecaca; }
.sidebar.theme-dark .sidebar-toast.ok {
  background: rgba(6, 78, 59, 0.96);
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.34);
}
.sidebar.theme-dark .sidebar-toast.err {
  background: rgba(127, 29, 29, 0.96);
  color: #fee2e2;
  border-color: rgba(248, 113, 113, 0.34);
}
.toast-enter-active, .toast-leave-active { transition: opacity 0.25s, transform 0.25s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(-12px); }

</style>
