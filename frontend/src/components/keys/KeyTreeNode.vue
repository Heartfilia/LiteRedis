<template>
  <div :class="['tree-node', { expanded: isExpanded }]">
    <!-- 目录节点 -->
    <div v-if="!node.isLeaf" class="node-row dir-row" @click="toggle">
      <span class="expand-icon">{{ isExpanded ? '▼' : '▶' }}</span>
      <span class="folder-icon">📁</span>
      <span class="node-label">{{ node.label }}</span>
      <span class="node-count">({{ node.count }})</span>
    </div>

    <!-- 叶节点（key） -->
    <div
      v-else
      :class="['node-row', 'leaf-row', { selected: selectedKey === node.fullPath }]"
      @click="selectKey(node.fullPath)"
    >
      <span class="leaf-indent" />
      <span
        class="type-dot"
        :style="{ background: getTypeColor(node.keyType).dot }"
        :title="node.keyType"
      />
      <span class="node-label leaf-label">{{ node.label }}</span>
      <span class="type-badge" :style="{ background: getTypeColor(node.keyType).bg, color: getTypeColor(node.keyType).text }">
        {{ getTypeColor(node.keyType).label }}
      </span>
      <span v-if="node.ttl > 0" class="ttl-badge">{{ node.ttl }}s</span>
    </div>

    <!-- 子节点递归 -->
    <div v-if="!node.isLeaf && isExpanded" class="children">
      <KeyTreeNode
        v-for="child in node.children"
        :key="child.fullPath"
        :node="child"
        :depth="depth + 1"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { getTypeColor } from '../../utils/typeColors.js'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
})

const workspaceStore = useWorkspaceStore()
const selectedKey = computed(() => workspaceStore.selectedKey)

const isExpanded = ref(props.depth < 1 || workspaceStore.keepPrevSearch) // 第一层默认展开，保留搜索模式全部展开

watch(() => workspaceStore.keepPrevSearch, (val) => {
  if (val) isExpanded.value = true
})

function toggle() {
  isExpanded.value = !isExpanded.value
}

function selectKey(fullPath) {
  workspaceStore.selectKey(fullPath)
}
</script>

<style scoped>
.tree-node { user-select: none; }
.node-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  cursor: pointer;
  border-radius: 3px;
  font-size: 13px;
  min-height: 24px;
}
.node-row:hover { background: #f0f4ff; }
.node-row.selected { background: #dbeafe; color: #1d4ed8; }
.dir-row { color: #444; }
.expand-icon { font-size: 8px; color: #999; width: 12px; flex-shrink: 0; }
.folder-icon { font-size: 12px; }
.node-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.leaf-label { font-family: monospace; font-size: 12px; }
.node-count { font-size: 11px; color: #999; flex-shrink: 0; }
.leaf-indent { width: 20px; flex-shrink: 0; }
.type-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.type-badge {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
  flex-shrink: 0;
  font-weight: 500;
}
.ttl-badge {
  font-size: 10px;
  color: #f59e0b;
  flex-shrink: 0;
}
.children { padding-left: 16px; }
</style>
