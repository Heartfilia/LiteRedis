<template>
  <Teleport to="body">
    <div class="overview-backdrop">
      <div class="overview-card">
        <div class="overview-header">
          <div class="overview-title-wrap">
            <div class="overview-title">{{ connectionName }}</div>
            <div class="overview-subtitle">{{ overview ? `${overview.host}:${overview.port}` : '' }}</div>
          </div>
          <button class="overview-close" @click="$emit('close')">✕</button>
        </div>

        <div class="overview-body">
          <div v-if="loading && !overview" class="overview-state">{{ t('keyEditor.loading') }}</div>
          <div v-else-if="error && !overview" class="overview-state error">{{ error }}</div>
          <template v-else>
            <div class="metrics-grid">
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.redisVersion') }}</span>
                <span class="metric-value">{{ overview?.redis_version || '-' }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.redisRole') }}</span>
                <span class="metric-value">{{ overview?.role || '-' }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.connectedClients') }}</span>
                <span class="metric-value">{{ overview?.connected_clients ?? '-' }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.instantOps') }}</span>
                <span class="metric-value">{{ overview?.instant_ops_per_sec ?? '-' }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.totalKeys') }}</span>
                <span class="metric-value">{{ displayTotalKeys }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.usedMemory') }}</span>
                <span class="metric-value">{{ overview?.used_memory || '-' }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.currentDb') }}</span>
                <span class="metric-value">DB {{ effectiveDB }}</span>
              </div>
              <div class="metric-card">
                <span class="metric-label">{{ t('sidebar.uptime') }}</span>
                <span class="metric-value">{{ overview?.uptime_human || '-' }}</span>
              </div>
            </div>

            <div class="console-panel">
              <div class="console-head">
                <div class="console-title">{{ t('sidebar.redisConsole') }}</div>
                <div class="console-tip">{{ t('sidebar.redisConsoleHint') }}</div>
              </div>

              <div ref="consoleOutputRef" class="console-output" @mousedown="handleConsoleMouseDown">
                <template v-if="consoleHistory.length">
                  <div
                    v-for="(entry, index) in consoleHistory"
                    :key="`${index}:${entry.command}`"
                    class="console-entry"
                  >
                    <div class="console-line console-command-line">
                      <span class="console-prompt">{{ entry.prompt }}</span>
                      <span class="console-command-text">{{ entry.command }}</span>
                    </div>
                    <div v-if="entry.meta" class="console-meta">{{ entry.meta }}</div>
                    <div v-if="entry.output" class="console-result" :class="{ error: !entry.success }">{{ entry.output }}</div>
                  </div>
                </template>
                <div v-else class="console-placeholder" />

                <div class="console-line console-live-line">
                  <span class="console-prompt">{{ consolePrompt }}</span>
                  <div class="console-input-wrap">
                    <div v-if="commandHint" class="console-input-overlay" aria-hidden="true">
                      <span class="console-input-overlay-typed">{{ command }}</span>
                      <span class="console-input-overlay-hint">{{ commandHint }}</span>
                    </div>
                    <input
                      ref="consoleInputRef"
                      v-model="command"
                      class="console-input"
                      type="text"
                      spellcheck="false"
                      autocomplete="off"
                      autocapitalize="off"
                      :disabled="running"
                      :placeholder="t('sidebar.redisConsolePlaceholder')"
                      @keydown.enter.prevent="runCommand"
                      @keydown.up.prevent="showPreviousCommand"
                      @keydown.down.prevent="showNextCommand"
                      @keydown.esc.prevent="resetHistoryNavigation"
                    />
                  </div>
                  <span v-if="running" class="console-status">{{ t('settings.checkingUpdate') }}</span>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { Teleport, computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { executeRedisCommand, getConnectionOverview } from '../../api/wails.js'
import { useI18n } from '../../i18n/index.js'
import { useWorkspaceStore } from '../../stores/workspace.js'

const props = defineProps({
  connId: { type: String, required: true },
  connectionName: { type: String, default: '' },
})

defineEmits(['close'])

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const overview = ref(null)
const loading = ref(false)
const error = ref('')
const command = ref('')
const consoleHistory = ref([])
const commandHistory = ref([])
const historyIndex = ref(-1)
const draftCommand = ref('')
const running = ref(false)
const consoleOutputRef = ref(null)
const consoleInputRef = ref(null)
let refreshTimer = null

const REDIS_COMMAND_HINTS = {
  append: ['key', 'value'],
  decr: ['key'],
  decrby: ['key', 'decrement'],
  del: ['key', '[key ...]'],
  exists: ['key', '[key ...]'],
  expire: ['key', 'seconds'],
  get: ['key'],
  hdel: ['key', 'field', '[field ...]'],
  hexists: ['key', 'field'],
  hget: ['key', 'field'],
  hgetall: ['key'],
  hincrby: ['key', 'field', 'increment'],
  hkeys: ['key'],
  hlen: ['key'],
  hmget: ['key', 'field', '[field ...]'],
  hmset: ['key', 'field', 'value', '[field value ...]'],
  hset: ['key', 'field', 'value', '[field value ...]'],
  hvals: ['key'],
  incr: ['key'],
  incrby: ['key', 'increment'],
  info: ['[section]'],
  keys: ['pattern'],
  llen: ['key'],
  lpop: ['key', '[count]'],
  lpush: ['key', 'element', '[element ...]'],
  lrange: ['key', 'start', 'stop'],
  lrem: ['key', 'count', 'element'],
  lset: ['key', 'index', 'element'],
  ltrim: ['key', 'start', 'stop'],
  mget: ['key', '[key ...]'],
  mset: ['key', 'value', '[key value ...]'],
  persist: ['key'],
  ping: ['[message]'],
  rename: ['key', 'newkey'],
  rpop: ['key', '[count]'],
  rpush: ['key', 'element', '[element ...]'],
  sadd: ['key', 'member', '[member ...]'],
  scard: ['key'],
  set: ['key', 'value', '[EX seconds|PX milliseconds|NX|XX ...]'],
  select: ['index'],
  setex: ['key', 'seconds', 'value'],
  sembers: ['key'],
  smembers: ['key'],
  sismember: ['key', 'member'],
  smismember: ['key', 'member', '[member ...]'],
  spop: ['key', '[count]'],
  srem: ['key', 'member', '[member ...]'],
  strlen: ['key'],
  ttl: ['key'],
  type: ['key'],
  unlink: ['key', '[key ...]'],
  zadd: ['key', 'score', 'member', '[score member ...]'],
  zcard: ['key'],
  zcount: ['key', 'min', 'max'],
  zrange: ['key', 'start', 'stop', '[WITHSCORES]'],
  zrangebyscore: ['key', 'min', 'max', '[WITHSCORES]', '[LIMIT offset count]'],
  zrank: ['key', 'member'],
  zrem: ['key', 'member', '[member ...]'],
  zrevrange: ['key', 'start', 'stop', '[WITHSCORES]'],
  zscore: ['key', 'member'],
}

const isActiveConnection = computed(() => workspaceStore.activeConnID === props.connId)
const effectiveDB = computed(() => {
  if (isActiveConnection.value) {
    return workspaceStore.currentDB ?? 0
  }
  return overview.value?.current_db ?? 0
})
const displayTotalKeys = computed(() => {
  if (isActiveConnection.value) {
    return workspaceStore.totalKeys ?? 0
  }
  return overview.value?.total_keys ?? '-'
})
const consolePrompt = computed(() => {
  const host = overview.value?.host || 'redis'
  const port = overview.value?.port || 6379
  return `${host}:${port}[${effectiveDB.value}]>`
})
const commandHint = computed(() => {
  const raw = command.value || ''
  const trimmed = raw.trim()
  if (!trimmed) return ''

  const endsWithSpace = /\s$/.test(raw)
  const parts = trimmed.split(/\s+/)
  const commandName = (parts[0] || '').toLowerCase()
  const signature = REDIS_COMMAND_HINTS[commandName]
  if (!signature) return ''

  if (parts.length === 1 && !endsWithSpace) {
    return ''
  }

  const typedArgCount = Math.max(parts.length - 1, 0)
  const remaining = signature.slice(typedArgCount)
  return remaining.join(' ')
})

async function refreshOverview() {
  loading.value = true
  try {
    overview.value = await getConnectionOverview(props.connId)
    error.value = ''
  } catch (e) {
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

function focusConsoleInput() {
  consoleInputRef.value?.focus()
}

function handleConsoleMouseDown(event) {
  const target = event.target
  if (!(target instanceof HTMLElement)) return
  if (target.closest('.console-live-line')) {
    return
  }

  // Let text selection inside output entries work normally.
  if (target.closest('.console-entry, .console-placeholder, .console-result, .console-command-line, .console-meta')) {
    return
  }

  focusConsoleInput()
}

async function scrollConsoleToBottom() {
  await nextTick()
  if (consoleOutputRef.value) {
    consoleOutputRef.value.scrollTop = consoleOutputRef.value.scrollHeight
  }
}

function rememberCommand(text) {
  const normalized = text.trim()
  if (!normalized) return
  commandHistory.value = [normalized, ...commandHistory.value.filter(item => item !== normalized)].slice(0, 100)
  historyIndex.value = -1
  draftCommand.value = ''
}

function resetHistoryNavigation() {
  historyIndex.value = -1
  command.value = draftCommand.value
}

function showPreviousCommand() {
  if (!commandHistory.value.length) return
  if (historyIndex.value === -1) {
    draftCommand.value = command.value
  }
  const nextIndex = Math.min(historyIndex.value + 1, commandHistory.value.length - 1)
  historyIndex.value = nextIndex
  command.value = commandHistory.value[nextIndex] || ''
}

function showNextCommand() {
  if (!commandHistory.value.length) return
  const nextIndex = historyIndex.value - 1
  if (nextIndex < 0) {
    resetHistoryNavigation()
    return
  }
  historyIndex.value = nextIndex
  command.value = commandHistory.value[nextIndex] || ''
}

function formatConsoleMeta(result) {
  if (!result) return ''
  const duration = Number.isFinite(result.elapsed_ms) ? `${result.elapsed_ms} ms` : ''
  if (!duration) return ''
  return `(${duration})`
}

async function runCommand() {
  const text = command.value.trim()
  if (!text || running.value) return

  const promptBeforeRun = consolePrompt.value
  rememberCommand(text)
  running.value = true

  try {
    const result = await executeRedisCommand(props.connId, text)
    consoleHistory.value.push({
      prompt: promptBeforeRun,
      command: text,
      success: !!result.success,
      meta: formatConsoleMeta(result),
      output: result.success ? (result.output || '(nil)') : `ERR ${result.error || t('keyEditor.failed')}`,
    })
    command.value = ''
    await refreshOverview()
    if (isActiveConnection.value) {
      workspaceStore.currentDB = overview.value?.current_db ?? workspaceStore.currentDB
      await workspaceStore.fetchTotalKeys()
    }
    await scrollConsoleToBottom()
    focusConsoleInput()
  } catch (e) {
    consoleHistory.value.push({
      prompt: promptBeforeRun,
      command: text,
      success: false,
      meta: '',
      output: `ERR ${e?.message || String(e)}`,
    })
    await scrollConsoleToBottom()
    focusConsoleInput()
  } finally {
    running.value = false
  }
}

watch(
  () => workspaceStore.currentDB,
  async (db, prevDb) => {
    if (!isActiveConnection.value || db === prevDb) return
    await refreshOverview()
    await scrollConsoleToBottom()
  }
)

watch(
  () => props.connId,
  async () => {
    consoleHistory.value = []
    commandHistory.value = []
    historyIndex.value = -1
    draftCommand.value = ''
    command.value = ''
    await refreshOverview()
    await scrollConsoleToBottom()
    focusConsoleInput()
  }
)

onMounted(async () => {
  await refreshOverview()
  await scrollConsoleToBottom()
  focusConsoleInput()
  refreshTimer = setInterval(refreshOverview, 5000)
})

onBeforeUnmount(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<style scoped>
.overview-backdrop {
  position: fixed;
  inset: 0;
  z-index: 12000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.36);
  backdrop-filter: blur(8px);
}

.overview-card {
  width: min(1080px, 96vw);
  max-height: 92vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  border-radius: 20px;
  border: 1px solid rgba(214, 224, 236, 0.94);
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 30px 80px rgba(15, 23, 42, 0.2);
}

.overview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px 14px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.96);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(255, 255, 255, 0.94));
}

.overview-title {
  font-size: 15px;
  font-weight: 700;
  color: #1e293b;
}

.overview-subtitle {
  margin-top: 3px;
  font-size: 12px;
  color: #64748b;
}

.overview-close {
  width: 30px;
  min-width: 30px;
  height: 30px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.96);
  color: #64748b;
  cursor: pointer;
}

.overview-body {
  overflow: auto;
  padding: 16px 18px 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.overview-state {
  padding: 16px;
  border-radius: 14px;
  background: rgba(248, 250, 252, 0.92);
  color: #64748b;
}

.overview-state.error {
  background: rgba(255, 241, 242, 0.92);
  color: #b91c1c;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px;
  border-radius: 16px;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.96), rgba(255, 255, 255, 0.94));
}

.metric-label {
  font-size: 11px;
  color: #64748b;
}

.metric-value {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.console-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border-radius: 18px;
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.98), rgba(255, 255, 255, 0.96));
}

.console-head {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.console-title {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
}

.console-tip {
  font-size: 11px;
  color: #64748b;
}

.console-output {
  min-height: 360px;
  max-height: 520px;
  overflow: auto;
  padding: 14px 14px 12px;
  border-radius: 14px;
  border: 1px solid rgba(15, 23, 42, 0.04);
  background: #0b1220;
  color: #dbeafe;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.6;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  cursor: text;
  user-select: text;
}

.console-entry + .console-entry {
  margin-top: 10px;
}

.console-meta {
  margin-top: 2px;
  padding-left: calc(1ch + 8px);
  color: #64748b;
  font-size: 11px;
}

.console-line {
  display: flex;
  align-items: center;
  gap: 8px;
}

.console-command-line {
  align-items: flex-start;
}

.console-live-line {
  margin-top: 8px;
}

.console-prompt {
  flex: 0 0 auto;
  color: #86efac;
  white-space: nowrap;
}

.console-command-text {
  color: #e2e8f0;
  min-width: 0;
  user-select: text;
}

.console-input {
  flex: 1;
  min-width: 0;
  height: 24px;
  padding: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: #f8fafc;
  font: inherit;
  caret-color: #f8fafc;
}

.console-input-wrap {
  flex: 1;
  min-width: 0;
  position: relative;
  display: flex;
  align-items: center;
}

.console-input-overlay {
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  pointer-events: none;
  white-space: pre;
  font: inherit;
  overflow: hidden;
}

.console-input-overlay-typed {
  color: transparent;
}

.console-input-overlay-hint {
  color: #6b7280;
}

.console-input:disabled {
  opacity: 0.92;
}

.console-input::placeholder {
  color: #64748b;
}

.console-result {
  padding-left: calc(1ch + 8px);
  color: #dbeafe;
  white-space: pre-wrap;
  word-break: break-word;
  user-select: text;
}

.console-result.error {
  color: #fda4af;
}

.console-placeholder {
  color: #94a3b8;
  user-select: text;
}

.console-status {
  flex: 0 0 auto;
  color: #fbbf24;
  font-size: 11px;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .metrics-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
