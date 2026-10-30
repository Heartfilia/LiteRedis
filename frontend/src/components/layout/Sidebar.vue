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
            <span class="btn-expand-glyph" aria-hidden="true">
              <svg viewBox="0 0 16 16" fill="none">
                <path d="M6 3.5L10.5 8L6 12.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </span>
          </button>
        </div>
      </div>

      <div class="sidebar-layer sidebar-expanded-layer" :class="{ visible: !sidebarCollapsed }">
      <!-- 连接列表（按分组） -->
      <div class="conn-list" tabindex="0" data-kb-container @keydown="onConnListKeydown">
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
          v-kbd-row="{ clickTarget: '.conn-main' }"
          :class="['conn-item', {
            active: activeConnID === conn.id,
            connecting: connectionsStore.isConnecting(conn.id),
            dragging: dragId === conn.id,
            'drop-before': isItemDropPreview(conn.id, 'before'),
            'drop-after': isItemDropPreview(conn.id, 'after'),
            'cross-group-preview': isItemCrossGroupPreview(conn.id),
            'drop-landed': landedId === conn.id,
          }]"
          draggable="true"
          @dragstart="onDragStart(conn.id, $event)"
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
                <svg
                  v-if="isReadOnlyConn(conn)"
                  class="conn-lock"
                  viewBox="0 0 24 24"
                  fill="none"
                  aria-hidden="true"
                  :title="t('sidebar.readOnlyBadge')"
                >
                  <rect x="5" y="10.5" width="14" height="9.5" rx="2" stroke="currentColor" stroke-width="1.6"/>
                  <path d="M8.5 10.5V7.8a3.5 3.5 0 017 0v2.7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
                </svg>
                {{ conn.name || t('sidebar.unnamed') }}
              <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-inline">{{ t('sidebar.connecting') }}</span>
            </span>
          </div>
          <div class="conn-actions">
            <button
              v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
              class="btn-tiny btn-conn-action btn-diagnose"
              :title="t('sidebar.diagnostics')"
              @click.stop="openDiagMenu($event, conn)"
            >
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M3 12h3.4l2.2-5.6 3.7 11.2 2.3-5.6H21" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </button>
            <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-spinner" />
            <button class="btn-tiny btn-conn-action" :title="t('sidebar.edit')" @click.stop="openEdit(conn)">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M4 20h4l9.5-9.5-4-4L4 16v4z" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/>
                <path d="M12.8 6.7l4 4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
              </svg>
            </button>
            <button
              v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
              class="btn-tiny btn-conn-action btn-disconnect"
              :title="t('sidebar.disconnect')"
              @click.stop="disconnectConn(conn.id)"
            >
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M7 7l10 10M17 7L7 17" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
                <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" opacity="0.7"/>
              </svg>
            </button>
            <InlineDeleteConfirm
              v-if="!connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
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
            v-kbd-row="{ tabindex: 0 }"
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
              v-kbd-row="{ clickTarget: '.conn-main' }"
              :class="['conn-item', 'grouped', {
                active: activeConnID === conn.id,
                connecting: connectionsStore.isConnecting(conn.id),
                dragging: dragId === conn.id,
                'drop-before': isItemDropPreview(conn.id, 'before'),
                'drop-after': isItemDropPreview(conn.id, 'after'),
                'cross-group-preview': isItemCrossGroupPreview(conn.id),
                'drop-landed': landedId === conn.id,
              }]"
              draggable="true"
              @dragstart="onDragStart(conn.id, $event)"
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
                  <svg
                    v-if="isReadOnlyConn(conn)"
                    class="conn-lock"
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden="true"
                    :title="t('sidebar.readOnlyBadge')"
                  >
                    <rect x="5" y="10.5" width="14" height="9.5" rx="2" stroke="currentColor" stroke-width="1.6"/>
                    <path d="M8.5 10.5V7.8a3.5 3.5 0 017 0v2.7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
                  </svg>
                  {{ conn.name || t('sidebar.unnamed') }}
                  <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-inline">{{ t('sidebar.connecting') }}</span>
                </span>
              </div>
              <div class="conn-actions">
                <button
                  v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
                  class="btn-tiny btn-conn-action btn-diagnose"
                  :title="t('sidebar.diagnostics')"
                  @click.stop="openDiagMenu($event, conn)"
                >
                  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path d="M3 12h3.4l2.2-5.6 3.7 11.2 2.3-5.6H21" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
                  </svg>
                </button>
                <span v-if="connectionsStore.isConnecting(conn.id)" class="connecting-spinner" />
                <button class="btn-tiny btn-conn-action" :title="t('sidebar.edit')" @click.stop="openEdit(conn)">
                  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path d="M4 20h4l9.5-9.5-4-4L4 16v4z" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/>
                    <path d="M12.8 6.7l4 4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
                  </svg>
                </button>
                <button
                  v-if="connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
                  class="btn-tiny btn-conn-action btn-disconnect"
                  :title="t('sidebar.disconnect')"
                  @click.stop="disconnectConn(conn.id)"
                >
                  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path d="M7 7l10 10M17 7L7 17" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
                    <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="1.5" opacity="0.7"/>
                  </svg>
                </button>
                <InlineDeleteConfirm
                  v-if="!connectionsStore.isConnected(conn.id) && !connectionsStore.isConnecting(conn.id)"
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
          <a class="sidebar-link-icon sidebar-link-share" :title="t('sidebar.quickShare')" @click="openQuickShare()">
            <span class="sidebar-link-glyph" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" width="12" height="12" aria-hidden="true">
                <path d="M2 2L13 13" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M2 2l7 20 4-9 9-4-20-7z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </span>
          </a>
        </div>
        <div class="footer-actions">
          <button
            class="btn-icon btn-theme"
            :title="settingsStore.themeMode === 'dark' ? '切换浅色主题' : '切换暗色主题'"
            @click="settingsStore.toggleTheme()"
          >
            <span class="btn-icon-glyph" aria-hidden="true">
              <svg v-if="settingsStore.themeMode === 'dark'" viewBox="0 0 24 24" fill="none">
                <path d="M18 14.5A7.5 7.5 0 119.5 6 6 6 0 0018 14.5z" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"/>
              </svg>
              <svg v-else viewBox="0 0 24 24" fill="none">
                <circle cx="12" cy="12" r="4" stroke="currentColor" stroke-width="1.8"/>
                <path d="M12 2.5v2.2M12 19.3v2.2M21.5 12h-2.2M4.7 12H2.5M18.7 5.3l-1.6 1.6M6.9 17.1l-1.6 1.6M18.7 18.7l-1.6-1.6M6.9 6.9L5.3 5.3" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
              </svg>
            </span>
          </button>
          <button class="btn-icon" :title="t('sidebar.manageConn')" @click="openConnManager()">
            <span class="btn-icon-glyph" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none">
                <path d="M12 5v14M5 12h14" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/>
              </svg>
            </span>
          </button>
          <button class="btn-icon btn-settings" :title="t('sidebar.settings')" @click="openSettings()">
            <span class="btn-icon-glyph" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none">
                <path d="M12 8.4A3.6 3.6 0 1112 15.6 3.6 3.6 0 0112 8.4z" stroke="currentColor" stroke-width="1.7"/>
                <path d="M4.5 13.5v-3l2.1-.6a5.9 5.9 0 01.7-1.6L6.2 6.3l2.1-2.1 2 1.1c.5-.3 1-.5 1.6-.7l.6-2.1h3l.6 2.1c.6.2 1.1.4 1.6.7l2-1.1 2.1 2.1-1.1 2c.3.5.5 1 .7 1.6l2.1.6v3l-2.1.6a5.9 5.9 0 01-.7 1.6l1.1 2-2.1 2.1-2-1.1c-.5.3-1 .5-1.6.7l-.6 2.1h-3l-.6-2.1a5.9 5.9 0 01-1.6-.7l-2 1.1-2.1-2.1 1.1-2a5.9 5.9 0 01-.7-1.6l-2.1-.6z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
              </svg>
            </span>
          </button>
          <button
            class="btn-icon btn-collapse"
            :title="t('sidebar.collapse')"
            :disabled="!hasConnections"
            @click="hasConnections && (sidebarCollapsed = true)"
          >
            <span class="btn-icon-glyph" aria-hidden="true">
              <svg viewBox="0 0 16 16" fill="none">
                <path d="M10 3.5L5.5 8l4.5 4.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </span>
          </button>
        </div>
      </div>
      </div>
    </div>

    <!-- 右键菜单：必须 Teleport 到 body，否则会被 .sidebar-layer 的 will-change 层叠上下文困住 -->
    <Teleport to="body">
      <div
        v-if="ctxMenu.visible"
        class="ctx-menu pop-in"
        :class="'theme-' + resolvedThemeMode"
        :style="{ top: ctxMenu.y + 'px', left: ctxMenu.x + 'px' }"
        @click.stop
      >
        <div class="ctx-item" @click="openEdit(ctxMenu.conn); ctxMenu.visible = false">{{ t('sidebar.edit') }}</div>
        <div class="ctx-divider" />
        <div class="ctx-item ctx-danger" @click="removeConnection(ctxMenu.conn.id); ctxMenu.visible = false">{{ t('sidebar.delete') }}</div>
      </div>
    </Teleport>

    <!-- 诊断菜单：概览 / 内存体检 / 后续监控 -->
    <Teleport to="body">
      <div
        v-if="diagMenu.visible"
        class="ctx-menu diag-menu"
        :class="'theme-' + resolvedThemeMode"
        :style="{ top: diagMenu.y + 'px', left: diagMenu.x + 'px' }"
        @click.stop
      >
        <div
          v-if="diagMenu.conn && !diagMenu.conn.is_cluster"
          class="ctx-item diag-item"
          @click="pickDiag('overview')"
        >
          <svg class="diag-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M4 6.5h7v5H4zM13 6.5h7v5h-7zM4 12.5h7V18H4zM13 12.5h7V18h-7z" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/>
          </svg>
          <span>{{ t('sidebar.connectionOverview') }}</span>
        </div>
        <div class="ctx-item diag-item" @click="pickDiag('memory')">
          <svg class="diag-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M3 12h3.4l2.2-5.6 3.7 11.2 2.3-5.6H21" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span>{{ t('sidebar.memoryDoctor') }}</span>
        </div>
        <div class="ctx-item diag-item" @click="pickDiag('pubsub')">
          <svg class="diag-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M4 10.5a11 11 0 0116 0" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
            <path d="M7 13.5a7 7 0 0110 0" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
            <circle cx="12" cy="17.5" r="1.6" fill="currentColor"/>
          </svg>
          <span>{{ t('sidebar.pubsub') }}</span>
        </div>
        <div class="ctx-item diag-item" @click="pickDiag('monitor')">
          <svg class="diag-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M4 19V5M4 19h16" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
            <path d="M7 15.5l3.6-4.2 3 2.6L20 7" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span>{{ t('sidebar.realtimeMonitor') }}</span>
        </div>
      </div>
    </Teleport>

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

    <MemoryDoctorModal
      v-if="memoryConn"
      :conn-id="memoryConn.id"
      :connection-name="memoryConn.name || t('sidebar.unnamed')"
      @close="memoryConn = null"
    />

    <PubSubModal
      v-if="pubsubConn"
      :conn-id="pubsubConn.id"
      :connection-name="pubsubConn.name || t('sidebar.unnamed')"
      @close="pubsubConn = null"
    />

    <RealtimeMonitorModal
      v-if="monitorConn"
      :conn-id="monitorConn.id"
      :connection-name="monitorConn.name || t('sidebar.unnamed')"
      @close="monitorConn = null"
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
import { moveRovingFocus } from '../../utils/keyboard.js'
import InlineDeleteConfirm from '../common/InlineDeleteConfirm.vue'
import ConnectionOverviewModal from '../connections/ConnectionOverviewModal.vue'
import MemoryDoctorModal from '../connections/MemoryDoctorModal.vue'
import RealtimeMonitorModal from '../connections/RealtimeMonitorModal.vue'
import PubSubModal from '../connections/PubSubModal.vue'
import { formatDebugMessage } from '../../utils/debug.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'

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
const memoryConn = ref(null)
const diagMenu = ref({ visible: false, x: 0, y: 0, conn: null })
const monitorConn = ref(null)
const pubsubConn = ref(null)

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

