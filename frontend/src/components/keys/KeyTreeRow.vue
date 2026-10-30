<template>
  <div
    class="tree-row"
    :class="`theme-${settingsStore.themeMode || 'light'}`"
    :style="{ paddingLeft: depth * 24 + 'px' }"
    data-vrow
    :data-row-index="index"
  >
    <span
      v-for="level in depth"
      :key="level"
      class="row-guide"
      :style="{ left: (level - 1) * 24 + 'px' }"
      aria-hidden="true"
    />

    <!-- 目录节点 -->
    <div
      v-if="!node.isLeaf"
      v-kbd-row="{ role: 'treeitem' }"
      class="node-row dir-row"
      :class="{ expanded: isExpanded }"
      :aria-expanded="isExpanded"
      @click="onDirClick"
    >
      <input
        v-if="selectionMode"
        class="leaf-check"
        type="checkbox"
        :checked="dirState === 'all'"
        :indeterminate="dirState === 'partial'"
        @click.stop
        @change="toggleDirSelection"
      />
      <span class="expand-icon" aria-hidden="true" @click.stop="toggle">
        <svg v-if="isExpanded" viewBox="0 0 16 16" fill="none">
          <path d="M3.5 6l4.5 4.5L12.5 6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <svg v-else viewBox="0 0 16 16" fill="none">
          <path d="M6 3.5L10.5 8 6 12.5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </span>
      <span class="folder-icon" aria-hidden="true">
        <svg viewBox="0 0 20 20" fill="none">
          <path d="M2.5 5.5a2 2 0 012-2H8l1.5 2H15.5a2 2 0 012 2v7a2 2 0 01-2 2h-11a2 2 0 01-2-2v-9z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
          <path d="M2.5 7.5h15" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
      </span>
      <span class="node-label">{{ node.label }}</span>
      <span class="node-count">({{ node.count }})</span>
    </div>

    <!-- 叶节点（key） -->
    <div
      v-else
      v-kbd-row="{ role: 'treeitem' }"
      :class="['node-row', 'leaf-row', { selected: selectedKey === node.fullPath, checked: isChecked }]"
      :aria-selected="selectedKey === node.fullPath"
      @click="onLeafClick(node.fullPath, $event)"
    >
      <input
        v-if="selectionMode"
        class="leaf-check"
        type="checkbox"
        :checked="isChecked"
        @click.stop
        @change="workspaceStore.toggleKeySelection(node.fullPath)"
      />
      <span class="leaf-indent" />
      <span class="node-label leaf-label">{{ node.label }}</span>
      <span v-if="node.ttl > 0" class="ttl-badge" title="TTL">T</span>
      <span class="type-badge" :style="{ background: getTypeColor(node.keyType).bg, color: getTypeColor(node.keyType).text }">
        {{ getTypeColor(node.keyType).label }}
      </span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useSettingsStore } from '../../stores/settings.js'
import { getTypeColor } from '../../utils/typeColors.js'

const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
  index: { type: Number, default: 0 },
})

const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const selectedKey = computed(() => workspaceStore.selectedKey)
const selectionMode = computed(() => workspaceStore.selectionMode)
const isChecked = computed(() => workspaceStore.selectedKeySet.has(props.node.fullPath))

// 目录节点下所有叶子 key，用于整目录勾选
const subtreeLeaves = computed(() => {
  const leaves = []
  const walk = (current) => {
    if (!current) return
    if (current.isLeaf) {
      leaves.push(current.fullPath)
      return
    }
    for (const child of current.children || []) walk(child)
  }
  walk(props.node)
  return leaves
})

const dirState = computed(() => {
  if (!selectionMode.value) return 'none'
  const leaves = subtreeLeaves.value
  if (!leaves.length) return 'none'
  const selected = workspaceStore.selectedKeySet
  let hit = 0
  for (const key of leaves) {
    if (selected.has(key)) hit++
  }
  if (hit === 0) return 'none'
  return hit === leaves.length ? 'all' : 'partial'
})
const isExpanded = computed(() => workspaceStore.isNodeExpanded(props.node.fullPath, props.depth))

