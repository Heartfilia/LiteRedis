<template>
  <div ref="rootEl" class="delete-wrap">
    <button class="btn-tiny danger" :class="{ 'danger-confirm': open }" @click.stop="toggleOpen">
      {{ label }}
    </button>
    <div v-if="open" class="delete-popover">
      <div class="delete-popover-arrow"></div>
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
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  label: { type: String, default: 'Delete' },
  confirmText: { type: String, default: 'Confirm delete?' },
  resetToken: { type: [String, Number, Boolean], default: '' },
})

const emit = defineEmits(['confirm'])

const open = ref(false)
const rootEl = ref(null)

function toggleOpen() {
  open.value = !open.value
}

function confirmDelete() {
  open.value = false
  emit('confirm')
}

function handleDocumentClick(event) {
  if (!open.value || !rootEl.value) return
  if (!rootEl.value.contains(event.target)) {
    open.value = false
  }
}

watch(() => props.resetToken, () => {
  open.value = false
})

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick)
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
