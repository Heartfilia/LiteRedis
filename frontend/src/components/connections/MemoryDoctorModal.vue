<template>
  <Teleport to="body">
    <div class="doctor-backdrop backdrop-in" :class="'theme-' + themeMode" @mousedown.self="handleClose">
      <div class="doctor-card dialog-in">
        <header class="doctor-header">
          <div class="doctor-title-wrap">
            <div class="doctor-title">{{ t('memoryDoctor.title') }}</div>
            <div class="doctor-subtitle">
              {{ connectionName }} · db={{ displayDB }} · {{ t('memoryDoctor.subtitle') }}
            </div>
          </div>
          <button class="doctor-close" :title="t('memoryDoctor.close')" @click="handleClose">✕</button>
        </header>

        <div class="doctor-toolbar">
          <label class="doctor-field">
            <span>{{ t('memoryDoctor.pattern') }}</span>
            <input v-model="pattern" :disabled="scanning" placeholder="*" spellcheck="false" @keydown.enter="startScan" />
          </label>
          <label class="doctor-field doctor-field-narrow">
            <span>{{ t('memoryDoctor.maxKeys') }}</span>
            <select v-model.number="maxKeys" :disabled="scanning">
              <option :value="10000">10,000</option>
              <option :value="50000">50,000</option>
              <option :value="200000">200,000</option>
            </select>
          </label>
          <button v-if="!scanning" class="doctor-run" @click="startScan">
            {{ report ? t('memoryDoctor.rescan') : t('memoryDoctor.start') }}
          </button>
          <button v-else class="doctor-run doctor-stop" @click="cancelScan">{{ t('memoryDoctor.cancel') }}</button>
        </div>

        <div v-if="scanning" class="doctor-progress">
          <div class="doctor-progress-track">
            <div class="doctor-progress-bar" :style="{ width: progressPercent + '%' }" />
          </div>
          <div class="doctor-progress-text">
            {{ t('memoryDoctor.scanning') }} · {{ formatCount(progress.scanned) }}
            <template v-if="progress.total > 0"> / {{ formatCount(progress.total) }}</template>
          </div>
        </div>

        <div class="doctor-body">
          <div v-if="error && !report" class="doctor-state error">{{ error }}</div>
          <div v-else-if="!report" class="doctor-state">{{ t('memoryDoctor.emptyHint') }}</div>
          <template v-else>
            <div class="doctor-metrics">
              <div class="doctor-metric">
                <span class="metric-label">{{ t('memoryDoctor.scanned') }}</span>
                <span class="metric-value">{{ formatCount(report.scanned_keys) }}</span>
              </div>
              <div class="doctor-metric">
                <span class="metric-label">{{ t('memoryDoctor.total') }}</span>
                <span class="metric-value">{{ formatCount(report.total_keys) }}</span>
              </div>
              <div class="doctor-metric">
                <span class="metric-label">{{ t('memoryDoctor.memory') }}</span>
                <span class="metric-value">{{ formatBytes(report.total_bytes) }}</span>
              </div>
              <div class="doctor-metric">
                <span class="metric-label">{{ t('memoryDoctor.avgSize') }}</span>
                <span class="metric-value">{{ formatBytes(report.avg_key_bytes) }}</span>
              </div>
              <div class="doctor-metric" :class="{ warn: report.no_ttl_keys > 0 }">
                <span class="metric-label">{{ t('memoryDoctor.noTTL') }}</span>
                <span class="metric-value">{{ formatCount(report.no_ttl_keys) }}</span>
              </div>
            </div>

            <div v-if="report.truncated" class="doctor-note">⚠ {{ t('memoryDoctor.truncated') }}</div>
            <div v-if="sizeSource === 'debug'" class="doctor-note info">ℹ {{ t('memoryDoctor.sizeApprox') }}</div>
            <div v-else-if="sizeSource === 'length'" class="doctor-note">⚠ {{ t('memoryDoctor.sizeUnavailable') }}</div>
            <div v-else-if="report.size_unavailable" class="doctor-note">⚠ {{ t('memoryDoctor.sizeUnavailable') }}</div>

            <div class="doctor-columns">
              <section class="doctor-section">
                <div class="doctor-section-title">{{ t('memoryDoctor.prefixes') }}</div>
                <div v-if="!report.prefixes.length" class="doctor-empty">-</div>
                <div v-for="item in report.prefixes" :key="item.prefix" class="bar-row">
                  <div class="bar-label" :title="item.prefix">{{ item.prefix }}</div>
                  <div class="bar-track">
                    <div class="bar-fill" :style="{ width: barPercent(report.prefixes, item) + '%' }" />
                  </div>
                  <div class="bar-value">{{ formatBytes(item.bytes) }} · {{ formatCount(item.keys) }}</div>
                </div>
              </section>

              <section class="doctor-section">
                <div class="doctor-section-title">{{ t('memoryDoctor.types') }}</div>
                <div v-if="!report.types.length" class="doctor-empty">-</div>
                <div v-for="item in report.types" :key="item.type" class="bar-row">
                  <div class="bar-label" :title="item.type">{{ item.type }}</div>
                  <div class="bar-track">
                    <div
                      class="bar-fill"
                      :style="{ width: barPercent(report.types, item) + '%', background: typeColor(item.type) }"
                    />
                  </div>
                  <div class="bar-value">{{ formatBytes(item.bytes) }} · {{ formatCount(item.keys) }}</div>
                </div>
              </section>
            </div>

            <section class="doctor-section">
              <div class="doctor-section-title">{{ t('memoryDoctor.bigKeys') }}</div>
              <div class="doctor-table-wrap">
                <table class="doctor-table">
                  <thead>
                    <tr>
                      <th class="col-index">#</th>
                      <th>{{ t('memoryDoctor.colKey') }}</th>
                      <th class="col-type">{{ t('memoryDoctor.colType') }}</th>
                      <th class="col-size">{{ lengthMode ? t('memoryDoctor.colItems') : t('memoryDoctor.colSize') }}</th>
                      <th class="col-ttl">{{ t('memoryDoctor.colTTL') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(item, index) in report.big_keys" :key="item.key">
                      <td class="col-index">{{ index + 1 }}</td>
                      <td class="col-key" :title="t('memoryDoctor.clickToCopy')" @click="copyKey(item.key)">{{ item.key }}</td>
                      <td class="col-type">
                        <span class="type-pill" :style="typePillStyle(item.type)">{{ item.type }}</span>
                      </td>
                      <td class="col-size">{{ sizeText(item) }}</td>
                      <td class="col-ttl">{{ ttlText(item.ttl) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </template>
        </div>

        <footer class="doctor-footer">
          <span v-if="report" class="doctor-meta">{{ t('memoryDoctor.elapsed') }} {{ report.elapsed_ms }} ms</span>
          <div class="doctor-actions">
            <button class="doctor-ghost" @click="handleClose">{{ t('memoryDoctor.close') }}</button>
            <button class="doctor-ghost" :disabled="!report" @click="exportMarkdown">{{ t('memoryDoctor.export') }}</button>
          </div>
        </footer>

        <Transition name="doctor-toast">
          <div v-if="toast" class="doctor-toast" :class="{ error: !toastOk }">{{ toast }}</div>
        </Transition>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { cancelMemoryScan, getConnectionDB, scanMemoryReport } from '../../api/wails.js'
import { EventsOff, EventsOn } from '../../../wailsjs/runtime/runtime.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { formatBytes, formatCount } from '../../utils/format.js'
import { getTypeColor } from '../../utils/typeColors.js'
import { copyToClipboard } from '../../utils/clipboard.js'

const props = defineProps({
  connId: { type: String, required: true },
  connectionName: { type: String, default: '' },
})
const emit = defineEmits(['close'])
let disposeEscape = null

const settingsStore = useSettingsStore()
const { t } = useI18n()

const themeMode = computed(() => settingsStore.themeMode || 'light')
const pattern = ref('*')
const maxKeys = ref(200000)
const scanning = ref(false)
const report = ref(null)
const currentDB = ref(null)
const error = ref('')
const progress = ref({ scanned: 0, total: 0 })
const toast = ref('')
const toastOk = ref(true)
let toastTimer = null

const sizeSource = computed(() => report.value?.size_source || 'memory')
const lengthMode = computed(() => sizeSource.value === 'length')

const progressPercent = computed(() => {
  const total = Number(progress.value.total) || 0
  if (total <= 0) return scanning.value ? 30 : 0
  return Math.max(2, Math.min(100, Math.round((Number(progress.value.scanned) / total) * 100)))
})

const displayDB = computed(() => {
  const db = report.value?.db ?? currentDB.value
  return db === null || db === undefined ? '-' : db
})

onMounted(async () => {
  disposeEscape = registerEscapeHandler(handleClose)
  EventsOn('memory:progress', handleProgress)
  try {
    currentDB.value = await getConnectionDB(props.connId)
  } catch (e) {
    currentDB.value = null
  }
})

onBeforeUnmount(() => {
  if (disposeEscape) disposeEscape()
  EventsOff('memory:progress')
  if (scanning.value) cancelMemoryScan().catch(() => {})
  if (toastTimer) clearTimeout(toastTimer)
})

function handleProgress(payload) {
  if (!scanning.value || !payload) return
  progress.value = {
    scanned: Number(payload.scanned) || 0,
    total: Number(payload.total) || 0,
  }
}

function showToast(message, success = true) {
  toast.value = message
  toastOk.value = success
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => { toast.value = '' }, 2600)
}

async function startScan() {
  if (scanning.value) return
  scanning.value = true
  error.value = ''
  progress.value = { scanned: 0, total: 0 }
  try {
    report.value = await scanMemoryReport(props.connId, pattern.value.trim() || '*', maxKeys.value)
  } catch (e) {
    error.value = e?.message || String(e)
    report.value = null
  } finally {
    scanning.value = false
  }
}

async function cancelScan() {
  try {
    await cancelMemoryScan()
  } catch (e) {
    // 取消失败不影响界面复位
  }
}

function handleClose() {
  if (scanning.value) {
    cancelScan()
    scanning.value = false
  }
  emit('close')
}

function barPercent(list, item) {
  const field = report.value?.size_unavailable ? 'keys' : 'bytes'
  const max = Math.max(1, ...list.map((entry) => Number(entry[field]) || 0))
  return Math.max(2, Math.round(((Number(item[field]) || 0) / max) * 100))
}

// 服务端连元素数量都拿不到时，大 key 表改展示元素数而不是字节数
function sizeText(item) {
  if (!lengthMode.value) return formatBytes(item.size)
  if (item.type === 'string') return formatBytes(item.len)
  return (item.len || 0) + ' ' + t('memoryDoctor.itemsUnit')
}

function typeColor(type) {
  return getTypeColor(type).dot
}

function typePillStyle(type) {
  const color = getTypeColor(type)
  return { background: color.bg, color: color.text }
}

function ttlText(ttl) {
  const value = Number(ttl)
  if (!Number.isFinite(value) || value < 0) return t('memoryDoctor.ttlNever')
  if (value < 60) return value + 's'
  if (value < 3600) return Math.round(value / 60) + 'm'
  if (value < 86400) return (value / 3600).toFixed(1) + 'h'
  return (value / 86400).toFixed(1) + 'd'
}

async function copyKey(key) {
  if (await copyToClipboard(key)) showToast(t('memoryDoctor.keyCopied'))
}

async function exportMarkdown() {
  if (!report.value) return
  const lines = []
  lines.push('# ' + t('memoryDoctor.title'))
  lines.push('')
  lines.push('- ' + t('memoryDoctor.connNameLabel') + ': ' + (props.connectionName || '-'))
  lines.push('- DB: ' + report.value.db)
  lines.push('- Pattern: ' + report.value.pattern)
  lines.push('- ' + t('memoryDoctor.scanned') + ': ' + report.value.scanned_keys + ' / ' + report.value.total_keys)
  lines.push('- ' + t('memoryDoctor.memory') + ': ' + formatBytes(report.value.total_bytes) + ' (' + t('memoryDoctor.avgSize') + ' ' + formatBytes(report.value.avg_key_bytes) + ')')
  lines.push('- ' + t('memoryDoctor.noTTL') + ': ' + report.value.no_ttl_keys)
  lines.push('')
  lines.push('## ' + t('memoryDoctor.prefixes'))
  lines.push('')
  lines.push('| ' + t('memoryDoctor.colPrefix') + ' | ' + t('memoryDoctor.colCount') + ' | ' + t('memoryDoctor.colSize') + ' |')
  lines.push('| --- | --- | --- |')
  for (const item of report.value.prefixes) {
    lines.push('| ' + item.prefix + ' | ' + item.keys + ' | ' + formatBytes(item.bytes) + ' |')
  }
  lines.push('')
  lines.push('## ' + t('memoryDoctor.bigKeys'))
  lines.push('')
  lines.push('| ' + t('memoryDoctor.colKey') + ' | ' + t('memoryDoctor.colType') + ' | ' + t('memoryDoctor.colSize') + ' | ' + t('memoryDoctor.colTTL') + ' |')
  lines.push('| --- | --- | --- | --- |')
  for (const item of report.value.big_keys) {
    lines.push('| ' + item.key + ' | ' + item.type + ' | ' + formatBytes(item.size) + ' | ' + ttlText(item.ttl) + ' |')
  }
  if (report.value.truncated) {
    lines.push('')
    lines.push('> ' + t('memoryDoctor.truncated'))
  }
  const copied = await copyToClipboard(lines.join('\n'))
  showToast(copied ? t('memoryDoctor.exported') : t('memoryDoctor.exportFailed'), copied)
}
</script>

<style scoped>
.doctor-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1300;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(15, 23, 42, 0.34);
  backdrop-filter: blur(3px);
}

.doctor-card {
  position: relative;
  width: min(1040px, 96vw);
  max-height: 92vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 20px;
  border: 1px solid rgba(214, 224, 236, 0.84);
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 32px 84px rgba(15, 23, 42, 0.22), 0 10px 24px rgba(148, 163, 184, 0.08);
}

.doctor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px 14px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(255, 255, 255, 0.94));
}

