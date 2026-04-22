<template>
  <div class="settings-panel">
    <div class="settings-header">
      <span>⚙️ 设置</span>
      <button class="btn-close" @click="$emit('close')">✕</button>
    </div>

    <div class="settings-body">
      <div class="section-title">Key 加载数量</div>

      <div class="setting-item">
        <label>
          <span class="label-text">Key 列表每次加载</span>
          <span class="label-hint">每次 SCAN 返回的最大 key 数量</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.keyScanCount" type="number" min="10" max="10000" step="10" />
          <span class="unit">个</span>
        </div>
      </div>

      <div class="section-title mt">Value 加载数量</div>

      <div class="setting-item">
        <label>
          <span class="label-text">Hash 每次加载 Field 数</span>
          <span class="label-hint">使用 HSCAN 分批读取，防止大 Hash 阻塞</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.hashLoadCount" type="number" min="10" max="100000" step="50" />
          <span class="unit">条</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">List 每次加载元素数</span>
          <span class="label-hint">使用 LRANGE 0 N-1 读取前 N 条</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.listLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">条</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">Set 每次加载成员数</span>
          <span class="label-hint">使用 SSCAN 迭代读取，不阻塞</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.setLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">个</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">ZSet 每次加载成员数</span>
          <span class="label-hint">使用 ZRANGE 0 N-1 按分数升序读取</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.zsetLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">个</span>
        </div>
      </div>

      <div class="setting-item">
        <label>
          <span class="label-text">Stream 每次加载条目数</span>
          <span class="label-hint">使用 XREVRANGE 读取最新 N 条</span>
        </label>
        <div class="input-unit">
          <input v-model.number="form.streamLoadCount" type="number" min="10" max="10000" step="50" />
          <span class="unit">条</span>
        </div>
      </div>
    </div>

    <div class="settings-footer">
      <span v-if="msg" :class="['save-msg', ok ? 'ok' : 'err']">{{ msg }}</span>
      <button class="btn-cancel" @click="reset">重置默认</button>
      <button class="btn-close-modal" @click="$emit('close')">取消</button>
      <button class="btn-save" :disabled="saving" @click="doSave">{{ saving ? '保存中...' : '保存设置' }}</button>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { useSettingsStore } from '../../stores/settings.js'

const emit = defineEmits(['close'])
const settingsStore = useSettingsStore()

const form = reactive({
  keyScanCount: 20,
  hashLoadCount: 20,
  listLoadCount: 20,
  setLoadCount: 20,
  zsetLoadCount: 20,
  streamLoadCount: 20,
})

const saving = ref(false)
const msg = ref('')
const ok = ref(true)

onMounted(async () => {
  await settingsStore.load()
  syncFromStore()
})

function syncFromStore() {
  form.keyScanCount = settingsStore.keyScanCount
  form.hashLoadCount = settingsStore.hashLoadCount
  form.listLoadCount = settingsStore.listLoadCount
  form.setLoadCount = settingsStore.setLoadCount
  form.zsetLoadCount = settingsStore.zsetLoadCount
  form.streamLoadCount = settingsStore.streamLoadCount
}

function reset() {
  form.keyScanCount = 20
  form.hashLoadCount = 20
  form.listLoadCount = 20
  form.setLoadCount = 20
  form.zsetLoadCount = 20
  form.streamLoadCount = 20
}

async function doSave() {
  saving.value = true
  msg.value = ''
  try {
    const result = await settingsStore.save({ ...form })
    ok.value = result.success
    msg.value = result.success ? '✓ 已保存，下次加载 key 时生效' : (result.message || '保存失败')
    if (result.success) {
      setTimeout(() => { msg.value = '' }, 3000)
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
.settings-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: white;
}
.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid #e5e7eb;
  font-weight: 600;
  font-size: 14px;
  color: #111827;
  background: #f9fafb;
}
.btn-close {
  background: transparent;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  color: #9ca3af;
  padding: 2px 8px;
  line-height: 1.4;
  transition: color 0.12s, border-color 0.12s;
}
.btn-close:hover { color: #dc2626; border-color: #fca5a5; background: #fff1f2; }

.settings-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.section-title {
  font-size: 11px;
  font-weight: 700;
  color: #9ca3af;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  margin-bottom: 10px;
}
.section-title.mt { margin-top: 20px; }

.setting-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  border-bottom: 1px solid #f3f4f6;
  gap: 12px;
}
.setting-item label {
  flex: 1;
  min-width: 0;
}
.label-text {
  display: block;
  font-size: 13px;
  color: #1f2937;
  font-weight: 500;
}
.label-hint {
  display: block;
  font-size: 11px;
  color: #9ca3af;
  margin-top: 2px;
}
.input-unit {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.input-unit input {
  width: 80px;
  padding: 5px 8px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  font-size: 13px;
  text-align: right;
  outline: none;
  transition: border-color 0.15s;
}
.input-unit input:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.15); }
.unit {
  font-size: 12px;
  color: #6b7280;
  white-space: nowrap;
}

.settings-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid #e5e7eb;
  background: #f9fafb;
}
.save-msg {
  flex: 1;
  font-size: 12px;
}
.save-msg.ok { color: #166534; }
.save-msg.err { color: #991b1b; }
.btn-save {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 6px 18px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.15s;
}
.btn-save:hover { background: #2563eb; }
.btn-save:disabled { background: #93c5fd; cursor: not-allowed; }
.btn-cancel {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.12s, border-color 0.12s;
}
.btn-cancel:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-close-modal {
  display: inline-flex; align-items: center; justify-content: center;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  transition: background 0.12s, border-color 0.12s;
}
.btn-close-modal:hover { background: #f3f4f6; border-color: #9ca3af; }
</style>
