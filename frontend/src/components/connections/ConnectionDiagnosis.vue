<template>
  <section class="connection-diagnosis">
    <button type="button" :disabled="busy" @click="run">{{ busy ? t('diagnose.running') : t('diagnose.open') }}</button>
    <button v-if="busy" type="button" @click="cancel">{{ t('diagnose.cancel') }}</button>
    <button v-if="report" type="button" @click="copy">{{ copied ? '✓' : t('diagnose.copy') }}</button>
    <p>{{ t('diagnose.hint') }}</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <ol v-if="report">
      <li v-for="stage in report.stages" :key="stage.stage" :data-stage="stage.stage" :data-status="stage.status">
        <strong>{{ label(stage.stage) }}</strong> · {{ t(`diagnose.${stage.status === 'failed' ? 'failedStatus' : stage.status}`) }} · {{ stage.duration_ms }} ms
        <small>{{ t(`diagnose.code_${stage.code}`) }}</small>
      </li>
    </ol>
  </section>
</template>
<script setup>
import { ref, watch, onBeforeUnmount } from 'vue'
import { diagnoseConnectionWithID, cancelConnectionDiagnosis } from '../../api/wails.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { useI18n } from '../../i18n/index.js'
const props = defineProps({ config: { type: Object, required: true } })
const { t } = useI18n()
const busy = ref(false)
const report = ref(null)
const copied = ref(false)
const error = ref('')
let generation = 0
function label(stage) { return t(`diagnose.stage_${stage}`) }
let activeRequest = ''
async function stopRequest() {
  const requestID = activeRequest
  if (!requestID) return
  // A cancel RPC can arrive before the start RPC registers. Retry that race,
  // keeping the start button disabled until the original invocation settles.
  for (let attempt = 0; attempt < 3 && activeRequest === requestID; attempt++) {
    try { if (await cancelConnectionDiagnosis(requestID)) return } catch { return }
    await new Promise(resolve => setTimeout(resolve, 50))
  }
}
function cancel() { generation++; stopRequest(); error.value = t('diagnose.cancelled') }
watch(() => JSON.stringify(props.config), () => { generation++; stopRequest(); report.value = null; error.value = '' })
onBeforeUnmount(() => { generation++; stopRequest() })
async function run() {
  if (busy.value) return
  const id = ++generation
  busy.value = true
  report.value = null
  error.value = ''
  copied.value = false
  const requestID = globalThis.crypto?.randomUUID?.() || `diagnosis-${Date.now()}-${id}`
  activeRequest = requestID
  try {
    const result = await diagnoseConnectionWithID(requestID, JSON.parse(JSON.stringify(props.config)))
    if (generation === id) report.value = result
  } catch { if (generation === id) error.value = t('diagnose.failed') }
  finally { if (activeRequest === requestID) { activeRequest = ''; busy.value = false } }
}
async function copy() {
  // Deliberately include only localized stages/status/codes. Never copy config or raw errors.
  copied.value = await copyToClipboard(report.value.stages.map(stage => `${label(stage.stage)}: ${stage.status} (${stage.code}, ${stage.duration_ms} ms)`).join('\n'))
  if (!copied.value) error.value = t('diagnose.failed')
}
</script>
<style scoped>
.connection-diagnosis { padding: 10px 0; font-size: 12px; }
button { background: transparent; color: inherit; cursor: pointer; padding: 5px 9px; border: 1px solid #94a3b8; border-radius: 5px; margin-right: 5px; }
button:disabled { opacity: .5; }
p { opacity: .8; }
ol { padding-left: 20px; }
li { padding: 5px; }
small { display: block; }
li[data-status=failed] { border-left: 3px solid #ef4444; }
li[data-status=success] { border-left: 3px solid #22c55e; }
</style>
