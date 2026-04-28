import { defineStore } from 'pinia'
import { getSettings, saveSettings } from '../api/wails.js'
import { setLanguage } from '../i18n/index.js'

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    loaded: false,
    keyScanCount: 20,
    hashLoadCount: 20,
    listLoadCount: 20,
    setLoadCount: 20,
    zsetLoadCount: 20,
    streamLoadCount: 20,
    searchHistoryLimit: 10,
    language: 'zh',
  }),

  actions: {
    async load() {
      try {
        const s = await getSettings()
        this.keyScanCount = s.key_scan_count || 20
        this.hashLoadCount = s.hash_load_count || 20
        this.listLoadCount = s.list_load_count || 20
        this.setLoadCount = s.set_load_count || 20
        this.zsetLoadCount = s.zset_load_count || 20
        this.streamLoadCount = s.stream_load_count || 20
        this.searchHistoryLimit = s.search_history_limit || 10
        this.language = s.language || 'zh'
        setLanguage(this.language)
        this.loaded = true
      } catch (e) {
        console.error('load settings failed', e)
      }
    },

    async save(values) {
      const payload = {
        key_scan_count: values.keyScanCount,
        hash_load_count: values.hashLoadCount,
        list_load_count: values.listLoadCount,
        set_load_count: values.setLoadCount,
        zset_load_count: values.zsetLoadCount,
        stream_load_count: values.streamLoadCount,
        search_history_limit: values.searchHistoryLimit,
        language: values.language,
      }
      const result = await saveSettings(payload)
      if (result.success) {
        Object.assign(this, values)
      }
      return result
    },
  },
})
