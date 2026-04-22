<template>
  <div class="sidebar" :class="{ collapsed: sidebarCollapsed }">
    <!-- 折叠状态：图标列表 + 底部展开按钮 -->
    <div v-if="sidebarCollapsed" class="sidebar-collapsed-bar">
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
      <button class="btn-expand-bottom" title="展开连接列表" @click="sidebarCollapsed = false">▶</button>
    </div>

    <template v-else>
      <!-- 连接列表（按分组） -->
      <div class="conn-list">
        <!-- 未分组连接 -->
        <div
          v-for="conn in (groupedConnections[''] || [])"
          :key="conn.id"
          :class="['conn-item', { active: activeConnID === conn.id }]"
          @contextmenu.prevent="showCtxMenu($event, conn)"
          @mouseleave="cancelDelete()"
        >
          <div class="conn-main" @click="handleConnect(conn)">
            <span class="conn-avatar" :style="{ background: connColor(conn) }">{{ connInitial(conn) }}</span>
            <span :class="['conn-dot', connectionsStore.isConnected(conn.id) ? 'connected' : 'disconnected']" />
            <span class="conn-name">{{ conn.name }}</span>
            <span class="conn-host">{{ conn.is_cluster ? '[Cluster]' : conn.host + ':' + conn.port }}</span>
          </div>
          <div class="conn-actions">
            <button
              v-if="connectionsStore.isConnected(conn.id)"
              class="btn-tiny btn-disconnect"
              title="断开连接"
              @click.stop="disconnectConn(conn.id)"
            >⊘</button>
            <button class="btn-tiny" title="编辑" @click.stop="openEdit(conn)">✎</button>
            <template v-if="confirmDeleteId !== conn.id">
              <button class="btn-tiny danger" title="删除" @click.stop="requestDelete(conn.id)">✕</button>
            </template>
            <template v-else>
              <button class="btn-tiny btn-confirm-yes" title="确认删除" @click.stop="confirmDelete(conn.id)">✓</button>
              <button class="btn-tiny btn-confirm-no" title="取消" @click.stop="cancelDelete()">✗</button>
            </template>
          </div>
        </div>

        <!-- 命名分组 -->
        <div v-for="(conns, group) in namedGroups" :key="group" class="group-block">
          <div
            class="group-header"
            @click="toggleGroup(group)"
            :class="{ 'drag-over': dragOverGroup === group }"
            @dragover.prevent="dragOverGroup = group"
            @dragleave="dragOverGroup = null"
            @drop.prevent="onDropToGroup(group)"
          >
            <span class="group-arrow">{{ collapsed[group] ? '▶' : '▼' }}</span>
            <span class="group-name">{{ group }}</span>
            <span class="group-count">{{ conns.length }}</span>
          </div>
          <div v-if="!collapsed[group]">
            <div
              v-for="conn in conns"
              :key="conn.id"
              :class="['conn-item', 'grouped', { active: activeConnID === conn.id }]"
              draggable="true"
              @dragstart="onDragStart(conn.id)"
              @dragend="dragOverGroup = null"
              @contextmenu.prevent="showCtxMenu($event, conn)"
              @mouseleave="cancelDelete()"
            >
              <div class="conn-main" @click="handleConnect(conn)">
                <span class="conn-avatar" :style="{ background: connColor(conn) }">{{ connInitial(conn) }}</span>
                <span :class="['conn-dot', connectionsStore.isConnected(conn.id) ? 'connected' : 'disconnected']" />
                <span class="conn-name">{{ conn.name }}</span>
                <span class="conn-host">{{ conn.is_cluster ? '[Cluster]' : conn.host + ':' + conn.port }}</span>
              </div>
              <div class="conn-actions">
                <button
                  v-if="connectionsStore.isConnected(conn.id)"
                  class="btn-tiny btn-disconnect"
                  title="断开连接"
                  @click.stop="disconnectConn(conn.id)"
                >⊘</button>
                <button class="btn-tiny" title="编辑" @click.stop="openEdit(conn)">✎</button>
                <template v-if="confirmDeleteId !== conn.id">
                  <button class="btn-tiny danger" title="删除" @click.stop="requestDelete(conn.id)">✕</button>
                </template>
                <template v-else>
                  <button class="btn-tiny btn-confirm-yes" title="确认删除" @click.stop="confirmDelete(conn.id)">✓</button>
                  <button class="btn-tiny btn-confirm-no" title="取消" @click.stop="cancelDelete()">✗</button>
                </template>
              </div>
            </div>
          </div>
        </div>

        <div v-if="connectionsStore.connections.length === 0" class="empty-hint">
          点击 ＋ 管理连接
        </div>
      </div>

      <!-- 底部操作区 -->
      <div class="sidebar-footer">
        <button class="btn-icon btn-collapse" title="折叠连接列表" @click="sidebarCollapsed = true">◀</button>
        <button class="btn-icon btn-settings" title="设置" @click="openSettings()">⚙</button>
        <button class="btn-icon" title="管理连接" @click="openConnManager()">＋</button>
      </div>
    </template>

    <!-- 右键菜单 -->
    <div
      v-if="ctxMenu.visible"
      class="ctx-menu"
      :style="{ top: ctxMenu.y + 'px', left: ctxMenu.x + 'px' }"
      @click.stop
    >
      <div class="ctx-item" @click="openEdit(ctxMenu.conn); ctxMenu.visible = false">✎ 编辑</div>
      <div class="ctx-divider" />
      <div class="ctx-item ctx-danger" @click="removeConnection(ctxMenu.conn.id); ctxMenu.visible = false">✕ 删除</div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, inject, onMounted, onBeforeUnmount } from 'vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useWorkspaceStore } from '../../stores/workspace.js'

