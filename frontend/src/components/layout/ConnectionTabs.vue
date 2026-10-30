<template>
  <div class="conn-tabs" :class="'theme-' + theme">
    <div class="tabs-scroll">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        class="conn-tab"
        :class="[
          { active: tab.id === workspaceStore.activeConnID, off: !tab.connected, dragging: dragId === tab.id },
          dropClass(tab.id),
        ]"
        :title="tab.title"
        draggable="true"
        @click="activate(tab)"
        @auxclick="onAuxClick($event, tab.id)"
        @dragstart="onDragStart(tab.id, $event)"
        @dragover.prevent="onDragOver(tab.id, $event)"
        @drop.prevent="onDrop(tab.id)"
        @dragend="onDragEnd"
      >
        <span class="tab-dot" :class="{ on: tab.connected }" />
        <span class="tab-name">{{ tab.name }}</span>
        <span class="tab-close" :title="t('tabs.close')" @click.stop="closeTab(tab.id)">✕</span>
      </button>
      <span v-if="!tabs.length" class="tabs-empty">{{ t('tabs.empty') }}</span>
    </div>
    <button type="button" class="tabs-new" :title="t('tabs.new')" @click="openManager">＋</button>
  </div>
</template>

<script setup>
import { computed, inject, ref, watch } from 'vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'

const connectionsStore = useConnectionsStore()
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
const openConnManager = inject('openConnManager', null)

const theme = computed(() => settingsStore.themeMode || 'light')

const tabs = computed(() =>
  (workspaceStore.openTabs || [])
    .map((id) => {
      const conn = (connectionsStore.connections || []).find((item) => item.id === id)
      if (!conn) return null
      const connected = connectionsStore.isConnected(id)
      return {
        id,
        name: conn.name || id,
        connected,
        title: (conn.name || id) + ' · ' + (connected ? t('tabs.connected') : t('tabs.disconnected')),
      }
    })
    .filter(Boolean)
)

// —— 拖拽排序 / 中键关闭 ——
const dragId = ref('')
const dropTarget = ref({ id: '', placement: 'before' })
// 拖拽松手那一下浏览器还会派发 click，短暂屏蔽避免误切换
let suppressClickUntil = 0

function onDragStart(id, event) {
  dragId.value = id
  dropTarget.value = { id: '', placement: 'before' }
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    try { event.dataTransfer.setData('text/plain', id) } catch (e) {}
  }
}

function onDragOver(id, event) {
  if (!dragId.value || dragId.value === id) return
  const rect = event.currentTarget.getBoundingClientRect()
  const placement = event.clientX - rect.left > rect.width / 2 ? 'after' : 'before'
  dropTarget.value = { id, placement }
}

function dropClass(id) {
  if (!dragId.value || dropTarget.value.id !== id) return ''
  return dropTarget.value.placement === 'after' ? 'drop-after' : 'drop-before'
}

function onDrop(id) {
  const source = dragId.value
  if (!source || source === id) {
    onDragEnd()
    return
  }
  moveTab(source, id, dropTarget.value.placement)
  onDragEnd()
}

function onDragEnd() {
  if (dragId.value) suppressClickUntil = Date.now() + 240
  dragId.value = ''
  dropTarget.value = { id: '', placement: 'before' }
}

function moveTab(sourceId, targetId, placement) {
  const list = [...(workspaceStore.openTabs || [])]
  const from = list.indexOf(sourceId)
  if (from === -1 || !list.includes(targetId)) return
  list.splice(from, 1)
  let insertAt = list.indexOf(targetId)
  if (insertAt === -1) insertAt = list.length
  if (placement === 'after') insertAt += 1
  list.splice(insertAt, 0, sourceId)
  if (list.join(',') === (workspaceStore.openTabs || []).join(',')) return
  workspaceStore.openTabs = list
}

function onAuxClick(event, id) {
  // 中键关闭，和浏览器标签页的习惯保持一致
  if (event.button !== 1) return
  event.preventDefault()
  closeTab(id)
}

async function activate(tab) {
  if (Date.now() < suppressClickUntil) return
  if (tab.id === workspaceStore.activeConnID) return
  const conn = (connectionsStore.connections || []).find((item) => item.id === tab.id)
  if (!conn) return
  if (tab.connected) {
    workspaceStore.setActiveConn(conn.id, conn.name, conn.db || 0)
    return
  }
  const result = await connectionsStore.connect(conn.id)
  if (result && result.success === false && result.message !== 'connecting') return
  workspaceStore.setActiveConn(conn.id, conn.name, conn.db || 0)
}

