<template>
  <div class="main-content">
    <div class="key-tree-panel">
      <KeyTree />
    </div>
    <div class="key-editor-panel">
      <KeyEditor />
    </div>
  </div>
</template>

<script setup>
import KeyTree from '../keys/KeyTree.vue'
import KeyEditor from '../editor/KeyEditor.vue'
</script>

<style scoped>
.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
  height: 100vh;
}
.key-tree-panel {
  width: 320px;
  min-width: 220px;
  max-width: 500px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-right: 1px solid #e0e0e0;
}
.key-editor-panel {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
