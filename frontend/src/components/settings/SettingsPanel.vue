<template>
  <div class="settings-panel">
    <div class="settings-header">
      <span>⚙️ {{ t('settings.title') }}</span>
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
      <button class="btn-cancel" @click="reset">{{ t('settings.reset') }}</button>
      <button class="btn-close-modal" @click="$emit('close')">{{ t('settings.close') }}</button>
      <button class="btn-save" :disabled="saving" @click="doSave">{{ saving ? t('settings.saving') : t('settings.save') }}</button>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'

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
  language: 'zh',
})

const saving = ref(false)
const msg = ref('')
const ok = ref(true)

onMounted(async () => {
  await settingsStore.load()
  syncFromStore()
})

function syncFromStore() {
  form.keyScanCount = settingsStore.keyScanCount
  form.hashLoadCount = settingsStore.hashLoadCount
  form.listLoadCount = settingsStore.listLoadCount
  form.setLoadCount = settingsStore.setLoadCount
  form.zsetLoadCount = settingsStore.zsetLoadCount
  form.streamLoadCount = settingsStore.streamLoadCount
  form.searchHistoryLimit = settingsStore.searchHistoryLimit
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
  form.language = 'zh'
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
</script>

<style scoped>
.settings-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: white;
}
.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid #e5e7eb;
  font-weight: 600;
  font-size: 14px;
  color: #111827;
  background: #f9fafb;
}
.btn-close {
  background: transparent;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  color: #9ca3af;
  padding: 2px 8px;
  line-height: 1.4;
  transition: color 0.12s, border-color 0.12s;
}
.btn-close:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; }

.settings-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.section-title {
  font-size: 11px;
  font-weight: 700;
  color: #9ca3af;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  margin-bottom: 10px;
}
.section-title.mt { margin-top: 20px; }

.setting-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #f3f4f6;
  gap: 12px;
}
.setting-item label {
  flex: 1;
  min-width: 0;
}
.label-text {
  display: block;
  font-size: 13px;
  color: #1f2937;
  font-weight: 500;
}
.label-hint {
  display: block;
  font-size: 11px;
  color: #9ca3af;
  margin-top: 2px;
}
.input-unit {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.input-unit input {
  width: 80px;
  padding: 5px 8px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 13px;
  text-align: right;
  outline: none;
  transition: border-color 0.15s;
}
.input-unit input:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.unit {
  font-size: 12px;
  color: #6b7280;
  white-space: nowrap;
}
.lang-select {
  padding: 5px 8px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 13px;
  outline: none;
  background: white;
  color: #1f2937;
  cursor: pointer;
}
.lang-select:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }

.settings-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
}
.save-msg {
  flex: 1;
  font-size: 12px;
}
.save-msg.ok { color: #166534; }
.save-msg.err { color: #991b1b; }
.btn-save {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 6px 18px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.15s;
}
.btn-save:hover { background: #2563eb; }
.btn-save:disabled { background: #93c5fd; cursor: not-allowed; }
.btn-cancel {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.12s, border-color 0.12s;
}
.btn-cancel:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-close-modal {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
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
