import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { loadLanguage } from './i18n/index.js'

loadLanguage()

const app = createApp(App)
app.use(createPinia())
app.mount('#app')
