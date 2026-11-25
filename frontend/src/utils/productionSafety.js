import { askConfirmation } from './confirm.js'
import { useI18n } from '../i18n/index.js'

// An additional UI safeguard, not an ACL or replacement for backend read-only checks.
export async function confirmProductionOperation(connection, db, action, count = 1) {
  if (connection?.environment !== 'production') return true
  const { t } = useI18n()
  return askConfirmation(t('environment.message', {
    name: connection.name || connection.id, db, action: t(`environment.${action}`), count,
  }), false, {
    title: t('environment.title'), danger: true,
    requiredText: action === 'delete' ? 'DELETE' : 'CONFIRM',
  })
}
