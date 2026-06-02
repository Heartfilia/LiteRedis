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
      <span v-if="node.ttl > 0" class="ttl-badge" title="TTL">T</span>
      <span class="type-badge" :style="{ background: getTypeColor(node.keyType).bg, color: getTypeColor(node.keyType).text }">
        {{ getTypeColor(node.keyType).label }}
      </span>
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
import { computed } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { getTypeColor } from '../../utils/typeColors.js'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
})

const workspaceStore = useWorkspaceStore()
const selectedKey = computed(() => workspaceStore.selectedKey)
const isExpanded = computed(() => workspaceStore.isNodeExpanded(props.node.fullPath, props.depth))

function toggle() {
  workspaceStore.setNodeExpanded(props.node.fullPath, !isExpanded.value)
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
  gap: 6px;
  padding: 0 10px;
  cursor: pointer;
  border-radius: 12px;
  font-size: 13px;
  min-height: 32px;
  color: #334155;
  transition: background 0.16s ease, color 0.16s ease, transform 0.16s ease, box-shadow 0.16s ease;
}
.node-row:hover {
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.92), rgba(241, 245, 249, 0.92));
  color: #0f172a;
}
.node-row.selected {
  background: linear-gradient(180deg, rgba(239, 246, 255, 0.96), rgba(219, 234, 254, 0.92));
  color: #1d4ed8;
  box-shadow: inset 0 0 0 1px rgba(147, 197, 253, 0.7);
}
.dir-row { color: #475569; }
.expand-icon {
  font-size: 9px;
  color: #94a3b8;
  width: 12px;
  flex-shrink: 0;
}
.folder-icon {
  font-size: 12px;
  filter: saturate(0.8);
}
.node-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.leaf-label {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}
.node-count {
  font-size: 11px;
  color: #94a3b8;
  flex-shrink: 0;
}
.leaf-indent { width: 16px; flex-shrink: 0; }
.type-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  box-shadow: 0 0 0 4px rgba(255, 255, 255, 0.9);
}
.type-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 999px;
  flex-shrink: 0;
  font-weight: 700;
  letter-spacing: 0.03em;
}
.ttl-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 999px;
  background: rgba(254, 243, 199, 0.95);
  color: #b45309;
  font-weight: 600;
  flex-shrink: 0;
}
.children {
  padding-left: 14px;
  margin-left: 9px;
  border-left: 1px dashed rgba(203, 213, 225, 0.72);
}
</style>
