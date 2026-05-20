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
          <div ref="sidebarRef" class="cm-sidebar" :class="{ 'suppress-hover': suppressHover }">
            <!-- 未分组连接 -->
            <div
              class="cm-group-zone"
              :class="{
                'drag-over': isGroupDropPreview(''),
                'cross-group-preview': isCrossGroupDropPreview(''),
              }"
              @dragover.prevent="onDragOverGroup('', $event)"
              @dragleave="onDragLeaveContainer"
              @drop.prevent="onDropToGroup('', '')"
            >
              <div class="cm-group-label">
                {{ t('connManager.ungrouped') }}
                <span
                  v-if="isGroupDropPreview('')"
                  class="cm-drop-group-hint"
                  :class="{ 'cross-group-preview': isCrossGroupDropPreview('') }"
                >{{ groupDropHint('') }}</span>
              </div>
              <div
                v-for="conn in (groupedConnections[''] || [])"
                :key="conn.id"
                :class="['cm-conn-item', {
                  selected: selectedConn?.id === conn.id,
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
                @click="selectConn(conn)"
                @contextmenu.prevent="showCtxMenu($event, conn)"
              >
                <div v-if="isItemDropPreview(conn.id, 'before')" class="cm-drop-indicator before" />
                <div v-if="isItemDropPreview(conn.id, 'after')" class="cm-drop-indicator after" />
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
                :class="{
                  'drag-over': isGroupDropPreview(group),
                  'cross-group-preview': isCrossGroupDropPreview(group),
                }"
                @click="toggleGroup(group)"
                @dragover.prevent="onDragOverGroup(group, $event)"
                @dragleave="onDragLeaveContainer"
                @drop.prevent="onDropToGroup(group, '')"
              >
                <span class="arrow">{{ collapsed[group] ? '▶' : '▼' }}</span>
                <span class="gname">{{ group }}</span>
                <span class="gcount">{{ conns.length }}</span>
                <span
                  v-if="isGroupDropPreview(group)"
                  class="cm-drop-group-hint"
                  :class="{ 'cross-group-preview': isCrossGroupDropPreview(group) }"
                >{{ groupDropHint(group) }}</span>
              </div>
              <div v-if="!collapsed[group]" class="cm-group-conns">
                <div
                  v-for="conn in conns"
                  :key="conn.id"
                  :class="['cm-conn-item', {
                    selected: selectedConn?.id === conn.id,
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
                  @click="selectConn(conn)"
                  @contextmenu.prevent="showCtxMenu($event, conn)"
                >
                  <div v-if="isItemDropPreview(conn.id, 'before')" class="cm-drop-indicator before" />
                  <div v-if="isItemDropPreview(conn.id, 'after')" class="cm-drop-indicator after" />
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
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import ConnectionForm from './ConnectionForm.vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'

const props = defineProps({
  initialConnection: { type: Object, default: null },
})

const emit = defineEmits(['close'])
const connectionsStore = useConnectionsStore()
const workspaceStore = useWorkspaceStore()
const { t } = useI18n()

const selectedConn = ref(null)
const collapsed = ref({})
const dragId = ref(null)
const dragOverGroup = ref(null)
const dragPreview = ref({ group: null, targetId: '', placement: 'before', mode: null })
const suppressHover = ref(false)
const sidebarRef = ref(null)
let dragExpandTimer = null
let suppressHoverTimer = null

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
    clusterAddrs: conn.clusterAddrs || conn.cluster_addrs || [],
    proxyEnabled: conn.proxyEnabled ?? conn.proxy_enabled,
    proxyUrl: conn.proxyUrl ?? conn.proxy_url ?? '',
    iconColor: conn.iconColor ?? conn.icon_color ?? '',
    sshEnabled: conn.ssh_enabled,
  }
}

watch(() => props.initialConnection, (conn) => {
  if (conn) {
    selectConn(conn)
  } else {
    selectedConn.value = null
  }
}, { immediate: true })

function toggleGroup(group) {
  collapsed.value[group] = !collapsed.value[group]
}

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
  releaseHoverState()
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
  return group ? `${t('connManager.moveToGroup')} ${group}` : t('connManager.moveToUngrouped')
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

function clearSuppressHover() {
  suppressHover.value = false
  if (suppressHoverTimer) {
    clearTimeout(suppressHoverTimer)
    suppressHoverTimer = null
  }
  window.removeEventListener('mousemove', clearSuppressHover)
}

