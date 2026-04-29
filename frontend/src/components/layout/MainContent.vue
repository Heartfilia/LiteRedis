<template>
  <div ref="mainContentRef" class="main-content">
    <div class="key-tree-panel" :style="{ width: panelWidth + 'px' }">
      <KeyTree />
    </div>
    <div class="resizer" @mousedown="startResize" />
    <div class="key-editor-panel">
      <KeyEditor />
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import KeyTree from '../keys/KeyTree.vue'
import KeyEditor from '../editor/KeyEditor.vue'

const MIN_WIDTH = 180
const MAX_WIDTH = 600
const DEFAULT_WIDTH = 320
const RESIZER_WIDTH = 7
const MIN_EDITOR_WIDTH = 820

const panelWidth = ref(DEFAULT_WIDTH)
const mainContentRef = ref(null)

function getPanelBounds() {
  const containerWidth = mainContentRef.value?.clientWidth || 0
  const dynamicMax = containerWidth > 0
    ? Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, containerWidth - MIN_EDITOR_WIDTH - RESIZER_WIDTH))
    : MAX_WIDTH
  return {
    min: MIN_WIDTH,
    max: dynamicMax,
  }
}

function startResize(e) {
  const startX = e.clientX
  const startWidth = panelWidth.value

  function onMouseMove(ev) {
    const delta = ev.clientX - startX
    const bounds = getPanelBounds()
    const newWidth = Math.max(bounds.min, Math.min(bounds.max, startWidth + delta))
    panelWidth.value = newWidth
  }

  function onMouseUp() {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }

  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}
</script>

<style scoped>
.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
  height: 100vh;
  min-width: 0;
}
.key-tree-panel {
  min-width: 180px;
  max-width: 600px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-right: 1px solid #e0e0e0;
  flex: 0 1 auto;
}
.resizer {
  width: 7px;
  cursor: col-resize;
  background: #f3f4f6;
  border-left: 1px solid #e5e7eb;
  border-right: 1px solid #e5e7eb;
  flex-shrink: 0;
  transition: background 0.15s, border-color 0.15s;
  z-index: 10;
}
.resizer:hover,
.resizer:active {
  background: #3b82f6;
  border-color: #3b82f6;
}
.key-editor-panel {
  flex: 1;
  min-width: 820px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
</style>
