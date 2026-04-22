<template>
  <div class="stream-editor">
    <div class="toolbar">
      <span class="count">{{ displayEntries.length }} / {{ entries.length }} 条</span>
    </div>
    <div class="stream-wrap">
      <div v-for="(entry, idx) in displayEntries" :key="entry.id" class="stream-entry">
        <div class="entry-header">
          <span class="entry-num">{{ idx + 1 }}</span>
          <span class="entry-id">{{ entry.id }}</span>
          <button class="btn-entry-copy" @click="copyEntry(entry)">
            {{ copiedEntry === entry.id ? '✓ 已复制' : '复制' }}
          </button>
        </div>
        <div class="entry-fields">
          <span v-for="(val, key) in entry.fields" :key="key" class="field-pair">
            <span class="field-key">{{ key }}</span>
            <span class="field-sep">:</span>
            <span class="field-val">{{ truncate(val) }}</span>
            <span v-if="val.length > 80" class="val-ellipsis" @click="openExpand(key, val)">…展开</span>
          </span>
        </div>
      </div>
    </div>

    <!-- 显示更多 -->
    <div v-if="entries.length > displayLimit" class="load-more">
      <button class="btn-load-more" @click="displayLimit += loadCount">
        显示更多（{{ displayLimit }}/{{ entries.length }}）
      </button>
    </div>

    <ExpandModal :show="expandShow" :title="expandTitle" :content="expandContent" @close="expandShow = false" />
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import ExpandModal from './ExpandModal.vue'

const props = defineProps({ keyValue: Object })
const settingsStore = useSettingsStore()

const entries = ref([])
const copiedEntry = ref(null)
const expandShow = ref(false)
const expandTitle = ref('')
const expandContent = ref('')

const loadCount = computed(() => settingsStore.streamLoadCount)
const displayLimit = ref(0)

watch(loadCount, (v) => { displayLimit.value = v }, { immediate: true })

const displayEntries = computed(() => entries.value.slice(0, displayLimit.value))

watch(() => props.keyValue, kv => {
  entries.value = kv?.stream_val || []
  displayLimit.value = loadCount.value
}, { immediate: true })

async function copyEntry(entry) {
  await copyToClipboard(JSON.stringify(entry.fields, null, 2))
  copiedEntry.value = entry.id
  setTimeout(() => { copiedEntry.value = null }, 1200)
}

function truncate(val, max = 80) {
  if (!val) return val
  return val.length > max ? val.slice(0, max) : val
}

function openExpand(key, val) {
  expandTitle.value = key
  expandContent.value = val
  expandShow.value = true
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
.entry-fields { display: flex; flex-wrap: wrap; gap: 6px; }
.field-pair { display: flex; align-items: baseline; gap: 2px; background: #ecfeff; padding: 2px 7px; border-radius: 4px; flex-wrap: wrap; border: 1px solid #cffafe; }
.field-key { color: #0e7490; font-weight: 600; font-size: 11px; }
.field-sep { color: #94a3b8; }
.field-val { color: #1f2937; font-family: monospace; word-break: break-all; max-width: 200px; font-size: 11px; }
.val-ellipsis { font-size: 11px; color: #3b82f6; cursor: pointer; white-space: nowrap; flex-shrink: 0; }
.val-ellipsis:hover { text-decoration: underline; }
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