// 诊断入口：把概览/体检收进同一个按钮，避免连接行按钮过多挤占名字宽度
function openDiagMenu(e, conn) {
  ctxMenu.value.visible = false
  const rect = e.currentTarget.getBoundingClientRect()
  diagMenu.value = { visible: true, x: rect.right + 6, y: rect.top - 4, conn }
}
function closeDiagMenu() { diagMenu.value.visible = false }
function pickDiag(kind) {
  const conn = diagMenu.value.conn
  diagMenu.value.visible = false
  if (!conn) return
  if (kind === 'overview') {
    openOverview(conn)
  } else if (kind === 'monitor') {
    monitorConn.value = conn
  } else if (kind === 'pubsub') {
    pubsubConn.value = conn
  } else {
    openMemory(conn)
  }
}

// 菜单打开期间接管 Esc
let disposeMenuEscape = null
watch(() => diagMenu.value.visible, (visible) => {
  if (visible) {
    disposeMenuEscape = registerEscapeHandler(closeDiagMenu)
    return
  }
  if (disposeMenuEscape) {
    disposeMenuEscape()
    disposeMenuEscape = null
  }
})

// 命令面板等外部入口通过 window 事件打开连接级面板
const CONN_PANEL_EVENTS = {
  'literedis:open-overview': 'overview',
  'literedis:open-memory': 'memory',
  'literedis:open-pubsub': 'pubsub',
  'literedis:open-monitor': 'monitor',
}

