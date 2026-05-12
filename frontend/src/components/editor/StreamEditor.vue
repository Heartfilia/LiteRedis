<template>
  <div class="stream-editor">
    <div class="toolbar">
      <span class="count">{{ displayEntries.length }} / {{ entries.length }} {{ t('settings.unitItem') }}</span>
    </div>
    <div class="stream-wrap">
      <div v-for="(entry, idx) in displayEntries" :key="entry.id" class="stream-entry">
        <div class="entry-header">
          <span class="entry-num">{{ idx + 1 }}</span>
          <span class="entry-id">{{ entry.id }}</span>
          <button class="btn-entry-copy" @click="copyEntry(entry)">
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
.load-more { display: flex; justify-content: center; padding: 6px 0; flex-shrink: 0; }
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
</style>
