<template>
  <div class="json-tree">
    <div class="json-row" :class="{ 'is-wrap': wrap }" @click="toggle" @contextmenu.stop.prevent="openNodeMenu">
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
        :wrap="wrap"
      />
    </div>

    <Teleport v-if="isRoot" to="body">
      <div
        v-if="menu.visible"
        class="json-menu"
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
import { computed, inject, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from 'vue'
import { copyToClipboard } from '../../utils/clipboard.js'
import { useI18n } from '../../i18n/index.js'
import { useSettingsStore } from '../../stores/settings.js'

const props = defineProps({
  data: { type: null, default: null },
  label: { type: [String, Number], default: null },
  depth: { type: Number, default: 0 },
  wrap: { type: Boolean, default: false },
})

const { t } = useI18n()
const settingsStore = useSettingsStore()
const theme = computed(() => settingsStore.themeMode || 'light')

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
  if (typeof value === 'string') return '"' + value + '"'
  return String(value)
})

function toggle() {
  if (!isContainer.value) return
  collapsed.value = !collapsed.value
}

// ---- 右键菜单：只有根节点渲染菜单，子节点共用同一份状态 ----
const injectedMenu = inject('jsonTreeMenu', null)
const isRoot = injectedMenu === null

const menu = reactive({ visible: false, x: 0, y: 0, key: '', value: '' })
const tip = ref('')
const tipX = ref(0)
const tipY = ref(0)
let tipTimer = null

function valueText(value) {
  if (value === undefined) return ''
  if (value === null) return 'null'
  if (typeof value === 'object') {
    try { return JSON.stringify(value, null, 2) } catch (e) { return String(value) }
  }
  return String(value)
}

function openMenu(event, key, value) {
  if (key === null || key === undefined) return
  menu.key = String(key)
  menu.value = valueText(value)
  const width = 150
  const height = 84
  menu.x = Math.max(8, Math.min(event.clientX, window.innerWidth - width - 8))
  menu.y = Math.max(8, Math.min(event.clientY, window.innerHeight - height - 8))
  menu.visible = true
}

const menuApi = { open: openMenu }
if (isRoot) provide('jsonTreeMenu', menuApi)

const nodeMenu = isRoot ? menuApi : injectedMenu

function openNodeMenu(event) {
  nodeMenu.open(event, props.label, props.data)
}

function closeMenu() {
  menu.visible = false
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
    event.stopPropagation()
  }
}

onMounted(() => {
  if (!isRoot) return
  document.addEventListener('mousedown', onDocumentMousedown, true)
  document.addEventListener('keydown', onDocumentKeydown, true)
})

onBeforeUnmount(() => {
  if (!isRoot) return
  document.removeEventListener('mousedown', onDocumentMousedown, true)
  document.removeEventListener('keydown', onDocumentKeydown, true)
  if (tipTimer) clearTimeout(tipTimer)
})
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
  padding: 0 4px;
  border-radius: 4px;
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
  border-radius: 7px;
  background: transparent;
  color: #334155;
  font-size: 11.5px;
  font-weight: 600;
  line-height: 1;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.12s, color 0.12s;
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
  border-radius: 8px;
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
