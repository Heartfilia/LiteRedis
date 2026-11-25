<template>
  <section class="command-templates" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <h3>{{ t('template.title') }}</h3>
    <p class="template-privacy">{{ t('template.privacy') }}</p>
    <p class="template-target">{{ connID }} · DB {{ db }}</p>
    <p v-if="message" :role="error ? 'alert' : 'status'">{{ message }}</p>
    <div class="template-form">
      <label>{{ t('template.name') }}<input v-model="name" maxlength="80" /></label>
      <label>{{ t('template.body') }}<textarea v-model="body" maxlength="4096" placeholder="GET {{key}}" rows="2" /></label>
      <button :disabled="!validBody || !name.trim()" @click="save">{{ t('template.save') }}</button>
    </div>
    <p v-if="!store.items.length">{{ t('template.empty') }}</p>
    <ul>
      <li v-for="item in store.items" :key="item.id">
        <span>{{ item.name }}</span><code>{{ item.template }}</code>
        <button @click="select(item)">{{ t('template.prepare') }}</button>
        <button @click="remove(item.id)">{{ t('template.remove') }}</button>
      </li>
    </ul>
    <div v-if="selected" class="template-prepare">
      <h4>{{ selected.name }}</h4>
      <label v-for="parameter in parameters" :key="parameter">
        {{ t('template.parameter') }}: {{ parameter }}
        <input v-model="values[parameter]" autocomplete="off" spellcheck="false" />
      </label>
      <h4>{{ t('template.preview') }}</h4>
      <pre>{{ preview || t('template.invalid') }}</pre>
      <p>{{ connID }} · DB {{ db }}</p>
      <button :disabled="!preview || !connID || !Number.isSafeInteger(db) || db < 0" @click="prepare">{{ t('template.prepare') }}</button>
    </div>
  </section>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { useCommandTemplatesStore } from '../../stores/commandTemplates.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { parseCommandTemplate, prepareCommandTemplate } from '../../utils/commandTemplates.js'

const props = defineProps({ connID: { type: String, default: '' }, db: { type: Number, default: 0 } })
const emit = defineEmits(['prepared'])
const { t } = useI18n()
const settingsStore = useSettingsStore()
const store = useCommandTemplatesStore()
const name = ref('')
const body = ref('')
const selected = ref(null)
const values = reactive(Object.create(null))
const message = ref('')
const error = ref(false)
const loaded = store.load()
if (!loaded.success) { error.value = true; message.value = t('template.invalid') + ': ' + loaded.message }
const validBody = computed(() => { try { parseCommandTemplate(body.value); return true } catch { return false } })
const parameters = computed(() => selected.value ? parseCommandTemplate(selected.value.template).parameters : [])
const preview = computed(() => {
  if (!selected.value) return ''
  try { return prepareCommandTemplate(selected.value.template, values) } catch { return '' }
})
function feedback(result, successText = '') {
  error.value = !result.success
  message.value = result.success ? successText : t('template.invalid') + ': ' + result.message
}
function save() {
  const result = store.save({ name: name.value, template: body.value })
  feedback(result, t('template.saved'))
  if (result.success) { name.value = ''; body.value = '' }
}
function clearSelection() {
  selected.value = null
  for (const key of Object.keys(values)) delete values[key]
}
function select(item) {
  clearSelection()
  selected.value = { ...item }
  for (const parameter of parameters.value) values[parameter] = ''
}
function remove(id) {
  const result = store.remove(id)
  feedback(result)
  if (result.success && selected.value?.id === id) clearSelection()
}
function prepare() {
  if (!preview.value || !props.connID || !Number.isSafeInteger(props.db) || props.db < 0) return
  emit('prepared', preview.value)
  clearSelection()
}
watch(() => [props.connID, props.db], clearSelection, { flush: 'sync' })
</script>

<style scoped>
.command-templates { padding: 12px; color: #334155; }
h3, h4 { margin: 8px 0; }
p { font-size: 12px; overflow-wrap: anywhere; }
.template-privacy { color: #92400e; }
.template-form, .template-prepare { display: grid; gap: 8px; }
label { display: grid; gap: 4px; font-size: 12px; }
input, textarea { width: 100%; box-sizing: border-box; padding: 7px; border: 1px solid #94a3b8; border-radius: 5px; background: #fff; color: inherit; }
button { padding: 5px 9px; border: 1px solid #94a3b8; border-radius: 5px; background: #f8fafc; color: inherit; cursor: pointer; }
button:disabled { opacity: .5; cursor: not-allowed; }
ul { list-style: none; padding: 0; }
li { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding: 8px 0; border-bottom: 1px solid #cbd5e1; }
code { flex: 1; min-width: 100px; overflow-wrap: anywhere; }
pre { max-height: 180px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; border: 1px solid #94a3b8; padding: 8px; }
.theme-dark { color: #e2e8f0; }
.theme-dark .template-privacy { color: #fcd34d; }
.theme-dark input, .theme-dark textarea, .theme-dark button { background: #0f172a; border-color: #475569; }
</style>
