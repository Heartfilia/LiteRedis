<template>
  <Teleport to="body">
    <div class="monitor-backdrop backdrop-in" :class="'theme-' + themeMode" @mousedown.self="handleClose">
      <div class="monitor-card dialog-in">
        <header class="monitor-header">
          <div class="monitor-title-wrap">
            <div class="monitor-title">{{ t('monitor.title') }}</div>
            <div class="monitor-subtitle">
              {{ connectionName }} · db={{ currentDB }} · {{ t('monitor.subtitle') }}
            </div>
          </div>
          <div class="monitor-tools">
            <select v-model.number="intervalMs" class="monitor-interval" @change="onIntervalChange">
              <option :value="2000">2s</option>
              <option :value="5000">5s</option>
              <option :value="10000">10s</option>
              <option :value="30000">30s</option>
              <option :value="60000">60s</option>
            </select>
            <button class="monitor-close" :title="t('monitor.close')" @click="handleClose">✕</button>
          </div>
        </header>

        <div class="monitor-tabs">
          <button class="monitor-tab" :class="{ active: tab === 'metrics' }" @click="switchTab('metrics')">
            {{ t('monitor.tabMetrics') }}
          </button>
          <button class="monitor-tab" :class="{ active: tab === 'slowlog' }" @click="switchTab('slowlog')">
            {{ t('monitor.tabSlowlog') }}
          </button>
        </div>

        <div v-if="tab === 'slowlog'" class="monitor-body">
          <div class="slowlog-head">
            <span class="slowlog-hint">{{ t('monitor.slowlogHint') }}</span>
            <button class="slowlog-refresh" :disabled="slowlogLoading" @click="loadSlowlog">
              {{ slowlogLoading ? t('monitor.sampling') : t('monitor.refresh') }}
            </button>
          </div>
          <div v-if="slowlogError" class="monitor-note error">⚠ {{ slowlogError }}</div>
          <div v-else-if="!slowlog.length" class="slowlog-empty">{{ t('monitor.slowlogEmpty') }}</div>
          <div v-else class="slowlog-list">
            <div
              v-for="entry in slowlog"
              :key="entry.id + '-' + (entry.node || '')"
              class="slowlog-row"
              :title="t('monitor.jumpToKey')"
              @click="jumpToKey(entry)"
            >
              <span class="slowlog-time">{{ formatTime(entry.timestamp) }}</span>
              <span class="slowlog-duration">{{ (entry.duration_us / 1000).toFixed(1) }} ms</span>
              <span class="slowlog-command">{{ entry.command }}</span>
              <span class="slowlog-client">{{ entry.client_addr || '-' }}</span>
            </div>
          </div>
        </div>

        <div v-else class="monitor-body">
          <div v-if="error" class="monitor-note error">
            ⚠ {{ error }}
            <button v-if="stopped" class="monitor-retry" @click="retry">{{ t('monitor.retry') }}</button>
          </div>
          <div v-else-if="paused" class="monitor-note info">{{ t('monitor.paused') }}</div>
          <div v-else-if="isCluster" class="monitor-note info">{{ t('monitor.clusterHint') }}</div>

          <div class="monitor-grid">
            <section class="metric-card">
              <div class="metric-head">
                <span class="metric-label">{{ t('monitor.ops') }}</span>
                <span class="metric-value">{{ formatCount(latest?.instant_ops_per_sec || 0) }}</span>
              </div>
              <svg class="spark" viewBox="0 0 100 26" preserveAspectRatio="none" aria-hidden="true">
                <polyline :points="sparkPoints(opsSeries, maxOf(opsSeries))" fill="none" stroke="#3b82f6" stroke-width="1.5" vector-effect="non-scaling-stroke" />
              </svg>
            </section>

            <section class="metric-card">
              <div class="metric-head">
                <span class="metric-label">{{ t('monitor.hit') }}</span>
                <span class="metric-value">{{ hitPercent }}%</span>
              </div>
              <svg class="spark" viewBox="0 0 100 26" preserveAspectRatio="none" aria-hidden="true">
                <polyline :points="sparkPoints(hitSeries, 100)" fill="none" stroke="#10b981" stroke-width="1.5" vector-effect="non-scaling-stroke" />
              </svg>
            </section>

            <section class="metric-card">
              <div class="metric-head">
                <span class="metric-label">{{ t('monitor.clients') }}</span>
                <span class="metric-value">{{ formatCount(latest?.connected_clients || 0) }}</span>
              </div>
              <svg class="spark" viewBox="0 0 100 26" preserveAspectRatio="none" aria-hidden="true">
                <polyline :points="sparkPoints(clientSeries, maxOf(clientSeries))" fill="none" stroke="#8b5cf6" stroke-width="1.5" vector-effect="non-scaling-stroke" />
              </svg>
            </section>
          </div>

          <section class="memory-card">
            <div class="memory-head">
              <span class="metric-label">{{ t('monitor.memory') }}</span>
              <span class="memory-text">
                {{ formatBytes(latest?.used_memory_bytes || 0) }}<template v-if="maxMemoryBytes > 0"> / {{ formatBytes(maxMemoryBytes) }}</template>
              </span>
            </div>
            <div class="memory-track">
              <div class="memory-fill" :class="{ warn: memoryPercent >= 85 }" :style="{ width: memoryPercent + '%' }" />
            </div>
            <div class="memory-meta">
              <span>{{ t('monitor.keys') }} {{ formatCount(latest?.total_keys || 0) }}</span>
              <span>{{ t('monitor.uptime') }} {{ latest?.uptime_human || '-' }}</span>
              <span v-if="latest?.evicted_keys">{{ t('monitor.evicted') }} {{ formatCount(latest.evicted_keys) }}</span>
              <span v-if="latest?.blocked_clients">{{ t('monitor.blocked') }} {{ latest.blocked_clients }}</span>
            </div>
          </section>

          <div class="monitor-foot-tip">
            <span class="dot" :class="{ live: !paused && !error }" />
            {{ paused ? t('monitor.pausedShort') : error ? t('monitor.disconnected') : t('monitor.live') }}
            <span v-if="samples.length > 1" class="sample-count">· {{ samples.length }}</span>
          </div>
        </div>

        <footer class="monitor-footer">
          <button class="monitor-ghost" @click="handleClose">{{ t('monitor.close') }}</button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { getConnectionOverview, getSlowLog } from '../../api/wails.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { formatBytes, formatCount } from '../../utils/format.js'

