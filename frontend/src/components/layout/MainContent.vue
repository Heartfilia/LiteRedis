<template>
  <div ref="mainContentRef" class="main-content">
    <div class="key-tree-panel" :style="{ width: panelWidth + 'px' }">
      <KeyTree />
    </div>
    <div class="resizer" @mousedown="startResize" />
    <div class="key-editor-panel">
      <div v-if="watermarkEnabled && watermarkText" class="editor-watermark-layer" :style="watermarkLayerStyle">
        <div
          v-for="n in watermarkItems"
          :key="n"
          class="editor-watermark-item"
          :style="watermarkItemStyle"
        >
          {{ watermarkText }}
        </div>
      </div>
      <KeyEditor />
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import KeyTree from '../keys/KeyTree.vue'
import KeyEditor from '../editor/KeyEditor.vue'
import { useSettingsStore } from '../../stores/settings.js'

const MIN_WIDTH = 250
const MAX_WIDTH = 600
const DEFAULT_WIDTH = 320
const RESIZER_WIDTH = 7
const MIN_EDITOR_WIDTH = 760

const panelWidth = ref(DEFAULT_WIDTH)
const mainContentRef = ref(null)
const settingsStore = useSettingsStore()
const watermarkItems = computed(() => {
  const density = Math.min(5, Math.max(1, Number(settingsStore.watermarkDensity) || 3))
  return Array.from({ length: density * density * 2 }, (_, index) => index + 1)
})

const watermarkEnabled = computed(() => !!settingsStore.watermarkEnabled)
const watermarkText = computed(() => settingsStore.watermarkText || '')
const watermarkLayerStyle = computed(() => {
  const density = Math.min(5, Math.max(1, Number(settingsStore.watermarkDensity) || 3))
  const columns = density + 1
  const gap = 220 - (density - 1) * 38
  return {
    '--wm-columns': `${columns}`,
    '--wm-gap': `${Math.max(90, gap)}px`,
  }
})
const watermarkItemStyle = computed(() => ({
  '--wm-size': `${Math.min(48, Math.max(10, Number(settingsStore.watermarkSize) || 16))}px`,
  '--wm-angle': `${Math.min(90, Math.max(-90, Number(settingsStore.watermarkAngle) || -22))}deg`,
  '--wm-opacity': `${Math.min(100, Math.max(1, Number(settingsStore.watermarkOpacity) || 12)) / 100}`,
}))

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
  min-width: 250px;
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
  min-width: 760px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  position: relative;
}
.editor-watermark-layer {
  position: absolute;
  inset: 0;
  z-index: 12;
  pointer-events: none;
  display: grid;
  grid-template-columns: repeat(var(--wm-columns), minmax(0, 1fr));
  grid-auto-rows: minmax(96px, 1fr);
  gap: var(--wm-gap);
  padding: 18px;
  overflow: hidden;
}
.editor-watermark-item {
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  font-size: var(--wm-size);
  color: rgba(71, 85, 105, var(--wm-opacity));
  transform: rotate(var(--wm-angle));
  user-select: none;
  white-space: nowrap;
}
.key-editor-panel > :not(.editor-watermark-layer) {
  position: relative;
  z-index: 1;
}
</style>
