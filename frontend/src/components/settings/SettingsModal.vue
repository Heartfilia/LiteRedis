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
  background: rgba(0, 0, 0, 0.55);
  display: flex;
  align-items: center;
  justify-content: center;
}
.settings-card {
  width: 520px;
  max-width: 95vw;
  max-height: 86vh;
  overflow-y: auto;
  border-radius: 12px;
  background: white;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}
</style>