const MAX_SAMPLES = 60

const props = defineProps({
  connId: { type: String, required: true },
  connectionName: { type: String, default: '' },
})
const emit = defineEmits(['close'])
let disposeEscape = null

const settingsStore = useSettingsStore()
const workspaceStore = useWorkspaceStore()
const { t } = useI18n()

const themeMode = computed(() => settingsStore.themeMode || 'light')
const latest = ref(null)
const samples = ref([])
const error = ref('')
const paused = ref(false)
const stopped = ref(false)
const tab = ref('metrics')
const slowlog = ref([])
const slowlogLoading = ref(false)
const slowlogError = ref('')
const intervalMs = ref(5000)
const isCluster = ref(false)
let timer = null
let disposed = false
let intervalTouched = false
let failures = 0

const currentDB = computed(() => latest.value?.current_db ?? '-')
const maxMemoryBytes = computed(() => latest.value?.max_memory_bytes || 0)
const hitPercent = computed(() => {
  const rate = Number(latest.value?.hit_rate) || 0
  return (rate * 100).toFixed(1)
})
const memoryPercent = computed(() => {
  const max = maxMemoryBytes.value
  if (max <= 0) return 0
  const used = Number(latest.value?.used_memory_bytes) || 0
  return Math.max(0, Math.min(100, Math.round((used / max) * 100)))
})

const opsSeries = computed(() => samples.value.map((item) => item.ops))
const hitSeries = computed(() => samples.value.map((item) => item.hit))
const clientSeries = computed(() => samples.value.map((item) => item.clients))

