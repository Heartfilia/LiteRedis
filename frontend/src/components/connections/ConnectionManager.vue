<template>
  <Teleport to="body">
    <div class="cm-backdrop" @click.self="$emit('close')">
      <div class="cm-card">
        <!-- 顶部 header -->
        <div class="cm-header">
          <span class="cm-title">{{ t('connManager.title') }}</span>
          <div class="cm-header-actions">
            <button class="btn-new" @click="newConnection">＋ {{ t('connManager.newConn') }}</button>
            <button class="btn-close" @click="$emit('close')">✕</button>
          </div>
        </div>

        <!-- 主体：左侧分组列表 + 右侧表单 -->
        <div class="cm-body">
          <!-- 左侧连接列表 -->
          <div class="cm-sidebar">
            <!-- 未分组连接 -->
            <div
              class="cm-group-zone"
              :class="{ 'drag-over': dragOverGroup === '' }"
              @dragover.prevent="dragOverGroup = ''"
              @dragleave="dragOverGroup = null"
              @drop.prevent="onDropToGroup('')"
            >
              <div class="cm-group-label">{{ t('connManager.ungrouped') }}</div>
              <div
                v-for="conn in (groupedConnections[''] || [])"
                :key="conn.id"
                :class="['cm-conn-item', { selected: selectedConn?.id === conn.id }]"
                draggable="true"
                @dragstart="onDragStart(conn.id)"
                @dragend="dragOverGroup = null"
                @click="selectConn(conn)"
                @contextmenu.prevent="showCtxMenu($event, conn)"
              >
                <span :class="['conn-dot', connectionsStore.isConnected(conn.id) ? 'connected' : 'disconnected']" />
                <div class="conn-info">
                  <span class="conn-name">{{ conn.name }}</span>
                  <span class="conn-host">{{ conn.is_cluster ? '[Cluster]' : conn.host + ':' + conn.port }}</span>
                </div>
              </div>
            </div>

            <!-- 命名分组 -->
            <div v-for="(conns, group) in namedGroups" :key="group" class="cm-group-block">
              <div
                class="cm-group-header"
                :class="{ 'drag-over': dragOverGroup === group }"
                @click="toggleGroup(group)"
                @dragover.prevent="dragOverGroup = group"
                @dragleave="dragOverGroup = null"
                @drop.prevent="onDropToGroup(group)"
              >
                <span class="arrow">{{ collapsed[group] ? '▶' : '▼' }}</span>
                <span class="gname">{{ group }}</span>
                <span class="gcount">{{ conns.length }}</span>
              </div>
              <div v-if="!collapsed[group]" class="cm-group-conns">
                <div
                  v-for="conn in conns"
                  :key="conn.id"
                  :class="['cm-conn-item', { selected: selectedConn?.id === conn.id }]"
                  draggable="true"
                  @dragstart="onDragStart(conn.id)"
                  @dragend="dragOverGroup = null"
                  @click="selectConn(conn)"
                  @contextmenu.prevent="showCtxMenu($event, conn)"
                >
                  <span :class="['conn-dot', connectionsStore.isConnected(conn.id) ? 'connected' : 'disconnected']" />
                  <div class="conn-info">
                    <span class="conn-name">{{ conn.name }}</span>
                    <span class="conn-host">{{ conn.is_cluster ? '[Cluster]' : conn.host + ':' + conn.port }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div v-if="connectionsStore.connections.length === 0" class="cm-empty">
              {{ t('connManager.noConnections') }}
            </div>
          </div>

          <!-- 右侧表单 -->
          <div class="cm-form-area">
            <ConnectionForm
              :connection="selectedConn"
              @saved="onSaved"
              @cancel="$emit('close')"
            />
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
        <div class="ctx-section-label">{{ t('connManager.moveToGroup') }}</div>
        <div class="ctx-item" @click="moveToGroup(ctxMenu.conn, '')">（{{ t('connManager.ungrouped') }}）</div>
        <div
          v-for="g in existingGroups"
          :key="g"
          class="ctx-item"
          @click="moveToGroup(ctxMenu.conn, g)"
        >{{ g }}</div>
        <div class="ctx-item" @click="moveToNewGroup(ctxMenu.conn)">{{ t('connManager.newGroup') }}</div>
        <div class="ctx-divider" />
        <div class="ctx-item ctx-danger" @click="deleteConn(ctxMenu.conn)">{{ t('connManager.deleteConn') }}</div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import ConnectionForm from './ConnectionForm.vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'

const emit = defineEmits(['close'])
const connectionsStore = useConnectionsStore()
const workspaceStore = useWorkspaceStore()
const { t } = useI18n()

const selectedConn = ref(null)
const collapsed = ref({})
const dragId = ref(null)
const dragOverGroup = ref(null)

const groupedConnections = computed(() => connectionsStore.groupedConnections)
const namedGroups = computed(() => {
  const g = { ...groupedConnections.value }
  delete g['']
  return g
})
const hasNamedGroups = computed(() => Object.keys(namedGroups.value).length > 0)
const existingGroups = computed(() => Object.keys(namedGroups.value))

function newConnection() {
  selectedConn.value = null
}

function selectConn(conn) {
  selectedConn.value = {
    ...conn,
    isCluster: conn.is_cluster,
    sshEnabled: conn.ssh_enabled,
  }
}

function toggleGroup(group) {
  collapsed.value[group] = !collapsed.value[group]
}

function onDragStart(connId) {
  dragId.value = connId
}

async function onDropToGroup(targetGroup) {
  dragOverGroup.value = null
  if (!dragId.value) return
  const conn = connectionsStore.connections.find(c => c.id === dragId.value)
  if (conn && conn.group !== targetGroup) {
    await connectionsStore.save(buildCfg(conn, targetGroup))
  }
  dragId.value = null
}

function buildCfg(conn, group) {
  return {
    id: conn.id,
    name: conn.name,
    group: group !== undefined ? group : (conn.group || ''),
    host: conn.host,
    port: conn.port,
    password: conn.password,
    db: conn.db,
    is_cluster: conn.is_cluster,
    cluster_addrs: conn.cluster_addrs || [],
    ssh_enabled: conn.ssh_enabled,
    ssh: conn.ssh || null,
  }
}

// 右键菜单
const ctxMenu = ref({ visible: false, x: 0, y: 0, conn: null })

function showCtxMenu(e, conn) {
  ctxMenu.value = { visible: true, x: e.clientX, y: e.clientY, conn }
}

function closeCtxMenu() {
  ctxMenu.value.visible = false
}

async function moveToGroup(conn, targetGroup) {
  closeCtxMenu()
  if (conn.group !== targetGroup) {
    await connectionsStore.save(buildCfg(conn, targetGroup))
  }
}

async function moveToNewGroup(conn) {
  closeCtxMenu()
  const name = prompt(t('connManager.newGroupPrompt'))
  if (name && name.trim()) {
    await connectionsStore.save(buildCfg(conn, name.trim()))
  }
}

async function deleteConn(conn) {
  closeCtxMenu()
  await connectionsStore.remove(conn.id)
  if (workspaceStore.activeConnID === conn.id) {
    workspaceStore.setActiveConn(null, '')
  }
  if (selectedConn.value?.id === conn.id) {
    selectedConn.value = null
  }
}

async function onSaved() {
  const savedId = selectedConn.value?.id
  await connectionsStore.loadConnections()
  // 保持选中同一条
  if (savedId) {
    const updated = connectionsStore.connections.find(c => c.id === savedId)
    if (updated) selectConn(updated)
  } else {
    // 新建后选中最新一条
    const last = connectionsStore.connections[connectionsStore.connections.length - 1]
    if (last) selectConn(last)
  }
}

function onDocClick() {
  closeCtxMenu()
}

onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<style scoped>
.cm-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.cm-card {
  width: 800px;
  max-width: 96vw;
  max-height: 88vh;
  background: white;
  border-radius: 12px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.3);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.cm-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  flex-shrink: 0;
}
.cm-title {
  font-size: 15px;
  font-weight: 600;
  color: #111827;
}
.cm-header-actions { display: flex; gap: 8px; align-items: center; }
.btn-new {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 5px 14px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.15s;
}
.btn-new:hover { background: #2563eb; }
.btn-close {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 4px 10px;
  background: transparent;
  color: #9ca3af;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: color 0.12s, border-color 0.12s;
}
.btn-close:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; }
.cm-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}
/* 左侧 */
.cm-sidebar {
  width: 260px;
  flex-shrink: 0;
  border-right: 1px solid #e5e7eb;
  overflow-y: auto;
  padding: 8px 0;
  background: #f9fafb;
}
.cm-group-zone {
  padding: 2px 0;
  border-radius: 4px;
  transition: background 0.15s;
}
.cm-group-zone.drag-over { background: #dbeafe; }
.cm-group-label {
  font-size: 10px;
  font-weight: 700;
  color: #9ca3af;
  text-transform: uppercase;
  letter-spacing: 0.6px;
  padding: 4px 14px 2px;
}
.cm-conn-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  cursor: pointer;
  border-radius: 6px;
  margin: 1px 6px;
  transition: background 0.12s;
}
.cm-conn-item:hover { background: #e8edf3; }
.cm-conn-item.selected { background: #dbeafe; }
.conn-dot {
  width: 7px; height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.conn-dot.connected { background: #22c55e; }
.conn-dot.disconnected { background: #d1d5db; }
.conn-info { min-width: 0; flex: 1; }
.conn-name { display: block; font-size: 13px; color: #1f2937; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.conn-host { display: block; font-size: 11px; color: #9ca3af; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cm-group-block { margin: 4px 0; }
.cm-group-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 14px;
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
  color: #6b7280;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-radius: 5px;
  margin: 0 6px;
  user-select: none;
  transition: background 0.12s;
}
.cm-group-header:hover { background: #e8edf3; }
.cm-group-header.drag-over { background: #dbeafe; border: 1px dashed #3b82f6; }
.arrow { font-size: 9px; }
.gname { flex: 1; }
.gcount {
  background: #e5e7eb;
  color: #6b7280;
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 10px;
  font-weight: 400;
}
.cm-group-conns { padding-left: 10px; }
.cm-empty {
  text-align: center;
  color: #d1d5db;
  font-size: 13px;
  padding: 30px 16px;
}
/* 右侧表单区 */
.cm-form-area {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}
/* 右键菜单 */
.ctx-menu {
  position: fixed;
  z-index: 2000;
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.12);
  min-width: 160px;
  padding: 4px 0;
}
.ctx-section-label {
  font-size: 10px;
  color: #9ca3af;
  font-weight: 700;
  text-transform: uppercase;
  padding: 4px 14px 2px;
  letter-spacing: 0.5px;
}
.ctx-item {
  padding: 7px 14px;
  font-size: 13px;
  color: #374151;
  cursor: pointer;
  white-space: nowrap;
}
.ctx-item:hover { background: #f3f4f6; }
.ctx-divider { height: 1px; background: #e5e7eb; margin: 4px 0; }
.ctx-danger { color: #dc2626; }
.ctx-danger:hover { background: #fff1f2; color: #dc2626; }
</style>
