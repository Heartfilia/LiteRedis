<template>
  <Teleport to="body">
    <div v-if="show" class="expand-overlay" :class="themeClass" @click.self="onClose">
      <div class="expand-modal">
        <div class="expand-header">
          <span class="expand-title" :title="title">{{ title }}</span>
          <div class="expand-header-actions">
            <button v-if="editable && !saving" class="btn-save-modal" :class="{ 'success-flash': saveFlashing, 'error-flash': errorFlashing }" @click="onSave">
              💾 {{ t('keyEditor.save') }}
            </button>
            <button class="btn-copy-modal" :class="{ copied }" @click="copy">
              {{ copied ? '✓ ' + t('keyEditor.copied') : '📋 ' + t('keyEditor.copy') }}
            </button>
            <button class="btn-close-modal" @click="onClose" :title="t('keyEditor.close')">✕</button>
          </div>
        </div>
        <div class="expand-body">
          <textarea
            v-if="editable"
            v-model="localContent"
            class="expand-textarea"
            :disabled="saving"
          />
          <pre v-else class="expand-content">{{ content }}</pre>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from '../../i18n/index.js'
import { useSettingsStore } from '../../stores/settings.js'
import { copyToClipboard } from '../../utils/clipboard.js'

const { t } = useI18n()
const settingsStore = useSettingsStore()

const props = defineProps({
  show:    { type: Boolean, default: false },
  title:   { type: String,  default: '' },
  content: { type: String,  default: '' },
  editable:{ type: Boolean, default: false },
  saving:  { type: Boolean, default: false },
})

const emit = defineEmits(['close', 'save'])

const localContent = ref(props.content)
const copied = ref(false)
const saveFlashing = ref(false)
const errorFlashing = ref(false)
const themeClass = computed(() => `theme-${settingsStore.themeMode || 'light'}`)
let saveFlashTimer = null
let errorFlashTimer = null

watch(() => props.show, (v) => {
  if (v) localContent.value = props.content
  copied.value = false
  if (!v) {
    saveFlashing.value = false
    errorFlashing.value = false
  }
})

watch(() => props.content, (v) => {
  if (props.show) localContent.value = v
})

watch(() => props.saving, (saving, previous) => {
  if (previous && !saving && props.show) {
    const success = localContent.value === props.content
    if (success) {
      if (saveFlashTimer) clearTimeout(saveFlashTimer)
      if (errorFlashTimer) {
        clearTimeout(errorFlashTimer)
        errorFlashTimer = null
      }
      errorFlashing.value = false
      saveFlashing.value = true
      saveFlashTimer = setTimeout(() => {
        saveFlashing.value = false
        saveFlashTimer = null
      }, 1100)
    } else {
      if (errorFlashTimer) clearTimeout(errorFlashTimer)
      if (saveFlashTimer) {
        clearTimeout(saveFlashTimer)
        saveFlashTimer = null
      }
      saveFlashing.value = false
      errorFlashing.value = true
      errorFlashTimer = setTimeout(() => {
        errorFlashing.value = false
        errorFlashTimer = null
      }, 1300)
    }
  }
})

function onClose() {
  emit('close')
}

function onSave() {
  emit('save', localContent.value)
}

async function copy() {
  const ok = await copyToClipboard(localContent.value)
  if (ok) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  }
}
</script>

