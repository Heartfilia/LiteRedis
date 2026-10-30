<template>
  <Teleport to="body">
    <div class="ps-overlay" :class="'theme-' + theme">
      <div class="ps-card">
        <div class="ps-header">
          <div class="ps-title-wrap">
            <span class="ps-title">{{ t('pubsub.title') }}</span>
            <span class="ps-conn">{{ connectionName }}</span>
            <span class="ps-pill" :class="running ? 'on' : 'off'">
              {{ running ? t('pubsub.running') : t('pubsub.stopped') }}
              <template v-if="running"> · {{ formatNumber(count) }}</template>
            </span>
          </div>
          <div class="ps-header-actions">
            <button class="ps-btn" :disabled="!messages.length" @click="clearMessages">{{ t('pubsub.clear') }}</button>
            <button class="ps-close" :title="t('keyEditor.close')" @click="$emit('close')">✕</button>
          </div>
        </div>

        <div class="ps-form">
          <label class="ps-field">
            <span>{{ t('pubsub.channels') }}</span>
            <input v-model="channelsInput" :disabled="running" :placeholder="t('pubsub.channelsHint')" spellcheck="false" />
          </label>
          <label class="ps-field">
            <span>{{ t('pubsub.patterns') }}</span>
            <input v-model="patternsInput" :disabled="running" :placeholder="t('pubsub.patternsHint')" spellcheck="false" />
          </label>
          <button class="ps-btn primary" :disabled="busy" @click="toggle">
            {{ running ? t('pubsub.stop') : t('pubsub.start') }}
          </button>
        </div>

        <div class="ps-publish">
          <span class="ps-publish-label">{{ t('pubsub.publish') }}</span>
          <input
            v-model="publishChannel"
            class="ps-publish-channel"
            :placeholder="t('pubsub.publishChannelHint')"
            spellcheck="false"
          />
          <input
            v-model="publishPayload"
            class="ps-publish-payload"
            :placeholder="t('pubsub.publishPayloadHint')"
            spellcheck="false"
            @keydown.enter="doPublish"
          />
          <button class="ps-btn" :disabled="publishing || !publishChannel.trim()" @click="doPublish">
            {{ publishing ? t('pubsub.publishing') : t('pubsub.publishAction') }}
          </button>
          <span class="ps-publish-status" :class="{ err: publishFailed }">{{ publishStatus }}</span>
        </div>

        <div v-if="error" class="ps-error">{{ error }}</div>

        <div class="ps-list">
          <div v-if="!messages.length" class="ps-empty">{{ t('pubsub.empty') }}</div>
          <div v-for="msg in messages" :key="msg.id" class="ps-row">
            <span class="ps-time">{{ msg.time }}</span>
            <span class="ps-channel" :title="msg.channel">{{ msg.channel }}</span>
            <span v-if="msg.pattern" class="ps-pattern" :title="msg.pattern">{{ msg.pattern }}</span>
            <span class="ps-size">{{ msg.size }} B</span>
            <span class="ps-payload" :title="msg.payload" @click="copyPayload(msg)">{{ msg.payload }}</span>
          </div>
        </div>

        <div class="ps-footer">
          <span class="ps-hint">{{ t('pubsub.hint') }}</span>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { EventsOn, EventsOff } from '../../../wailsjs/runtime/runtime.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { startPubSub, stopPubSub, getPubSubStatus, publishMessage } from '../../api/wails.js'

const props = defineProps({
  connId: { type: String, required: true },
  connectionName: { type: String, default: '' },
})
const emit = defineEmits(['close'])

const settingsStore = useSettingsStore()
const { t } = useI18n()
const theme = computed(() => settingsStore.themeMode || 'light')

const channelsInput = ref('')
const patternsInput = ref('')
const running = ref(false)
const busy = ref(false)
const count = ref(0)
const error = ref('')
const messages = ref([])
const publishChannel = ref('')
const publishPayload = ref('')
const publishing = ref(false)
const publishStatus = ref('')
const publishFailed = ref(false)
let publishTimer = null
let seq = 0

const MAX_MESSAGES = 300

function formatNumber(n) {
  return n > 999 ? (n / 1000).toFixed(1) + 'k' : String(n)
}

function formatTime(ts) {
  const d = new Date(ts)
  const pad = (v, m = 2) => String(v).padStart(m, '0')
  return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds()) + '.' + pad(d.getMilliseconds(), 3)
}

