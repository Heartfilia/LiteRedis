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
  padding: 9px 13px;
  border-radius: 11px;
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.14);
  backdrop-filter: blur(14px);
  font-size: 12px;
  line-height: 1.45;
  font-weight: 600;
  letter-spacing: 0.01em;
  word-break: break-word;
  pointer-events: none;
  transform-origin: top right;
}

.floating-message.ok {
  background: linear-gradient(135deg, rgba(240, 253, 244, 0.98), rgba(220, 252, 231, 0.96));
  color: #166534;
  border: 1px solid rgba(110, 231, 183, 0.92);
  box-shadow: 0 14px 28px rgba(34, 197, 94, 0.16), 0 0 0 1px rgba(255, 255, 255, 0.35) inset;
  animation: successPulse 0.42s ease-out;
}

.floating-message.err {
  background: linear-gradient(135deg, rgba(255, 241, 242, 0.98), rgba(255, 228, 230, 0.96));
  color: #991b1b;
  border: 1px solid rgba(253, 164, 175, 0.9);
  box-shadow: 0 14px 28px rgba(239, 68, 68, 0.12), 0 0 0 1px rgba(255, 255, 255, 0.3) inset;
}

.float-toast-enter-active,
.float-toast-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease, filter 0.22s ease;
}

.float-toast-enter-from,
.float-toast-leave-to {
  opacity: 0;
  transform: translateY(-10px) scale(0.97);
  filter: blur(3px);
}

@keyframes successPulse {
  0% {
    transform: translateY(-3px) scale(0.985);
  }

  58% {
    transform: translateY(0) scale(1.018);
  }

  100% {
    transform: translateY(0) scale(1);
  }
}

</style>