function maxOf(series) {
  return Math.max(1, ...series)
}

function sparkPoints(series, max) {
  if (!series.length) return ''
  const width = 100
  const height = 26
  const step = series.length > 1 ? width / (series.length - 1) : 0
  return series
    .map((value, index) => {
      const x = series.length > 1 ? index * step : width
      const y = height - Math.min(1, value / max) * height
      return x.toFixed(1) + ',' + y.toFixed(1)
    })
    .join(' ')
}

function pushSample(data) {
  const next = samples.value.slice()
  next.push({
    ops: Number(data.instant_ops_per_sec) || 0,
    hit: (Number(data.hit_rate) || 0) * 100,
    clients: Number(data.connected_clients) || 0,
  })
  while (next.length > MAX_SAMPLES) next.shift()
  samples.value = next
}

async function poll() {
  if (disposed) return
  try {
    const data = await getConnectionOverview(props.connId)
    if (disposed) return
    latest.value = data
    error.value = ''
    failures = 0
    pushSample(data)
    if (data.is_cluster) {
      isCluster.value = true
      if (!intervalTouched && intervalMs.value < 5000) {
        intervalMs.value = 5000
      }
    }
  } catch (e) {
    if (disposed) return
    failures++
    error.value = e?.message || String(e)
    if (failures >= 3) {
      // 连续失败说明连接已经不可用，别继续每几秒打一次注定失败的请求
      stopped.value = true
      stopTimer()
      error.value = error.value + ' · ' + t('monitor.stoppedHint')
    }
  } finally {
    schedule()
  }
}

async function switchTab(next) {
  if (tab.value === next) return
  tab.value = next
  stopTimer()
  if (next === 'slowlog') {
    await loadSlowlog()
    return
  }
  if (!paused.value && !stopped.value) poll()
}

async function loadSlowlog() {
  slowlogLoading.value = true
  slowlogError.value = ''
  try {
    slowlog.value = await getSlowLog(props.connId, 64)
  } catch (e) {
    slowlogError.value = e?.message || String(e)
  } finally {
    slowlogLoading.value = false
  }
}

function formatTime(timestamp) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleTimeString()
}

// 慢查询里绝大多数命令的第二段就是 key，点了直接跳到编辑器
function jumpToKey(entry) {
  const parts = String(entry?.command || '').trim().split(/\s+/)
  const key = parts.length > 1 ? parts[1] : ''
  if (!key) return
  emit('close')
  workspaceStore.selectKey(key)
}

function retry() {
  stopped.value = false
  failures = 0
  error.value = ''
  poll()
}

function schedule() {
  // 切到慢查询页时暂停指标采样，减少无谓的 INFO 请求
  if (disposed || paused.value || stopped.value || tab.value !== 'metrics') return
  if (timer) clearTimeout(timer)
  timer = setTimeout(poll, intervalMs.value)
}

function stopTimer() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
}

function onIntervalChange() {
  intervalTouched = true
  stopTimer()
  schedule()
}

function onVisibilityChange() {
  if (document.hidden) {
    paused.value = true
    stopTimer()
    return
  }
  if (!paused.value) return
  paused.value = false
  poll()
}

function handleClose() {
  stopTimer()
  disposed = true
  emit('close')
}

onMounted(() => {
  disposeEscape = registerEscapeHandler(handleClose)
  document.addEventListener('visibilitychange', onVisibilityChange)
  poll()
})

onBeforeUnmount(() => {
  if (disposeEscape) disposeEscape()
  disposed = true
  stopTimer()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<style scoped>
.monitor-backdrop {
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

.monitor-card {
  width: min(720px, 96vw);
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 20px;
  border: 1px solid rgba(214, 224, 236, 0.84);
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 32px 84px rgba(15, 23, 42, 0.22);
}

.monitor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px 14px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(255, 255, 255, 0.94));
}

.monitor-title {
  font-size: 15px;
  font-weight: 700;
  color: #1e293b;
}

.monitor-subtitle {
  margin-top: 4px;
  font-size: 12px;
  color: #64748b;
}

.monitor-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}