function splitList(value) {
  return String(value || '')
    .split(/[,，\s]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function onBatch(batch) {
  if (!Array.isArray(batch) || !batch.length) return
  const incoming = batch
    .filter((item) => item && item.conn_id === props.connId)
    .map((item) => ({
      id: ++seq,
      time: formatTime(item.at || Date.now()),
      channel: item.channel || '',
      pattern: item.pattern || '',
      size: item.size || 0,
      payload: item.payload || '',
    }))
  if (!incoming.length) return
  count.value += incoming.length
  messages.value = incoming.concat(messages.value).slice(0, MAX_MESSAGES)
}

async function toggle() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    if (running.value) {
      await stopPubSub(props.connId)
      running.value = false
    } else {
      const channels = splitList(channelsInput.value)
      const patterns = splitList(patternsInput.value)
      if (!channels.length && !patterns.length) {
        error.value = t('pubsub.needTarget')
        return
      }
      const result = await startPubSub(props.connId, channels, patterns)
      if (!result?.success) {
        error.value = result?.message || t('pubsub.startFailed')
        return
      }
      running.value = true
      // 发布频道默认跟第一个订阅频道对齐，方便自己发一条测试
      if (!publishChannel.value.trim() && channels.length) {
        publishChannel.value = channels[0]
      }
    }
  } catch (e) {
    error.value = e?.message || String(e)
  } finally {
    busy.value = false
  }
}

async function doPublish() {
  const channel = publishChannel.value.trim()
  if (!channel || publishing.value) return
  publishing.value = true
  publishFailed.value = false
  try {
    const receivers = await publishMessage(props.connId, channel, publishPayload.value)
    publishStatus.value = t('pubsub.published').replace('{n}', receivers ?? 0)
    publishPayload.value = ''
  } catch (e) {
    publishFailed.value = true
    publishStatus.value = e?.message || String(e)
  } finally {
    publishing.value = false
    if (publishTimer) clearTimeout(publishTimer)
    publishTimer = setTimeout(() => { publishStatus.value = '' }, 3000)
  }
}

function clearMessages() {
  messages.value = []
  count.value = 0
}

async function copyPayload(msg) {
  await copyToClipboard(msg.payload)
}

let disposeEscape = null
onMounted(async () => {
  disposeEscape = registerEscapeHandler(() => emit('close'))
  EventsOn('pubsub:batch', onBatch)
  try {
    const status = await getPubSubStatus(props.connId)
    if (status?.running) {
      running.value = true
      channelsInput.value = (status.channels || []).join(', ')
      patternsInput.value = (status.patterns || []).join(', ')
      count.value = status.count || 0
    }
  } catch (e) {
    // 状态同步失败不影响手动开启
  }
})

onBeforeUnmount(async () => {
  EventsOff('pubsub:batch')
  if (disposeEscape) disposeEscape()
  if (running.value) {
    try { await stopPubSub(props.connId) } catch (e) {}
  }
})
</script>

