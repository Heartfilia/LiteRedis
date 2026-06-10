<template>
  <div ref="mainContentRef" class="main-content" :class="`theme-${settingsStore.themeMode || 'light'}`">
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

const MIN_WIDTH = 220
const MAX_WIDTH = 600
const DEFAULT_WIDTH = 320
const RESIZER_WIDTH = 2
const MIN_EDITOR_WIDTH = 700

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
  const safeEditorMin = containerWidth > 0
    ? Math.min(MIN_EDITOR_WIDTH, Math.max(420, Math.floor(containerWidth * 0.56)))
    : MIN_EDITOR_WIDTH
  const dynamicMax = containerWidth > 0
    ? Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, containerWidth - safeEditorMin - RESIZER_WIDTH))
    : MAX_WIDTH
  return {
    min: MIN_WIDTH,
    max: dynamicMax,
  }
}

function startResize(e) {
  e.preventDefault()
  const startX = e.clientX
  const startWidth = panelWidth.value
  const root = mainContentRef.value

  function onMouseMove(ev) {
    ev.preventDefault()
    const delta = ev.clientX - startX
    const bounds = getPanelBounds()
    const newWidth = Math.max(bounds.min, Math.min(bounds.max, startWidth + delta))
    panelWidth.value = newWidth
  }

  function onMouseUp() {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    document.body.style.cursor = ''
    document.body.classList.remove('is-resizing-panels')
    if (root) root.classList.remove('is-resizing')
  }

  document.body.style.cursor = 'col-resize'
  document.body.classList.add('is-resizing-panels')
  if (root) root.classList.add('is-resizing')
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
  padding: var(--app-shell-gap) var(--app-shell-gap) var(--app-shell-gap) var(--app-shell-inner-gap);
  gap: 0;
}
.main-content.is-resizing,
.main-content.is-resizing * {
  cursor: col-resize !important;
}
.main-content.is-resizing .key-tree-panel,
.main-content.is-resizing .resizer {
  user-select: none;
  -webkit-user-select: none;
}
.main-content.is-resizing .key-editor-panel::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 20;
  pointer-events: auto;
}
.key-tree-panel {
  min-width: 220px;
  max-width: 600px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex: 0 1 auto;
  user-select: none;
  -webkit-user-select: none;
  border: 1px solid var(--app-panel-border);
  border-radius: var(--app-panel-radius);
  background: var(--app-panel-bg);
  box-shadow: 0 14px 30px rgba(148, 163, 184, 0.16);
  backdrop-filter: blur(14px);
}
.resizer {
  width: 2px;
  cursor: col-resize;
  margin: 8px 0;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(203, 213, 225, 0.08), rgba(148, 163, 184, 0.34), rgba(203, 213, 225, 0.08));
  border: none;
  flex-shrink: 0;
  transition: background 0.15s, border-color 0.15s;
  z-index: 10;
  position: relative;
}
.resizer:hover,
.resizer:active {
  background: linear-gradient(180deg, rgba(96, 165, 250, 0.08), #60a5fa, rgba(96, 165, 250, 0.08));
}
.key-editor-panel {
  flex: 1;
  min-width: 700px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  position: relative;
  border: 1px solid var(--app-panel-border);
  border-radius: var(--app-panel-radius);
  background: var(--app-panel-bg);
  box-shadow: 0 18px 36px rgba(148, 163, 184, 0.14);
  backdrop-filter: blur(14px);
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
.main-content.theme-dark .key-tree-panel,
.main-content.theme-dark .key-editor-panel {
  backdrop-filter: none;
  box-shadow: 0 14px 30px rgba(2, 6, 23, 0.28);
}

.main-content.theme-dark .key-tree-panel {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.985), rgba(11, 18, 32, 0.995));
  border-color: rgba(51, 65, 85, 0.92);
}

.main-content.theme-dark .key-editor-panel {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.99), rgba(2, 6, 23, 0.995));
  border-color: rgba(51, 65, 85, 0.92);
}
</style>
