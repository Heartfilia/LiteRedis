<template>
  <Teleport to="body">
    <div v-if="confirmation" class="confirmation-overlay" :class="{ dark: settingsStore.themeMode === 'dark' }" @keydown.stop="trapKeys">
      <section class="confirmation-dialog" :class="{ 'is-danger': confirmation.danger, 'is-quit': confirmation.kind === 'quit' }" role="alertdialog" aria-modal="true" aria-labelledby="confirmation-title" aria-describedby="confirmation-message">
        <div class="confirmation-body">
          <span class="confirmation-icon" aria-hidden="true">
            <svg v-if="confirmation.kind === 'quit'" viewBox="0 0 24 24" fill="none">
              <path d="M12 3v9M6.3 5.8a8 8 0 1 0 11.4 0" />
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="8.5" />
              <path d="M12 10.5v5M12 7.5v.1" />
            </svg>
          </span>
          <div class="confirmation-copy">
            <h2 id="confirmation-title">{{ confirmation.title || t(confirmation.notice ? 'reliability.noticeTitle' : 'reliability.confirmTitle') }}</h2>
            <p id="confirmation-message">{{ confirmation.message }}</p>
          </div>
        </div>
        <div class="confirmation-actions">
          <span class="confirmation-key-hint" aria-hidden="true">Esc<span>{{ t('keyEditor.cancel') }}</span></span>
          <div class="confirmation-buttons">
            <button ref="cancelButton" class="confirmation-cancel" @click="resolveConfirmation(false)">{{ confirmation.cancelLabel || t(confirmation.notice ? 'keyEditor.confirm' : 'keyEditor.cancel') }}</button>
            <button v-if="!confirmation.notice" class="confirmation-accept" @click="resolveConfirmation(true)">{{ confirmation.acceptLabel || t('keyEditor.confirm') }}</button>
          </div>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<script setup>
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { confirmation, resolveConfirmation } from '../../utils/confirm.js'
import { registerEscapeHandler } from '../../utils/shortcuts.js'
import { useI18n } from '../../i18n/index.js'
import { useSettingsStore } from '../../stores/settings.js'
const { t } = useI18n()
const settingsStore = useSettingsStore()
const cancelButton = ref(null)
let disposeEscape = null
let previousFocus = null
watch(confirmation, async (current) => {
  disposeEscape?.()
  if (current) {
    previousFocus = document.activeElement
    disposeEscape = registerEscapeHandler(() => resolveConfirmation(false))
    await nextTick()
    cancelButton.value?.focus()
  } else {
    disposeEscape = null
    previousFocus?.focus?.()
  }
})
function trapKeys(event) {
  if ((event.metaKey || event.ctrlKey) && ['w', 'q'].includes(event.key.toLowerCase())) {
    event.preventDefault()
    return
  }
  if (event.key === 'Escape') { event.preventDefault(); resolveConfirmation(false) }
  if (event.key === 'Tab') {
    event.preventDefault()
    const buttons = [...event.currentTarget.querySelectorAll('button')]
    const direction = event.shiftKey ? -1 : 1
    buttons[(buttons.indexOf(document.activeElement) + direction + buttons.length) % buttons.length]?.focus()
  }
}
onBeforeUnmount(() => { disposeEscape?.(); resolveConfirmation(false) })
</script>

<style scoped>
.confirmation-overlay {
  position: fixed;
  inset: 0;
  z-index: 200000;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(15, 23, 42, 0.32);
  -webkit-backdrop-filter: blur(4px);
  backdrop-filter: blur(4px);
  animation: confirmation-fade 140ms ease-out;
}
.confirmation-dialog {
  --dialog-bg: #fff;
  --dialog-text: #1e293b;
  --dialog-muted: #64748b;
  --dialog-border: #e2e8f0;
  --dialog-footer: #f8fafc;
  --dialog-accent: #2563eb;
  --dialog-accent-hover: #1d4ed8;
  --dialog-icon-bg: #eff6ff;
  width: min(460px, 100%);
  max-height: calc(100dvh - 48px);
  overflow-y: auto;
  border: 1px solid var(--dialog-border);
  border-radius: 18px;
  background: var(--dialog-bg);
  color: var(--dialog-text);
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.2), 0 4px 16px rgba(15, 23, 42, 0.08);
  animation: confirmation-enter 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}
