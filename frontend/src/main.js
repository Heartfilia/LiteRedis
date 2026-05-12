import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { loadLanguage } from './i18n/index.js'

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
app.use(createPinia())
app.mount('#app')
