<template>
  <div ref="rootEl" class="delete-wrap">
    <button class="btn-tiny danger" :class="{ 'danger-confirm': open }" @click.stop="toggleOpen">
      {{ label }}
    </button>
    <div v-if="open" ref="popoverEl" class="delete-popover" :class="popoverPlacementClass">
      <div class="delete-popover-arrow" :class="popoverPlacementClass"></div>
      <div class="delete-popover-content">
        <span class="delete-popover-text">{{ confirmText }}</span>
        <div class="delete-popover-btns">
          <button class="btn-tiny btn-confirm-yes" @click.stop="confirmDelete">✓</button>
          <button class="btn-tiny btn-confirm-no" @click.stop="open = false">✕</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  label: { type: String, default: 'Delete' },
  confirmText: { type: String, default: 'Confirm delete?' },
  resetToken: { type: [String, Number, Boolean], default: '' },
})

const emit = defineEmits(['confirm'])

let activeCloser = null

const open = ref(false)
const rootEl = ref(null)
const popoverEl = ref(null)
const popoverPlacement = ref('bottom')

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
  if (!rootEl.value.contains(event.target)) {
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
  const spaceBelow = viewportHeight - rootRect.bottom
  const spaceAbove = rootRect.top
  if (spaceBelow < popoverRect.height + 8 && spaceAbove > spaceBelow) {
    popoverPlacement.value = 'top'
  } else {
    popoverPlacement.value = 'bottom'
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
  document.addEventListener('pointerdown', handleDocumentPointerDown, true)
  window.addEventListener('resize', handleWindowChange)
  window.addEventListener('scroll', handleWindowChange, true)
})

onBeforeUnmount(() => {
  close()
  document.removeEventListener('pointerdown', handleDocumentPointerDown, true)
  window.removeEventListener('resize', handleWindowChange)
  window.removeEventListener('scroll', handleWindowChange, true)
})
</script>

<style scoped>
.delete-wrap {
  position: relative;
  display: inline-flex;
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

.delete-popover {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  z-index: 100;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  padding: 8px 10px;
  white-space: nowrap;
}
.delete-popover.top {
  top: auto;
  bottom: calc(100% + 6px);
}
.delete-popover.bottom {
  top: calc(100% + 6px);
  bottom: auto;
}

.delete-popover-arrow {
  position: absolute;
  top: -5px;
  right: 10px;
  width: 10px;
  height: 10px;
  background: #fff;
  border-left: 1px solid #e5e7eb;
  border-top: 1px solid #e5e7eb;
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

.delete-popover-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.delete-popover-text {
  font-size: 12px;
  color: #dc2626;
  font-weight: 500;
}

.delete-popover-btns {
  display: flex;
  gap: 4px;
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
