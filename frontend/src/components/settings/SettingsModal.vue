<template>
  <Teleport to="body">
    <div class="settings-backdrop" @click.self="$emit('close')">
      <div class="settings-card">
        <SettingsPanel @close="$emit('close')" />
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import SettingsPanel from './SettingsPanel.vue'
defineEmits(['close'])
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
  border: 1px solid rgba(226, 232, 240, 0.9);
  box-shadow: 0 28px 80px rgba(15, 23, 42, 0.26);
  backdrop-filter: blur(18px);
}
</style>
