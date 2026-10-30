<template>
  <div ref="rootRef" class="json-tree">
    <div v-if="topPad" class="json-pad" :style="{ height: topPad + 'px' }" aria-hidden="true" />
    <div
      v-for="row in windowRows"
      :key="row.path"
      class="json-row"
      :class="{ 'is-wrap': wrap }"
      data-vrow
      :data-vkey="row.path"
      :data-vindex="row.index"
      @click="toggleRow(row)"
      @contextmenu.stop.prevent="openNodeMenu($event, row)"
    >
      <span v-if="row.isContainer" class="json-caret">{{ expandedPaths.has(row.path) ? '▾' : '▸' }}</span>
      <span v-else class="json-caret placeholder" />
      <span v-if="row.label !== null" class="json-key">{{ row.label }}</span>
      <span v-if="row.label !== null" class="json-sep">:</span>
      <span v-if="!row.isContainer" class="json-value" :class="valueClass(row.value)">{{ displayValue(row.value) }}</span>
      <span v-else class="json-summary">{{ summary(row) }}</span>
    </div>
    <div v-if="bottomPad" class="json-pad" :style="{ height: bottomPad + 'px' }" aria-hidden="true" />

    <Teleport to="body">
      <div
        v-if="menu.visible"
        class="json-menu pop-in"
        :class="`theme-${theme}`"
        :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
        @mousedown.stop
        @click.stop
        @contextmenu.prevent
      >
        <button class="json-menu-item" @click="copyMenu('key')">{{ t('keyEditor.jsonCopyKey') }}</button>
        <button class="json-menu-item" @click="copyMenu('value')">{{ t('keyEditor.jsonCopyValue') }}</button>
      </div>
      <div
        v-if="tip"
        class="json-copy-tip"
        :class="`theme-${theme}`"
        :style="{ left: tipX + 'px', top: tipY + 'px' }"
      >{{ tip }}</div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { copyToClipboard } from '../../utils/clipboard.js'
import { useI18n } from '../../i18n/index.js'
import { useSettingsStore } from '../../stores/settings.js'
import { flattenJsonRows } from '../../utils/jsonTree.js'
import { useVariableVirtualizer } from '../../utils/virtualList.js'

const props = defineProps({
  data: { type: null, default: null },
  wrap: { type: Boolean, default: false },
})

const { t } = useI18n()
const settingsStore = useSettingsStore()
const theme = computed(() => settingsStore.themeMode || 'light')

// 展开状态按路径记；根节点默认展开，其余默认折叠，避免大 JSON 一屏铺满
const expandedPaths = ref(new Set(['']))
watch(
  () => props.data,
  () => {
    expandedPaths.value = new Set([''])
  }
)

const rows = computed(() => flattenJsonRows(props.data, (path) => expandedPaths.value.has(path)))

// 大 JSON 只渲染可视区附近的行；换行模式下行高不固定，用变高窗口
const rootRef = ref(null)
const virtualizer = useVariableVirtualizer({
  containerRef: rootRef,
  items: rows,
  keyOf: (row) => row.path,
  resetToken: () => props.data,
  estimate: () => (props.wrap ? 44 : 22),
  overscan: 10,
  threshold: 200,
})
// 换行开关会改变每行高度，量一量让窗口重新对齐
watch(
  () => props.wrap,
  () => {
    nextTick(() => virtualizer.measureRows())
  }
)

const topPad = computed(() => virtualizer.topPad.value)
const bottomPad = computed(() => virtualizer.bottomPad.value)
const windowRows = computed(() => {
  const list = rows.value
  if (!virtualizer.active.value) {
    return list.map((row, index) => ({ ...row, index }))
  }
  const start = virtualizer.start.value
  const end = virtualizer.end.value
  const out = []
  for (let i = start; i < end; i++) {
    out.push({ ...list[i], index: i })
  }
  return out
})

function toggleRow(row) {
  if (!row.isContainer) return
  const next = new Set(expandedPaths.value)
  if (next.has(row.path)) next.delete(row.path)
  else next.add(row.path)
  expandedPaths.value = next
}

function summary(row) {
  const value = row.value
  const isArray = Array.isArray(value)
  const count = isArray ? value.length : Object.keys(value).length
  return (isArray ? '[' : '{') + ' ' + count + ' 项 ' + (isArray ? ']' : '}')
}

function valueClass(value) {
  if (value === null) return 'null'
  if (typeof value === 'string') return 'string'
  if (typeof value === 'number') return 'number'
  if (typeof value === 'boolean') return 'boolean'
  return 'string'
}

function displayValue(value) {
  if (value === null) return 'null'
  if (typeof value === 'string') return '"' + value + '"'
  return String(value)
}

// ---- 右键菜单：复制 key / value ----
const menu = reactive({ visible: false, x: 0, y: 0, key: '', value: '' })
const tip = ref('')
const tipX = ref(0)
const tipY = ref(0)
let tipTimer = null