function toggle() {
  workspaceStore.setNodeExpanded(props.node.fullPath, !isExpanded.value)
}

function selectKey(fullPath) {
  if (workspaceStore.selectionMode) {
    workspaceStore.toggleKeySelection(fullPath)
    return
  }
  workspaceStore.selectKey(fullPath)
}

function onDirClick() {
  if (workspaceStore.selectionMode) {
    toggleDirSelection()
    return
  }
  toggle()
}

function toggleDirSelection() {
  workspaceStore.toggleKeysSelection(subtreeLeaves.value)
}

function onLeafClick(fullPath, event) {
  if (workspaceStore.selectionMode) {
    // Shift 连选：从锚点到当前 key
    if (event && event.shiftKey) {
      workspaceStore.selectKeyRangeTo(fullPath)
      return
    }
    workspaceStore.toggleKeySelection(fullPath)
    return
  }
  selectKey(fullPath)
}
</script>

<style scoped>
.tree-row {
  position: relative;
}
/* 每层目录画一条虚线导轨，和原来嵌套 children 的边框观感一致 */
.row-guide {
  position: absolute;
  top: 0;
  bottom: 0;
  border-left: 1px dashed rgba(203, 213, 225, 0.72);
  pointer-events: none;
}
.tree-row.theme-dark .row-guide {
  border-left-color: rgba(71, 85, 105, 0.72);
}
.leaf-check {
  width: 13px;
  height: 13px;
  margin-right: 2px;
  accent-color: #4e9af1;
  cursor: pointer;
  flex-shrink: 0;
}
.theme-dark .leaf-check { accent-color: #60a5fa; }
.node-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  cursor: pointer;
  border-radius: var(--radius-lg);
  font-size: 13px;
  min-height: 32px;
  color: #27364a;
  transition: background var(--motion-base) ease, color var(--motion-base) ease, box-shadow var(--motion-base) ease;
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
  color: #94a3b8;
  width: 12px;
  height: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.expand-icon svg {
  width: 12px;
  height: 12px;
  display: block;
}
.folder-icon {
  width: 13px;
  height: 13px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #7c8da5;
  flex-shrink: 0;
}
.folder-icon svg {
  width: 13px;
  height: 13px;
  display: block;
}
.node-label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.leaf-label {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.01em;
}
.node-count {
  font-size: 11px;
  color: #94a3b8;
  flex-shrink: 0;
}
.leaf-indent { width: 16px; flex-shrink: 0; }
.type-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: var(--radius-pill);
  flex-shrink: 0;
  font-weight: 700;
  letter-spacing: 0.03em;
}
.ttl-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: var(--radius-pill);
  background: rgba(254, 243, 199, 0.95);
  color: #b45309;
  font-weight: 600;
  flex-shrink: 0;
}
.tree-row.theme-dark .node-row {
  color: #e5edf8;
}
.tree-row.theme-dark .node-row:hover {
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.92), rgba(15, 23, 42, 0.94));
  color: #f8fafc;
}
.tree-row.theme-dark .node-row.selected {
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.3), rgba(30, 64, 175, 0.2));
  color: #dbeafe;
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.46);
}
.tree-row.theme-dark .dir-row,
.tree-row.theme-dark .node-count,
.tree-row.theme-dark .expand-icon {
  color: #94a3b8;
}
.tree-row.theme-dark .ttl-badge {
  background: rgba(120, 53, 15, 0.42);
  color: #fdba74;
}
.tree-row {
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}
:global(.platform-windows) .tree-row .node-row {
  min-height: 31px;
}
:global(.platform-windows) .tree-row .leaf-label {
  letter-spacing: 0;
  font-weight: 600;
}
</style>
