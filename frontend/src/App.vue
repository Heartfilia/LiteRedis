<template>
  <div class="app-layout" :class="[fontSizeClass, `theme-${settingsStore.themeMode || 'light'}`]">
    <Sidebar :theme-mode="settingsStore.themeMode || 'light'" />
    <MainContent />
    <ConnectionManager
      v-if="showConnManager"
      :initial-connection="connManagerInitialConnection"
      @close="showConnManager = false"
    />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
    <CommandPalette v-if="showPalette" @close="showPalette = false" />
    <WelcomeWizard
      v-if="showWelcome"
      @skip="dismissWelcome(false)"
      @create="dismissWelcome(true)"
    />
    <WhatsNewModal
      v-if="showWhatsNew"
      :version="whatsNew.version"
      :body="whatsNew.body"
      :url="whatsNew.url"
      @close="closeWhatsNew"
    />
    <Teleport to="body">
      <Transition name="app-toast">
        <div
          v-if="connectionsStore.globalToast"
          class="app-toast"
          :class="[connectionsStore.globalToastOk ? 'ok' : 'err', `theme-${settingsStore.themeMode || 'light'}`]"
        >
          {{ connectionsStore.globalToast }}
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount, ref, provide, Teleport, Transition, watch } from 'vue'
import { installGlobalShortcuts } from './utils/shortcuts.js'
import Sidebar from './components/layout/Sidebar.vue'
import MainContent from './components/layout/MainContent.vue'
import ConnectionManager from './components/connections/ConnectionManager.vue'
import WhatsNewModal from './components/common/WhatsNewModal.vue'
import WelcomeWizard from './components/common/WelcomeWizard.vue'
import CommandPalette from './components/common/CommandPalette.vue'
import SettingsModal from './components/settings/SettingsModal.vue'
import { useConnectionsStore } from './stores/connections.js'
import { useSettingsStore } from './stores/settings.js'
import { getAppVersion, getReleaseNotes, markVersionSeen, completeOnboarding } from './api/wails.js'
import { RELEASE_NOTES } from './changelog.js'

const connectionsStore = useConnectionsStore()
const settingsStore = useSettingsStore()

const showConnManager = ref(false)
const showSettings = ref(false)
const connManagerInitialConnection = ref(null)
const fontSizeClass = computed(() => `font-${settingsStore.fontSizeLevel || 'small'}`)

provide('openConnManager', (connection = null) => {
  connManagerInitialConnection.value = connection
  showConnManager.value = true
})
provide('openSettings', () => { showSettings.value = true })

// ---- 每个版本第一次打开时提示「更新内容」 ----
const showWhatsNew = ref(false)
const whatsNew = ref({ version: '', body: '', url: '' })

// 版本号归一化：去掉前缀 v，便于和 changelog 里的 key 对应
function notesKey(version) {
  return String(version || '').replace(/^v/i, '').trim()
}

// changelog 里最后加的一条，开发环境每次启动都用它预览
function latestLocalNotes() {
  const keys = Object.keys(RELEASE_NOTES)
  if (!keys.length) return ''
  return RELEASE_NOTES[keys[keys.length - 1]] || ''
}

async function maybeShowWhatsNew() {
  let version = ''
  try {
    version = String((await getAppVersion()) || '').trim()
  } catch (e) {
    return
  }
  // 开发版（编译期 AppVersion 为 dev）：每次启动都弹，方便确认文案和样式
  if (!version || version.toLowerCase() === 'dev') {
    const body = latestLocalNotes()
    if (!body) return
    whatsNew.value = { version: '', body, url: '' }
    showWhatsNew.value = true
    return
  }
  if (settingsStore.lastSeenVersion === version) return
  // 本地 changelog 优先，不受网络与 GitHub 限流影响
  const localNotes = RELEASE_NOTES[notesKey(version)]
  if (localNotes) {
    whatsNew.value = {
      version: notesKey(version),
      body: localNotes,
      url: 'https://github.com/Heartfilia/LiteRedis/releases/tag/v' + notesKey(version),
    }
    showWhatsNew.value = true
    return
  }
  try {
    const notes = await getReleaseNotes(version)
    // 拉不到说明就什么都不做，也不标记版本，下次启动继续尝试
    if (!notes || !notes.found || !String(notes.body || '').trim()) return
    whatsNew.value = { version, body: notes.body, url: notes.url || '' }
    showWhatsNew.value = true
  } catch (e) {
    // 静默失败，不打扰用户
  }
}

