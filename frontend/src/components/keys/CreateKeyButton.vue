<template>
  <div ref="wrapRef" class="create-key-wrap" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <button class="create-key-btn" :title="t('keyTree.createKey')" @click="toggleOpen">
      <span>+</span>
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        class="create-key-popover"
        :class="`theme-${settingsStore.themeMode || 'light'}`"
        :style="popoverStyle"
      >
        <div class="popover-header">
          <span>{{ t('keyTree.createKey') }}</span>
          <button class="popover-close" @click="close">✕</button>
        </div>

        <div class="form-grid">
          <label>{{ t('keyTree.keyName') }}</label>
          <input v-model="form.key" type="text" :placeholder="t('keyTree.keyNamePlaceholder')" />

          <label>{{ t('keyTree.keyType') }}</label>
          <select v-model="form.type">
            <option value="string">string</option>
            <option value="hash">hash</option>
            <option value="list">list</option>
            <option value="set">set</option>
            <option value="zset">zset</option>
            <option value="stream">stream</option>
          </select>

          <label>{{ t('keyTree.ttlSeconds') }}</label>
          <input v-model.number="form.ttl" type="number" :placeholder="t('keyTree.ttlPlaceholder')" />
        </div>

        <div class="default-value-box">
          <template v-if="form.type === 'string'">
            <label>{{ t('keyTree.defaultValue') }}</label>
            <textarea v-model="form.stringValue" rows="3" />
          </template>

          <template v-else-if="form.type === 'hash'">
            <label>{{ t('keyTree.defaultField') }}</label>
            <input v-model="form.field" type="text" placeholder="field" />
            <label>{{ t('keyTree.defaultValue') }}</label>
            <textarea v-model="form.value" rows="3" />
          </template>

          <template v-else-if="form.type === 'list'">
            <label>{{ t('keyTree.defaultValue') }}</label>
            <textarea v-model="form.listValue" rows="3" :placeholder="t('keyTree.listDefaultHint')" />
          </template>

          <template v-else-if="form.type === 'set'">
            <label>{{ t('keyTree.defaultMember') }}</label>
            <input v-model="form.member" type="text" :placeholder="t('keyTree.defaultMember')" />
          </template>

          <template v-else-if="form.type === 'zset'">
            <label>{{ t('keyTree.defaultMember') }}</label>
            <input v-model="form.member" type="text" :placeholder="t('keyTree.defaultMember')" />
            <label>{{ t('keyTree.defaultScore') }}</label>
            <input v-model.number="form.score" type="number" step="any" />
          </template>

          <template v-else-if="form.type === 'stream'">
            <label>{{ t('keyTree.defaultField') }}</label>
            <input v-model="form.field" type="text" placeholder="field" />
            <label>{{ t('keyTree.defaultValue') }}</label>
            <textarea v-model="form.value" rows="3" />
          </template>
        </div>

        <div v-if="msg" :class="['create-msg', ok ? 'ok' : 'err']">{{ msg }}</div>

        <div class="popover-actions">
          <button class="btn-cancel" @click="close">{{ t('keyEditor.cancel') }}</button>
          <button class="btn-create" :disabled="saving" @click="submit">
            {{ saving ? '…' : t('keyTree.create') }}
          </button>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { reactive, ref, nextTick } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'

const workspaceStore = useWorkspaceStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()

const open = ref(false)
const saving = ref(false)
const msg = ref('')
const ok = ref(true)
const wrapRef = ref(null)
const popoverStyle = ref({})

const form = reactive(defaultForm())

function defaultForm() {
  return {
    key: '',
    type: 'string',
    ttl: -1,
    stringValue: '',
    field: 'field',
    value: '',
    listValue: '',
    member: '',
    score: 0,
  }
}

function resetForm() {
  Object.assign(form, defaultForm())
  msg.value = ''
  ok.value = true
}

function toggleOpen() {
  open.value = !open.value
  if (open.value) {
    resetForm()
    nextTick(() => {
      const rect = wrapRef.value?.getBoundingClientRect()
      if (rect) {
        popoverStyle.value = {
          top: `${rect.bottom + 6}px`,
          left: `${Math.max(8, rect.right - 300)}px`,
        }
      }
    })
  }
}

