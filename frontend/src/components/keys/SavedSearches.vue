<template>
  <details class="saved-searches" @toggle="load">
    <summary>{{ t('savedSearch.title') }}</summary>
    <form @submit.prevent="save">
      <input v-model="name" :aria-label="t('savedSearch.name')" :placeholder="t('savedSearch.name')" maxlength="80" />
      <input v-model="pattern" :aria-label="t('savedSearch.pattern')" :placeholder="t('savedSearch.pattern')" maxlength="1024" />
      <button :disabled="busy || !workspace.activeConnID">{{ t('savedSearch.save') }}</button>
    </form>
    <p v-if="message" role="status">{{ message }}</p>
    <div v-for="item in entries" :key="item.id" class="saved-search-row">
      <button class="saved-search-run" :disabled="busy" @click="run(item)">{{ item.name }} · DB {{ item.db }}<small>{{ item.pattern }}</small></button>
      <button :disabled="busy" :aria-label="t('savedSearch.remove')" @click="remove(item.id)">×</button>
    </div>
    <p v-if="!entries.length">{{ t('savedSearch.empty') }}</p>
  </details>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useSavedSearchesStore } from '../../stores/savedSearches.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useConnectionsStore } from '../../stores/connections.js'
import { useI18n } from '../../i18n/index.js'
import { confirmDiscardEdits } from '../../utils/editSafety.js'
const { t } = useI18n()
const store = useSavedSearchesStore()
const workspace = useWorkspaceStore()
const connections = useConnectionsStore()
const name = ref('')
const pattern = ref('')
const message = ref('')
const busy = ref(false)
const entries = computed(() => store.forConnection(workspace.activeConnID).slice().sort((a, b) => b.lastUsed - a.lastUsed))
function errorText(result) { return t(result.code === 'invalid_input' ? 'savedSearch.invalid' : result.code === 'limit_reached' ? 'savedSearch.limit' : 'savedSearch.error') }
function load() {
  const result = store.load()
  if (!result.success) message.value = errorText(result)
}
function save() {
  const result = store.add({ name: name.value, pattern: pattern.value || workspace.activeSession?.pattern || '*', connID: workspace.activeConnID, db: workspace.currentDB })
  message.value = result.success ? t('savedSearch.saved') : errorText(result)
  if (result.success) { name.value = ''; pattern.value = '' }
}
function remove(id) {
  const result = store.remove(id)
  message.value = result.success ? '' : errorText(result)
}
async function run(item) {
  if (busy.value) return
  busy.value = true
  message.value = ''
  const connID = workspace.activeConnID
  const db = workspace.currentDB
  try {
    if (item.connID !== connID || !connections.isConnected(connID)) throw new Error(t('savedSearch.connectFirst'))
    const conn = connections.connections.find(c => c.id === connID)
    if (conn?.is_cluster && item.db !== 0) throw new Error(t('savedSearch.clusterDB'))
    if (!await confirmDiscardEdits()) return
    if (workspace.activeConnID !== connID || workspace.currentDB !== db) return
    if (item.db !== db) {
      const result = await workspace.switchDB(item.db)
      if (result?.cancelled || workspace.activeConnID !== connID || workspace.currentDB !== item.db) return
    }
    if (conn?.is_cluster && !conn.allow_cluster_scan || !['*', '?', '['].some(char => item.pattern.includes(char))) await workspace.searchExact(item.pattern)
    else await workspace.search(item.pattern)
    const result = store.touch(item.id)
    if (!result.success) message.value = errorText(result)
  } catch (error) { message.value = error.message || String(error) }
  finally { busy.value = false }
}
</script>

<style scoped>
.saved-searches { margin-top: 6px; font-size: 12px; color: inherit; }
summary { cursor: pointer; padding: 5px 0; }
form { display: flex; flex-wrap: wrap; gap: 5px; padding: 6px 0; }
input { width: 100%; min-width: 0; padding: 5px; border: 1px solid var(--app-panel-border); border-radius: 5px; background: var(--app-panel-bg); color: inherit; }
button { cursor: pointer; border: 1px solid var(--app-panel-border); background: var(--app-panel-bg); color: inherit; border-radius: 5px; padding: 4px 7px; }
button:disabled { opacity: .5; cursor: not-allowed; }
.saved-search-row { display: flex; gap: 5px; margin: 4px 0; }
.saved-search-run { flex: 1; min-width: 0; text-align: left; overflow-wrap: anywhere; }
small { display: block; opacity: .7; }
p { font-size: 11px; overflow-wrap: anywhere; }
</style>
