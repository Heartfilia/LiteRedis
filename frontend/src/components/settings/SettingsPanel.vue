<template>
  <div class="settings-panel">
    <div class="settings-header">
      <span class="settings-title-trigger" @click="handleTitleClick">⚙️ {{ t('settings.title') }}</span>
      <button class="btn-close" @click="$emit('close')">✕</button>
    </div>

    <div class="settings-body">
      <div class="section-title">{{ t('settings.keyLoadCount') }}</div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.keyScanCount') }}</span>
          <span class="label-hint">{{ t('settings.keyScanHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.keyScanCount" type="number" min="10" max="10000" step="10" />
          <span class="unit">{{ t('settings.unitCount') }}</span>
        </div>
      </div>

      <div class="section-title mt">{{ t('settings.valueLoadCount') }}</div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.hashLoadCount') }}</span>
          <span class="label-hint">{{ t('settings.hashLoadHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.hashLoadCount" type="number" min="10" max="100000" step="50" />
          <span class="unit">{{ t('settings.unitItem') }}</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.listLoadCount') }}</span>
          <span class="label-hint">{{ t('settings.listLoadHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.listLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">{{ t('settings.unitItem') }}</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.setLoadCount') }}</span>
          <span class="label-hint">{{ t('settings.setLoadHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.setLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">{{ t('settings.unitMember') }}</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.zsetLoadCount') }}</span>
          <span class="label-hint">{{ t('settings.zsetLoadHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.zsetLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">{{ t('settings.unitMember') }}</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.streamLoadCount') }}</span>
          <span class="label-hint">{{ t('settings.streamLoadHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.streamLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">{{ t('settings.unitItem') }}</span>
        </div>
      </div>

      <div class="section-title mt">{{ t('settings.other') }}</div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.searchHistoryLimit') }}</span>
          <span class="label-hint">{{ t('settings.searchHistoryHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.searchHistoryLimit" type="number" min="1" max="100" step="1" />
          <span class="unit">{{ t('settings.unitItem') }}</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.keyDisplayMode') }}</span>
          <span class="label-hint">{{ t('settings.keyDisplayModeHint') }}</span>
        </label>
        <div class="input-unit">
          <select v-model="form.keyDisplayMode" class="lang-select">
            <option value="tree">{{ t('settings.keyDisplayTree') }}</option>
            <option value="flat">{{ t('settings.keyDisplayFlat') }}</option>
          </select>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.fontSizeLevel') }}</span>
          <span class="label-hint">{{ t('settings.fontSizeLevelHint') }}</span>
        </label>
        <div class="input-unit">
          <select v-model="form.fontSizeLevel" class="lang-select">
            <option value="small">{{ t('settings.fontSizeSmall') }}</option>
            <option value="medium">{{ t('settings.fontSizeMedium') }}</option>
            <option value="large">{{ t('settings.fontSizeLarge') }}</option>
          </select>
        </div>
      </div>

      <div class="section-title mt">{{ t('settings.watermark') }}</div>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.watermarkEnabled') }}</span>
          <span class="label-hint">{{ t('settings.watermarkEnabledHint') }}</span>
        </label>
        <div class="input-unit">
          <input v-model="form.watermarkEnabled" type="checkbox" class="check-input" />
        </div>
      </div>

      <template v-if="form.watermarkEnabled">
        <div class="setting-item">
          <label>
            <span class="label-text">{{ t('settings.watermarkText') }}</span>
            <span class="label-hint">{{ t('settings.watermarkTextHint') }}</span>
          </label>
          <div class="input-unit input-unit-wide">
            <input v-model="form.watermarkText" type="text" class="text-input" />
          </div>
        </div>

        <div class="setting-item">
          <label>
            <span class="label-text">{{ t('settings.watermarkSize') }}</span>
            <span class="label-hint">{{ t('settings.watermarkSizeHint') }}</span>
          </label>
          <div class="input-unit">
            <input v-model.number="form.watermarkSize" type="number" min="10" max="48" step="1" />
            <span class="unit">px</span>
          </div>
        </div>

        <div class="setting-item">
          <label>
            <span class="label-text">{{ t('settings.watermarkAngle') }}</span>
            <span class="label-hint">{{ t('settings.watermarkAngleHint') }}</span>
          </label>
          <div class="input-unit">
            <input v-model.number="form.watermarkAngle" type="number" min="-90" max="90" step="1" />
            <span class="unit">deg</span>
          </div>
        </div>

        <div class="setting-item">
          <label>
            <span class="label-text">{{ t('settings.watermarkOpacity') }}</span>
            <span class="label-hint">{{ t('settings.watermarkOpacityHint') }}</span>
          </label>
          <div class="input-unit">
            <input v-model.number="form.watermarkOpacity" type="number" min="1" max="100" step="1" />
            <span class="unit">%</span>
          </div>
        </div>

        <div class="setting-item">
          <label>
            <span class="label-text">{{ t('settings.watermarkDensity') }}</span>
            <span class="label-hint">{{ t('settings.watermarkDensityHint') }}</span>
          </label>
          <div class="input-unit">
            <input v-model.number="form.watermarkDensity" type="number" min="1" max="5" step="1" />
            <span class="unit">{{ t('settings.unitLevel') }}</span>
          </div>
        </div>
      </template>

      <div class="setting-item">
        <label>
          <span class="label-text">{{ t('settings.language') }}</span>
          <span class="label-hint">{{ t('settings.languageHint') }}</span>
        </label>
        <div class="input-unit">
          <select v-model="form.language" class="lang-select">
            <option value="zh">中文</option>
            <option value="en">English</option>
          </select>
        </div>
      </div>
    </div>

    <!-- floating toast -->
    <Teleport to="body">
      <Transition name="toast">
        <div v-if="msg" class="settings-toast" :class="ok ? 'ok' : 'err'">{{ msg }}</div>
      </Transition>
    </Teleport>

    <div class="settings-footer">
      <div class="footer-status">
        <button class="version-box" :disabled="checkingUpdate" @click="checkUpdate">
          {{ t('settings.version') }} {{ appVersion }}
        </button>
        <span
          v-if="hasUpdate"
          class="mini-icon"
          :title="t('settings.openRelease')"
          role="button"
          tabindex="0"
          @click="openRelease"
          @keydown.enter.prevent="openRelease"
          @keydown.space.prevent="openRelease"
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 2C6.48 2 2 6.58 2 12.24c0 4.52 2.87 8.35 6.84 9.78.5.09.68-.22.68-.48 0-.24-.01-.87-.01-1.71-2.78.62-3.37-1.35-3.37-1.35-.46-1.18-1.12-1.49-1.12-1.49-.92-.64.07-.63.07-.63 1.02.07 1.56 1.07 1.56 1.07.9 1.57 2.36 1.12 2.94.86.09-.67.36-1.12.66-1.38-2.22-.26-4.56-1.15-4.56-5.13 0-1.13.39-2.06 1.03-2.78-.1-.26-.45-1.3.1-2.72 0 0 .84-.28 2.75 1.06a9.2 9.2 0 0 1 5 0c1.9-1.34 2.74-1.06 2.74-1.06.55 1.42.2 2.46.1 2.72.64.72 1.03 1.65 1.03 2.78 0 3.99-2.34 4.87-4.57 5.12.36.32.68.95.68 1.92 0 1.39-.01 2.5-.01 2.84 0 .26.18.58.69.48A10.55 10.55 0 0 0 22 12.24C22 6.58 17.52 2 12 2z"/>
          </svg>
        </span>
      </div>
      <div class="footer-actions">
        <button class="btn-cancel" @click="reset">{{ t('settings.reset') }}</button>
        <button class="btn-close-modal" @click="$emit('close')">{{ t('settings.close') }}</button>
        <button class="btn-save" :disabled="saving" @click="doSave">{{ saving ? t('settings.applying') : t('settings.apply') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { getAppVersion, checkLatestRelease } from '../../api/wails.js'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime.js'

const { t, setLanguage } = useI18n()

const emit = defineEmits(['close'])
const settingsStore = useSettingsStore()

const form = reactive({
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
})

const saving = ref(false)
const checkingUpdate = ref(false)
const appVersion = ref('dev')
const hasUpdate = ref(false)
const msg = ref('')
const ok = ref(true)
const titleTapCount = ref(0)
let titleTapTimer = null

onMounted(async () => {
  await settingsStore.load()
  syncFromStore()
  try {
    appVersion.value = await getAppVersion()
  } catch {
    appVersion.value = 'dev'
  }
})

function syncFromStore() {
  form.keyScanCount = settingsStore.keyScanCount
  form.hashLoadCount = settingsStore.hashLoadCount
  form.listLoadCount = settingsStore.listLoadCount
  form.setLoadCount = settingsStore.setLoadCount
  form.zsetLoadCount = settingsStore.zsetLoadCount
  form.streamLoadCount = settingsStore.streamLoadCount
  form.searchHistoryLimit = settingsStore.searchHistoryLimit
  form.keyDisplayMode = settingsStore.keyDisplayMode
  form.fontSizeLevel = settingsStore.fontSizeLevel
  form.watermarkEnabled = settingsStore.watermarkEnabled
  form.watermarkText = settingsStore.watermarkText
  form.watermarkSize = settingsStore.watermarkSize
  form.watermarkAngle = settingsStore.watermarkAngle
  form.watermarkOpacity = settingsStore.watermarkOpacity
  form.watermarkDensity = settingsStore.watermarkDensity
  form.language = settingsStore.language
}

function reset() {
  form.keyScanCount = 20
  form.hashLoadCount = 20
  form.listLoadCount = 20
  form.setLoadCount = 20
  form.zsetLoadCount = 20
  form.streamLoadCount = 20
  form.searchHistoryLimit = 10
  form.keyDisplayMode = 'tree'
  form.fontSizeLevel = 'small'
  form.watermarkEnabled = false
  form.watermarkText = ''
  form.watermarkSize = 16
  form.watermarkAngle = -22
  form.watermarkOpacity = 12
  form.watermarkDensity = 3
  form.language = 'zh'
}

function handleTitleClick() {
  titleTapCount.value += 1
  if (titleTapTimer) clearTimeout(titleTapTimer)
  titleTapTimer = setTimeout(() => {
    titleTapCount.value = 0
    titleTapTimer = null
  }, 1200)

  if (titleTapCount.value >= 5) {
    titleTapCount.value = 0
    settingsStore.enableDebugMode()
    ok.value = true
    msg.value = t('settings.debugModeEnabled')
    setTimeout(() => { msg.value = '' }, 3000)
  }
}

async function doSave() {
  saving.value = true
  msg.value = ''
  try {
    const result = await settingsStore.save({ ...form })
    ok.value = result.success
    msg.value = result.success ? '✓ ' + t('settings.saveOk') : (result.message || t('settings.saveErr'))
    if (result.success) {
      setLanguage(form.language)
      setTimeout(() => { msg.value = '' }, 3000)
    }
  } catch (e) {
    ok.value = false
    msg.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}

async function checkUpdate() {
  checkingUpdate.value = true
  msg.value = ''
  try {
    const result = await checkLatestRelease()
    if (result.error) {
      hasUpdate.value = false
      ok.value = false
      msg.value = result.error || t('settings.updateFailed')
      setTimeout(() => { msg.value = '' }, 3000)
      return
    }
    ok.value = true
    if (result.need_update) {
      hasUpdate.value = true
      msg.value = t('settings.updateAvailable').replace('{version}', `v${result.latest}`)
      setTimeout(() => { msg.value = '' }, 3000)
      return
    }
    hasUpdate.value = false
    msg.value = t('settings.latestVersion')
    setTimeout(() => { msg.value = '' }, 3000)
  } catch (e) {
    hasUpdate.value = false
    ok.value = false
    msg.value = e.message || String(e) || t('settings.updateFailed')
    setTimeout(() => { msg.value = '' }, 3000)
  } finally {
    checkingUpdate.value = false
  }
}

function openRelease() {
  BrowserOpenURL('https://github.com/Heartfilia/LiteRedis/releases/latest')
}
</script>

<style scoped>
.settings-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: linear-gradient(180deg, rgba(255,255,255,0.96) 0%, rgba(248,250,252,0.94) 100%);
  overflow: hidden;
}
.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.8);
  font-weight: 600;
  font-size: 14px;
  color: #111827;
  background: linear-gradient(180deg, rgba(255,255,255,0.92), rgba(247,250,252,0.88));
}
.settings-title-trigger {
  cursor: pointer;
  user-select: none;
  letter-spacing: 0.01em;
}
.btn-close {
  background: rgba(255,255,255,0.76);
  border: 1px solid rgba(226,232,240,0.9);
  border-radius: 999px;
  font-size: 14px;
  cursor: pointer;
  color: #9ca3af;
  width: 30px;
  height: 30px;
  padding: 0;
  line-height: 1.4;
  transition: color 0.12s, border-color 0.12s, background 0.12s, transform 0.12s;
}
.btn-close:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; transform: translateY(-1px); }

.settings-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 18px 20px;
}

.section-title {
  display: inline-flex;
  align-items: center;
  font-size: 11px;
  font-weight: 700;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.9px;
  margin-bottom: 12px;
  padding: 5px 10px;
  border-radius: 999px;
  background: rgba(255,255,255,0.72);
  border: 1px solid rgba(226,232,240,0.86);
  box-shadow: 0 4px 12px rgba(148, 163, 184, 0.06);
}
.section-title.mt { margin-top: 24px; }

.setting-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border: 1px solid rgba(226, 232, 240, 0.88);
  border-radius: 16px;
  background: rgba(255,255,255,0.72);
  box-shadow: 0 6px 18px rgba(148, 163, 184, 0.06);
  gap: 16px;
  margin-bottom: 10px;
}
.setting-item label {
  flex: 1;
  min-width: 0;
}
.label-text {
  display: block;
  font-size: 13px;
  color: #0f172a;
  font-weight: 500;
}
.label-hint {
  display: block;
  font-size: 11px;
  color: #94a3b8;
  margin-top: 3px;
  line-height: 1.45;
}
.input-unit {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.input-unit input {
  width: 80px;
  padding: 7px 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 12px;
  font-size: 13px;
  text-align: right;
  outline: none;
  background: rgba(255,255,255,0.94);
  transition: border-color 0.15s, box-shadow 0.15s, background 0.15s;
}
.text-input {
  width: 220px !important;
  text-align: left !important;
}
.input-unit-wide {
  width: 220px;
  justify-content: flex-end;
}
.check-input {
  width: 16px !important;
  height: 16px;
  accent-color: #3b82f6;
}
.input-unit input:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.unit {
  font-size: 12px;
  color: #6b7280;
  white-space: nowrap;
}
.lang-select {
  padding: 7px 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 12px;
  font-size: 13px;
  outline: none;
  background: rgba(255,255,255,0.94);
  color: #1f2937;
  cursor: pointer;
}
.lang-select:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }

.settings-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-top: 1px solid rgba(226, 232, 240, 0.82);
  background: linear-gradient(180deg, rgba(255,255,255,0.88), rgba(246,249,252,0.96));
}
.footer-status {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.version-box {
  min-width: 0;
  font-size: 11px;
  color: #64748b;
  white-space: nowrap;
  background: rgba(255,255,255,0.72);
  border: 1px solid rgba(226,232,240,0.92);
  border-radius: 999px;
  cursor: pointer;
  padding: 6px 10px;
}
.save-msg {
  flex: 1;
  font-size: 12px;
}
.save-msg.ok { color: #166534; }
.save-msg.err { color: #991b1b; }
.btn-save {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 32px;
  padding: 6px 18px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 12px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  font-weight: 500;
  box-sizing: border-box;
  transition: background 0.15s;
}
.btn-save:hover { background: #2563eb; }
.btn-save:disabled { background: #93c5fd; cursor: not-allowed; }
.btn-cancel {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 32px;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 12px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  font-weight: 500;
  box-sizing: border-box;
  transition: background 0.12s, border-color 0.12s;
}
.btn-cancel:hover { background: #f3f4f6; border-color: #9ca3af; }
.mini-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  color: #64748b;
  border-radius: 999px;
  cursor: pointer;
  flex: 0 0 auto;
  transition: color 0.12s, background 0.12s;
}
.mini-icon:hover {
  color: #111827;
  background: rgba(255, 255, 255, 0.6);
}
.mini-icon svg {
  width: 14px;
  height: 14px;
  fill: currentColor;
}
.mini-icon:focus-visible {
  outline: 2px solid #93c5fd;
  outline-offset: 1px;
}
.version-box:hover {
  color: #374151;
}
.version-box:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}
.btn-close-modal {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 32px;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 12px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  font-weight: 500;
  box-sizing: border-box;
  transition: background 0.12s, border-color 0.12s;
}
.btn-close-modal:hover { background: #f3f4f6; border-color: #9ca3af; }
.settings-toast {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  padding: 10px 24px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  box-shadow: 0 8px 24px rgba(0,0,0,0.12);
  pointer-events: none;
}
.settings-toast.ok { background: #f0fdf4; color: #166534; border: 1px solid #bbf7d0; }
.settings-toast.err { background: #fff1f2; color: #991b1b; border: 1px solid #fecaca; }
.toast-enter-active, .toast-leave-active { transition: opacity 0.25s, transform 0.25s; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateX(-50%) translateY(-12px); }
</style>
