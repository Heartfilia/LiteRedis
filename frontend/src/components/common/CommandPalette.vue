<template>
  <Teleport to="body">
    <div class="cmdk-overlay" :class="'theme-' + theme" @mousedown.self="$emit('close')">
      <div class="cmdk-card">
        <div class="cmdk-input-row">
          <svg class="cmdk-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="11" cy="11" r="6.5" stroke="currentColor" stroke-width="1.7" />
            <path d="M16 16l4 4" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
          </svg>
          <input
            ref="inputRef"
            v-model="query"
            class="cmdk-input"
            :placeholder="t('cmdk.placeholder')"
            spellcheck="false"
            @keydown.down.prevent="move(1)"
            @keydown.up.prevent="move(-1)"
            @keydown.enter.prevent="run()"
            @keydown.esc.prevent="$emit('close')"
          />
          <span class="cmdk-count">{{ results.length }}</span>
        </div>

        <div ref="listRef" class="cmdk-list">
          <div v-if="!results.length" class="cmdk-empty">{{ t('cmdk.empty') }}</div>
          <template v-for="group in groupedResults" :key="group.title">
            <div class="cmdk-group-title">{{ group.title }}</div>
            <button
              v-for="item in group.items"
              :key="item.id"
              type="button"
              class="cmdk-item"
              :class="{ active: item.index === activeIndex, disabled: item.disabled }"
              :aria-disabled="item.disabled ? 'true' : undefined"
              @mouseenter="activeIndex = item.index"
              @click="run(item.index)"
            >
              <span class="cmdk-item-label">{{ item.label }}</span>
              <span v-if="item.hint" class="cmdk-item-hint">{{ item.hint }}</span>
            </button>
          </template>
        </div>

        <div class="cmdk-footer">
          <span>↑↓ {{ t('cmdk.nav') }}</span>
          <span>Enter {{ t('cmdk.run') }}</span>
          <span>Esc {{ t('cmdk.close') }}</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, inject, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { getDatabases, getCommandHistory, scanKeys } from '../../api/wails.js'

const emit = defineEmits(['close'])

const connectionsStore = useConnectionsStore()
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
const openConnManager = inject('openConnManager', null)
const openSettings = inject('openSettings', null)

const theme = computed(() => settingsStore.themeMode || 'light')
const query = ref('')
const activeIndex = ref(0)
const inputRef = ref(null)
const listRef = ref(null)

// 最近在命令台执行过的命令
const commandHistory = ref([])
// 输入时按 pattern 去库里扫出来的 key
const keyMatches = ref([])
let keySearchTimer = null
let keySearchSeq = 0

// Redis glob 里 * ? [ ] \ 都是特殊字符，搜索词里出现时要转义
function escapeGlob(text) {
  let out = ''
  for (const ch of text) {
    if (ch === '*' || ch === '?' || ch === '[' || ch === ']' || ch === '\\') out += '\\'
    out += ch
  }
  return out
}

async function searchKeys(text) {
  const seq = ++keySearchSeq
  try {
    const result = await scanKeys(workspaceStore.activeConnID, '*' + escapeGlob(text) + '*', 20, 0)
    if (seq !== keySearchSeq) return
    keyMatches.value = (result?.keys || []).map((key) => key.name)
  } catch (e) {
    if (seq === keySearchSeq) keyMatches.value = []
  }
}

watch(query, (value) => {
  activeIndex.value = 0
  const text = value.trim()
  if (keySearchTimer) {
    clearTimeout(keySearchTimer)
    keySearchTimer = null
  }
  // 连线上库扫 key 是全量 SCAN，只在够长且已连接时才触发，并做防抖
  if (!connReady.value || text.length < 2) {
    keySearchSeq++
    keyMatches.value = []
    return
  }
  keySearchTimer = setTimeout(() => {
    keySearchTimer = null
    searchKeys(text)
  }, 260)
})

watch(() => workspaceStore.activeConnID, () => {
  keySearchSeq++
  keyMatches.value = []
})

function close() {
  emit('close')
}