const connectionsStore = useConnectionsStore()
const workspaceStore = useWorkspaceStore()

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

// 连接图标颜色（淡雅色板，按 id 哈希取色）
const AVATAR_COLORS = [
  '#5c7f9e', '#6e8c6a', '#8a6a7a', '#7a6e8a', '#8a7a5a',
  '#5a7a7a', '#7a5a6a', '#6a7a5a', '#7a6a5a', '#5a6a7a',
  '#6a5a8a', '#7a8a6a',
]

function connColor(conn) {
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

// 右键菜单
const ctxMenu = ref({ visible: false, x: 0, y: 0, conn: null })
function showCtxMenu(e, conn) {
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, conn }
}
function closeCtxMenu() { ctxMenu.value.visible = false }
onMounted(() => document.addEventListener('click', closeCtxMenu))
onBeforeUnmount(() => document.removeEventListener('click', closeCtxMenu))

// 编辑连接（打开 ConnectionManager）
function openEdit(conn) {
  openConnManager()
}

// 分组折叠状态
const collapsed = ref({})
function toggleGroup(group) {
  collapsed.value[group] = !collapsed.value[group]
}

// 拖拽
const dragId = ref(null)
const dragOverGroup = ref(null)
function onDragStart(connId) {
  dragId.value = connId
}
async function onDropToGroup(targetGroup) {
  dragOverGroup.value = null
  if (!dragId.value) return
  const conn = connectionsStore.connections.find(c => c.id === dragId.value)
  if (conn && conn.group !== targetGroup) {
    await connectionsStore.save({
      id: conn.id,
      name: conn.name,
      group: targetGroup,
      host: conn.host,
      port: conn.port,
      password: conn.password,
      db: conn.db,
      is_cluster: conn.is_cluster,
      cluster_addrs: conn.cluster_addrs || [],
      ssh_enabled: conn.ssh_enabled,
      ssh: conn.ssh || null,
    })
  }
  dragId.value = null
}

async function handleConnect(conn) {
  if (connectionsStore.isConnected(conn.id)) {
    workspaceStore.setActiveConn(conn.id, conn.name, conn.db || 0)
    await workspaceStore.fetchTotalKeys()
    await workspaceStore.search('*')
    return
  }
  const result = await connectionsStore.connect(conn.id)
  if (result.success) {
    workspaceStore.setActiveConn(conn.id, conn.name, result.init_db || 0)
    await workspaceStore.fetchTotalKeys()
    await workspaceStore.search('*')
  } else {
    alert('连接失败: ' + result.message)
  }
}

async function removeConnection(id) {
  ctxMenu.value.visible = false
  const result = await connectionsStore.remove(id)
  if (activeConnID.value === id) {
    workspaceStore.setActiveConn(null, '')
  }
  if (!result?.success) {
    alert('删除失败: ' + (result?.message || '未知错误'))
  }
}

