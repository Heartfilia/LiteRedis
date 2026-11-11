import { ref } from 'vue'
import { useI18n } from '../i18n/index.js'
import { registerEditorGuard } from './editSafety.js'

export const uncertainWrites = ref([])
const pending = new Set()
registerEditorGuard(Symbol('pending-writes'), () => ({ dirty: false, saving: pending.size > 0 }))
export function acknowledgeWrite(scope) {
  uncertainWrites.value = uncertainWrites.value.filter(item => item.scope !== scope)
}

// Latch uncertain outcomes per key, or per connection for Console/batches.
export async function guardedWrite(method, args, invoke, wholeConnection = false) {
  const { t } = useI18n()
  const connID = args[0]
  const key = wholeConnection ? null : args[1]?.key ?? args[1]
  const scope = JSON.stringify([connID, key])
  const failure = (code, message) => ({ success: false, code, message, error: message })
  if (uncertainWrites.value.some(item => item.connID === connID && (item.key === null || key === null || item.key === key))) return failure('outcome_unknown', t('reliability.outcomeUnknown'))
  if ([...pending].some(item => item === scope || item === JSON.stringify([connID, null])) || (wholeConnection && [...pending].some(item => JSON.parse(item)[0] === connID))) return failure('busy', t('reliability.saveInProgress'))
  pending.add(scope)
  try {
    let result
    try { result = await invoke() }
    catch { result = failure('outcome_unknown', t('reliability.outcomeUnknown')) }
    if (result?.code === 'outcome_unknown' || result?.code === 'partial_possible') {
      const detail = result.message || result.error
      uncertainWrites.value = [...uncertainWrites.value, { scope, connID, key, method, detail: detail ? String(detail).slice(0, 2000) : '' }]
      // Do not feed transport diagnostics into legacy connection-loss handlers:
      // an unknown response must retain the current editor and its draft.
      const message = t('reliability.outcomeUnknown')
      return { ...result, message, error: message }
    }
    return result
  } finally { pending.delete(scope) }
}