.confirmation-body { display: flex; align-items: flex-start; gap: 16px; padding: 28px 26px 26px; }
.confirmation-icon {
  display: grid;
  place-items: center;
  flex: 0 0 46px;
  height: 46px;
  border-radius: 14px;
  background: var(--dialog-icon-bg);
  color: var(--dialog-accent);
}
.confirmation-icon svg { width: 24px; height: 24px; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
.confirmation-copy { flex: 1; min-width: 0; padding-top: 2px; }
.confirmation-copy h2 { margin: 0; font-size: 16px; font-weight: 650; line-height: 1.5; letter-spacing: -0.2px; }
.confirmation-copy p { margin: 8px 0 0; font-size: 13px; color: var(--dialog-muted); white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.8; }
.is-quit { width: min(360px, 100%); border-radius: 14px; }
.is-quit .confirmation-body { padding: 22px 20px; gap: 12px; }
.is-quit .confirmation-icon { flex-basis: 38px; height: 38px; border-radius: 11px; }
.is-quit .confirmation-icon svg { width: 21px; height: 21px; }
.is-quit .confirmation-copy h2 { font-size: 15px; }
.is-quit .confirmation-copy p { margin-top: 6px; font-size: 12px; line-height: 1.7; }
.confirmation-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 24px;
  border-top: 1px solid var(--dialog-border);
  background: var(--dialog-footer);
}
.confirmation-key-hint { display: inline-flex; align-items: center; gap: 6px; color: var(--dialog-muted); font-size: 11px; }
.is-quit .confirmation-actions { padding: 13px 20px; gap: 12px; }
.confirmation-key-hint > span { font-size: 12px; }
.confirmation-buttons { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 10px; margin-left: auto; }
.confirmation-buttons button {
  min-height: 34px;
  padding: 7px 16px;
  border: 1px solid var(--dialog-border);
  border-radius: 8px;
  font: inherit;
  font-size: 12px;
  font-weight: 550;
  line-height: 1.5;
  cursor: pointer;
  transition: background 120ms, border-color 120ms, box-shadow 120ms;
}
.confirmation-cancel { background: var(--dialog-bg); color: var(--dialog-text); }
.confirmation-cancel:hover { background: var(--dialog-footer); border-color: #94a3b8; }
.confirmation-buttons .confirmation-accept { background: var(--dialog-accent); border-color: var(--dialog-accent); color: #fff; }
.confirmation-accept:hover { background: var(--dialog-accent-hover); border-color: var(--dialog-accent-hover); }
.confirmation-buttons button:focus-visible { outline: 2px solid var(--dialog-accent); outline-offset: 3px; }
.is-danger { --dialog-accent: #dc2626; --dialog-accent-hover: #b91c1c; --dialog-icon-bg: #fef2f2; }
.dark .confirmation-dialog {
  --dialog-bg: #1e293b;
  --dialog-text: #f1f5f9;
  --dialog-muted: #a8b6c9;
  --dialog-border: #36465d;
  --dialog-footer: #192334;
  --dialog-accent: #3b82f6;
  --dialog-accent-hover: #2563eb;
  --dialog-icon-bg: #243957;
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.4);
}
.dark .is-danger { --dialog-accent: #dc4545; --dialog-accent-hover: #c93636; --dialog-icon-bg: #402b36; }
@keyframes confirmation-fade { from { opacity: 0; } to { opacity: 1; } }
@keyframes confirmation-enter { from { opacity: 0; transform: translateY(6px) scale(0.98); } to { opacity: 1; transform: translateY(0) scale(1); } }
@media (max-width: 480px) {
  .confirmation-body { padding: 22px 20px; gap: 12px; }
  .confirmation-actions { padding: 16px 20px; }
  .confirmation-key-hint { display: none; }
}
@media (prefers-reduced-motion: reduce) {
  .confirmation-overlay, .confirmation-dialog { animation: none; }
  .confirmation-buttons button { transition: none; }
}
</style>
