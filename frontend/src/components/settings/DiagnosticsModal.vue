<template>
  <Teleport to="body">
    <div class="diag-overlay" :class="'theme-' + theme">
      <div class="diag-card">
        <div class="diag-header">
          <span class="diag-title">{{ t('diagnostics.title') }}</span>
          <div class="diag-header-actions">
            <button class="diag-btn" :disabled="loading" @click="load">{{ t('diagnostics.refresh') }}</button>
            <button class="diag-btn" :disabled="!diag" @click="copySummary">{{ copied ? t('diagnostics.copied') : t('diagnostics.copy') }}</button>
            <button class="diag-btn" :disabled="exporting" @click="exportLog">{{ exporting ? t('diagnostics.exporting') : t('diagnostics.exportLog') }}</button>
            <button class="diag-close" :title="t('keyEditor.close')" @click="$emit('close')">✕</button>
          </div>
        </div>

        <div class="diag-body">
          <div v-if="error" class="diag-error">{{ error }}</div>

          <section class="diag-section">
            <div class="diag-section-title">{{ t('diagnostics.overview') }}</div>
            <dl class="diag-list">
              <div class="diag-row"><dt>{{ t('diagnostics.version') }}</dt><dd>{{ diag?.app_version || '-' }}</dd></div>
              <div class="diag-row"><dt>{{ t('diagnostics.platform') }}</dt><dd>{{ diag?.platform || '-' }}</dd></div>
              <div class="diag-row"><dt>{{ t('diagnostics.configPath') }}</dt><dd class="mono">{{ diag?.config_path || '-' }}</dd></div>
              <div class="diag-row"><dt>{{ t('diagnostics.logPath') }}</dt><dd class="mono">{{ diag?.debug_log_path || '-' }}<span class="muted"> · {{ formatSize(diag?.debug_log_size) }}</span></dd></div>
              <div class="diag-row">
                <dt>{{ t('diagnostics.localApi') }}</dt>
                <dd>
                  <span :class="['diag-pill', diag?.local_api_running ? 'ok' : 'off']">
                    {{ diag?.local_api_running ? t('diagnostics.running') : t('diagnostics.stopped') }}
                    <template v-if="diag?.local_api_running"> · 127.0.0.1:{{ diag.local_api_port }}</template>
                  </span>
                </dd>
              </div>
            </dl>
          </section>

          <section class="diag-section">
            <div class="diag-section-title">{{ t('diagnostics.connections') }}</div>
            <div v-if="!diag || !diag.connections.length" class="diag-empty">{{ t('diagnostics.noConnections') }}</div>
            <table v-else class="diag-table">
              <thead>
                <tr>
                  <th>{{ t('diagnostics.connName') }}</th>
                  <th>{{ t('diagnostics.connAddress') }}</th>
                  <th>{{ t('diagnostics.connStatus') }}</th>
                  <th class="num">{{ t('diagnostics.connLatency') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="c in diag.connections" :key="c.id">
                  <td>{{ c.name }}</td>
                  <td class="mono">{{ c.host }}:{{ c.port }}</td>
                  <td>
                    <span :class="['diag-pill', c.connected ? 'ok' : 'off']">
                      {{ c.connected ? t('diagnostics.connected') : t('diagnostics.disconnected') }}
                    </span>
                    <span v-if="c.error" class="diag-err-text">{{ c.error }}</span>
                  </td>
                  <td class="num">{{ c.latency_ms >= 0 ? c.latency_ms + ' ms' : '-' }}</td>
                </tr>
              </tbody>
            </table>
          </section>

          <section class="diag-section">
            <div class="diag-section-title">{{ t('diagnostics.logs') }}</div>
            <pre class="diag-log">{{ logText }}</pre>
          </section>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { getDiagnostics, exportDebugLog } from '../../api/wails.js'

const emit = defineEmits(['close'])

const settingsStore = useSettingsStore()
const { t } = useI18n()
const theme = computed(() => settingsStore.themeMode || 'light')

const diag = ref(null)
const loading = ref(false)
const exporting = ref(false)
const copied = ref(false)
const error = ref('')

const logText = computed(() => {
  const lines = diag.value?.log_tail || []
  return lines.length ? lines.join('\n') : t('diagnostics.emptyLog')
})

function formatSize(bytes) {
  const n = Number(bytes) || 0
  if (n <= 0) return '0 B'
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / 1024 / 1024).toFixed(1) + ' MB'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    diag.value = await getDiagnostics()
  } catch (e) {
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

async function exportLog() {
  exporting.value = true
  try {
    const path = await exportDebugLog()
    if (path) error.value = ''
  } catch (e) {
    error.value = e?.message || String(e)
  } finally {
    exporting.value = false
  }
}

async function copySummary() {
  if (!diag.value) return
  const d = diag.value
  const lines = [
    'LiteRedis ' + d.app_version + ' (' + d.platform + ')',
    'config: ' + d.config_path,
    'log: ' + d.debug_log_path + ' (' + formatSize(d.debug_log_size) + ')',
    'local api: ' + (d.local_api_running ? 'running :' + d.local_api_port : 'stopped'),
    'connections:',
    ...d.connections.map(c => '  - ' + c.name + ' ' + c.host + ':' + c.port + ' ' + (c.connected ? 'connected' : 'disconnected') + (c.latency_ms >= 0 ? ' ' + c.latency_ms + 'ms' : ''))
  ]
  const ok = await copyToClipboard(lines.join('\n'))
  if (ok) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  }
}

let disposeEscape = null
onMounted(() => {
  disposeEscape = registerEscapeHandler(() => emit('close'))
  load()
})
onBeforeUnmount(() => { if (disposeEscape) disposeEscape() })
</script>

<style scoped>
.diag-overlay {
  position: fixed;
  inset: 0;
  z-index: 10075;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.45);
  animation: diagFade 160ms ease;
}
@keyframes diagFade {
  from { background: rgba(15, 23, 42, 0); }
  to { background: rgba(15, 23, 42, 0.45); }
}
.diag-card {
  width: min(760px, 94vw);
  max-height: 82vh;
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-lg);
  background: #fff;
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: var(--shadow-lg);
  animation: diagPop 200ms var(--ease-out);
}
@keyframes diagPop {
  from { opacity: 0; transform: translateY(10px) scale(0.985); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.diag-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(239, 246, 255, 0.9));
  border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  flex-shrink: 0;
}
.diag-title {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
}
.diag-header-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.diag-btn {
  height: 26px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-sm);
  background: #fff;
  color: #475569;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out), color var(--motion-fast) var(--ease-out);
}
.diag-btn:hover:not(:disabled) {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.diag-btn:disabled {
  opacity: 0.55;
  cursor: default;
}
.diag-close {
  width: 26px;
  height: 26px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: #64748b;
  cursor: pointer;
}
.diag-close:hover {
  background: #fff;
  border-color: rgba(203, 213, 225, 0.9);
  color: #1e293b;
}
.diag-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px 16px 16px;
}
.diag-error {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-md);
  font-size: 11.5px;
  color: #b91c1c;
  background: rgba(254, 242, 242, 0.96);
  border: 1px solid rgba(252, 165, 165, 0.9);
}
.diag-section + .diag-section {
  margin-top: 16px;
}
.diag-section-title {
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: #94a3b8;
  text-transform: uppercase;
  margin-bottom: 8px;
}
.diag-list {
  margin: 0;
}
.diag-row {
  display: flex;
  gap: 12px;
  padding: 4px 0;
  font-size: 11.5px;
}
.diag-row dt {
  flex: 0 0 96px;
  color: #64748b;
}
.diag-row dd {
  margin: 0;
  min-width: 0;
  color: #1e293b;
  word-break: break-all;
}
.mono {
  font-family: ui-monospace, Menlo, monospace;
  font-size: 11px;
}
.muted {
  color: #94a3b8;
}
.diag-pill {
  display: inline-block;
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  font-size: 10.5px;
  font-weight: 600;
  line-height: 1.6;
}
.diag-pill.ok {
  color: #15803d;
  background: rgba(220, 252, 231, 0.95);
  border: 1px solid rgba(134, 239, 172, 0.9);
}
.diag-pill.off {
  color: #b91c1c;
  background: rgba(254, 242, 242, 0.95);
  border: 1px solid rgba(252, 165, 165, 0.9);
}
.diag-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11.5px;
}
.diag-table th {
  text-align: left;
  font-weight: 600;
  color: #94a3b8;
  padding: 4px 8px 6px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
}
.diag-table td {
  padding: 6px 8px;
  border-bottom: 1px solid rgba(241, 245, 249, 0.96);
  color: #1e293b;
}
.diag-table .num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.diag-err-text {
  margin-left: 6px;
  font-size: 10.5px;
  color: #b91c1c;
}
.diag-empty {
  font-size: 11.5px;
  color: #94a3b8;
  padding: 6px 0;
}
.diag-log {
  margin: 0;
  padding: 10px 12px;
  max-height: 220px;
  overflow: auto;
  border-radius: var(--radius-md);
  background: #0f172a;
  color: #cbd5e1;
  font: 11px/1.65 ui-monospace, Menlo, monospace;
  white-space: pre-wrap;
  word-break: break-all;
}
/* 深色 */
.diag-overlay.theme-dark .diag-card {
  background: rgba(15, 23, 42, 0.99);
  border-color: rgba(51, 65, 85, 0.92);
}
.diag-overlay.theme-dark .diag-header {
  border-bottom-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.99));
}
.diag-overlay.theme-dark .diag-title { color: #e2e8f0; }
.diag-overlay.theme-dark .diag-btn {
  background: rgba(15, 23, 42, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #cbd5e1;
}
.diag-overlay.theme-dark .diag-btn:hover:not(:disabled) {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
.diag-overlay.theme-dark .diag-close { color: #94a3b8; }
.diag-overlay.theme-dark .diag-close:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #e2e8f0;
}
.diag-overlay.theme-dark .diag-row dt { color: #94a3b8; }
.diag-overlay.theme-dark .diag-row dd { color: #cbd5e1; }
.diag-overlay.theme-dark .diag-table th {
  color: #94a3b8;
  border-bottom-color: rgba(51, 65, 85, 0.9);
}
.diag-overlay.theme-dark .diag-table td {
  color: #cbd5e1;
  border-bottom-color: rgba(51, 65, 85, 0.8);
}
.diag-overlay.theme-dark .diag-error {
  color: #fda4af;
  background: rgba(76, 5, 25, 0.9);
  border-color: rgba(244, 63, 94, 0.4);
}
</style>