<style scoped>
.expand-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}
.expand-modal {
  background: white;
  border-radius: 10px;
  width: min(820px, 90vw);
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  border: 1px solid rgba(226, 232, 240, 0.84);
  box-shadow: 0 28px 56px rgba(15, 23, 42, 0.2), 0 10px 24px rgba(148, 163, 184, 0.08);
}
.expand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  border-radius: 10px 10px 0 0;
  flex-shrink: 0;
}
.expand-title {
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
  color: #1d4ed8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 600px;
}
.expand-header-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.btn-copy-modal,
.btn-save-modal {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 30px;
  padding: 4px 13px;
  background: rgba(255, 255, 255, 0.94);
  color: #475569;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 6px;
  font-size: 11px;
  line-height: 1;
  font-weight: 600;
  cursor: pointer;
  min-width: 74px;
  box-sizing: border-box;
  transition: background 0.12s, border-color 0.12s, color 0.12s, transform 0.12s;
}
.btn-save-modal {
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  border-color: rgba(191, 219, 254, 0.96);
}
.btn-save-modal:hover { background: linear-gradient(180deg, #f8fbff, #eff6ff); border-color: #60a5fa; color: #1d4ed8; transform: translateY(-1px); }
.btn-save-modal.success-flash {
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.96), rgba(22, 163, 74, 0.96));
  color: #f0fdf4;
  border-color: rgba(22, 163, 74, 0.98);
  box-shadow: 0 10px 24px rgba(34, 197, 94, 0.2), 0 0 0 1px rgba(220, 252, 231, 0.22) inset;
  animation: modalSaveFlashPulse 0.42s ease;
}
.btn-save-modal.error-flash {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.96), rgba(220, 38, 38, 0.96));
  color: #fff7f7;
  border-color: rgba(220, 38, 38, 0.98);
  box-shadow: 0 10px 24px rgba(239, 68, 68, 0.18), 0 0 0 1px rgba(254, 202, 202, 0.18) inset;
  animation: modalErrorFlashPulse 0.48s ease;
}
.btn-copy-modal:hover { background: #f8fafc; border-color: #94a3b8; color: #1e293b; transform: translateY(-1px); }
.btn-copy-modal.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  transform: translateY(-1px);
  animation: copyPulse 0.26s ease;
}
.btn-close-modal {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 30px;
  background: transparent;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  color: #9ca3af;
  padding: 4px 9px;
  line-height: 1;
  box-sizing: border-box;
  transition: color 0.12s, border-color 0.12s, background 0.12s, transform 0.12s;
}
.btn-close-modal:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; transform: translateY(-1px); }
.expand-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}
.expand-content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #1f2937;
}
.expand-textarea {
  width: 100%;
  height: 100%;
  min-height: 300px;
  resize: vertical;
  padding: 10px 12px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 8px;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #1f2937;
  outline: none;
  box-sizing: border-box;
}
.expand-textarea:focus {
  border-color: rgba(96, 165, 250, 0.92);
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.32);
}
.expand-textarea:disabled {
  background: #f9fafb;
  color: #9ca3af;
}
.expand-overlay.theme-dark .btn-save-modal.success-flash {
  background: linear-gradient(135deg, rgba(9, 59, 44, 0.98), rgba(13, 78, 58, 0.96));
  color: #d1fae5;
  border-color: rgba(52, 211, 153, 0.44);
  box-shadow: 0 12px 26px rgba(5, 150, 105, 0.22), 0 0 0 1px rgba(167, 243, 208, 0.08) inset;
}

.expand-overlay.theme-dark .btn-save-modal.error-flash {
  background: linear-gradient(135deg, rgba(76, 20, 27, 0.98), rgba(127, 29, 29, 0.96));
  color: #ffe4e6;
  border-color: rgba(251, 113, 133, 0.42);
  box-shadow: 0 12px 26px rgba(190, 24, 93, 0.18), 0 0 0 1px rgba(255, 228, 230, 0.06) inset;
}

.expand-overlay.theme-dark .btn-copy-modal {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.expand-overlay.theme-dark .btn-copy-modal.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}

.expand-overlay.theme-dark .btn-copy-modal:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}

.expand-overlay.theme-dark .btn-close-modal {
  border-color: rgba(71, 85, 105, 0.96);
  color: #94a3b8;
  background: rgba(15, 23, 42, 0.94);
}

.expand-overlay.theme-dark .btn-close-modal:hover {
  color: #fecaca;
  border-color: rgba(248, 113, 113, 0.42);
  background: rgba(127, 29, 29, 0.56);
}

.expand-overlay.theme-dark .expand-content {
  color: #e2e8f0;
}

.expand-overlay.theme-dark .expand-textarea {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}

.expand-overlay.theme-dark .expand-textarea:disabled {
  background: rgba(30, 41, 59, 0.9);
  color: #64748b;
}

@keyframes copyPulse {
  0% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-1px) scale(1.012); }
  100% { transform: translateY(-1px) scale(1); }
}

@keyframes modalSaveFlashPulse {
  0% { transform: translateY(0) scale(1); }
  48% { transform: translateY(-1px) scale(1.02); }
  100% { transform: translateY(0) scale(1); }
}

@keyframes modalErrorFlashPulse {
  0% { transform: translateY(0) scale(1); }
  38% { transform: translateY(-1px) scale(1.015); }
  72% { transform: translateY(0) scale(0.995); }
  100% { transform: translateY(0) scale(1); }
}
</style>