function close() {
  open.value = false
  msg.value = ''
}

function buildPayload() {
  return {
    key: form.key.trim(),
    type: form.type,
    ttl: Number.isFinite(form.ttl) ? form.ttl : -1,
    string_value: form.stringValue,
    field: form.field,
    value: form.value,
    list_value: form.listValue,
    member: form.member,
    score: Number.isFinite(form.score) ? form.score : 0,
  }
}

async function submit() {
  if (saving.value) return
  saving.value = true
  msg.value = ''
  try {
    const result = await workspaceStore.createKey(buildPayload())
    ok.value = !!result?.success
    msg.value = result?.success ? t('keyTree.createSuccess') : (result?.message || t('keyEditor.saveFailed'))
    if (result?.success) {
      close()
    }
  } catch (e) {
    ok.value = false
    msg.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.create-key-wrap {
  display: inline-flex;
  position: relative;
  flex-shrink: 0;
}
.create-key-btn {
  height: 30px;
  min-height: 30px;
  min-width: 30px;
  padding: 0;
  border-radius: 10px;
  border: 1px solid rgba(191, 219, 254, 0.92);
  background: linear-gradient(180deg, #ffffff, #f8fbff);
  color: #2563eb;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  box-sizing: border-box;
  box-shadow: 0 8px 16px rgba(191, 219, 254, 0.16);
  transition: all 0.16s ease;
}
.create-key-btn span {
  font-size: 16px;
  font-weight: 700;
  line-height: 1;
  transform: translateY(-1px);
}
.create-key-btn:hover {
  color: #1d4ed8;
  border-color: #93c5fd;
  background: linear-gradient(180deg, #f8fbff, #eff6ff);
  box-shadow: 0 10px 18px rgba(191, 219, 254, 0.22);
  transform: translateY(-1px);
}
.create-key-popover {
  position: fixed;
  z-index: 10000;
  width: 300px;
  padding: 12px;
  border: 1px solid rgba(226, 232, 240, 0.84);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 24px 48px rgba(15, 23, 42, 0.16), 0 8px 18px rgba(148, 163, 184, 0.08);
  backdrop-filter: blur(8px);
}
.popover-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
}
.popover-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: #9ca3af;
  cursor: pointer;
  font-size: 12px;
  line-height: 1;
  transition: color 0.14s ease, transform 0.14s ease;
}
.popover-close:hover {
  color: #475569;
  transform: translateY(-1px);
}
.form-grid,
.default-value-box {
  display: grid;
  grid-template-columns: 78px 1fr;
  gap: 8px;
  align-items: center;
}
.default-value-box {
  margin-top: 10px;
}
.form-grid label,
.default-value-box label {
  font-size: 12px;
  color: #6b7280;
}
.form-grid input,
.form-grid select,
.default-value-box input,
.default-value-box textarea {
  width: 100%;
  padding: 6px 8px;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  font-size: 12px;
  outline: none;
  color: #1f2937;
  box-sizing: border-box;
}
.default-value-box textarea {
  resize: vertical;
  min-height: 60px;
}
.form-grid input:focus,
.form-grid select:focus,
.default-value-box input:focus,
.default-value-box textarea:focus {
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.12);
}
.popover-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}
.btn-cancel,
.btn-create {
  min-height: 30px;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
  line-height: 1;
  box-sizing: border-box;
}
.btn-cancel {
  border: 1px solid #d1d5db;
  background: #fff;
  color: #4b5563;
}
.btn-create {
  border: none;
  background: #2563eb;
  color: #fff;
}
.btn-create:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
.create-msg {
  margin-top: 10px;
  padding: 6px 8px;
  border-radius: 8px;
  font-size: 12px;
}
.create-msg.ok {
  background: #f0fdf4;
  color: #166534;
}
.create-msg.err {
  background: #fff1f2;
  color: #991b1b;
}
.create-key-wrap.theme-dark .create-key-btn {
  border-color: rgba(71, 85, 105, 0.96);
  background: linear-gradient(180deg, rgba(30, 41, 59, 0.96), rgba(15, 23, 42, 0.96));
  color: #93c5fd;
  box-shadow: 0 10px 18px rgba(2, 6, 23, 0.3);
}