const activeConn = computed(() => {
  const id = workspaceStore.activeConnID
  return (connectionsStore.connections || []).find((conn) => conn.id === id) || null
})
const connReady = computed(() => !!activeConn.value && connectionsStore.isConnected(activeConn.value.id))
const activeKey = computed(() => workspaceStore.selectedKey || '')

// 连接级面板由 Sidebar 里的监听器打开，这里只负责发事件
function emitUiEvent(name) {
  window.dispatchEvent(new CustomEvent(name))
}

async function copyActiveKey() {
  if (!activeKey.value) return
  const ok = await copyToClipboard(activeKey.value)
  connectionsStore.showGlobalToast(ok ? t('cmdk.copiedKey') : t('cmdk.copyFailed'), ok)
}

// 动作：只放全局能直接执行的，避免引入跨组件的事件总线
const actions = computed(() => {
  const items = []
  if (openConnManager) {
    items.push({
      id: 'action:new-conn',
      label: t('cmdk.newConnection'),
      hint: t('cmdk.hintConnection'),
      run: () => openConnManager(),
    })
  }
  if (openSettings) {
    items.push({
      id: 'action:settings',
      label: t('cmdk.openSettings'),
      hint: t('cmdk.hintSettings'),
      run: () => openSettings(),
    })
  }
  const connHint = connReady.value
    ? activeConn.value.name || workspaceStore.activeConnID
    : t('cmdk.needConnection')
  items.push({
    id: 'action:console',
    label: t('cmdk.openConsole'),
    hint: connReady.value && activeConn.value.is_cluster ? t('cmdk.clusterUnsupported') : connHint,
    disabled: !connReady.value || !!activeConn.value.is_cluster,
    run: () => emitUiEvent('literedis:open-overview'),
  })
  items.push({
    id: 'action:pubsub',
    label: t('cmdk.openPubSub'),
    hint: connHint,
    disabled: !connReady.value,
    run: () => emitUiEvent('literedis:open-pubsub'),
  })
  items.push({
    id: 'action:monitor',
    label: t('cmdk.openMonitor'),
    hint: connHint,
    disabled: !connReady.value,
    run: () => emitUiEvent('literedis:open-monitor'),
  })
  items.push({
    id: 'action:memory',
    label: t('cmdk.openMemory'),
    hint: connHint,
    disabled: !connReady.value,
    run: () => emitUiEvent('literedis:open-memory'),
  })
  items.push({
    id: 'action:diagnostics',
    label: t('cmdk.openDiagnostics'),
    hint: t('cmdk.hintDiagnostics'),
    run: () => emitUiEvent('literedis:open-diagnostics'),
  })
  items.push({
    id: 'action:copy-key',
    label: t('cmdk.copyKey'),
    hint: activeKey.value || t('cmdk.needKey'),
    disabled: !activeKey.value,
    run: () => copyActiveKey(),
  })
  items.push({
    id: 'action:refresh',
    label: t('cmdk.refreshKey'),
    hint: '⌘R',
    run: () => emitUiEvent('literedis:refresh-key'),
  })
  items.push({
    id: 'action:theme',
    label: t('cmdk.toggleTheme'),
    hint: settingsStore.themeMode === 'dark' ? t('cmdk.themeLight') : t('cmdk.themeDark'),
    run: () => settingsStore.toggleTheme(),
  })
  return items
})

const connectionItems = computed(() =>
  (connectionsStore.connections || []).map((conn) => ({
    id: 'conn:' + conn.id,
    label: conn.name || conn.id,
    hint: connectionsStore.isConnected(conn.id) ? t('cmdk.connected') : t('cmdk.disconnected'),
    run: () => {
      if (connectionsStore.isConnected(conn.id)) {
        workspaceStore.setActiveConn(conn.id, conn.name, conn.db || 0)
      } else {
        connectionsStore.connect(conn.id)
      }
    },
  }))
)

const dbCount = ref(16)
const databaseItems = computed(() => {
  const total = Math.min(Number(dbCount.value) || 16, 64)
  return Array.from({ length: total }, (_, db) => ({
    id: 'db:' + db,
    label: t('cmdk.switchDb').replace('{db}', db),
    hint: db === (workspaceStore.currentDB || 0) ? t('cmdk.current') : '',
    run: () => workspaceStore.switchDB(db),
  }))
})

