<template>
  <Teleport to="body">
    <div v-if="show" class="expand-overlay" @click.self="$emit('close')">
      <div class="expand-modal">
        <div class="expand-header">
          <span class="expand-title" :title="title">{{ title }}</span>
          <div class="expand-header-actions">
            <button class="btn-copy-modal" @click="copy">
              {{ copied ? '✓ 已复制' : '📋 复制' }}
            </button>
            <button class="btn-close-modal" @click="$emit('close')" title="关闭">✕</button>
          </div>
        </div>
        <div class="expand-body">
          <pre class="expand-content">{{ content }}</pre>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, watch } from 'vue'
import { copyToClipboard } from '../../utils/clipboard.js'

const props = defineProps({
  show:    { type: Boolean, default: false },
  title:   { type: String,  default: '查看值' },
  content: { type: String,  default: '' },
})
defineEmits(['close'])

const copied = ref(false)

watch(() => props.show, (v) => { if (!v) copied.value = false })

async function copy() {
  const ok = await copyToClipboard(props.content)
  if (ok) {
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  }
}
</script>

<style scoped>
.expand-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}
.expand-modal {
  background: white;
  border-radius: 10px;
  width: min(820px, 90vw);
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.22);
}
.expand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e7eb;
  background: #f9fafb;
  border-radius: 10px 10px 0 0;
  flex-shrink: 0;
}
.expand-title {
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
  color: #1d4ed8;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 600px;
}
.expand-header-actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.btn-copy-modal {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 4px 13px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  min-width: 74px;
  transition: background 0.12s, border-color 0.12s;
}
.btn-copy-modal:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-close-modal {
  display: inline-flex; align-items: center; justify-content: center;
  background: transparent;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 13px;
  cursor: pointer;
  color: #9ca3af;
  padding: 4px 9px;
  line-height: 1.4;
  transition: color 0.12s, border-color 0.12s;
}
.btn-close-modal:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; }
.expand-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}
.expand-content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.7;
  color: #1f2937;
}
</style>
