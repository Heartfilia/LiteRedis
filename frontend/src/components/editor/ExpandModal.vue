<template>
  <Teleport to="body">
    <div v-if="show" class="expand-overlay" @click.self="onClose">
      <div class="expand-modal">
        <div class="expand-header">
          <span class="expand-title" :title="title">{{ title }}</span>
          <div class="expand-header-actions">
            <button v-if="editable && !saving" class="btn-save-modal" @click="onSave">
              💾 {{ t('keyEditor.save') }}
            </button>
            <button class="btn-copy-modal" @click="copy">
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
import { ref, watch } from 'vue'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'

const { t } = useI18n()

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

watch(() => props.show, (v) => {
  if (v) localContent.value = props.content
  copied.value = false
})

watch(() => props.content, (v) => {
  if (props.show) localContent.value = v
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
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.22);
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
  padding: 4px 13px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  min-width: 74px;
  transition: background 0.12s, border-color 0.12s;
}
.btn-save-modal {
  background: #3b82f6;
  color: white;
  border-color: #3b82f6;
}
.btn-save-modal:hover { background: #2563eb; }
.btn-copy-modal:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-close-modal {
  display: inline-flex; align-items: center; justify-content: center;
  background: transparent;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  color: #9ca3af;
  padding: 4px 9px;
  line-height: 1.4;
  transition: color 0.12s, border-color 0.12s;
}
.btn-close-modal:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; }
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
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #1f2937;
  outline: none;
  box-sizing: border-box;
}
.expand-textarea:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59,130,246,.15);
}
.expand-textarea:disabled {
  background: #f9fafb;
  color: #9ca3af;
}
</style>