<style scoped>
.ps-overlay {
  position: fixed;
  inset: 0;
  z-index: 10078;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.45);
  animation: psFade 160ms ease;
}
@keyframes psFade {
  from { background: rgba(15, 23, 42, 0); }
  to { background: rgba(15, 23, 42, 0.45); }
}
.ps-card {
  width: min(880px, 94vw);
  height: min(72vh, 640px);
  display: flex;
  flex-direction: column;
  border-radius: var(--radius-lg);
  background: #fff;
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: var(--shadow-lg);
  animation: psPop 200ms var(--ease-out);
}
@keyframes psPop {
  from { opacity: 0; transform: translateY(10px) scale(0.985); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}
.ps-header {
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
.ps-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.ps-title {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
}
.ps-conn {
  font-size: 11.5px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}
.ps-pill {
  font-size: 10.5px;
  font-weight: 600;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  line-height: 1.7;
}
.ps-pill.on {
  color: #15803d;
  background: rgba(220, 252, 231, 0.95);
  border: 1px solid rgba(134, 239, 172, 0.9);
}
.ps-pill.off {
  color: #64748b;
  background: rgba(241, 245, 249, 0.95);
  border: 1px solid rgba(203, 213, 225, 0.9);
}
.ps-header-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.ps-btn {
  height: 26px;
  padding: 0 12px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-sm);
  background: #fff;
  color: #475569;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--motion-fast) var(--ease-out), border-color var(--motion-fast) var(--ease-out), color var(--motion-fast) var(--ease-out);
}
.ps-btn:hover:not(:disabled) {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.ps-btn:disabled {
  opacity: 0.55;
  cursor: default;
}
.ps-btn.primary {
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  border-color: rgba(191, 219, 254, 0.96);
}
.ps-btn.primary:hover:not(:disabled) {
  border-color: #60a5fa;
  color: #1d4ed8;
}
.ps-close {
  width: 26px;
  height: 26px;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: #64748b;
  cursor: pointer;
}
.ps-close:hover {
  background: #fff;
  border-color: rgba(203, 213, 225, 0.9);
  color: #1e293b;
}
.ps-form {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  flex-shrink: 0;
  flex-wrap: wrap;
}
.ps-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 200px;
}
.ps-field span {
  font-size: 11px;
  color: #64748b;
}
.ps-field input {
  height: 30px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-sm);
  font: 12px ui-monospace, Menlo, monospace;
  color: #1e293b;
  background: #fff;
  outline: none;
}
.ps-field input:focus {
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.32);
}
.ps-field input:disabled {
  background: #f8fafc;
  color: #94a3b8;
}
.ps-publish {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  flex-shrink: 0;
  flex-wrap: wrap;
}
.ps-publish-label {
  flex: 0 0 auto;
  font-size: 11px;
  color: #64748b;
}
.ps-publish-channel,
.ps-publish-payload {
  height: 28px;
  padding: 0 10px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-sm);
  font: 12px ui-monospace, Menlo, monospace;
  color: #1e293b;
  background: #fff;
  outline: none;
}
.ps-publish-channel {
  width: 180px;
  flex: 0 0 auto;
}
.ps-publish-payload {
  flex: 1;
  min-width: 160px;
}
.ps-publish-channel:focus,
.ps-publish-payload:focus {
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.32);
}
.ps-publish-status {
  flex: 0 0 auto;
  font-size: 11px;
  color: #15803d;
  min-width: 56px;
}
.ps-publish-status.err {
  color: #b91c1c;
}
.ps-error {
  margin: 10px 16px 0;
  padding: 7px 10px;
  border-radius: var(--radius-md);
  font-size: 11.5px;
  color: #b91c1c;
  background: rgba(254, 242, 242, 0.96);
  border: 1px solid rgba(252, 165, 165, 0.9);
}
.ps-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px;
}
.ps-empty {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #94a3b8;
  font-size: 12px;
}
.ps-row {
  display: grid;
  grid-template-columns: 88px minmax(80px, 160px) auto 56px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
  padding: 5px 6px;
  border-radius: var(--radius-sm);
  font-size: 11.5px;
}
.ps-row:hover {
  background: rgba(248, 250, 252, 0.96);
}
.ps-time {
  color: #94a3b8;
  font-family: ui-monospace, Menlo, monospace;
  font-size: 10.5px;
}
.ps-channel {
  color: #1d4ed8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ps-pattern {
  color: #7c3aed;
  font-size: 10.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ps-size {
  color: #94a3b8;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.ps-payload {
  color: #334155;
  font-family: ui-monospace, Menlo, monospace;
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}
.ps-payload:hover {
  color: #0f172a;
  text-decoration: underline;
}
.ps-footer {
  padding: 8px 16px;
  border-top: 1px solid rgba(226, 232, 240, 0.9);
  flex-shrink: 0;
}
.ps-hint {
  font-size: 10.5px;
  color: #94a3b8;
}
/* 深色 */
.ps-overlay.theme-dark .ps-card {
  background: rgba(15, 23, 42, 0.99);
  border-color: rgba(51, 65, 85, 0.92);
}
.ps-overlay.theme-dark .ps-header {
  border-bottom-color: rgba(51, 65, 85, 0.94);
  background: linear-gradient(180deg, rgba(17, 24, 39, 0.99), rgba(11, 18, 32, 0.99));
}
.ps-overlay.theme-dark .ps-title { color: #e2e8f0; }
.ps-overlay.theme-dark .ps-conn { color: #94a3b8; }
.ps-overlay.theme-dark .ps-form { border-bottom-color: rgba(51, 65, 85, 0.94); }
.ps-overlay.theme-dark .ps-field span { color: #94a3b8; }
.ps-overlay.theme-dark .ps-field input {
  background: rgba(11, 18, 32, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #e2e8f0;
}
.ps-overlay.theme-dark .ps-btn {
  background: rgba(15, 23, 42, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #cbd5e1;
}
.ps-overlay.theme-dark .ps-close { color: #94a3b8; }
.ps-overlay.theme-dark .ps-close:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #e2e8f0;
}
.ps-overlay.theme-dark .ps-row:hover { background: rgba(30, 41, 59, 0.6); }
.ps-overlay.theme-dark .ps-channel { color: #93c5fd; }
.ps-overlay.theme-dark .ps-pattern { color: #c4b5fd; }
.ps-overlay.theme-dark .ps-payload { color: #cbd5e1; }
.ps-overlay.theme-dark .ps-footer { border-top-color: rgba(51, 65, 85, 0.94); }
.ps-overlay.theme-dark .ps-publish {
  border-bottom-color: rgba(51, 65, 85, 0.94);
}
.ps-overlay.theme-dark .ps-publish-label { color: #94a3b8; }
.ps-overlay.theme-dark .ps-publish-channel,
.ps-overlay.theme-dark .ps-publish-payload {
  background: rgba(11, 18, 32, 0.96);
  border-color: rgba(71, 85, 105, 0.96);
  color: #e2e8f0;
}
.ps-overlay.theme-dark .ps-publish-status { color: #6ee7b7; }
.ps-overlay.theme-dark .ps-publish-status.err { color: #fda4af; }
.ps-overlay.theme-dark .ps-error {
  color: #fda4af;
  background: rgba(76, 5, 25, 0.9);
  border-color: rgba(244, 63, 94, 0.4);
}
</style>