function openPanelForActiveConn(kind) {
  const conn = activeConn.value
  if (!conn || !connectionsStore.isConnected(conn.id)) return
  if (kind === 'overview') {
    // 集群连接没有概览/命令台
    if (conn.is_cluster) return
    overviewConn.value = conn
    return
  }
  if (kind === 'memory') memoryConn.value = conn
  if (kind === 'pubsub') pubsubConn.value = conn
  if (kind === 'monitor') monitorConn.value = conn
}

function handlePanelEvent(event) {
  const kind = CONN_PANEL_EVENTS[event.type]
  if (kind) openPanelForActiveConn(kind)
}

onMounted(() => {
  document.addEventListener('click', closeCtxMenu)
  document.addEventListener('click', closeDiagMenu)
  for (const name of Object.keys(CONN_PANEL_EVENTS)) {
    window.addEventListener(name, handlePanelEvent)
  }
})
onBeforeUnmount(() => {
  if (disposeMenuEscape) disposeMenuEscape()
  document.removeEventListener('click', closeCtxMenu)
  document.removeEventListener('click', closeDiagMenu)
  for (const name of Object.keys(CONN_PANEL_EVENTS)) {
    window.removeEventListener(name, handlePanelEvent)
  }
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

function openMemory(conn) {
  memoryConn.value = conn
}

function isReadOnlyConn(conn) {
  return !!(conn?.read_only ?? conn?.readOnly)
}

function openQuickShare() {
  BrowserOpenURL('https://share.litetools.top')
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

// 连接列表：Tab 停在列表上一次，↑/↓ 在连接间移动，Enter 打开
function onConnListKeydown(event) {
  moveRovingFocus(event, event.currentTarget, '.conn-item')
}

// 拖拽
const dragId = ref(null)
const landedId = ref('')
let landedTimer = null
let clickSuppressedUntil = 0
const dragOverGroup = ref(null)
const dragPreview = ref({ group: null, targetId: '', placement: 'before', mode: null })
function onDragStart(connId, event) {
  dragId.value = connId
  dragPreview.value = { group: null, targetId: '', placement: 'before', mode: null }
  // effectAllowed=move 让浏览器显示"移动"光标，拖拽意图更明确
  if (event && event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    try { event.dataTransfer.setData('text/plain', connId) } catch (e) {}
  }
  document.body.classList.add('is-conn-dragging')
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
  document.body.classList.remove('is-conn-dragging')
  // 拖拽结束后短暂屏蔽点击，避免松手那一下误触「打开连接」
  clickSuppressedUntil = Date.now() + 280
  dragOverGroup.value = null
  dragId.value = null
  dragPreview.value = { group: null, targetId: '', placement: 'before', mode: null }
}

// 落位后给被移动的连接一个短暂的落位高亮
function markLanded(connId) {
  if (!connId) return
  landedId.value = connId
  if (landedTimer) clearTimeout(landedTimer)
  landedTimer = setTimeout(() => {
    landedId.value = ''
    landedTimer = null
  }, 360)
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
  const movingId = dragId.value
  if (!movingId) return
  if (movingId !== targetId) {
    const result = await connectionsStore.moveConnection(movingId, targetId, targetGroup, placement)
    if (!result || result.success !== false) markLanded(movingId)
  }
  onDragEnd()
}

async function handleConnect(conn) {
  // 刚拖拽完的 280ms 内忽略点击，避免松手误触打开连接
  if (Date.now() < clickSuppressedUntil) return
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
      if (!conn.is_cluster || conn.allow_cluster_scan) {
        await workspaceStore.search('*')
      }
      workspaceStore.fetchTotalKeys()
    }
    return
  }
  const result = await connectionsStore.connect(conn.id)
  if (result?.message === 'connecting') return
  if (result.success) {
    const restored = workspaceStore.setActiveConn(conn.id, conn.name, result.init_db || 0)
    workspaceStore.keyValueError = null
    workspaceStore.keyValueLoading = false
    if (restored) {
      await workspaceStore.refreshAfterReconnect(conn.id)
    } else {
      if (!conn.is_cluster || conn.allow_cluster_scan) {
        await workspaceStore.search('*')
      }
      workspaceStore.fetchTotalKeys()
    }
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
  transition: opacity var(--motion-slow) ease, transform var(--motion-slow) ease;
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
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  flex-shrink: 0;
  transition: opacity var(--motion-base), box-shadow var(--motion-base);
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
  background: transparent;
  color: #64748b;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  width: 28px;
  height: 28px;
  font-size: 11px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  margin-top: 6px;
  box-shadow: none;
  transition: background var(--motion-base) ease, color var(--motion-base) ease, border-color var(--motion-base) ease;
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
.btn-expand-glyph svg {
  width: 12px;
  height: 12px;
  display: block;
}
.btn-expand-bottom:hover {
  background: #eff6ff;
  color: #2563eb;
  border-color: #93c5fd;
}
.sidebar.theme-dark .btn-expand-bottom {
  background: transparent;
  color: #94a3b8;
  border-color: transparent;
  box-shadow: none;
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
  border-radius: var(--radius-sm);
  transition: color var(--motion-base), background var(--motion-base);
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
  transition: transform var(--motion-base) ease;
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
.sidebar-link-share { color: inherit; }
.footer-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.btn-icon {
  background: transparent;
  color: #64748b;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  width: 24px; height: 24px;
  min-width: 24px; min-height: 24px;
  font-size: 14px; line-height: 1;
  cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  box-sizing: border-box;
  overflow: hidden;
  box-shadow: none;
  transition: background var(--motion-base), color var(--motion-base), border-color var(--motion-base), box-shadow var(--motion-base);
}
.btn-icon:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(239, 246, 255, 0.96));
  color: #2563eb;
  border-color: rgba(147, 197, 253, 0.92);
  box-shadow: none;
}
.btn-icon-glyph {
  width: 14px;
  height: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  transform-origin: center center;
  transition: transform var(--motion-base) ease;
}
.btn-icon-glyph svg {
  width: 14px;
  height: 14px;
  display: block;
  flex-shrink: 0;
}
.btn-icon:hover .btn-icon-glyph {
  transform: scale(1.08);
}
.btn-settings {
  color: #5b6b82;
}
.btn-settings:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(238, 246, 255, 0.98));
  color: #2563eb;
}
.btn-theme {
  color: #ca8a04;
  box-shadow: none;
}
.btn-theme:hover {
  background: linear-gradient(180deg, rgba(255, 255, 255, 1), rgba(254, 240, 138, 0.98));
  color: #a16207;
  box-shadow: none;
}
.btn-collapse {
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
  border-color: transparent;
  background: transparent;
  box-shadow: none;
}
.btn-icon:disabled .btn-icon-glyph {
  transform: none !important;
}
.btn-icon:disabled:hover {
  color: #a8b4c5;
  border-color: transparent;
  background: transparent;
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
  border-radius: var(--radius-md);
  margin: 2px 8px;
  position: relative;
  overflow: visible;
  border: 1px solid transparent;
  transition: background var(--motion-fast), border-color var(--motion-fast), box-shadow var(--motion-fast);
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
/* 落位高亮：松开鼠标后被移动的连接会闪一下，确认"落在这里了" */
.conn-item.drop-landed {
  animation: connLanded 320ms var(--ease-out);
}
@keyframes connLanded {
  0% {
    background: rgba(96, 165, 250, 0.26);
    box-shadow: 0 0 0 1px rgba(96, 165, 250, 0.45);
  }
  100% {
    background: transparent;
    box-shadow: none;
  }
}
.conn-lock {
  width: 11px;
  height: 11px;
  margin-right: 3px;
  vertical-align: -1px;
  color: #f59e0b;
  flex-shrink: 0;
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
  transform-origin: left center;
  animation: dropLineIn var(--motion-fast) var(--ease-out);
  background: #69b1ff;
  border-radius: var(--radius-pill);
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
@keyframes dropLineIn {
  from {
    transform: scaleX(0.2);
    opacity: 0.4;
  }
  to {
    transform: scaleX(1);
    opacity: 1;
  }
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
  border-radius: var(--radius-xs);
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
/* 悬浮覆盖：悬停时按钮不再参与布局，避免把连接名挤窄 */
.conn-actions {
  display: none;
  position: absolute;
  right: 5px;
  top: 50%;
  transform: translateY(-50%);
  gap: 2px;
  padding: 2px 3px;
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 2px 10px rgba(15, 23, 42, 0.14);
  z-index: 2;
}
.sidebar.theme-dark .conn-actions {
  background: rgba(30, 41, 59, 0.96);
  box-shadow: 0 2px 10px rgba(2, 6, 23, 0.5);
}
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
  border-radius: var(--radius-sm);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 22px;
  line-height: 1;
}
.btn-conn-action {
  font-size: 11px;
}
.btn-conn-action svg {
  width: 12px;
  height: 12px;
  display: block;
  flex-shrink: 0;
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
  border-radius: var(--radius-sm);
  padding: 1px 5px;
  font-size: 11px;
  cursor: pointer;
  transition: background var(--motion-base) ease, border-color var(--motion-base) ease, color var(--motion-base) ease;
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
  border-radius: var(--radius-xs);
  transition: background var(--motion-base);
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
  border-radius: var(--radius-pill);
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
  border-radius: var(--radius-md);
  margin: 1px 8px;
  user-select: none;
  transition: background var(--motion-base), border-color var(--motion-base);
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
.diag-menu {
  min-width: 168px;
  padding: 5px 0;
}

.diag-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.diag-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 14px;
  opacity: 0.72;
}

.diag-item:hover .diag-icon {
  opacity: 1;
}

.ctx-menu {
  position: fixed;
  z-index: 2000;
  background: white;
  border: 1px solid #e0e0e0;
  border-radius: var(--radius-sm);
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
.ctx-menu.theme-dark {
  background: rgba(15, 23, 42, 0.98);
  border-color: rgba(71, 85, 105, 0.92);
  box-shadow: 0 18px 40px rgba(2, 6, 23, 0.48);
}
.ctx-menu.theme-dark .ctx-item {
  color: #e2e8f0;
}
.ctx-menu.theme-dark .ctx-item:hover {
  background: rgba(30, 41, 59, 0.92);
}
.ctx-menu.theme-dark .ctx-danger {
  color: #fca5a5;
}
.ctx-menu.theme-dark .ctx-danger:hover {
  background: rgba(127, 29, 29, 0.56);
  color: #fecaca;
}
.ctx-menu.theme-dark .ctx-divider {
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
.sidebar.platform-windows,
:global(.platform-windows) .sidebar {
  -webkit-font-smoothing: auto;
  text-rendering: geometricPrecision;
}

:global(.platform-windows) .sidebar .conn-name {
  font-weight: 500;
  letter-spacing: 0;
}

:global(.platform-windows) .sidebar .btn-icon-glyph,
:global(.platform-windows) .sidebar .btn-expand-glyph,
:global(.platform-windows) .sidebar .btn-conn-action svg,
:global(.platform-windows) .sidebar .sidebar-link-glyph svg {
  transform: translateZ(0);
}
.toast-enter-active, .toast-leave-active { transition: opacity 0.25s, transform 0.25s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(-12px); }

</style>