const recentKeyItems = computed(() =>
  (workspaceStore.recentKeyList || []).map((key) => ({
    id: 'key:' + key,
    label: key,
    hint: t('cmdk.jumpKey'),
    run: () => workspaceStore.selectKey(key),
  }))
)

// 输入时从当前连接扫出来的 key
const keyMatchItems = computed(() =>
  (keyMatches.value || []).map((key) => ({
    id: 'match:' + key,
    label: key,
    hint: t('cmdk.openKey'),
    run: () => workspaceStore.selectKey(key),
  }))
)

// 最近在命令台执行过的命令，点一下直接打开命令台执行
const commandItems = computed(() => {
  const cluster = !!activeConn.value?.is_cluster
  const hint = !connReady.value
    ? t('cmdk.needConnection')
    : cluster
      ? t('cmdk.clusterUnsupported')
      : t('cmdk.runInConsole')
  return (commandHistory.value || []).map((command) => ({
    id: 'cmd:' + command,
    label: command,
    hint,
    disabled: !connReady.value || cluster,
    run: () => window.dispatchEvent(new CustomEvent('literedis:run-command', { detail: { command } })),
  }))
})

function matches(label, hint, q) {
  if (!q) return true
  const text = (label + ' ' + (hint || '')).toLowerCase()
  return text.includes(q)
}

const results = computed(() => {
  const q = query.value.trim().toLowerCase()
  const sections = [
    { title: t('cmdk.groupActions'), items: actions.value.filter((item) => matches(item.label, item.hint, q)) },
    // 命令只在搜索时出现，避免一打开面板就混进一堆历史命令
    { title: t('cmdk.groupCommands'), items: q ? commandItems.value.filter((item) => matches(item.label, '', q)) : [] },
    { title: t('cmdk.groupKeyMatch'), items: keyMatchItems.value.filter((item) => matches(item.label, '', q)) },
    { title: t('cmdk.groupConnections'), items: connectionItems.value.filter((item) => matches(item.label, item.hint, q)) },
    { title: t('cmdk.groupDatabases'), items: q ? [] : databaseItems.value.slice(0, 16) },
    { title: t('cmdk.groupRecent'), items: recentKeyItems.value.filter((item) => matches(item.label, '', q)) },
  ]
  let index = 0
  const flat = []
  for (const section of sections) {
    if (!section.items.length) continue
    section.items = section.items.map((item) => ({ ...item, group: section.title }))
    for (const item of section.items) {
      flat.push({ ...item, index: index++ })
    }
  }
  return flat
})

const groupedResults = computed(() => {
  const groups = []
  for (const item of results.value) {
    const last = groups[groups.length - 1]
    if (last && last.title === item.group) {
      last.items.push(item)
    } else {
      groups.push({ title: item.group, items: [item] })
    }
  }
  return groups
})

function move(delta) {
  const total = results.value.length
  if (!total) return
  // 跳过暂时用不了的条目（比如没连接时的连接级面板）
  for (let step = 0; step < total; step++) {
    const next = (activeIndex.value + delta + total) % total
    activeIndex.value = next
    if (!results.value[next].disabled) break
  }
  scrollActiveIntoView()
}

function scrollActiveIntoView() {
  nextTick(() => {
    const el = listRef.value?.querySelector('.cmdk-item.active')
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'nearest' })
    }
  })
}

function run(index) {
  const current = results.value[typeof index === 'number' ? index : activeIndex.value]
  if (!current) return
  if (current.disabled) {
    // 面板留着，只提示为什么不能执行
    connectionsStore.showGlobalToast(current.hint || t('cmdk.needConnection'), false)
    return
  }
  close()
  try {
    current.run()
  } catch (e) {
    // 动作失败不应该让面板卡住
  }
}

