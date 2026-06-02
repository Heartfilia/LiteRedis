import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { loadLanguage } from './i18n/index.js'
import { useConnectionsStore } from './stores/connections.js'
import { useWorkspaceStore } from './stores/workspace.js'

loadLanguage()

const isWindows = (() => {
  const uaPlatform = navigator.userAgentData?.platform || ''
  const platform = navigator.platform || ''
  const ua = navigator.userAgent || ''
  return /win/i.test(uaPlatform) || /win/i.test(platform) || /windows/i.test(ua)
})()

if (isWindows) {
  window.addEventListener('wheel', (event) => {
    if (event.ctrlKey) {
      event.preventDefault()
    }
  }, { passive: false })
}

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.mount('#app')

const connectionsStore = useConnectionsStore(pinia)
const workspaceStore = useWorkspaceStore(pinia)
connectionsStore.startHeartbeat(workspaceStore)

document.addEventListener('visibilitychange', () => {
  connectionsStore.setHeartbeatVisibility(document.visibilityState === 'visible')
})
