<template>
  <div class="json-tree">
    <div class="json-row" :style="{ paddingLeft: depth * 14 + 'px' }" @click="toggle">
      <span v-if="isContainer" class="json-caret">{{ collapsed ? '▸' : '▾' }}</span>
      <span v-else class="json-caret placeholder" />
      <span v-if="label !== null" class="json-key">{{ label }}</span>
      <span v-if="label !== null" class="json-sep">:</span>
      <span v-if="!isContainer" class="json-value" :class="valueClass">{{ displayValue }}</span>
      <span v-else class="json-summary">{{ summary }}</span>
    </div>
    <div v-if="isContainer && !collapsed">
      <JsonTree
        v-for="child in children"
        :key="child.key"
        :data="child.value"
        :label="child.label"
        :depth="depth + 1"
      />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  data: { type: null, default: null },
  label: { type: [String, Number], default: null },
  depth: { type: Number, default: 0 },
})

const isArray = computed(() => Array.isArray(props.data))
const isContainer = computed(() => props.data !== null && typeof props.data === 'object')

const children = computed(() => {
  if (!isContainer.value) return []
  if (isArray.value) {
    return props.data.map((value, index) => ({ key: String(index), label: index, value }))
  }
  return Object.keys(props.data).map(key => ({ key, label: key, value: props.data[key] }))
})

// 第一层展开，更深的默认折叠，避免大 JSON 一屏铺满
const collapsed = ref(props.depth >= 1)
watch(() => props.data, () => {
  collapsed.value = props.depth >= 1
})

const summary = computed(() => {
  if (!isContainer.value) return ''
  const count = children.value.length
  return (isArray.value ? '[' : '{') + ' ' + count + ' 项 ' + (isArray.value ? ']' : '}')
})

const valueClass = computed(() => {
  const value = props.data
  if (value === null) return 'null'
  if (typeof value === 'string') return 'string'
  if (typeof value === 'number') return 'number'
  if (typeof value === 'boolean') return 'boolean'
  return 'string'
})

const displayValue = computed(() => {
  const value = props.data
  if (value === null) return 'null'
  if (typeof value === 'string') {
    const text = value.length > 200 ? value.slice(0, 200) + '…' : value
    return '"' + text + '"'
  }
  return String(value)
})

function toggle() {
  if (!isContainer.value) return
  collapsed.value = !collapsed.value
}
</script>

<style scoped>
.json-tree {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.7;
}
.json-row {
  display: flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
  cursor: default;
}
.json-row:hover {
  background: rgba(148, 163, 184, 0.12);
  border-radius: 4px;
}
.json-caret {
  width: 10px;
  flex: 0 0 10px;
  color: #94a3b8;
  cursor: pointer;
  font-size: 10px;
}
.json-caret.placeholder {
  cursor: default;
}
.json-key {
  color: var(--json-key, #1d4ed8);
}
.json-sep {
  color: var(--json-sep, #94a3b8);
}
.json-value.string { color: var(--json-string, #15803d); }
.json-value.number { color: var(--json-number, #b45309); }
.json-value.boolean { color: var(--json-boolean, #a21caf); }
.json-value.null { color: var(--json-null, #94a3b8); }
.json-summary {
  color: var(--json-summary, #64748b);
  cursor: pointer;
}
</style>
