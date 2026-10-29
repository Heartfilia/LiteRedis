<template>
  <div class="json-viewer" :class="`theme-${theme}`">
    <div class="json-viewer-bar">
      <button
        class="json-wrap-btn"
        :class="{ active: wrap }"
        :title="t('keyEditor.jsonAutoWrap')"
        @click="wrap = !wrap"
      >
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M4 6.5h16" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
          <path d="M4 12.5h11a3.5 3.5 0 0 1 0 7H9" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" />
          <path d="M11 16.5 8 19.5l3 3" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span>{{ t('keyEditor.jsonAutoWrap') }}</span>
      </button>
    </div>
    <div class="json-viewer-body">
      <JsonTree :data="data" :depth="0" :wrap="wrap" />
    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useI18n } from '../../i18n/index.js'
import JsonTree from './JsonTree.vue'

defineProps({
  data: { type: null, default: null },
})

const settingsStore = useSettingsStore()
const { t } = useI18n()
const theme = computed(() => settingsStore.themeMode || 'light')
// 默认不换行，长内容横向滚动；打开后按面板宽度折行
const wrap = ref(false)
</script>

<style scoped>
.json-viewer {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-md);
  background: rgba(255, 255, 255, 0.9);
  overflow: hidden;
  --json-key: #1d4ed8;
  --json-sep: #94a3b8;
  --json-string: #15803d;
  --json-number: #b45309;
  --json-boolean: #a21caf;
  --json-null: #94a3b8;
  --json-summary: #64748b;
}
.json-viewer.theme-dark {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.75);
  --json-key: #93c5fd;
  --json-sep: #64748b;
  --json-string: #86efac;
  --json-number: #fcd34d;
  --json-boolean: #f0abfc;
  --json-null: #94a3b8;
  --json-summary: #94a3b8;
}
.json-viewer-bar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  padding: 6px 8px 2px;
  flex-shrink: 0;
}
.json-wrap-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 22px;
  padding: 0 8px;
  border: 1px solid rgba(203, 213, 225, 0.96);
  border-radius: var(--radius-sm);
  background: rgba(255, 255, 255, 0.94);
  color: #64748b;
  font-size: 10.5px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  transition: background var(--motion-fast), border-color var(--motion-fast), color var(--motion-fast);
}
.json-wrap-btn svg {
  width: 12px;
  height: 12px;
  display: block;
}
.json-wrap-btn:hover {
  background: #f8fafc;
  border-color: #94a3b8;
  color: #1e293b;
}
.json-wrap-btn.active {
  background: rgba(239, 246, 255, 0.96);
  border-color: rgba(147, 197, 253, 0.96);
  color: #1d4ed8;
}
.json-viewer.theme-dark .json-wrap-btn {
  background: rgba(15, 23, 42, 0.94);
  border-color: rgba(71, 85, 105, 0.96);
  color: #94a3b8;
}
.json-viewer.theme-dark .json-wrap-btn:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: #60a5fa;
  color: #e2e8f0;
}
.json-viewer.theme-dark .json-wrap-btn.active {
  background: rgba(30, 64, 175, 0.3);
  border-color: rgba(96, 165, 250, 0.5);
  color: #dbeafe;
}
.json-viewer-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 4px 8px 10px;
}
</style>