function closeTab(id) {
  const list = [...(workspaceStore.openTabs || [])]
  const index = list.indexOf(id)
  if (index === -1) return
  list.splice(index, 1)
  workspaceStore.openTabs = list
  if (workspaceStore.activeConnID !== id) return
  const nextId = list[Math.min(index, list.length - 1)]
  const nextConn = (connectionsStore.connections || []).find((item) => item.id === nextId)
  if (!nextConn) {
    workspaceStore.activeConnID = null
    workspaceStore.activeConnName = ''
    return
  }
  workspaceStore.setActiveConn(nextConn.id, nextConn.name, nextConn.db || 0)
}

function openManager() {
  if (openConnManager) openConnManager()
}

// 连接被删除时同步清掉对应的标签
watch(
  () => (connectionsStore.connections || []).map((item) => item.id).join(','),
  () => {
    const alive = new Set((connectionsStore.connections || []).map((item) => item.id))
    const kept = (workspaceStore.openTabs || []).filter((id) => alive.has(id))
    if (kept.length !== (workspaceStore.openTabs || []).length) {
      workspaceStore.openTabs = kept
    }
  }
)
</script>

<style scoped>
.conn-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  flex-shrink: 0;
  padding: 0 2px 6px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  margin-bottom: 6px;
}
.tabs-scroll {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  scrollbar-width: none;
}
.tabs-scroll::-webkit-scrollbar {
  display: none;
}
.conn-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
  max-width: 220px;
  height: 26px;
  padding: 0 8px;
  border: 1px solid rgba(203, 213, 225, 0.9);
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.9);
  color: #475569;
  font-size: 11.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out), color var(--motion-fast) var(--ease-out);
}
.conn-tab:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.conn-tab.active {
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.98), rgba(219, 234, 254, 0.95));
  border-color: rgba(147, 197, 253, 0.96);
  color: #1d4ed8;
}
.conn-tab.off {
  color: #94a3b8;
}
/* 拖拽中的标签变淡，目标位置画一条竖线 */
.conn-tab.dragging {
  opacity: 0.45;
}
.conn-tab.drop-before::after,
.conn-tab.drop-after::after {
  content: '';
  position: absolute;
  top: 3px;
  bottom: 3px;
  width: 2px;
  border-radius: 1px;
  background: #3b82f6;
}
.conn-tab.drop-before::after { left: -4px; }
.conn-tab.drop-after::after { right: -4px; }
.conn-tabs.theme-dark .conn-tab.drop-before::after,
.conn-tabs.theme-dark .conn-tab.drop-after::after {
  background: #60a5fa;
}
.tabs-empty {
  font-size: 11px;
  color: #94a3b8;
  padding: 0 4px;
  white-space: nowrap;
}
.tab-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #cbd5e1;
  flex: 0 0 auto;
}
.tab-dot.on {
  background: #22c55e;
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.18);
}
.tab-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tab-close {
  flex: 0 0 auto;
  width: 14px;
  height: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  font-size: 10px;
  color: #94a3b8;
}
.tab-close:hover {
  background: rgba(239, 68, 68, 0.12);
  color: #dc2626;
}
.tabs-new {
  flex: 0 0 auto;
  width: 26px;
  height: 26px;
  border: 1px dashed rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-md);
  background: transparent;
  color: #64748b;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  transition: background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out), color var(--motion-fast) var(--ease-out);
}
.tabs-new:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
/* 深色 */
.conn-tabs.theme-dark {
  border-bottom-color: rgba(51, 65, 85, 0.94);
}
.conn-tabs.theme-dark .conn-tab {
  background: rgba(15, 23, 42, 0.9);
  border-color: rgba(71, 85, 105, 0.96);
  color: #cbd5e1;
}
.conn-tabs.theme-dark .conn-tab.active {
  background: rgba(30, 64, 175, 0.34);
  border-color: rgba(96, 165, 250, 0.5);
  color: #dbeafe;
}
.conn-tabs.theme-dark .conn-tab.off { color: #64748b; }
.conn-tabs.theme-dark .tabs-new {
  border-color: rgba(71, 85, 105, 0.96);
  color: #94a3b8;
}
</style>
