<template>
  <Teleport to="body">
    <div class="conflict-overlay" :class="`theme-${settingsStore.themeMode || 'light'}`">
      <section ref="dialog" class="conflict-dialog" role="dialog" aria-modal="true" :aria-label="t('conflict.title')" tabindex="-1" @keydown.tab="trapFocus">
        <header><h3>{{ t('conflict.title') }}</h3><button @click="$emit('close')">{{ t('conflict.close') }}</button></header>
        <p class="conflict-target">{{ snapshot.key }}<template v-if="snapshot.isHash"> / {{ snapshot.field }}</template></p>
        <p>{{ t('conflict.hint') }} · 64 KiB</p>
        <div class="conflict-columns">
          <section v-for="column in columns" :key="column.name" :data-conflict-column="column.name">
            <h4>{{ t(`conflict.${column.name}`) }}</h4>
            <p v-if="column.missing">{{ t('conflict.missing') }}</p>
            <p v-if="column.binary">{{ t('conflict.binary') }}</p>
            <p v-if="column.name === 'current' && snapshot.isHash && current?.truncated && !current?.value">{{ t('conflict.hashTooLarge') }}</p>
            <p v-if="column.truncated">{{ t('conflict.truncated') }} · 64 KiB</p>
            <p v-if="column.changed">{{ t('conflict.typeChanged') }}: {{ snapshot.isHash ? 'hash' : 'string' }} → {{ current.type }}</p>
            <p v-if="column.name === 'current' && loading" role="status">{{ t('conflict.loading') }}</p>
            <p v-else-if="column.name === 'current' && failed" role="alert">{{ t('conflict.failed') }} <button @click="loadCurrent">{{ t('conflict.open') }}</button></p>
            <pre v-else-if="!column.missing && !column.changed">{{ column.value }}</pre>
            <small v-if="column.name === 'current' && current">{{ current.total_bytes ?? 0 }} bytes</small>
          </section>
        </div>
        <footer><button @click="copyDraft">{{ copied ? '✓ ' : '' }}{{ t('conflict.copyDraft') }}</button><span v-if="copyFailed" role="alert">{{ t('conflict.failed') }}</span></footer>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { getEditConflict } from '../../api/wails.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import { copyToClipboard } from '../../utils/clipboard.js'
import { createRequestGuard } from '../../utils/requestGuard.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { conflictPreview } from '../../utils/conflictComparison.js'

const props = defineProps({ snapshot: { type: Object, required: true } })
const emit = defineEmits(['close'])
let disposeEscape = null
const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
const loading = ref(true)
const failed = ref(false)
const current = ref(null)
const copied = ref(false)
const copyFailed = ref(false)
const dialog = ref(null)
const previousFocus = typeof document !== 'undefined' ? document.activeElement : null
const guard = createRequestGuard(() => ({
  connID: workspaceStore.activeConnID, db: workspaceStore.currentDB,
  key: workspaceStore.selectedKey, snapshot: props.snapshot,
}))
const columns = computed(() => {
  const server = current.value
  return [
    { name: 'original', ...conflictPreview(props.snapshot.original), missing: props.snapshot.originalExists === false },
    { name: 'draft', ...conflictPreview(props.snapshot.draft) },
    { name: 'current', ...conflictPreview(server?.value), missing: !!server && !server.exists,
      binary: !!server?.binary, truncated: !!server?.truncated || conflictPreview(server?.value).truncated,
      changed: !!server?.type && server.type !== 'none' && server.type !== (props.snapshot.isHash ? 'hash' : 'string') },
  ]
})
async function copyDraft() {
  copied.value = await copyToClipboard(props.snapshot.draft)
  copyFailed.value = !copied.value
}
function trapFocus(event) {
  const buttons = [...dialog.value.querySelectorAll('button:not(:disabled)')]
  const first = buttons[0]
  const last = buttons[buttons.length - 1]
  if (event.shiftKey && (document.activeElement === first || document.activeElement === dialog.value)) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
async function loadCurrent() {
  loading.value = true
  failed.value = false
  current.value = null
  const snapshot = props.snapshot
  const request = guard.begin()
  if (request.context.connID !== snapshot.connID || request.context.db !== snapshot.db || request.context.key !== snapshot.key) {
    loading.value = false
    failed.value = true
    guard.finish(request)
    return
  }
  try {
    const result = await getEditConflict(snapshot.connID, snapshot.db, snapshot.key, snapshot.field || '', !!snapshot.isHash)
    if (guard.isCurrent(request)) current.value = result
  } catch {
    if (guard.isCurrent(request)) failed.value = true
  } finally {
    if (guard.isCurrent(request)) loading.value = false
    guard.finish(request)
  }
}
onMounted(() => {
  disposeEscape = registerEscapeHandler(() => { emit('close'); return true })
  dialog.value?.focus()
  loadCurrent()
})
onBeforeUnmount(() => { guard.invalidateAll(); disposeEscape?.(); previousFocus?.focus?.() })
</script>

<style scoped>
.conflict-overlay { position: fixed; inset: 0; z-index: 10020; display: flex; align-items: center; justify-content: center; background: #0009; }
.conflict-dialog { width: min(1280px, 95vw); max-height: 92vh; overflow: auto; padding: 18px; border-radius: 12px; background: #fff; color: #1e293b; box-shadow: 0 24px 64px #0005; }
header, footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
h3, h4 { margin: 0; }
p { font-size: 13px; }
.conflict-target { overflow-wrap: anywhere; font-family: monospace; }
.conflict-columns { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 16px 0; }
.conflict-columns > section { min-width: 0; border: 1px solid #cbd5e1; border-radius: 6px; padding: 10px; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; overflow: auto; max-height: 55vh; font-size: 12px; }
button { padding: 6px 12px; border: 1px solid #94a3b8; border-radius: 6px; background: #f8fafc; color: inherit; cursor: pointer; }
.theme-dark .conflict-dialog { background: #0f172a; color: #e2e8f0; }
.theme-dark button { background: #1e293b; }
.theme-dark .conflict-columns > section { border-color: #475569; }
@media (max-width: 700px) { .conflict-columns { grid-template-columns: 1fr; } }
</style>
