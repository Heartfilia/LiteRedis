<template>
  <div class="app-layout" :class="fontSizeClass">
    <Sidebar />
    <MainContent />
    <ConnectionManager v-if="showConnManager" @close="showConnManager = false" />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
  </div>
</template>

<script setup>
import { computed, onMounted, ref, provide } from 'vue'
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
const fontSizeClass = computed(() => `font-${settingsStore.fontSizeLevel || 'small'}`)

provide('openConnManager', () => { showConnManager.value = true })
provide('openSettings', () => { showSettings.value = true })

onMounted(() => {
  connectionsStore.loadConnections()
  settingsStore.load()
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
  display: flex;
  height: 100vh;
  min-width: 1220px;
  min-height: 720px;
  overflow: hidden;
  background: #fff;
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
</style>
