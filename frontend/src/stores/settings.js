import { defineStore } from 'pinia'
import { getSettings, saveSettings } from '../api/wails.js'
import { setLanguage } from '../i18n/index.js'
import { useWorkspaceStore } from './workspace.js'

const DEFAULT_LOAD_COUNT = 20

export function normalizeLoadCount(value, fallback = DEFAULT_LOAD_COUNT) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric <= 0) return fallback
  return Math.min(10000, Math.trunc(numeric))
}

const THEME_STORAGE_KEY = 'liteRedis_themeMode'
const THEME_PREFERENCES = ['light', 'green', 'dark', 'auto']

// 主题选择：light / green（护眼）/ dark / auto（跟随系统）
export function normalizeThemePreference(value) {
  return THEME_PREFERENCES.includes(value) ? value : 'light'
}

// 系统当前的明暗偏好
export function systemThemeMode() {
  if (typeof window === 'undefined' || !window.matchMedia) return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

// 系统主题变化时，只有选了「跟随系统」才需要重算
let systemWatcher = null
function watchSystemTheme(store) {
  if (systemWatcher || typeof window === 'undefined' || !window.matchMedia) return
  const query = window.matchMedia('(prefers-color-scheme: dark)')
  const handler = () => {
    if (store && store.themePreference === 'auto') store.applyTheme()
  }
  if (query.addEventListener) query.addEventListener('change', handler)
  else if (query.addListener) query.addListener(handler)
  systemWatcher = query
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    loaded: false,
    debugMode: false,
    // 实际生效的主题（light | dark），组件都用它拼 theme-xxx 类名
    themeMode: 'light',
    // 用户选择（light | green | dark | auto），auto 跟随系统
    themePreference: 'light',
    keyScanCount: 200,
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
    localApiEnabled: false,
    localApiPort: 64201,
    // 允许本地 API 直接读取 Redis（调试用，默认关）
    localApiRedisEnabled: false,
    // 关闭本地 API 的 token 校验（纯本机自用）
    localApiAuthDisabled: false,
    proxyEnabled: false,
    proxyUrl: '',
    // 已看过更新说明的版本号，用于每个版本只弹一次「更新内容」
    lastSeenVersion: '',
    // 是否已看过首次使用向导
    onboardingDone: false,
    // 顶部连接标签栏，默认关闭（和没有标签栏之前一样从左侧切换）
    connectionTabsEnabled: false,
  }),

  actions: {
    applyTheme() {
      this.themeMode = this.themePreference === 'auto' ? systemThemeMode() : this.themePreference
      if (typeof document === 'undefined') return
      document.documentElement.setAttribute('data-theme', this.themeMode)
      document.body.setAttribute('data-theme', this.themeMode)
    },

    // 切换主题：菜单里选具体的一项（light | dark | auto）
    setTheme(preference) {
      const next = normalizeThemePreference(preference)
      this.themePreference = next
      try {
        localStorage.setItem(THEME_STORAGE_KEY, next)
      } catch (e) {}
      watchSystemTheme(this)
      this.applyTheme()
    },

    // 命令面板里的快捷切换：只在明暗之间切，不动「跟随系统」的选择
    toggleTheme() {
      this.setTheme(this.themeMode === 'dark' ? 'light' : 'dark')
    },

    enableDebugMode() {
      this.debugMode = true
    },

    async load() {
      try {
        try {
          const savedThemeMode = localStorage.getItem(THEME_STORAGE_KEY)
          if (savedThemeMode) this.themePreference = normalizeThemePreference(savedThemeMode)
        } catch (e) {}
        watchSystemTheme(this)
        const s = await getSettings()
        this.keyScanCount = normalizeLoadCount(s.key_scan_count)
        this.hashLoadCount = normalizeLoadCount(s.hash_load_count)
        this.listLoadCount = normalizeLoadCount(s.list_load_count)
        this.setLoadCount = normalizeLoadCount(s.set_load_count)
        this.zsetLoadCount = normalizeLoadCount(s.zset_load_count)
        this.streamLoadCount = normalizeLoadCount(s.stream_load_count)
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
        this.localApiEnabled = s.local_api_enabled === true
        this.localApiPort = s.local_api_port || 64201
        this.localApiRedisEnabled = s.local_api_redis_enabled === true
        this.localApiAuthDisabled = s.local_api_auth_disabled === true
        this.proxyEnabled = !!s.proxy_enabled
        this.proxyUrl = s.proxy_url || ''
        this.lastSeenVersion = s.last_seen_version || ''
        this.onboardingDone = s.onboarding_done === true
        this.connectionTabsEnabled = s.connection_tabs_enabled === true
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
        connection_tabs_enabled: values.connectionTabsEnabled === true,
        font_size_level: values.fontSizeLevel,
        watermark_enabled: values.watermarkEnabled,
        watermark_text: values.watermarkText,
        watermark_size: values.watermarkSize,
        watermark_angle: values.watermarkAngle,
        watermark_opacity: values.watermarkOpacity,
        watermark_density: values.watermarkDensity,
        language: values.language,
        local_api_enabled: values.localApiEnabled,
        local_api_port: values.localApiPort,
        local_api_redis_enabled: values.localApiRedisEnabled === true,
        local_api_auth_disabled: values.localApiAuthDisabled === true,
        proxy_enabled: values.proxyEnabled,
        proxy_url: values.proxyUrl,
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