function releaseHoverState() {
  suppressHover.value = true
  const sidebarEl = sidebarRef.value
  if (sidebarEl) {
    sidebarEl.style.pointerEvents = 'none'
    void sidebarEl.offsetHeight
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (sidebarRef.value) {
          sidebarRef.value.style.pointerEvents = ''
        }
      })
    })
  }
  if (suppressHoverTimer) clearTimeout(suppressHoverTimer)
  window.removeEventListener('mousemove', clearSuppressHover)
  window.addEventListener('mousemove', clearSuppressHover, { once: true })
  suppressHoverTimer = setTimeout(() => {
    clearSuppressHover()
  }, 180)
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
    proxy_enabled: conn.proxy_enabled,
    proxy_url: conn.proxy_url || '',
    icon_color: conn.icon_color || '',
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
  const result = await connectionsStore.remove(conn.id)
  if (result?.success) {
    if (workspaceStore.activeConnID === conn.id) {
      workspaceStore.setActiveConn(null, '')
    }
    if (selectedConn.value?.id === conn.id) {
      selectedConn.value = null
    }
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
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  if (dragExpandTimer) clearTimeout(dragExpandTimer)
  if (suppressHoverTimer) clearTimeout(suppressHoverTimer)
  window.removeEventListener('mousemove', clearSuppressHover)
  if (sidebarRef.value) {
    sidebarRef.value.style.pointerEvents = ''
  }
})
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
  background: linear-gradient(180deg, #f8fafc 0%, #f4f7fb 100%);
}
/* 左侧 */
.cm-sidebar {
  width: 260px;
  flex-shrink: 0;
  border-right: 1px solid #e6ebf2;
  overflow-y: auto;
  padding: 10px 0;
  background: linear-gradient(180deg, #f8fafc 0%, #f3f6fa 100%);
  box-shadow: inset -1px 0 0 rgba(255, 255, 255, 0.6);
}
.cm-sidebar.suppress-hover .cm-conn-item:hover {
  background: transparent;
}
.cm-sidebar.suppress-hover .cm-conn-item.selected:hover {
  background: #dbeafe;
}
.cm-group-zone {
  padding: 2px 0;
  border-radius: 4px;
  position: relative;
  transition: background 0.15s;
}
.cm-group-zone.drag-over { background: #dbeafe; }
.cm-group-zone.cross-group-preview.drag-over { background: rgba(16, 185, 129, 0.12); }
.cm-group-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 10px;
  font-weight: 700;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.72px;
  padding: 5px 14px 3px;
}
.cm-conn-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  cursor: pointer;
  border-radius: 8px;
  margin: 2px 8px;
  position: relative;
  border: 1px solid transparent;
  transition: background 0.12s, border-color 0.12s, box-shadow 0.12s;
}
.cm-conn-item:hover {
  background: rgba(255, 255, 255, 0.72);
  border-color: rgba(203, 213, 225, 0.7);
}
.cm-conn-item.selected {
  background: linear-gradient(180deg, #eff6ff 0%, #dbeafe 100%);
  border-color: rgba(96, 165, 250, 0.38);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.08);
}
.cm-conn-item.drop-before,
.cm-conn-item.drop-after {
  background: rgba(59, 130, 246, 0.08);
}
.cm-conn-item.cross-group-preview.drop-before,
.cm-conn-item.cross-group-preview.drop-after {
  background: rgba(16, 185, 129, 0.1);
}
.cm-conn-item.dragging {
  opacity: 0.46;
  transform: scale(0.988);
  background: rgba(59, 130, 246, 0.12);
  box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.18);
}
.cm-drop-indicator {
  position: absolute;
  left: 10px;
  right: 10px;
  height: 0;
  pointer-events: none;
  z-index: 2;
}
.cm-drop-indicator::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  height: 2px;
  background: #3b82f6;
  border-radius: 999px;
  box-shadow: 0 0 10px rgba(59, 130, 246, 0.22);
}
.cm-conn-item.cross-group-preview .cm-drop-indicator::before {
  background: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.24);
}
.cm-drop-indicator::after {
  content: '';
  position: absolute;
  left: -1px;
  top: -3px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #3b82f6;
  box-shadow: 0 0 0 2px #f9fafb, 0 0 10px rgba(59, 130, 246, 0.26);
}
.cm-conn-item.cross-group-preview .cm-drop-indicator::after {
  background: #10b981;
  box-shadow: 0 0 0 2px #f9fafb, 0 0 10px rgba(16, 185, 129, 0.3);
}
.cm-drop-indicator.before { top: -1px; }
.cm-drop-indicator.after { bottom: -1px; }
.cm-drop-indicator.after::after { top: -3px; }
.conn-dot {
  width: 7px; height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.conn-dot.connected { background: #22c55e; }
.conn-dot.disconnected { background: #d1d5db; }
.conn-info { min-width: 0; flex: 1; }
.conn-name { display: block; font-size: 13px; color: #1f2937; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cm-conn-item.selected .conn-name { color: #0f172a; }
.conn-host { display: block; font-size: 11px; color: #94a3b8; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cm-conn-item.selected .conn-host { color: #64748b; }
.cm-group-block { margin: 4px 0; }
.cm-group-header {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 5px 14px;
  cursor: pointer;
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.58px;
  border-radius: 8px;
  margin: 1px 8px;
  user-select: none;
  border: 1px solid transparent;
  transition: background 0.12s, border-color 0.12s;
}
.cm-group-header:hover {
  background: rgba(255, 255, 255, 0.64);
  border-color: rgba(226, 232, 240, 0.8);
}
.cm-group-header.drag-over { background: #dbeafe; border: 1px dashed #3b82f6; }
.cm-group-header.cross-group-preview.drag-over {
  background: rgba(16, 185, 129, 0.12);
  border-color: #10b981;
}
.cm-drop-group-hint {
  margin-left: auto;
  font-size: 10px;
  color: #2563eb;
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.22);
  border-radius: 999px;
  padding: 1px 6px;
  line-height: 1.5;
}
.cm-drop-group-hint.cross-group-preview {
  color: #047857;
  background: rgba(16, 185, 129, 0.12);
  border-color: rgba(16, 185, 129, 0.26);
}
.arrow { font-size: 9px; }
.gname { flex: 1; }
.gcount {
  background: rgba(226, 232, 240, 0.9);
  color: #64748b;
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
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  background: linear-gradient(180deg, #fbfcfe 0%, #f7f9fc 100%);
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