// 内联删除确认
const confirmDeleteId = ref(null)
function requestDelete(id) { confirmDeleteId.value = id }
function confirmDelete(id) { confirmDeleteId.value = null; removeConnection(id) }
function cancelDelete() { confirmDeleteId.value = null }

// 断开连接
async function disconnectConn(id) {
  await connectionsStore.disconnect(id)
  if (activeConnID.value === id) {
    workspaceStore.setActiveConn(null, '')
  }
}
</script>

<style scoped>
.sidebar {
  width: 240px;
  min-width: 240px;
  background: #1e2a3a;
  color: #c9d1d9;
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
  position: relative;
  transition: width 0.2s ease, min-width 0.2s ease;
}
.sidebar.collapsed {
  width: 44px;
  min-width: 44px;
}

/* ===== 折叠栏 ===== */
.sidebar-collapsed-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
  padding: 8px 0 10px;
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
  box-shadow: 0 0 0 2px #1e2a3a, 0 0 0 3.5px rgba(255,255,255,0.55);
}
.btn-expand-bottom {
  background: #4a5568;
  color: #a0aec0;
  border: none;
  border-radius: 4px;
  width: 28px;
  height: 24px;
  font-size: 10px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 6px;
}
.btn-expand-bottom:hover { background: #718096; color: white; }

/* ===== 展开状态 footer ===== */
.sidebar-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  padding: 8px 12px;
  border-top: 1px solid #2d3748;
}
.btn-icon {
  background: #4e9af1;
  color: white;
  border: none;
  border-radius: 4px;
  width: 24px; height: 24px;
  font-size: 14px; line-height: 1;
  cursor: pointer;
  display: flex; align-items: center; justify-content: center;
}
.btn-icon:hover { background: #3a85e0; }
.btn-settings { background: #4a5568; }
.btn-settings:hover { background: #718096; }
.btn-collapse { background: #4a5568; font-size: 10px; }
.btn-collapse:hover { background: #718096; }

/* ===== 连接列表 ===== */
.conn-list { flex: 1; overflow-y: auto; padding: 8px 0; }
.conn-item {
  display: flex;
  align-items: center;
  padding: 5px 10px;
  cursor: pointer;
  border-radius: 4px;
  margin: 1px 6px;
}
.conn-item.grouped { margin-left: 14px; margin-right: 6px; }
.conn-item:hover { background: #2d3748; }
.conn-item.active { background: #2d4a6e; }
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

.conn-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.conn-dot.connected { background: #4CAF50; }
.conn-dot.disconnected { background: #9e9e9e; }
.conn-name { font-size: 13px; color: #e2e8f0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex: 1; }
.conn-host { font-size: 11px; color: #718096; white-space: nowrap; }
.conn-actions { display: none; gap: 2px; }
.conn-item:hover .conn-actions { display: flex; }
.btn-tiny {
  background: transparent;
  border: 1px solid #4a5568;
  color: #a0aec0;
  border-radius: 3px;
  padding: 1px 5px;
  font-size: 11px;
  cursor: pointer;
}
.btn-tiny:hover { background: #4a5568; }
.btn-tiny.danger:hover { background: #e53e3e; border-color: #e53e3e; color: white; }
.btn-disconnect { color: #f59e0b; border-color: #f59e0b; }
.btn-disconnect:hover { background: rgba(245,158,11,0.2); color: #d97706; }
.btn-confirm-yes { color: #16a34a; border-color: #16a34a; }
.btn-confirm-yes:hover { background: #16a34a; color: white; }
.btn-confirm-no { color: #dc2626; border-color: #dc2626; }
.btn-confirm-no:hover { background: #dc2626; color: white; }
.empty-hint { text-align: center; color: #4a5568; font-size: 12px; padding: 20px; }

/* ===== 分组 ===== */
.group-block { margin: 2px 0; }
.group-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 12px;
  cursor: pointer;
  color: #a0aec0;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-radius: 3px;
  margin: 0 6px;
  user-select: none;
  transition: background 0.15s;
}
.group-header:hover { background: #2d3748; }
.group-header.drag-over { background: #2d4a6e; border: 1px dashed #4e9af1; }
.group-arrow { font-size: 9px; flex-shrink: 0; }
.group-name { flex: 1; }
.group-count {
  background: #2d3748;
  color: #718096;
  font-size: 10px;
  padding: 0 5px;
  border-radius: 10px;
  font-weight: 400;
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
</style>