.doctor-title {
  font-size: 15px;
  font-weight: 700;
  color: #1e293b;
}

.doctor-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: #64748b;
}

.doctor-close {
  border: none;
  background: transparent;
  color: #94a3b8;
  font-size: 15px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: var(--radius-md);
  line-height: 1;
}

.doctor-close:hover {
  background: rgba(148, 163, 184, 0.16);
  color: #475569;
}

.doctor-toolbar {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  padding: 14px 18px 10px;
}

.doctor-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
  flex: 1;
  min-width: 0;
}

.doctor-field-narrow {
  flex: 0 0 150px;
}

.doctor-field span {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #94a3b8;
}

.doctor-field input,
.doctor-field select {
  height: 34px;
  padding: 0 10px;
  border-radius: 9px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #1e293b;
  font-size: 13px;
  outline: none;
}

.doctor-field input:focus,
.doctor-field select:focus {
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.18);
}

.doctor-run {
  height: 34px;
  padding: 0 18px;
  border: none;
  border-radius: 9px;
  background: linear-gradient(180deg, #3b82f6, #2563eb);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}

.doctor-run:disabled {
  opacity: 0.55;
  cursor: default;
}

.doctor-stop {
  background: linear-gradient(180deg, #f87171, #dc2626);
}

.doctor-progress {
  padding: 0 18px 8px;
}

.doctor-progress-track {
  height: 6px;
  border-radius: var(--radius-pill);
  background: rgba(226, 232, 240, 0.9);
  overflow: hidden;
}

.doctor-progress-bar {
  height: 100%;
  border-radius: var(--radius-pill);
  background: linear-gradient(90deg, #60a5fa, #2563eb);
  transition: width 0.25s ease;
}

.doctor-progress-text {
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
}

.doctor-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 18px 16px;
}

.doctor-state {
  padding: 56px 0;
  text-align: center;
  font-size: 13px;
  color: #94a3b8;
}

.doctor-state.error {
  color: #dc2626;
}

.doctor-metrics {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.doctor-metric {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border-radius: var(--radius-lg);
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.86);
}

.doctor-metric.warn {
  border-color: rgba(251, 191, 36, 0.6);
  background: rgba(254, 243, 199, 0.5);
}

.metric-label {
  font-size: 11px;
  color: #94a3b8;
}

.metric-value {
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.doctor-note.info {
  border-color: rgba(96, 165, 250, 0.55);
  background: rgba(219, 234, 254, 0.6);
  color: #1e40af;
}

.doctor-note {
  margin-bottom: 10px;
  padding: 9px 12px;
  border-radius: 10px;
  border: 1px solid rgba(251, 191, 36, 0.55);
  background: rgba(254, 243, 199, 0.55);
  color: #92400e;
  font-size: 12px;
}

.doctor-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.doctor-section {
  min-width: 0;
  margin-bottom: 16px;
}

.doctor-section-title {
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #64748b;
}

.doctor-empty {
  font-size: 12px;
  color: #94a3b8;
}

.bar-row {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr) 118px;
  align-items: center;
  gap: 10px;
  padding: 3px 0;
}

.bar-label {
  font-size: 12px;
  color: #334155;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.bar-track {
  height: 8px;
  border-radius: var(--radius-pill);
  background: rgba(226, 232, 240, 0.86);
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  border-radius: var(--radius-pill);
  background: linear-gradient(90deg, #93c5fd, #3b82f6);
  transition: width 0.3s ease;
}

.bar-value {
  font-size: 11px;
  color: #64748b;
  text-align: right;
  white-space: nowrap;
}

.doctor-table-wrap {
  max-height: 320px;
  overflow: auto;
  border-radius: var(--radius-lg);
  border: 1px solid rgba(226, 232, 240, 0.96);
}

.doctor-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.doctor-table thead th {
  position: sticky;
  top: 0;
  z-index: 1;
  padding: 9px 12px;
  text-align: left;
  font-weight: 600;
  color: #64748b;
  background: rgba(248, 250, 252, 0.98);
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  white-space: nowrap;
}

.doctor-table tbody td {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(241, 245, 249, 0.96);
  color: #334155;
  vertical-align: middle;
}

.doctor-table tbody tr:last-child td {
  border-bottom: none;
}

.doctor-table tbody tr:hover td {
  background: rgba(239, 246, 255, 0.7);
}

.col-index {
  width: 44px;
  color: #94a3b8;
}

.col-key {
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  cursor: pointer;
}

.col-key:hover {
  color: #2563eb;
  text-decoration: underline;
}

.col-type {
  width: 90px;
}

.col-size {
  width: 92px;
  font-variant-numeric: tabular-nums;
}

.col-ttl {
  width: 90px;
  font-variant-numeric: tabular-nums;
  color: #64748b;
}

.type-pill {
  display: inline-block;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.doctor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 18px;
  border-top: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.9);
}

.doctor-meta {
  font-size: 12px;
  color: #94a3b8;
}

.doctor-actions {
  display: flex;
  gap: 10px;
}

.doctor-ghost {
  height: 32px;
  padding: 0 16px;
  border-radius: 9px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #475569;
  font-size: 12px;
  cursor: pointer;
}

.doctor-ghost:hover:not(:disabled) {
  border-color: #93c5fd;
  color: #2563eb;
}

.doctor-ghost:disabled {
  opacity: 0.5;
  cursor: default;
}

.doctor-toast {
  position: absolute;
  left: 50%;
  bottom: 64px;
  transform: translateX(-50%);
  padding: 8px 16px;
  border-radius: var(--radius-pill);
  font-size: 12px;
  color: #065f46;
  background: rgba(209, 250, 229, 0.96);
  border: 1px solid rgba(16, 185, 129, 0.4);
  box-shadow: 0 10px 26px rgba(15, 23, 42, 0.14);
}

.doctor-toast.error {
  color: #991b1b;
  background: rgba(254, 226, 226, 0.96);
  border-color: rgba(239, 68, 68, 0.4);
}

.doctor-toast-enter-active,
.doctor-toast-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.doctor-toast-enter-from,
.doctor-toast-leave-to {
  opacity: 0;
  transform: translate(-50%, 6px);
}

/* 深色主题 */
.doctor-backdrop.theme-dark {
  background: rgba(2, 6, 23, 0.6);
}

.doctor-backdrop.theme-dark .doctor-card {
  border-color: rgba(51, 65, 85, 0.82);
  background: rgba(15, 23, 42, 0.98);
  box-shadow: 0 34px 86px rgba(2, 6, 23, 0.54);
}

.doctor-backdrop.theme-dark .doctor-header {
  border-bottom-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.94));
}

.doctor-backdrop.theme-dark .doctor-title,
.doctor-backdrop.theme-dark .metric-value {
  color: #e2e8f0;
}

.doctor-backdrop.theme-dark .doctor-subtitle,
.doctor-backdrop.theme-dark .doctor-meta,
.doctor-backdrop.theme-dark .doctor-progress-text,
.doctor-backdrop.theme-dark .bar-value {
  color: #94a3b8;
}

.doctor-backdrop.theme-dark .doctor-field input,
.doctor-backdrop.theme-dark .doctor-field select {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.9);
  color: #e2e8f0;
}

