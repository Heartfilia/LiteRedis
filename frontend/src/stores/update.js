import { defineStore } from 'pinia'
import { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { checkLatestRelease, startUpdate } from '../api/wails.js'
import { useI18n } from '../i18n/index.js'

const { t } = useI18n()

let messageTimer = null

// 更新状态放在全局 store：设置面板关掉再打开、或在更新过程中关闭面板，状态与进度都不会丢
export const useUpdateStore = defineStore('update', {
  state: () => ({
    checking: false,
    updating: false,
    // download / verify / install / restart
    stage: 'download',
    percent: 0,
    hasUpdate: false,
    latest: '',
    message: '',
    messageOk: true,
    listening: false,
  }),

  getters: {
    // 已安装完成、等待应用自行退出重启的阶段
    restarting: (state) => state.updating && state.stage === 'restart',
  },

  actions: {
    ensureListener() {
      if (this.listening) return
      this.listening = true
      try {
        EventsOn('update:progress', (progress) => this.onProgress(progress))
      } catch (e) {
        // 非 Wails 环境（单测 / 浏览器预览）里没有 runtime，忽略即可
        this.listening = false
      }
    },

    setMessage(text, ok = true, clearAfter = 0) {
      this.message = text || ''
      this.messageOk = ok
      if (messageTimer) {
        clearTimeout(messageTimer)
        messageTimer = null
      }
      if (!text || clearAfter <= 0) return
      messageTimer = setTimeout(() => {
        this.message = ''
        messageTimer = null
      }, clearAfter)
    },

    clearMessage() {
      this.setMessage('', true, 0)
    },

    onProgress(progress) {
      if (!progress || !this.updating) return
      // 已进入重启阶段，忽略迟到的事件，避免按钮从「重启中」跳回「更新中 x%」
      if (this.stage === 'restart') return
      const percent = Number(progress.percent)
      this.percent = Math.max(0, Math.min(100, Math.round(Number.isFinite(percent) ? percent : 0)))
      if (progress.stage) this.stage = progress.stage
      if (progress.stage === 'download') {
        this.setMessage(t('settings.updateDownloading').replace('{percent}', this.percent), true, 0)
      } else if (progress.stage === 'verify') {
        this.setMessage(t('settings.updateVerifying'), true, 0)
      } else if (progress.stage === 'install') {
        this.setMessage(t('settings.updateInstalling'), true, 0)
      }
    },

    // 版本按钮的唯一入口：有更新就安装，否则检查；检查/更新进行中直接忽略
    async runPrimaryAction() {
      if (this.checking || this.updating) return
      if (this.hasUpdate) {
        await this.install()
        return
      }
      await this.check()
    },

    // silent：打开设置面板时的自动检查，只更新状态不弹任何提示（失败也静默）
    async check({ silent = false } = {}) {
      if (this.checking || this.updating) return
      this.checking = true
      if (!silent) this.clearMessage()
      try {
        const result = await checkLatestRelease()
        if (result.error) {
          this.hasUpdate = false
          if (!silent) this.setMessage(result.error, false, 4000)
          return
        }
        if (result.need_update) {
          this.hasUpdate = true
          this.latest = result.latest || ''
          if (!silent) this.setMessage(t('settings.updateAvailable').replace('{version}', `v${result.latest}`), true, 4000)
          return
        }
        this.hasUpdate = false
        if (!silent) this.setMessage(t('settings.latestVersion'), true, 3000)
      } catch (e) {
        this.hasUpdate = false
        if (!silent) this.setMessage(e?.message || String(e) || t('settings.updateFailed'), false, 4000)
      } finally {
        this.checking = false
      }
    },

    async install() {
      if (this.updating) return
      this.ensureListener()
      this.checking = false
      this.updating = true
      this.stage = 'download'
      this.percent = 0
      this.clearMessage()
      // 安装成功后应用会自行退出重启，此时不要复位状态，否则按钮会闪回版本号
      let keepUpdating = false
      try {
        const result = await startUpdate()
        if (result.success && result.needs_restart) {
          keepUpdating = true
          this.stage = 'restart'
          this.percent = 100
          this.hasUpdate = false
          this.setMessage(result.message || t('settings.updateRestarting'), true, 0)
          return
        }
        this.setMessage(
          result.message || (result.success ? t('settings.updateStarted') : t('settings.updateFailed')),
          !!result.success,
          result.success ? 0 : 5000
        )
        if (result.success) this.hasUpdate = false
      } catch (e) {
        this.setMessage(e?.message || String(e) || t('settings.updateFailed'), false, 5000)
      } finally {
        if (!keepUpdating) this.updating = false
      }
    },
  },
})
