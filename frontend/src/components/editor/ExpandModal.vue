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
  border-radius: 8px;
  width: min(820px, 90vw);
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.22);
}
.expand-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #eee;
  background: #fafafa;
  border-radius: 8px 8px 0 0;
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
  padding: 4px 12px;
  border: 1px solid #ddd;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  background: white;
  color: #333;
  min-width: 72px;
}
.btn-copy-modal:hover { background: #f0f0f0; }
.btn-close-modal {
  background: transparent;
  border: 1px solid #eee;
  border-radius: 4px;
  font-size: 14px;
  cursor: pointer;
  color: #999;
  padding: 2px 8px;
  line-height: 1.4;
}
.btn-close-modal:hover { color: #e53e3e; border-color: #e53e3e; }
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
  color: #333;
}
</style>