async function closeWhatsNew() {
  showWhatsNew.value = false
  const version = whatsNew.value.version
  // 开发预览不记录，下次启动继续弹
  if (!version) return
  try {
    await markVersionSeen(version)
    settingsStore.lastSeenVersion = version
  } catch (e) {
    // 写失败最多下次再弹一次，不影响使用
  }
}

// ---- Cmd/Ctrl+K 命令面板 ----
const showPalette = ref(false)
function togglePalette() {
  showPalette.value = !showPalette.value
}

// ---- 首次使用向导（只在完全没有连接时出现，看过/跳过都不再弹） ----
const showWelcome = ref(false)

async function maybeShowWelcome() {
  if (settingsStore.onboardingDone) return false
  if ((connectionsStore.connections || []).length > 0) {
    // 已有连接说明是老用户，直接标记已看过，避免以后删空连接又弹出来
    try { await completeOnboarding() } catch (e) {}
    settingsStore.onboardingDone = true
    return false
  }
  showWelcome.value = true
  return true
}

async function dismissWelcome(openCreate) {
  showWelcome.value = false
  try {
    await completeOnboarding()
    settingsStore.onboardingDone = true
  } catch (e) {}
  if (openCreate) showConnManager.value = true
}

onMounted(async () => {
  // Cmd/Ctrl+R 刷新当前 key，Esc 关闭最上层浮层
  installGlobalShortcuts()
  await Promise.all([connectionsStore.loadConnections(), settingsStore.load()])
  // 屏蔽原生右键菜单（输入框/可编辑区域保留，编辑器行首的自定义菜单另行处理）
  document.addEventListener('contextmenu', suppressContextMenu)
  window.addEventListener('literedis:command-palette', togglePalette)
  // 新用户先看引导；已经弹过引导就不再叠加「更新内容」弹窗
  const welcomed = await maybeShowWelcome()
  if (!welcomed) maybeShowWhatsNew()
})

function suppressContextMenu(e) {
  const target = e.target
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) {
    return
  }
  e.preventDefault()
}

onBeforeUnmount(() => {
  document.removeEventListener('contextmenu', suppressContextMenu)
  window.removeEventListener('literedis:command-palette', togglePalette)
})

let toastTimer = null
watch(() => connectionsStore.globalToast, (message) => {
  if (toastTimer) {
    clearTimeout(toastTimer)
    toastTimer = null
  }
  if (message) {
    toastTimer = setTimeout(() => {
      connectionsStore.clearGlobalToast()
      toastTimer = null
    }, 2600)
  }
})

onBeforeUnmount(() => {
  if (toastTimer) {
    clearTimeout(toastTimer)
    toastTimer = null
  }
})
</script>