let disposeEscape = null
onMounted(async () => {
  disposeEscape = registerEscapeHandler(close)
  nextTick(() => inputRef.value && inputRef.value.focus())
  try {
    const list = await getCommandHistory()
    if (Array.isArray(list)) commandHistory.value = list
  } catch (e) {
    // 读不到历史不影响面板其他功能
  }
  if (workspaceStore.activeConnID) {
    try {
      const count = await getDatabases(workspaceStore.activeConnID)
      if (count > 0) dbCount.value = count
    } catch (e) {
      // 读不到就按 16 个兜底
    }
  }
})
onBeforeUnmount(() => {
  if (disposeEscape) disposeEscape()
  if (keySearchTimer) {
    clearTimeout(keySearchTimer)
    keySearchTimer = null
  }
  keySearchSeq++
})
</script>

<style scoped>
.cmdk-overlay {
  position: fixed;
  inset: 0;
  z-index: 10085;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 14vh;
  background: rgba(15, 23, 42, 0.38);
  animation: cmdkFade 140ms ease;
}
@keyframes cmdkFade {
  from { background: rgba(15, 23, 42, 0); }
  to { background: rgba(15, 23, 42, 0.38); }
}
.cmdk-card {
  width: min(620px, 92vw);
  max-height: 62vh;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-lg);
  background: #fff;
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: var(--shadow-lg);
  overflow: hidden;
  animation: cmdkPop 160ms var(--ease-out);
}
@keyframes cmdkPop {
  from { opacity: 0; transform: translateY(-8px) scale(0.99); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.cmdk-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
}
.cmdk-icon {
  width: 15px;
  height: 15px;
  color: #94a3b8;
  flex: 0 0 auto;
}
.cmdk-input {
  flex: 1;
  min-width: 0;
  height: 22px;
  border: none;
  background: transparent;
  font-size: 13px;
  color: #0f172a;
  outline: none;
}
.cmdk-count {
  flex: 0 0 auto;
  font-size: 10.5px;
  color: #94a3b8;
  font-variant-numeric: tabular-nums;
}
.cmdk-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px;
}
.cmdk-group-title {
  padding: 8px 8px 4px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: #94a3b8;
  text-transform: uppercase;
}
.cmdk-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 7px 10px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  text-align: left;
  cursor: pointer;
}
.cmdk-item.active {
  background: rgba(239, 246, 255, 0.96);
}
.cmdk-item.disabled .cmdk-item-label {
  color: #a2aec0;
}
.cmdk-item.disabled .cmdk-item-hint {
  color: #c3cdd9;
}
.cmdk-item-label {
  flex: 1;
  min-width: 0;
  font-size: 12.5px;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cmdk-item.active .cmdk-item-label {
  color: #1d4ed8;
}
.cmdk-item-hint {
  flex: 0 0 auto;
  font-size: 10.5px;
  color: #94a3b8;
}
.cmdk-empty {
  padding: 22px 10px;
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
}
.cmdk-footer {
  display: flex;
  gap: 14px;
  padding: 8px 14px;
  border-top: 1px solid rgba(226, 232, 240, 0.9);
  background: rgba(248, 250, 252, 0.96);
  font-size: 10.5px;
  color: #94a3b8;
}
/* 深色 */
.cmdk-overlay.theme-dark .cmdk-card {
  background: rgba(15, 23, 42, 0.99);
  border-color: rgba(51, 65, 85, 0.92);
}
.cmdk-overlay.theme-dark .cmdk-input-row {
  border-bottom-color: rgba(51, 65, 85, 0.94);
}
.cmdk-overlay.theme-dark .cmdk-input { color: #e2e8f0; }
.cmdk-overlay.theme-dark .cmdk-item.active { background: rgba(30, 64, 175, 0.32); }
.cmdk-overlay.theme-dark .cmdk-item-label { color: #cbd5e1; }
.cmdk-overlay.theme-dark .cmdk-item.disabled .cmdk-item-label { color: #64748b; }
.cmdk-overlay.theme-dark .cmdk-item.disabled .cmdk-item-hint { color: #526379; }
.cmdk-overlay.theme-dark .cmdk-item.active .cmdk-item-label { color: #dbeafe; }
.cmdk-overlay.theme-dark .cmdk-footer {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: rgba(11, 18, 32, 0.96);
}
</style>
