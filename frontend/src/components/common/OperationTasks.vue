<template>
  <aside v-if="tasks.length" class="operation-tasks">
    <button class="task-toggle" @click="expanded = !expanded">{{ t('reliability.tasks') }} · {{ tasks.filter(task => task.state === 'running').length }} {{ t('reliability.running') }} {{ expanded ? '▾' : '▴' }}</button>
    <div v-if="expanded" class="task-list">
      <p>{{ t('reliability.taskLimits') }}</p>
      <article v-for="task in tasks" :key="task.id">
        <strong>{{ t('reliability.kind_' + task.kind) }} · {{ t('reliability.state_' + task.state) }}</strong>
        <div>{{ connectionName(task.conn_id) }} / DB {{ task.db }}<template v-if="task.target_conn_id"> → {{ connectionName(task.target_conn_id) }} / DB {{ task.target_db }}</template></div>
        <div>{{ task.processed }}/{{ task.total }} · ✓ {{ task.succeeded }} · ↷ {{ task.skipped }} · ✕ {{ task.failed }}</div>
        <progress :value="task.processed" :max="Math.max(1, task.total)" />
        <div v-if="task.state === 'outcome_unknown'" role="alert">{{ t('reliability.outcomeUnknown') }}</div>
        <div v-else-if="task.message">{{ task.message }}</div>
        <button v-if="task.state === 'running'" :disabled="cancelling.has(task.id)" @click="cancel(task.id)">{{ t('reliability.cancelTask') }}</button>
        <button v-if="task.failures?.length" @click="copyFailures(task)">{{ t('reliability.copyFailures') }}</button>
      </article>
      <div v-if="message" role="status">{{ message }}</div>
    </div>
  </aside>
</template>

<script setup>
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { listOperationTasks, cancelOperationTask } from '../../api/wails.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useI18n } from '../../i18n/index.js'

const { t } = useI18n()
const connections = useConnectionsStore()
const tasks = ref([])
const expanded = ref(false)
const message = ref('')
const cancelling = ref(new Set())
let timer = null
let stopped = false
function connectionName(id) { return connections.connections.find(item => item.id === id)?.name || id }
async function refresh() {
  try {
    const result = await listOperationTasks()
    if (!stopped && Array.isArray(result)) {
      if (result.some(task => task.state === 'running' && !tasks.value.some(previous => previous.id === task.id))) expanded.value = true
      tasks.value = result
    }
  } catch { /* Older development bridge may not expose task APIs yet. */ }
  finally { if (!stopped) timer = setTimeout(refresh, 750) }
}
async function cancel(id) {
  cancelling.value = new Set([...cancelling.value, id])
  try { await cancelOperationTask(id) }
  catch (error) { message.value = String(error); cancelling.value.delete(id) }
}
async function copyFailures(task) {
  const copied = await copyToClipboard(JSON.stringify({ id: task.id, state: task.state, processed: task.processed, total: task.total, failed: task.failed, failures: task.failures }, null, 2))
  message.value = copied ? t('keyEditor.copied') : t('keyEditor.copyFailed')
}
onMounted(refresh)
onBeforeUnmount(() => { stopped = true; clearTimeout(timer) })
</script>

<style scoped>
.operation-tasks { position: fixed; bottom: 12px; right: 16px; z-index: 100; max-width: min(460px, 90vw); color: var(--text-main, #334155); background: var(--app-panel-bg, #fff); border: 1px solid #94a3b8; border-radius: 8px; box-shadow: 0 4px 20px #0002; }
.task-toggle { width: 100%; padding: 8px 14px; }
.task-list { max-height: 50vh; overflow: auto; padding: 10px; overflow-wrap: anywhere; font-size: 12px; }
article { padding: 10px 0; border-top: 1px solid #94a3b8; }
progress { width: 100%; }
button { cursor: pointer; margin: 2px; }
</style>