<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: 'HarmonyOS Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif; }
.app-layout {
  --ui-font-title: 13px;
  --ui-font-body: 12px;
  --ui-font-caption: 11px;
  --ui-font-badge: 10px;
  --app-shell-gap: 6px;
  --app-shell-inner-gap: 2px;
  --app-panel-radius: 12px;
  --app-text: #0f172a;
  --app-muted: #64748b;
  --app-panel-border: rgba(214, 224, 236, 0.9);
  --app-panel-bg: rgba(255, 255, 255, 0.92);
  --app-sidebar-bg: linear-gradient(180deg, rgba(244, 248, 252, 0.98), rgba(236, 242, 249, 0.98));
  position: relative;
  display: flex;
  height: 100vh;
  min-width: 1220px;
  min-height: 720px;
  overflow: hidden;
  background:
    radial-gradient(circle at top left, rgba(226, 239, 255, 0.72), transparent 34%),
    radial-gradient(circle at right 18%, rgba(215, 244, 236, 0.55), transparent 28%),
    linear-gradient(180deg, #f7fafc 0%, #eef3f9 100%);
  color: var(--app-text);
}
.app-layout.theme-dark {
  --app-text: #e2e8f0;
  --app-muted: #a8b6c9;
  --app-panel-border: rgba(51, 65, 85, 0.92);
  --app-panel-bg: rgba(15, 23, 42, 0.985);
  --app-sidebar-bg:
    radial-gradient(circle at top left, rgba(37, 99, 235, 0.1), transparent 26%),
    linear-gradient(180deg, rgba(15, 23, 42, 0.995), rgba(11, 18, 32, 0.995));
  background:
    radial-gradient(circle at top left, rgba(37, 99, 235, 0.18), transparent 28%),
    radial-gradient(circle at right 18%, rgba(20, 184, 166, 0.12), transparent 24%),
    linear-gradient(180deg, #0b1220 0%, #0f172a 100%);
}
.app-layout.font-medium {
  --ui-font-title: 14px;
  --ui-font-body: 13px;
  --ui-font-caption: 12px;
  --ui-font-badge: 11px;
}
.app-layout.font-large {
  --ui-font-title: 15px;
  --ui-font-body: 14px;
  --ui-font-caption: 13px;
  --ui-font-badge: 12px;
}
.app-layout,
.app-layout input,
.app-layout select,
.app-layout textarea,
.app-layout button {
  font-size: var(--ui-font-body);
}
.app-layout .settings-header,
.app-layout .cm-title,
.app-layout .key-name,
.app-layout .conn-name,
.app-layout .node-row,
.app-layout .label-text,
.app-layout h3,
.app-layout .cluster-empty-title {
  font-size: var(--ui-font-title) !important;
}
.app-layout .label-hint,
.app-layout .unit,
.app-layout .conn-host,
.app-layout .keep-label,
.app-layout .cluster-hint,
.app-layout .ttl-info,
.app-layout .btn-sm,
.app-layout .btn-xs,
.app-layout .btn-tiny,
.app-layout .btn-retry,
.app-layout .count,
.app-layout .rename-msg,
.app-layout .section-tip,
.app-layout .section-status,
.app-layout .entry-id,
.app-layout .field-key,
.app-layout .field-val,
.app-layout .val-ellipsis,
.app-layout .db-label,
.app-layout .key-count {
  font-size: var(--ui-font-caption) !important;
}
.app-layout .type-badge,
.app-layout .ttl-badge,
.app-layout .sort-icon,
.app-layout .node-count,
.app-layout .expand-icon,
.app-layout .group-arrow,
.app-layout .num-cell,
.app-layout .idx-badge,
.app-layout .num-badge,
.app-layout .section-title,
.app-layout .connecting-inline,
.app-layout .group-count {
  font-size: var(--ui-font-badge) !important;
}
.app-toast {
  position: fixed;
  top: 18px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 6000;
  padding: 10px 14px;
  border-radius: 11px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.01em;
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.14);
  backdrop-filter: blur(14px);
  border: 1px solid transparent;
  max-width: min(520px, calc(100vw - 24px));
  word-break: break-word;
  transform-origin: top center;
}
.app-toast.ok {
  background: linear-gradient(135deg, rgba(240, 253, 244, 0.98), rgba(220, 252, 231, 0.96));
  color: #166534;
  border-color: rgba(110, 231, 183, 0.92);
  box-shadow: 0 14px 28px rgba(34, 197, 94, 0.16), 0 0 0 1px rgba(255, 255, 255, 0.35) inset;
  animation: appToastSuccessPulse 0.42s ease-out;
}
.app-toast.err {
  background: linear-gradient(135deg, rgba(255, 241, 242, 0.98), rgba(255, 228, 230, 0.96));
  color: #991b1b;
  border-color: rgba(253, 164, 175, 0.9);
  box-shadow: 0 14px 28px rgba(239, 68, 68, 0.12), 0 0 0 1px rgba(255, 255, 255, 0.3) inset;
}
.app-toast.theme-dark.ok {
  background: linear-gradient(135deg, rgba(9, 59, 44, 0.94), rgba(13, 78, 58, 0.92));
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.5);
  box-shadow: 0 16px 34px rgba(5, 150, 105, 0.22), 0 0 0 1px rgba(167, 243, 208, 0.08) inset;
}
.app-toast.theme-dark.err {
  background: linear-gradient(135deg, rgba(76, 20, 27, 0.94), rgba(101, 26, 37, 0.92));
  color: #fee2e2;
  border-color: rgba(251, 113, 133, 0.42);
  box-shadow: 0 16px 34px rgba(190, 24, 93, 0.18), 0 0 0 1px rgba(255, 228, 230, 0.06) inset;
}
.app-toast-enter-active,
.app-toast-leave-active {
  transition: opacity var(--motion-slow) ease, transform var(--motion-slow) ease, filter var(--motion-slow) ease;
}
.app-toast-enter-from,
.app-toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-10px) scale(0.97);
  filter: blur(3px);
}

@keyframes appToastSuccessPulse {
  0% {
    transform: translateX(-50%) translateY(-3px) scale(0.985);
  }

  58% {
    transform: translateX(-50%) translateY(0) scale(1.018);
  }

  100% {
    transform: translateX(-50%) translateY(0) scale(1);
  }
}
body {
  background: #edf3f9;
}
body[data-theme='dark'] {
  background: #0b1220;
  color: #e2e8f0;
}
</style>
