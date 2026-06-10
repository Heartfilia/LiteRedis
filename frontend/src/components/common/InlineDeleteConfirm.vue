<template>
  <div ref="rootEl" class="delete-wrap" :class="[{ open }, `theme-${settingsStore.themeMode || 'light'}`]">
    <button
      type="button"
      class="btn-tiny danger"
      :class="[{ 'danger-confirm': open }, { 'icon-only': iconOnly }]"
      @click.stop="toggleOpen"
    >
      <svg v-if="iconOnly" viewBox="0 0 24 24" width="15" height="15" aria-hidden="true">
        <path
          d="M6.7 6.7l10.6 10.6M17.3 6.7L6.7 17.3"
          fill="none"
          stroke="currentColor"
          stroke-width="1.9"
          stroke-linecap="round"
        />
      </svg>
      <template v-else>{{ label }}</template>
    </button>
    <Teleport to="body">
      <div
        v-if="open"
        ref="popoverEl"
        class="delete-popover"
        :class="[popoverPlacementClass, `theme-${settingsStore.themeMode || 'light'}`]"
        :style="popoverStyle"
        @pointerdown.stop
        @click.stop
      >
        <div class="delete-popover-arrow" :class="popoverPlacementClass"></div>
        <div class="delete-popover-content">
          <span class="delete-popover-text">{{ confirmText }}</span>
          <div class="delete-popover-btns">
            <button
              type="button"
              class="btn-tiny btn-confirm-yes"
              @pointerdown.prevent.stop="confirmDelete"
            >✓</button>
            <button
              type="button"
              class="btn-tiny btn-confirm-no"
              @pointerdown.prevent.stop="close"
            >✕</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { Teleport, computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'

const props = defineProps({
  label: { type: String, default: 'Delete' },
  confirmText: { type: String, default: 'Confirm delete?' },
  resetToken: { type: [String, Number, Boolean], default: '' },
  iconOnly: { type: Boolean, default: false },
})

const emit = defineEmits(['confirm'])
const settingsStore = useSettingsStore()

let activeCloser = null

const open = ref(false)
const rootEl = ref(null)
const popoverEl = ref(null)
const popoverPlacement = ref('bottom')
const popoverStyle = ref({})

const popoverPlacementClass = computed(() => ({
  top: popoverPlacement.value === 'top',
  bottom: popoverPlacement.value !== 'top',
}))

function toggleOpen() {
  if (open.value) {
    close()
    return
  }
  if (activeCloser && activeCloser !== close) {
    activeCloser()
  }
  open.value = true
  activeCloser = close
  nextTick(() => {
    updatePlacement()
    ensureVisible()
  })
}

function confirmDelete() {
  close()
  emit('confirm')
}

function close() {
  open.value = false
  if (activeCloser === close) {
    activeCloser = null
  }
}

function handleDocumentPointerDown(event) {
  if (!open.value || !rootEl.value) return
  const path = typeof event.composedPath === 'function' ? event.composedPath() : []
  const isInside = rootEl.value.contains(event.target) || path.includes(rootEl.value) || (popoverEl.value && path.includes(popoverEl.value))
  if (!isInside) {
    close()
  }
}

function handleWindowChange() {
  if (!open.value) return
  updatePlacement()
}

function updatePlacement() {
  if (!rootEl.value || !popoverEl.value) return
  popoverPlacement.value = 'bottom'
  const rootRect = rootEl.value.getBoundingClientRect()
  const popoverRect = popoverEl.value.getBoundingClientRect()
  const viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0
  const viewportWidth = window.innerWidth || document.documentElement.clientWidth || 0
  const spaceBelow = viewportHeight - rootRect.bottom
  const spaceAbove = rootRect.top
  if (spaceBelow < popoverRect.height + 8 && spaceAbove > spaceBelow) {
    popoverPlacement.value = 'top'
  } else {
    popoverPlacement.value = 'bottom'
  }

  const left = Math.max(8, Math.min(rootRect.right - popoverRect.width, viewportWidth - popoverRect.width - 8))
  const top = popoverPlacement.value === 'top'
    ? Math.max(8, rootRect.top - popoverRect.height - 6)
    : Math.min(viewportHeight - popoverRect.height - 8, rootRect.bottom + 6)

  popoverStyle.value = {
    left: `${left}px`,
    top: `${top}px`,
  }
}

function ensureVisible() {
  if (!rootEl.value) return
  rootEl.value.scrollIntoView({ block: 'nearest', inline: 'nearest' })
}

watch(() => props.resetToken, () => {
  close()
})

onMounted(() => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
  window.addEventListener('resize', handleWindowChange)
  window.addEventListener('scroll', handleWindowChange, true)
})

