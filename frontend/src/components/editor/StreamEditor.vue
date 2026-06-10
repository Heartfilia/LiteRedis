<template>
  <div class="stream-editor" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <div class="toolbar">
      <span class="count">{{ displayEntries.length }} / {{ entries.length }} {{ t('settings.unitItem') }}</span>
    </div>
    <div class="stream-wrap">
      <div v-for="(entry, idx) in displayEntries" :key="entry.id" class="stream-entry">
        <div class="entry-header">
          <span class="entry-num">{{ idx + 1 }}</span>
          <span class="entry-id">{{ entry.id }}</span>
          <button class="btn-entry-copy" :class="{ copied: copiedEntry === entry.id }" @click="copyEntry(entry)">
            {{ copiedEntry === entry.id ? '✓ ' + t('keyEditor.copied') : t('keyEditor.copy') }}
          </button>
        </div>
        <div class="entry-fields">
          <span v-for="(val, key) in entry.fields" :key="key" class="field-pair">
            <span class="field-key">{{ key }}</span>
            <span class="field-sep">:</span>
            <span class="field-val">{{ val }}</span>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import './editorShared.css'

const props = defineProps({ keyValue: Object })
const settingsStore = useSettingsStore()
const { t } = useI18n()

const entries = ref([])
const copiedEntry = ref(null)

// Stream 暂不分页，直接显示全部
const displayEntries = computed(() => entries.value)

watch(() => props.keyValue, kv => {
  entries.value = kv?.stream_val || []
}, { immediate: true })

async function copyEntry(entry) {
  await copyToClipboard(JSON.stringify(entry.fields, null, 2))
  copiedEntry.value = entry.id
  setTimeout(() => { copiedEntry.value = null }, 1200)
}
</script>

<style scoped>
.stream-editor { display: flex; flex-direction: column; height: 100%; gap: 7px; }
.stream-wrap { flex: 1; overflow-y: auto; }
.stream-entry { border-bottom: 1px solid rgba(241, 245, 249, 0.96); padding: 8px 10px; font-size: 12px; }
.stream-entry:hover { background: rgba(248, 250, 252, 0.9); }
.entry-header { display: flex; align-items: center; gap: 6px; margin-bottom: 5px; }
.entry-header,
.entry-num,
.entry-id,
.btn-entry-copy,
.field-key,
.field-sep {
  user-select: none;
  -webkit-user-select: none;
}
.entry-num {
  background: rgba(236, 254, 255, 0.96);
  color: #0e7490;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 10px;
  min-width: 24px;
  text-align: center;
  flex-shrink: 0;
  font-weight: 600;
}
.entry-id { font-family: monospace; color: #0e7490; font-weight: 500; font-size: 11px; flex: 1; }
.btn-entry-copy {
  margin-left: auto;
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 24px;
  padding: 0 9px;
  background: rgba(255, 255, 255, 0.94);
  color: #475569;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, color 0.12s, transform 0.12s;
  white-space: nowrap;
  flex-shrink: 0;
}
.btn-entry-copy:hover { background: #f8fafc; border-color: #94a3b8; color: #1e293b; transform: translateY(-1px); }
.btn-entry-copy.copied {
  background: rgba(191, 219, 254, 0.96);
  color: #1d4ed8;
  border-color: rgba(96, 165, 250, 0.92);
  transform: translateY(-1px);
  animation: copyPulse 0.26s ease;
}
.entry-fields { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.field-pair {
  display: flex;
  align-items: center;
  gap: 4px;
  background: rgba(236, 254, 255, 0.84);
  padding: 3px 8px;
  border-radius: 6px;
  border: 1px solid rgba(207, 250, 254, 0.96);
  min-width: 0;
}
.field-key { color: #0e7490; font-weight: 600; font-size: 11px; }
.field-sep { color: #94a3b8; }
.field-val {
  color: #1f2937;
  font-family: monospace;
  font-size: 11px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stream-editor.theme-dark {
  color: #e2e8f0;
}

.stream-editor.theme-dark .entry-num {
  background: rgba(8, 47, 73, 0.42);
  color: #67e8f9;
}

.stream-editor.theme-dark .stream-entry {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}

.stream-editor.theme-dark .stream-entry:hover {
  background: rgba(30, 41, 59, 0.66);
}

.stream-editor.theme-dark .entry-id,
.stream-editor.theme-dark .field-key {
  color: #67e8f9;
}

.stream-editor.theme-dark .btn-entry-copy {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
.stream-editor.theme-dark .btn-entry-copy.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}

.stream-editor.theme-dark .btn-entry-copy:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: rgba(103, 232, 249, 0.26);
}

@keyframes copyPulse {
  0% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-1px) scale(1.012); }
  100% { transform: translateY(-1px) scale(1); }
}

.stream-editor.theme-dark .field-pair {
  background: rgba(8, 47, 73, 0.34);
  border-color: rgba(6, 182, 212, 0.18);
}

.stream-editor.theme-dark .field-sep {
  color: #64748b;
}

.stream-editor.theme-dark .field-val {
  color: #cbd5e1;
}
</style>
