import { useSettingsStore } from '../stores/settings.js'

function firstLine(message) {
  return String(message || '').split('\n')[0].trim()
}

export function formatDebugMessage(message, fallback = '') {
  const settingsStore = useSettingsStore()
  if (settingsStore.debugMode) {
    return message || fallback
  }
  return firstLine(message) || fallback
}
