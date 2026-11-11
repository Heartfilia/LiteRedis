import { useI18n } from '../i18n/index.js'
import { askConfirmation } from './confirm.js'

const editors = new Map()
export function registerEditorGuard(id, inspect) {
  editors.set(id, inspect)
  return () => editors.delete(id)
}
export function hasUnsavedEdits() {
  return [...editors.values()].some((inspect) => inspect().dirty || inspect().saving)
}
export function hasSavingEdits() {
  return [...editors.values()].some((inspect) => inspect().saving)
}
export async function confirmDiscardEdits({ alwaysConfirm = false, message, dialog } = {}) {
  const { t } = useI18n()
  const registered = [...editors.entries()]
  const states = registered.map(([, inspect]) => inspect())
  if (states.some((state) => state.saving)) {
    await askConfirmation(t('reliability.saveInProgress'), true)
    return false
  }
  const dirty = states.some((state) => state.dirty)
  if (!dirty && !alwaysConfirm) return true
  if (!await askConfirmation(message || t('reliability.discardDraft'), false, dialog)) return false
  // A different editor must never be discarded using an older confirmation.
  if (editors.size !== registered.length || registered.some(([id, inspect]) => editors.get(id) !== inspect)) return false
  // The editor may have started saving while confirmation was open.
  if ([...editors.values()].some(inspect => inspect().saving)) return false
  states.forEach((state) => state.discard?.())
  return true
}
export function editResultMessage(result) {
  const { t } = useI18n()
  const keys = { conflict: 'editConflict', outcome_unknown: 'outcomeUnknown', partial_possible: 'outcomeUnknown', permission_denied: 'editPermission', unsupported: 'editUnsupported', cluster_redirect: 'clusterRedirect' }
  return keys[result?.code] ? t('reliability.' + keys[result.code]) : result?.message || t('keyEditor.saveFailed')
}
