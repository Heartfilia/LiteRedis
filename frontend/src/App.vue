<template>
  <div class="app-layout">
    <Sidebar />
    <MainContent />
    <ConnectionManager v-if="showConnManager" @close="showConnManager = false" />
    <SettingsModal v-if="showSettings" @close="showSettings = false" />
  </div>
</template>

<script setup>
import { onMounted, ref, provide } from 'vue'
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
  display: flex;
  height: 100vh;
  min-width: 1220px;
  min-height: 720px;
  overflow: hidden;
  background: #fff;
}
</style>
