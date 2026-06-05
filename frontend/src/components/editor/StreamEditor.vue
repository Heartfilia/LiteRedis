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
.stream-editor { display: flex; flex-direction: column; height: 100%; gap: 8px; }
.toolbar { display: flex; align-items: center; }
.count { font-size: 12px; color: #9ca3af; margin-left: auto; }
.stream-wrap { flex: 1; overflow-y: auto; }
.stream-entry { border-bottom: 1px solid #f0f0f0; padding: 7px 10px; font-size: 12px; }
.stream-entry:hover { background: #f9fafb; }
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
  background: #ecfeff;
  color: #0e7490;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  min-width: 24px;
  text-align: center;
  flex-shrink: 0;
  font-weight: 500;
}
.entry-id { font-family: monospace; color: #0e7490; font-weight: 500; font-size: 11px; flex: 1; }
.btn-entry-copy {
  margin-left: auto;
  display: inline-flex; align-items: center; justify-content: center;
  padding: 2px 9px;
  background: #fff;
  color: #6b7280;
  border: 1px solid #e5e7eb;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
  white-space: nowrap;
  flex-shrink: 0;
}
.btn-entry-copy:hover { background: #f3f4f6; border-color: #d1d5db; color: #374151; }
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
  background: #ecfeff;
  padding: 2px 7px;
  border-radius: 4px;
  border: 1px solid #cffafe;
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
.load-more {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 36px;
  padding: 5px 12px;
  margin: -8px -12px 0;
  border-top: 1px solid #e8e8e8;
  background: #fafafa;
  flex-shrink: 0;
  box-sizing: border-box;
}
.btn-load-more {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 4px 18px;
  background: #fff;
  color: #6b7280;
  border: 1px solid #e5e7eb;
  border-radius: 20px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}
.btn-load-more:hover { background: #f9fafb; border-color: #9ca3af; color: #374151; }
:global(.app-layout.theme-dark) .stream-editor {
  color: #e2e8f0;
}
:global(.app-layout.theme-dark) .stream-editor .count {
  color: #94a3b8;
}
:global(.app-layout.theme-dark) .stream-editor .entry-num {
  background: rgba(8, 47, 73, 0.42);
  color: #67e8f9;
}
:global(.app-layout.theme-dark) .stream-editor .stream-entry {
  border-bottom-color: rgba(30, 41, 59, 0.92);
}
:global(.app-layout.theme-dark) .stream-editor .stream-entry:hover {
  background: rgba(30, 41, 59, 0.66);
}
:global(.app-layout.theme-dark) .stream-editor .entry-id,
:global(.app-layout.theme-dark) .stream-editor .field-key {
  color: #67e8f9;
}
:global(.app-layout.theme-dark) .stream-editor .btn-entry-copy {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
:global(.app-layout.theme-dark) .stream-editor .btn-entry-copy.copied {
  background: rgba(30, 64, 175, 0.34);
  color: #dbeafe;
  border-color: rgba(147, 197, 253, 0.72);
  box-shadow: 0 0 14px rgba(59, 130, 246, 0.2);
}
:global(.app-layout.theme-dark) .stream-editor .btn-entry-copy:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #e2e8f0;
  border-color: rgba(103, 232, 249, 0.26);
}
:global(.app-layout.theme-dark) .stream-editor .field-pair {
  background: rgba(8, 47, 73, 0.34);
  border-color: rgba(6, 182, 212, 0.18);
}
:global(.app-layout.theme-dark) .stream-editor .field-sep {
  color: #64748b;
}
:global(.app-layout.theme-dark) .stream-editor .field-val {
  color: #cbd5e1;
}
:global(.app-layout.theme-dark) .stream-editor .load-more {
  border-top-color: rgba(51, 65, 85, 0.92);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.82), rgba(15, 23, 42, 0.98));
}
:global(.app-layout.theme-dark) .stream-editor .btn-load-more {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.98), rgba(15, 23, 42, 0.98));
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}
:global(.app-layout.theme-dark) .stream-editor .btn-load-more:hover {
  background: linear-gradient(180deg, rgba(8, 47, 73, 0.44), rgba(30, 41, 59, 0.98));
  border-color: rgba(103, 232, 249, 0.28);
  color: #e2e8f0;
}
.stream-editor.theme-dark {
  color: #e2e8f0;
}

.stream-editor.theme-dark .count {
  color: #94a3b8;
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