.monitor-interval {
  height: 28px;
  padding: 0 8px;
  border-radius: var(--radius-md);
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #1e293b;
  font-size: 12px;
  outline: none;
}

.monitor-close {
  border: none;
  background: transparent;
  color: #94a3b8;
  font-size: 15px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: var(--radius-md);
  line-height: 1;
}

.monitor-close:hover {
  background: rgba(148, 163, 184, 0.16);
  color: #475569;
}

.monitor-tabs {
  display: flex;
  gap: 6px;
  padding: 0 18px 12px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
}
.monitor-tab {
  height: 26px;
  padding: 0 12px;
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  background: transparent;
  color: #64748b;
  font-size: 12px;
  cursor: pointer;
}
.monitor-tab:hover { background: rgba(241, 245, 249, 0.9); }
.monitor-tab.active {
  background: rgba(219, 234, 254, 0.75);
  border-color: rgba(147, 197, 253, 0.9);
  color: #1d4ed8;
  font-weight: 600;
}
.slowlog-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}
.slowlog-hint { font-size: 11px; color: #94a3b8; }
.slowlog-refresh {
  height: 24px;
  padding: 0 10px;
  border-radius: var(--radius-md);
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #475569;
  font-size: 11px;
  cursor: pointer;
}
.slowlog-refresh:disabled { opacity: 0.6; cursor: default; }
.slowlog-empty {
  padding: 48px 0;
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
}
.slowlog-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 360px;
  overflow-y: auto;
}
.slowlog-row {
  display: grid;
  grid-template-columns: 74px 72px minmax(0, 1fr) 130px;
  gap: 8px;
  align-items: center;
  padding: 6px 8px;
  border-radius: var(--radius-md);
  cursor: pointer;
  font-size: 12px;
  color: #334155;
}
.slowlog-row:hover { background: rgba(239, 246, 255, 0.9); }
.slowlog-time { color: #64748b; font-variant-numeric: tabular-nums; }
.slowlog-duration { color: #dc2626; font-weight: 600; font-variant-numeric: tabular-nums; }
.slowlog-command {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.slowlog-client { color: #94a3b8; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.monitor-backdrop.theme-dark .monitor-tabs { border-bottom-color: rgba(51, 65, 85, 0.94); }
.monitor-backdrop.theme-dark .monitor-tab { color: #94a3b8; }
.monitor-backdrop.theme-dark .monitor-tab:hover { background: rgba(30, 41, 59, 0.9); }
.monitor-backdrop.theme-dark .monitor-tab.active {
  background: rgba(30, 58, 95, 0.7);
  border-color: rgba(59, 130, 246, 0.6);
  color: #bfdbfe;
}
.monitor-backdrop.theme-dark .slowlog-row { color: #cbd5e1; }
.monitor-backdrop.theme-dark .slowlog-row:hover { background: rgba(30, 41, 59, 0.9); }
.monitor-backdrop.theme-dark .slowlog-refresh {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(30, 41, 59, 0.7);
  color: #cbd5e1;
}
.monitor-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px 18px 16px;
}

.monitor-note {
  margin-bottom: 12px;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 12px;
  border: 1px solid rgba(251, 191, 36, 0.55);
  background: rgba(254, 243, 199, 0.55);
  color: #92400e;
}

.monitor-note.info {
  border-color: rgba(96, 165, 250, 0.5);
  background: rgba(219, 234, 254, 0.6);
  color: #1e40af;
}

.monitor-note.error {
  border-color: rgba(248, 113, 113, 0.55);
  background: rgba(254, 226, 226, 0.7);
  color: #991b1b;
}
.monitor-retry {
  margin-left: 8px;
  height: 22px;
  padding: 0 10px;
  border-radius: var(--radius-sm);
  border: 1px solid currentColor;
  background: transparent;
  color: inherit;
  font-size: 11px;
  cursor: pointer;
}

.monitor-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.metric-card {
  padding: 12px 14px 8px;
  border-radius: var(--radius-lg);
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.86);
}

.metric-head {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.metric-label {
  font-size: 11px;
  color: #94a3b8;
}

.metric-value {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.spark {
  display: block;
  width: 100%;
  height: 30px;
  margin-top: 6px;
}

.memory-card {
  padding: 14px;
  border-radius: var(--radius-lg);
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.86);
}

.memory-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 8px;
}

.memory-text {
  font-size: 14px;
  font-weight: 700;
  color: #1e293b;
  font-variant-numeric: tabular-nums;
}

.memory-track {
  height: 8px;
  border-radius: var(--radius-pill);
  background: rgba(226, 232, 240, 0.9);
  overflow: hidden;
}

.memory-fill {
  height: 100%;
  border-radius: var(--radius-pill);
  background: linear-gradient(90deg, #60a5fa, #2563eb);
  transition: width 0.35s ease;
}

.memory-fill.warn {
  background: linear-gradient(90deg, #fbbf24, #dc2626);
}

.memory-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 10px;
  font-size: 11px;
  color: #64748b;
}

.monitor-foot-tip {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 11px;
  color: #94a3b8;
}

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #cbd5e1;
}

.dot.live {
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.18);
}

.sample-count {
  color: #cbd5e1;
}

.monitor-footer {
  display: flex;
  justify-content: flex-end;
  padding: 10px 18px;
  border-top: 1px solid rgba(226, 232, 240, 0.96);
  background: rgba(248, 250, 252, 0.9);
}

.monitor-ghost {
  height: 32px;
  padding: 0 16px;
  border-radius: 9px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #fff;
  color: #475569;
  font-size: 12px;
  cursor: pointer;
}

.monitor-ghost:hover {
  border-color: #93c5fd;
  color: #2563eb;
}

/* 深色主题 */
.monitor-backdrop.theme-dark {
  background: rgba(2, 6, 23, 0.6);
}

.monitor-backdrop.theme-dark .monitor-card {
  border-color: rgba(51, 65, 85, 0.82);
  background: rgba(15, 23, 42, 0.98);
  box-shadow: 0 34px 86px rgba(2, 6, 23, 0.54);
}

.monitor-backdrop.theme-dark .monitor-header {
  border-bottom-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.94));
}

.monitor-backdrop.theme-dark .monitor-title,
.monitor-backdrop.theme-dark .metric-value,
.monitor-backdrop.theme-dark .memory-text {
  color: #e2e8f0;
}

.monitor-backdrop.theme-dark .monitor-subtitle,
.monitor-backdrop.theme-dark .metric-label,
.monitor-backdrop.theme-dark .memory-meta,
.monitor-backdrop.theme-dark .monitor-foot-tip {
  color: #94a3b8;
}

.monitor-backdrop.theme-dark .metric-card,
.monitor-backdrop.theme-dark .memory-card {
  border-color: rgba(51, 65, 85, 0.9);
  background: rgba(30, 41, 59, 0.6);
}

.monitor-backdrop.theme-dark .memory-track {
  background: rgba(51, 65, 85, 0.8);
}

.monitor-backdrop.theme-dark .monitor-interval {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.9);
  color: #e2e8f0;
}

.monitor-backdrop.theme-dark .monitor-footer {
  border-top-color: rgba(51, 65, 85, 0.94);
  background: rgba(15, 23, 42, 0.94);
}

.monitor-backdrop.theme-dark .monitor-ghost {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(30, 41, 59, 0.7);
  color: #cbd5e1;
}

.monitor-backdrop.theme-dark .monitor-note {
  border-color: rgba(217, 119, 6, 0.6);
  background: rgba(120, 53, 15, 0.32);
  color: #fcd34d;
}

.monitor-backdrop.theme-dark .monitor-note.info {
  border-color: rgba(59, 130, 246, 0.55);
  background: rgba(30, 58, 95, 0.5);
  color: #bfdbfe;
}

.monitor-backdrop.theme-dark .monitor-note.error {
  border-color: rgba(185, 28, 28, 0.6);
  background: rgba(127, 29, 29, 0.42);
  color: #fecaca;
}
</style>