.doctor-backdrop.theme-dark .doctor-metric {
  border-color: rgba(51, 65, 85, 0.9);
  background: rgba(30, 41, 59, 0.6);
}

.doctor-backdrop.theme-dark .doctor-metric.warn {
  border-color: rgba(217, 119, 6, 0.6);
  background: rgba(120, 53, 15, 0.28);
}

.doctor-backdrop.theme-dark .metric-label {
  color: #94a3b8;
}

.doctor-backdrop.theme-dark .doctor-section-title {
  color: #94a3b8;
}

.doctor-backdrop.theme-dark .bar-label {
  color: #cbd5e1;
}

.doctor-backdrop.theme-dark .bar-track,
.doctor-backdrop.theme-dark .doctor-progress-track {
  background: rgba(51, 65, 85, 0.8);
}

.doctor-backdrop.theme-dark .doctor-table-wrap {
  border-color: rgba(51, 65, 85, 0.9);
}

.doctor-backdrop.theme-dark .doctor-table thead th {
  background: rgba(30, 41, 59, 0.98);
  border-bottom-color: rgba(51, 65, 85, 0.9);
  color: #94a3b8;
}

.doctor-backdrop.theme-dark .doctor-table tbody td {
  border-bottom-color: rgba(51, 65, 85, 0.6);
  color: #cbd5e1;
}

.doctor-backdrop.theme-dark .doctor-table tbody tr:hover td {
  background: rgba(30, 58, 95, 0.5);
}

.doctor-backdrop.theme-dark .doctor-footer {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: rgba(15, 23, 42, 0.94);
}

.doctor-backdrop.theme-dark .doctor-ghost {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(30, 41, 59, 0.7);
  color: #cbd5e1;
}

.doctor-backdrop.theme-dark .doctor-note.info {
  border-color: rgba(59, 130, 246, 0.55);
  background: rgba(30, 58, 95, 0.5);
  color: #bfdbfe;
}

.doctor-backdrop.theme-dark .doctor-note {
  border-color: rgba(217, 119, 6, 0.6);
  background: rgba(120, 53, 15, 0.32);
  color: #fcd34d;
}
</style>
