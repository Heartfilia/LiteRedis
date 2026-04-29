<template>
  <transition name="float-toast">
    <div v-if="visible && message" :class="['floating-message', tone]">
      {{ message }}
    </div>
  </transition>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps({
  message: { type: String, default: '' },
  success: { type: Boolean, default: true },
  duration: { type: Number, default: 2200 },
})

const visible = ref(false)
let hideTimer = null

const tone = computed(() => props.success ? 'ok' : 'err')

function clearHideTimer() {
  if (hideTimer) {
    clearTimeout(hideTimer)
    hideTimer = null
  }
}

watch(() => props.message, (message) => {
  clearHideTimer()
  visible.value = !!message
  if (message) {
    hideTimer = setTimeout(() => {
      visible.value = false
      hideTimer = null
    }, props.duration)
  }
})

onBeforeUnmount(() => {
  clearHideTimer()
})
</script>

<style scoped>
.floating-message {
  position: absolute;
  top: 8px;
  right: 10px;
  z-index: 25;
  max-width: min(360px, calc(100% - 20px));
  padding: 8px 12px;
  border-radius: 10px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.14);
  backdrop-filter: blur(8px);
  font-size: 12px;
  line-height: 1.45;
  word-break: break-word;
  pointer-events: none;
}

.floating-message.ok {
  background: rgba(240, 253, 244, 0.96);
  color: #166534;
  border: 1px solid rgba(134, 239, 172, 0.9);
}

.floating-message.err {
  background: rgba(255, 241, 242, 0.96);
  color: #991b1b;
  border: 1px solid rgba(253, 164, 175, 0.9);
}

.float-toast-enter-active,
.float-toast-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.float-toast-enter-from,
.float-toast-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
