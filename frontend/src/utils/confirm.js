import { shallowRef } from 'vue'

export const confirmation = shallowRef(null)

// WebView does not implement browser confirm/alert. One app-owned dialog at a
// time; overlapping navigation is declined instead of replayed after the user.
export function askConfirmation(message, notice = false, options = {}) {
  if (confirmation.value) return Promise.resolve(false)
  return new Promise(resolve => { confirmation.value = { ...options, message, notice, resolve } })
}

export function resolveConfirmation(accepted) {
  const current = confirmation.value
  confirmation.value = null
  current?.resolve(accepted)
}
