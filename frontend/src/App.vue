<template>
  <div class="app-layout" :class="fontSizeClass">
    <Sidebar />
    <MainContent />
    <ConnectionManager
      v-if="showConnManager"
      :initial-connection="connManagerInitialConnection"
      @close="showConnManager = false"
    />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
    <Teleport to="body">
      <Transition name="app-toast">
        <div
          v-if="connectionsStore.globalToast"
          class="app-toast"
          :class="connectionsStore.globalToastOk ? 'ok' : 'err'"
        >
          {{ connectionsStore.globalToast }}
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount, ref, provide, Teleport, Transition, watch } from 'vue'
import Sidebar from './components/layout/Sidebar.vue'
import MainContent from './components/layout/MainContent.vue'
import ConnectionManager from './components/connections/ConnectionManager.vue'
import SettingsModal from './components/settings/SettingsModal.vue'
import { useConnectionsStore } from './stores/connections.js'
import { useSettingsStore } from './stores/settings.js'

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

onMounted(() => {
  connectionsStore.loadConnections()
  settingsStore.load()
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
  border-radius: 10px;
  font-size: 12px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.18);
  border: 1px solid transparent;
  max-width: min(520px, calc(100vw - 24px));
  word-break: break-word;
}
.app-toast.ok {
  background: #f0fdf4;
  color: #166534;
  border-color: #bbf7d0;
}
.app-toast.err {
  background: #fff1f2;
  color: #991b1b;
  border-color: #fecaca;
}
.app-toast-enter-active,
.app-toast-leave-active {
  transition: opacity 0.25s, transform 0.25s;
}
.app-toast-enter-from,
.app-toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-12px);
}
body {
  background: #edf3f9;
}
</style>