.create-key-wrap.theme-dark .create-key-btn:hover {
  color: #dbeafe;
  border-color: rgba(96, 165, 250, 0.44);
  background: linear-gradient(180deg, rgba(30, 64, 175, 0.24), rgba(30, 41, 59, 0.98));
  box-shadow: 0 12px 22px rgba(2, 6, 23, 0.36);
}

.create-key-wrap.theme-dark .create-key-popover {
  border-color: rgba(51, 65, 85, 0.82);
  background: rgba(15, 23, 42, 0.98);
  box-shadow: 0 26px 52px rgba(2, 6, 23, 0.46), 0 8px 18px rgba(2, 6, 23, 0.18);
  backdrop-filter: none;
}

.create-key-popover.theme-dark {
  border-color: rgba(51, 65, 85, 0.82);
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.985), rgba(11, 18, 32, 0.995));
  box-shadow: 0 26px 52px rgba(2, 6, 23, 0.46), 0 8px 18px rgba(2, 6, 23, 0.18);
  backdrop-filter: none;
}

.create-key-wrap.theme-dark .popover-header {
  color: #e2e8f0;
}

.create-key-popover.theme-dark .popover-header {
  color: #e2e8f0;
}

.create-key-wrap.theme-dark .popover-close {
  color: #94a3b8;
}

.create-key-popover.theme-dark .popover-close {
  color: #94a3b8;
}

.create-key-wrap.theme-dark .popover-close:hover {
  color: #e2e8f0;
}

.create-key-popover.theme-dark .popover-close:hover {
  color: #e2e8f0;
}

.create-key-wrap.theme-dark .form-grid label,
.create-key-wrap.theme-dark .default-value-box label {
  color: #94a3b8;
}

.create-key-popover.theme-dark .form-grid label,
.create-key-popover.theme-dark .default-value-box label {
  color: #94a3b8;
}

.create-key-wrap.theme-dark .form-grid input,
.create-key-wrap.theme-dark .form-grid select,
.create-key-wrap.theme-dark .default-value-box input,
.create-key-wrap.theme-dark .default-value-box textarea {
  background: rgba(15, 23, 42, 0.94);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}

.create-key-popover.theme-dark .form-grid input,
.create-key-popover.theme-dark .form-grid select,
.create-key-popover.theme-dark .default-value-box input,
.create-key-popover.theme-dark .default-value-box textarea {
  background: rgba(11, 18, 32, 0.98);
  color: #e2e8f0;
  border-color: rgba(71, 85, 105, 0.96);
}

.create-key-popover.theme-dark .form-grid select option {
  background: #0f172a;
  color: #e2e8f0;
}

.create-key-wrap.theme-dark .btn-cancel {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(15, 23, 42, 0.94);
  color: #cbd5e1;
}

.create-key-popover.theme-dark .btn-cancel {
  border-color: rgba(71, 85, 105, 0.96);
  background: rgba(11, 18, 32, 0.98);
  color: #cbd5e1;
}

.create-key-popover.theme-dark .btn-cancel:hover {
  background: rgba(30, 41, 59, 0.96);
  border-color: rgba(96, 165, 250, 0.34);
  color: #e2e8f0;
}

.create-key-popover.theme-dark .btn-create {
  box-shadow: 0 10px 20px rgba(30, 64, 175, 0.22);
}

.create-key-popover.theme-dark .btn-create:hover:not(:disabled) {
  background: #1d4ed8;
  box-shadow: 0 12px 24px rgba(30, 64, 175, 0.28);
}

.create-key-wrap.theme-dark .create-msg.ok {
  background: rgba(6, 78, 59, 0.94);
  color: #d1fae5;
}

.create-key-popover.theme-dark .create-msg.ok {
  background: rgba(6, 78, 59, 0.94);
  color: #d1fae5;
}

.create-key-wrap.theme-dark .create-msg.err {
  background: rgba(127, 29, 29, 0.94);
  color: #fee2e2;
}

.create-key-popover.theme-dark .create-msg.err {
  background: rgba(127, 29, 29, 0.94);
  color: #fee2e2;
}
</style>
