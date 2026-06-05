import { defineStore } from 'pinia'
import { getSettings, saveSettings } from '../api/wails.js'
import { setLanguage } from '../i18n/index.js'
import { useWorkspaceStore } from './workspace.js'

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    loaded: false,
    debugMode: false,
    themeMode: 'light',
    keyScanCount: 20,
    hashLoadCount: 20,
    listLoadCount: 20,
    setLoadCount: 20,
    zsetLoadCount: 20,
    streamLoadCount: 20,
    searchHistoryLimit: 10,
    keyDisplayMode: 'tree',
    fontSizeLevel: 'small',
    watermarkEnabled: false,
    watermarkText: '',
    watermarkSize: 16,
    watermarkAngle: -22,
    watermarkOpacity: 12,
    watermarkDensity: 3,
    language: 'zh',
  }),

  actions: {
    applyTheme() {
      if (typeof document === 'undefined') return
      document.documentElement.setAttribute('data-theme', this.themeMode)
      document.body.setAttribute('data-theme', this.themeMode)
    },

    toggleTheme() {
      this.themeMode = this.themeMode === 'dark' ? 'light' : 'dark'
      try {
        localStorage.setItem('liteRedis_themeMode', this.themeMode)
      } catch (e) {}
      this.applyTheme()
    },

    enableDebugMode() {
      this.debugMode = true
    },

    async load() {
      try {
        try {
          const savedThemeMode = localStorage.getItem('liteRedis_themeMode')
          if (savedThemeMode === 'light' || savedThemeMode === 'dark') {
            this.themeMode = savedThemeMode
          }
        } catch (e) {}
        const s = await getSettings()
        this.keyScanCount = s.key_scan_count || 20
        this.hashLoadCount = s.hash_load_count || 20
        this.listLoadCount = s.list_load_count || 20
        this.setLoadCount = s.set_load_count || 20
        this.zsetLoadCount = s.zset_load_count || 20
        this.streamLoadCount = s.stream_load_count || 20
        this.searchHistoryLimit = s.search_history_limit || 10
        this.keyDisplayMode = s.key_display_mode || 'tree'
        this.fontSizeLevel = s.font_size_level || 'small'
        this.watermarkEnabled = !!s.watermark_enabled
        this.watermarkText = s.watermark_text || ''
        this.watermarkSize = s.watermark_size || 16
        this.watermarkAngle = Number.isFinite(s.watermark_angle) ? s.watermark_angle : -22
        this.watermarkOpacity = s.watermark_opacity || 12
        this.watermarkDensity = s.watermark_density || 3
        this.language = s.language || 'zh'
        setLanguage(this.language)
        this.loaded = true
        this.applyTheme()
      } catch (e) {
        console.error('load settings failed', e)
        this.applyTheme()
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
        key_display_mode: values.keyDisplayMode,
        font_size_level: values.fontSizeLevel,
        watermark_enabled: values.watermarkEnabled,
        watermark_text: values.watermarkText,
        watermark_size: values.watermarkSize,
        watermark_angle: values.watermarkAngle,
        watermark_opacity: values.watermarkOpacity,
        watermark_density: values.watermarkDensity,
        language: values.language,
      }
      const result = await saveSettings(payload)
      if (result.success) {
        Object.assign(this, values)
        useWorkspaceStore().trimAllSearchHistory(this.searchHistoryLimit)
      }
      return result
    },
  },
})