onBeforeUnmount(() => {
  close()
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
  window.removeEventListener('resize', handleWindowChange)
  window.removeEventListener('scroll', handleWindowChange, true)
})
</script>

<style scoped>
.delete-wrap {
  position: relative;
  display: inline-flex;
}

.delete-wrap.open {
  z-index: 10030;
}

.btn-tiny.icon-only {
  padding: 0;
}

.btn-tiny.danger-confirm {
  background: #dc2626;
  color: #fff;
  border-color: #dc2626;
}

.btn-tiny.danger-confirm:hover {
  background: #b91c1c;
  border-color: #b91c1c;
}

.delete-wrap.theme-dark > .btn-tiny {
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.delete-wrap.theme-dark > .btn-tiny:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #f8fafc;
  border-color: rgba(96, 165, 250, 0.34);
}

.delete-wrap.theme-dark > .btn-tiny.danger-confirm,
.delete-wrap.theme-dark > .btn-tiny.danger-confirm:hover {
  color: #fff;
  border-color: #dc2626;
}

.delete-popover {
  position: fixed;
  z-index: 20050;
  isolation: isolate;
  background: #fff;
  background-color: #fff;
  border: 1px solid rgba(226, 232, 240, 0.84);
  border-radius: 8px;
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.16), 0 8px 18px rgba(148, 163, 184, 0.08);
  padding: 8px 10px;
  white-space: nowrap;
  opacity: 1;
  backdrop-filter: none;
  pointer-events: auto;
}

.delete-popover.theme-dark {
  background: rgba(15, 23, 42, 0.98);
  background-color: rgba(15, 23, 42, 0.98);
  border-color: rgba(51, 65, 85, 0.82);
  box-shadow: 0 20px 40px rgba(2, 6, 23, 0.46), 0 8px 18px rgba(2, 6, 23, 0.18);
}
.delete-popover.top {
  transform-origin: bottom right;
}
.delete-popover.bottom {
  transform-origin: top right;
}

.delete-popover-arrow {
  position: absolute;
  top: -5px;
  right: 10px;
  width: 10px;
  height: 10px;
  background: #fff;
  background-color: #fff;
  border-left: 1px solid rgba(226, 232, 240, 0.84);
  border-top: 1px solid rgba(226, 232, 240, 0.84);
  transform: rotate(45deg);
}
.delete-popover-arrow.top {
  top: auto;
  bottom: -5px;
  transform: rotate(225deg);
}
.delete-popover-arrow.bottom {
  top: -5px;
  bottom: auto;
  transform: rotate(45deg);
}

.delete-popover.theme-dark .delete-popover-arrow {
  background: rgba(15, 23, 42, 0.98);
  background-color: rgba(15, 23, 42, 0.98);
  border-left-color: rgba(51, 65, 85, 0.82);
  border-top-color: rgba(51, 65, 85, 0.82);
}

.delete-popover-content {
  display: flex;
  align-items: center;
  gap: 8px;
  position: relative;
  z-index: 1;
  background: #fff;
  background-color: #fff;
}

.delete-popover.theme-dark .delete-popover-content {
  background: rgba(15, 23, 42, 0.98);
  background-color: rgba(15, 23, 42, 0.98);
}

.delete-popover-text {
  font-size: 12px;
  color: #dc2626;
  font-weight: 500;
}

.delete-popover.theme-dark .delete-popover-text {
  color: #e2e8f0;
}

.delete-popover-btns {
  display: flex;
  gap: 4px;
}

.delete-popover .btn-tiny {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  min-width: 28px;
  height: 26px;
  min-height: 26px;
  padding: 0;
  border: 1px solid currentColor;
  border-radius: 7px;
  cursor: pointer;
  background: #fff;
  color: #475569;
  font-size: 12px;
  line-height: 1;
  font-weight: 700;
  box-sizing: border-box;
  transition: background 0.14s ease, border-color 0.14s ease, color 0.14s ease, transform 0.14s ease;
}

.delete-popover .btn-tiny:hover {
  transform: translateY(-1px);
}

.delete-popover .btn-tiny:active {
  transform: translateY(0);
}

.delete-popover.theme-dark .btn-tiny {
  background: rgba(30, 41, 59, 0.92);
  color: #cbd5e1;
  border-color: rgba(71, 85, 105, 0.96);
}

.delete-popover.theme-dark .btn-tiny:hover {
  border-color: rgba(96, 165, 250, 0.3);
}

.btn-confirm-yes {
  color: #16a34a;
  border-color: #16a34a;
}

.btn-confirm-yes:hover {
  background: #16a34a;
  color: #fff;
}

.btn-confirm-no {
  color: #dc2626;
  border-color: #dc2626;
}

.btn-confirm-no:hover {
  background: #dc2626;
  color: #fff;
}

</style>
