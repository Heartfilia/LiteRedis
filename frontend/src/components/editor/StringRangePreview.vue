<template>
  <details class="range-preview" @toggle="toggle">
    <summary>{{ t('range.title') }}</summary>
    <p>{{ t('range.hint') }}</p>
    <form @submit.prevent="read(Number(offsetInput))">
      <label>{{ t('range.offset') }} <input v-model="offsetInput" type="number" min="0" step="1" :disabled="loading" /></label>
      <button :disabled="loading">{{ t('range.read') }}</button>
      <button type="button" :disabled="loading || !page || page.offset === 0" @click="read(Math.max(0, page.offset - 65536))">{{ t('range.previous') }}</button>
      <button type="button" :disabled="loading || !page?.has_more" @click="read(page.next_offset)">{{ t('range.next') }}</button>
    </form>
    <p v-if="loading" role="status">{{ t('range.loading') }}</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <template v-if="page">
      <p class="range-position">[{{ page.start }}, {{ page.end }}) / {{ page.total_bytes }} bytes · {{ page.encoding }}</p>
      <p v-if="page.binary">{{ t('range.binary') }}</p>
      <pre tabindex="0">{{ page.value }}</pre>
    </template>
  </details>
</template>
<script setup>
import { ref, watch, onBeforeUnmount } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'
import { getStringRangePreview } from '../../api/wails.js'
const props = defineProps({ keyName: { type: String, required: true } })
const workspace = useWorkspaceStore()
const { t } = useI18n()
const page = ref(null)
const error = ref('')
const offsetInput = ref('0')
const loading = ref(false)
let sequence = 0
function invalidate() { sequence++; loading.value = false; page.value = null; error.value = ''; offsetInput.value = '0' }
watch(() => [props.keyName, workspace.activeConnID, workspace.currentDB], invalidate)
onBeforeUnmount(() => { sequence++ })
function toggle(event) { if (event.target.open && !page.value && !loading.value) void read(0) }
async function read(offset) {
  if (loading.value) return
  if (!Number.isSafeInteger(offset) || offset < 0) { error.value = t('range.invalid'); return }
  const id = ++sequence
  const connID = workspace.activeConnID
  const db = workspace.currentDB
  const key = props.keyName
  loading.value = true
  error.value = ''
  page.value = null
  try {
    const result = await getStringRangePreview(connID, db, key, offset)
    if (id !== sequence || connID !== workspace.activeConnID || db !== workspace.currentDB || key !== props.keyName) return
    page.value = result
    offsetInput.value = String(result.offset)
  } catch { if (id === sequence) error.value = t('range.failed') }
  finally { if (id === sequence) loading.value = false }
}
</script>
<style scoped>
.range-preview { border: 1px solid #94a3b8; border-radius: 6px; padding: 8px; font-size: 12px; }
summary { cursor: pointer; }
p { margin: 6px 0; }
form { display: flex; align-items: center; gap: 5px; flex-wrap: wrap; }
input { width: 130px; color: inherit; background: transparent; border: 1px solid #94a3b8; padding: 4px; }
button { color: inherit; background: transparent; border: 1px solid #94a3b8; padding: 4px 8px; border-radius: 4px; cursor: pointer; }
button:disabled { opacity: .4; cursor: not-allowed; }
pre { max-height: 320px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
</style>
