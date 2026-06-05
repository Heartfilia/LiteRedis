<template>
  <Teleport to="body">
    <div class="settings-backdrop" :class="`theme-${settingsStore.themeMode || 'light'}`">
      <div class="settings-card">
        <SettingsPanel @close="$emit('close')" />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { useSettingsStore } from '../../stores/settings.js'
import SettingsPanel from './SettingsPanel.vue'
defineEmits(['close'])
const settingsStore = useSettingsStore()
</script>

<style scoped>
.settings-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background:
    radial-gradient(circle at top, rgba(255, 255, 255, 0.16), transparent 30%),
    rgba(15, 23, 42, 0.42);
  backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
}
.settings-card {
  width: 520px;
  max-width: 95vw;
  max-height: 86vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(226, 232, 240, 0.82);
  box-shadow: 0 32px 84px rgba(15, 23, 42, 0.22), 0 10px 24px rgba(148, 163, 184, 0.08);
  backdrop-filter: blur(18px);
}
:global(body[data-theme='dark']) .settings-card {
  background: rgba(15, 23, 42, 0.92);
  border-color: rgba(51, 65, 85, 0.82);
  box-shadow: 0 34px 86px rgba(2, 6, 23, 0.52), 0 12px 28px rgba(2, 6, 23, 0.22);
}
.settings-backdrop.theme-dark .settings-card {
  background: rgba(15, 23, 42, 0.92);
  border-color: rgba(51, 65, 85, 0.82);
  box-shadow: 0 34px 86px rgba(2, 6, 23, 0.52), 0 12px 28px rgba(2, 6, 23, 0.22);
}
</style>