function valueText(value) {
  if (value === undefined) return ''
  if (value === null) return 'null'
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value, null, 2)
    } catch (e) {
      return String(value)
    }
  }
  return String(value)
}

function openNodeMenu(event, row) {
  if (row.label === null || row.label === undefined) return
  menu.key = String(row.label)
  menu.value = valueText(row.value)
  const width = 150
  const height = 84
  menu.x = Math.max(8, Math.min(event.clientX, window.innerWidth - width - 8))
  menu.y = Math.max(8, Math.min(event.clientY, window.innerHeight - height - 8))
  menu.visible = true
}

function showTip(text) {
  tip.value = text
  tipX.value = menu.x
  tipY.value = menu.y
  if (tipTimer) clearTimeout(tipTimer)
  tipTimer = setTimeout(() => {
    tip.value = ''
    tipTimer = null
  }, 1300)
}

async function copyMenu(kind) {
  const text = kind === 'key' ? menu.key : menu.value
  menu.visible = false
  const ok = await copyToClipboard(text)
  if (ok) showTip(t('keyEditor.jsonCopied'))
}

function onDocumentMousedown(event) {
  if (!menu.visible) return
  const el = event.target
  if (el && el.closest && el.closest('.json-menu')) return
  menu.visible = false
}

function onDocumentKeydown(event) {
  if (event.key === 'Escape' && menu.visible) {
    menu.visible = false
  }
}

onMounted(() => {
  document.addEventListener('mousedown', onDocumentMousedown, true)
  document.addEventListener('keydown', onDocumentKeydown, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocumentMousedown, true)
  document.removeEventListener('keydown', onDocumentKeydown, true)
  if (tipTimer) clearTimeout(tipTimer)
})
</script>

<style scoped>
/* 自己就是滚动容器，窗口化才能拿到准确的视口高度 */
.json-tree {
  flex: 1;
  min-height: 0;
  overflow: auto;
  position: relative;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.7;
}
.json-pad {
  pointer-events: none;
}
.json-row {
  display: flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
  cursor: default;
  padding: 0 4px;
  border-radius: var(--radius-xs);
}
.json-row:hover {
  background: rgba(148, 163, 184, 0.12);
}
/* 自动换行：整行改为文本流，只折 value，key 名称保持完整不拆 */
.json-row.is-wrap {
  display: block;
  white-space: normal;
}
.json-row.is-wrap .json-caret {
  display: inline-block;
  width: 10px;
  margin-right: 2px;
}
.json-row.is-wrap .json-sep {
  margin-right: 3px;
}
.json-row.is-wrap .json-key {
  white-space: nowrap;
}
.json-row.is-wrap .json-value {
  white-space: pre-wrap;
  word-break: break-all;
  overflow-wrap: anywhere;
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

/* 右键菜单 */
.json-menu {
  position: fixed;
  z-index: 10060;
  min-width: 132px;
  padding: 5px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  border-radius: 10px;
  border: 1px solid rgba(203, 213, 225, 0.94);
  background: #fff;
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.18), 0 2px 6px rgba(15, 23, 42, 0.06);
}
.json-menu-item {
  text-align: left;
  padding: 7px 10px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: #334155;
  font-size: 11.5px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  white-space: nowrap;
  transition: background var(--motion-fast), color var(--motion-fast);
}
.json-menu-item:hover {
  background: rgba(239, 246, 255, 0.94);
  color: #1d4ed8;
}
.json-menu.theme-dark {
  background: rgba(15, 23, 42, 0.98);
  border-color: rgba(51, 65, 85, 0.94);
  box-shadow: 0 14px 30px rgba(2, 6, 23, 0.5), 0 2px 6px rgba(2, 6, 23, 0.3);
}
.json-menu.theme-dark .json-menu-item {
  color: #cbd5e1;
}
.json-menu.theme-dark .json-menu-item:hover {
  background: rgba(30, 41, 59, 0.96);
  color: #dbeafe;
}
.json-copy-tip {
  position: fixed;
  z-index: 10061;
  padding: 5px 9px;
  border-radius: var(--radius-md);
  font-size: 11px;
  line-height: 1;
  color: #15803d;
  background: rgba(240, 253, 244, 0.98);
  border: 1px solid rgba(134, 239, 172, 0.9);
  box-shadow: 0 8px 18px rgba(22, 163, 74, 0.14);
  animation: jsonTipIn 160ms ease;
  pointer-events: none;
}
.json-copy-tip.theme-dark {
  color: #6ee7b7;
  background: rgba(6, 78, 59, 0.96);
  border-color: rgba(16, 185, 129, 0.5);
}
@keyframes jsonTipIn {
  from { opacity: 0; transform: translateY(-3px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
