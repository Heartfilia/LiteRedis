<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="extract-menu-backdrop"
      @click.stop="emit('close')"
      @contextmenu.prevent="emit('close')"
    >
      <div
        class="extract-menu"
        :class="`theme-${theme}`"
        :style="{ left: x + 'px', top: y + 'px' }"
        @click.stop
      >
        <button class="extract-menu-item" @click="emit('open')">
          🌐 {{ t('keyEditor.browserExtract') }}
        </button>
        <button v-if="showCopy" class="extract-menu-item" @click="emit('copy')">
          🔗 {{ t('keyEditor.copyLink') }}
        </button>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from '../../i18n/index.js'

const props = defineProps({
  visible: { type: Boolean, default: false },
  x: { type: Number, default: 0 },
  y: { type: Number, default: 0 },
  theme: { type: String, default: 'light' },
  showCopy: { type: Boolean, default: false },
})
const emit = defineEmits(['open', 'copy', 'close'])
const { t } = useI18n()

function onKeydown(e) {
  if (e.key === 'Escape' && props.visible) emit('close')
}
onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.extract-menu-backdrop {
  position: fixed;
  inset: 0;
  z-index: 3000;
}
.extract-menu {
  position: fixed;
  min-width: 140px;
  padding: 4px;
  background: #fff;
  border: 1px solid rgba(203, 213, 225, 0.9);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.18);
}
.extract-menu-item {
  display: block;
  width: 100%;
  padding: 6px 10px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  font-size: 13px;
  text-align: left;
  cursor: pointer;
  color: #374151;
  white-space: nowrap;
}
.extract-menu-item:hover {
  background: rgba(59, 130, 246, 0.1);
  color: #2563eb;
}
.extract-menu.theme-dark {
  background: rgba(30, 41, 59, 0.98);
  border-color: rgba(51, 65, 85, 0.9);
}
.extract-menu.theme-dark .extract-menu-item {
  color: #cbd5e1;
}
.extract-menu.theme-dark .extract-menu-item:hover {
  background: rgba(59, 130, 246, 0.2);
  color: #93c5fd;
}
</style>
