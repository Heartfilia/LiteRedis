<template>
  <div class="create-key-wrap">
    <button class="create-key-btn" :title="t('keyTree.createKey')" @click="toggleOpen">
      <span>+</span>
    </button>

    <div v-if="open" class="create-key-popover">
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
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace.js'
import { useI18n } from '../../i18n/index.js'

const workspaceStore = useWorkspaceStore()
const { t } = useI18n()

const open = ref(false)
const saving = ref(false)
const msg = ref('')
const ok = ref(true)

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
  height: 27px;
  min-width: 28px;
  padding: 0 8px;
  border-radius: 6px;
  border: 1px solid #d1d5db;
  background: #fff;
  color: #4b5563;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}
.create-key-btn span {
  font-size: 16px;
  line-height: 1;
  transform: translateY(-1px);
}
.create-key-btn:hover {
  color: #2563eb;
  border-color: #93c5fd;
  background: #f8fbff;
}
.create-key-popover {
  position: absolute;
  top: 36px;
  right: 0;
  z-index: 30;
  width: 300px;
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.98);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.14);
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
  border: none;
  background: transparent;
  color: #9ca3af;
  cursor: pointer;
  font-size: 12px;
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
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  cursor: pointer;
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
</style>
