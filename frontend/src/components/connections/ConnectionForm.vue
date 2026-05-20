<template>
  <div class="connection-form">
    <div class="form-scroll">
      <h3>{{ isEdit ? t('connManager.editConn') : t('connManager.newConn') }}</h3>

      <div class="form-group">
        <label>{{ t('connManager.nameRequired') }}</label>
        <input v-model="form.name" type="text" placeholder="My Redis" />
      </div>

      <div class="form-group">
        <label>{{ t('connManager.groupOptional') }}</label>
        <input
          v-model="form.group"
          type="text"
          :placeholder="t('connManager.groupPlaceholder')"
          @focus="showGroupSuggestions = true"
          @blur="hideGroupSuggestions"
        />
        <div v-if="showGroupSuggestions && filteredGroups.length" class="group-suggestions">
          <button
            v-for="g in filteredGroups"
            :key="g"
            type="button"
            class="group-suggestion-item"
            @mousedown.prevent="selectGroup(g)"
          >
            {{ g }}
          </button>
        </div>
      </div>

      <div class="form-group toggle-row">
        <label>{{ t('connManager.clusterMode') }}</label>
        <input type="checkbox" v-model="form.isCluster" />
      </div>

      <template v-if="!form.isCluster">
        <div class="form-row">
          <div class="form-group flex1">
            <label>Host *</label>
            <input v-model="form.host" type="text" placeholder="127.0.0.1" />
          </div>
          <div class="form-group w100">
            <label>Port</label>
            <input v-model.number="form.port" type="number" placeholder="6379" min="1" max="65535" />
          </div>
        </div>
        <div class="form-group">
          <label>Password</label>
          <input v-model="form.password" type="password" :placeholder="t('connManager.passwordPlaceholder')" />
        </div>
        <div class="form-group w80">
          <label>{{ t('connManager.dbIndex') }}</label>
          <input v-model.number="form.db" type="number" min="0" max="15" placeholder="0" />
        </div>
      </template>

      <template v-else>
        <div class="form-group">
          <label>{{ t('connManager.clusterAddrsLabel') }}</label>
          <textarea v-model="clusterAddrsText" rows="4" placeholder="127.0.0.1:7000&#10;127.0.0.1:7001&#10;127.0.0.1:7002" />
        </div>
        <div class="form-group">
          <label>Password</label>
          <input v-model="form.password" type="password" :placeholder="t('connManager.passwordPlaceholder')" />
        </div>
      </template>

      <div class="proxy-section">
        <div class="form-group toggle-row">
          <label>{{ t('connManager.proxyEnabled') }}</label>
          <input type="checkbox" v-model="form.proxyEnabled" />
        </div>

        <template v-if="form.proxyEnabled">
          <div class="proxy-panel">
            <div class="form-group">
              <label>{{ t('connManager.proxyUrl') }}</label>
              <input
                v-model="form.proxyUrl"
                type="text"
                :placeholder="t('connManager.proxyUrlPlaceholder')"
              />
            </div>
          </div>
        </template>
      </div>

      <div class="ssh-section">
        <div class="form-group toggle-row">
          <label>{{ t('connManager.sshEnabled') }}</label>
          <input type="checkbox" v-model="form.sshEnabled" />
        </div>

        <template v-if="form.sshEnabled">
          <div class="ssh-panel">
            <div class="form-row">
              <div class="form-group flex1">
                <label>{{ t('connManager.sshHostRequired') }}</label>
                <input v-model="form.ssh.host" type="text" placeholder="jump.example.com" />
              </div>
              <div class="form-group w80">
                <label>Port</label>
                <input v-model.number="form.ssh.port" type="number" placeholder="22" min="1" max="65535" />
              </div>
            </div>
            <div class="form-row">
              <div class="form-group flex1">
                <label>{{ t('connManager.sshUserRequired') }}</label>
                <input v-model="form.ssh.user" type="text" placeholder="ubuntu" />
              </div>
              <div class="form-group flex1">
                <label>SSH Password</label>
                <input v-model="form.ssh.password" type="password" :placeholder="t('connManager.sshPasswordPlaceholder')" />
              </div>
            </div>
            <div class="form-row">
              <div class="form-group flex1">
                <label>{{ t('connManager.sshPrivateKey') }}</label>
                <input v-model="form.ssh.private_key_path" type="text" :placeholder="t('connManager.sshPrivateKeyPlaceholder')" />
              </div>
            </div>
            <div class="form-row">
              <div class="form-group flex1">
                <label>{{ t('connManager.sshPassphrase') }}</label>
                <input v-model="form.ssh.passphrase" type="password" :placeholder="t('connManager.sshPassphrasePlaceholder')" />
              </div>
            </div>
          </div>
        </template>
      </div>

      <div class="icon-color-section">
        <div class="form-group">
          <label>Icon Color</label>
          <div class="icon-color-card">
            <div class="icon-color-preview">
              <span class="icon-preview-badge" :style="{ background: previewIconColor }">
                {{ previewInitial }}
              </span>
              <div class="icon-preview-text">
                <div class="icon-preview-title">Connection Icon</div>
                <div class="icon-preview-subtitle">
                  {{ normalizedIconColor ? normalizedIconColor.toUpperCase() : 'Default palette' }}
                </div>
              </div>
            </div>
            <div class="icon-color-row">
              <input v-model="form.iconColor" class="icon-color-picker" type="color" />
              <input
                v-model="form.iconColor"
                class="icon-color-text"
                type="text"
                placeholder="#5C7F9E"
              />
              <button type="button" class="btn-secondary btn-color-reset" @click="form.iconColor = ''">
                Default
              </button>
            </div>
            <div class="icon-color-swatches">
              <button
                v-for="color in ICON_COLOR_PRESETS"
                :key="color"
                type="button"
                class="icon-swatch"
                :class="{ active: normalizedIconColor === color }"
                :style="{ background: color }"
                @click="form.iconColor = color"
              />
            </div>
          </div>
          <div class="field-hint">Leave empty to use the default connection color.</div>
        </div>
      </div>
    </div>

    <div class="form-actions">
      <button class="btn-secondary" @click="$emit('cancel')">{{ t('connManager.cancel') }}</button>
      <button class="btn-secondary" :disabled="testing" @click="handleTest">
        {{ testing ? t('connManager.testing') : t('connManager.testConn') }}
      </button>
      <button class="btn-primary" :disabled="saving || !isDirty" :class="{ disabled: saving || !isDirty }" @click="handleSave">
        {{ saving ? t('connManager.saving') : t('connManager.save') }}
      </button>
    </div>
    <div class="form-messages">
      <div v-if="testMsg" :class="['test-msg', testOk ? 'ok' : 'err']">{{ testMsg }}</div>
      <div v-if="saveOkMsg" class="test-msg ok">{{ saveOkMsg }}</div>
      <div v-if="saveMsg" class="test-msg err">{{ saveMsg }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { useConnectionsStore } from '../../stores/connections.js'
import { useI18n } from '../../i18n/index.js'
import { formatDebugMessage } from '../../utils/debug.js'
const props = defineProps({
  connection: { type: Object, default: null },
})
const emit = defineEmits(['cancel', 'saved'])

const connectionsStore = useConnectionsStore()
const { t } = useI18n()

const isEdit = computed(() => !!props.connection?.id)
const existingGroups = computed(() => Object.keys(connectionsStore.groupedConnections).filter(g => g))
const filteredGroups = computed(() => {
  const keyword = form.group.trim().toLowerCase()
  if (!keyword) return existingGroups.value
  return existingGroups.value.filter(g => g.toLowerCase().includes(keyword))
})
const ICON_COLOR_PRESETS = [
  '#5c7f9e', '#6e8c6a', '#8a6a7a', '#7a6e8a',
  '#8a7a5a', '#5a7a7a', '#d97706', '#2563eb',
]

const defaultForm = () => ({
  id: '',
  name: '',
  sort_order: 0,
  group: '',
  host: '127.0.0.1',
  port: 6379,
  password: '',
  db: 0,
  isCluster: false,
  clusterAddrs: [],
  proxyEnabled: false,
  proxyUrl: '',
  iconColor: '',
  sshEnabled: false,
  ssh: { host: '', port: 22, user: '', password: '', private_key_path: '', passphrase: '' },
})

const form = reactive(defaultForm())
const clusterAddrsText = ref('')
const initialSnapshot = ref('')
const showGroupSuggestions = ref(false)
let hideGroupTimer = null

const normalizedIconColor = computed(() => normalizeIconColor(form.iconColor))
const previewIconColor = computed(() => normalizedIconColor.value || '#5c7f9e')
const previewInitial = computed(() => {
  const value = (form.name || form.host || '?').trim()
  return value ? value[0].toUpperCase() : '?'
})

function clearHideGroupTimer() {
  if (hideGroupTimer) {
    clearTimeout(hideGroupTimer)
    hideGroupTimer = null
  }
}

function hideGroupSuggestions() {
  clearHideGroupTimer()
  hideGroupTimer = setTimeout(() => {
    showGroupSuggestions.value = false
    hideGroupTimer = null
  }, 120)
}

function selectGroup(group) {
  clearHideGroupTimer()
  form.group = group
  showGroupSuggestions.value = false
}

function snapshotValue() {
  return JSON.stringify({
    id: form.id || '',
    name: form.name || '',
    group: form.group || '',
    host: form.host || '',
    port: form.port || 6379,
    password: form.password || '',
    db: form.db || 0,
    isCluster: !!form.isCluster,
    cluster_addrs: form.isCluster
      ? clusterAddrsText.value.split('\n').map(s => s.trim()).filter(Boolean)
      : [],
    proxyEnabled: !!form.proxyEnabled,
    proxyUrl: form.proxyUrl || '',
    iconColor: form.iconColor || '',
    sshEnabled: !!form.sshEnabled,
    ssh: form.sshEnabled ? {
      host: form.ssh.host || '',
      port: form.ssh.port || 22,
      user: form.ssh.user || '',
      password: form.ssh.password || '',
      private_key_path: form.ssh.private_key_path || '',
      passphrase: form.ssh.passphrase || '',
    } : null,
  })
}

const isDirty = computed(() => snapshotValue() !== initialSnapshot.value)

watch(() => props.connection, (conn) => {
  if (conn) {
    Object.assign(form, {
      ...defaultForm(),
      ...conn,
      group: conn.group || '',
      isCluster: conn.isCluster ?? conn.is_cluster,
      proxyEnabled: conn.proxyEnabled ?? conn.proxy_enabled,
      proxyUrl: conn.proxyUrl ?? conn.proxy_url ?? '',
      iconColor: conn.iconColor ?? conn.icon_color ?? '',
      ssh: conn.ssh ? { ...conn.ssh } : { host: '', port: 22, user: '', password: '', private_key_path: '', passphrase: '' },
    })
    clusterAddrsText.value = (conn.clusterAddrs || conn.cluster_addrs || []).join('\n')
  } else {
    Object.assign(form, defaultForm())
    clusterAddrsText.value = ''
  }
  showGroupSuggestions.value = false
  clearHideGroupTimer()
  saveOkMsg.value = ''
  saveMsg.value = ''
  initialSnapshot.value = snapshotValue()
}, { immediate: true })

const testing = ref(false)
const saving = ref(false)
const testMsg = ref('')
const testOk = ref(false)
const saveOkMsg = ref('')
const saveMsg = ref('')

function buildCfg() {
  const cfg = {
    id: form.id,
    name: form.name,
    sort_order: form.sort_order || 0,
    group: form.group || '',
    host: form.host,
    port: form.port || 6379,
    password: form.password,
    db: form.db || 0,
    is_cluster: form.isCluster,
    cluster_addrs: form.isCluster
      ? clusterAddrsText.value.split('\n').map(s => s.trim()).filter(Boolean)
      : [],
    proxy_enabled: form.proxyEnabled,
    proxy_url: form.proxyEnabled ? form.proxyUrl.trim() : '',
    icon_color: normalizeIconColor(form.iconColor),
    ssh_enabled: form.sshEnabled,
    ssh: form.sshEnabled ? {
      host: form.ssh.host,
      port: form.ssh.port || 22,
      user: form.ssh.user,
      password: form.ssh.password,
      private_key_path: form.ssh.private_key_path,
      passphrase: form.ssh.passphrase,
    } : null,
  }
  return cfg
}

function normalizeIconColor(value) {
  const color = (value || '').trim()
  if (!color) return ''
  return /^#[0-9a-fA-F]{6}$/.test(color) ? color : ''
}

async function handleTest() {
  testMsg.value = ''
  testing.value = true
  try {
    const result = await connectionsStore.test(buildCfg())
    testOk.value = result.success
    testMsg.value = result.success
      ? '✓ ' + formatDebugMessage(result.message, t('connManager.testOk'))
      : '✗ ' + formatDebugMessage(result.message, t('connManager.testFailed'))
  } catch (e) {
    testOk.value = false
    testMsg.value = '✗ ' + formatDebugMessage(e.message || String(e), t('connManager.testFailed'))
  } finally {
    testing.value = false
  }
}

async function handleSave() {
  if (!isDirty.value) return
  saveMsg.value = ''
  saveOkMsg.value = ''
  if (!form.name.trim()) { saveMsg.value = t('connManager.nameRequiredErr'); return }
  if (!form.isCluster && !form.host.trim()) { saveMsg.value = t('connManager.hostRequiredErr'); return }

  saving.value = true
  try {
    const result = await connectionsStore.save(buildCfg())
    if (result.success) {
      saveOkMsg.value = t('connManager.saveOk')
      initialSnapshot.value = snapshotValue()
      emit('saved')
    } else {
      saveMsg.value = formatDebugMessage(result.message, t('connManager.saveFailed'))
    }
  } catch (e) {
    saveMsg.value = formatDebugMessage(e.message || String(e), t('connManager.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.connection-form {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-width: 480px;
  min-height: 0;
  background: #ffffff;
  border: 1px solid #eef2f7;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.04);
}
.form-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px 16px 6px;
}
h3 {
  margin: 0 0 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f1f5f9;
  font-size: 15px;
  color: #111827;
  font-weight: 600;
}
.form-group { margin-bottom: 12px; position: relative; }
.form-group label { display: block; font-size: 12px; color: #6b7280; margin-bottom: 4px; font-weight: 500; }
input[type=text], input[type=password], input[type=number], textarea {
  width: 100%; padding: 6px 9px; border: 1px solid #d1d5db; border-radius: 6px;
  font-size: 13px; box-sizing: border-box; outline: none; color: #1f2937;
  transition: border-color 0.15s;
}
input:focus, textarea:focus { border-color: #3b82f6; box-shadow: 0 0 0 2px rgba(59,130,246,.12); }
textarea { resize: vertical; font-family: monospace; }
.group-suggestions {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 20;
  background: #fff;
  border: 1px solid #d1d5db;
  border-radius: 8px;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.12);
  max-height: 180px;
  overflow-y: auto;
}
.group-suggestion-item {
  width: 100%;
  border: none;
  background: transparent;
  text-align: left;
  padding: 8px 10px;
  font-size: 13px;
  color: #374151;
  cursor: pointer;
}
.group-suggestion-item:hover {
  background: #eff6ff;
  color: #2563eb;
}
.toggle-row { display: flex; align-items: center; justify-content: space-between; }
.toggle-row input[type=checkbox] { width: auto; }
.form-row { display: flex; gap: 8px; }
.flex1 { flex: 1; }
.w100 { width: 100px; }
.w80 { width: 80px; }
.proxy-section { border-top: 1px solid #e5e7eb; padding-top: 12px; margin-top: 4px; }
.proxy-panel { background: #f9fafb; padding: 12px; border-radius: 6px; margin-top: 8px; border: 1px solid #e5e7eb; }
.ssh-section { border-top: 1px solid #e5e7eb; padding-top: 12px; margin-top: 4px; }
.ssh-panel { background: #f9fafb; padding: 12px; border-radius: 6px; margin-top: 8px; border: 1px solid #e5e7eb; }
.icon-color-section { border-top: 1px solid #e5e7eb; padding-top: 12px; margin-top: 4px; }
.icon-color-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: linear-gradient(180deg, #fbfdff 0%, #f8fafc 100%);
}
.icon-color-preview {
  display: flex;
  align-items: center;
  gap: 10px;
}
.icon-preview-badge {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  box-shadow: inset 0 1px 0 rgba(255,255,255,0.2);
}
.icon-preview-text { min-width: 0; }
.icon-preview-title { font-size: 13px; font-weight: 600; color: #1f2937; }
.icon-preview-subtitle { font-size: 12px; color: #6b7280; }
.icon-color-row { display: flex; gap: 8px; align-items: center; }
.icon-color-picker {
  width: 42px;
  min-width: 42px;
  height: 34px;
  padding: 3px;
  border-radius: 6px;
  cursor: pointer;
}
.icon-color-text { flex: 1; min-width: 0; text-transform: uppercase; }
.btn-color-reset { flex-shrink: 0; }
.icon-color-swatches {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.icon-swatch {
  width: 22px;
  height: 22px;
  border-radius: 6px;
  border: 2px solid transparent;
  cursor: pointer;
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.28);
  transition: transform 0.12s, box-shadow 0.12s, border-color 0.12s;
}
.icon-swatch:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 10px rgba(15, 23, 42, 0.14);
}
.icon-swatch.active {
  border-color: #1f2937;
  box-shadow: 0 0 0 1px #1f2937;
}
.field-hint { margin-top: 4px; font-size: 12px; color: #9ca3af; }
.form-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
  min-height: 46px;
  padding: 6px 16px 4px;
  border-top: 1px solid #eef2f7;
  background: linear-gradient(180deg, #ffffff 0%, #fbfcfe 100%);
  flex-shrink: 0;
  box-sizing: border-box;
}
.form-messages {
  min-height: 10px;
  padding: 0 16px 4px;
  background: linear-gradient(180deg, #ffffff 0%, #fbfcfe 100%);
  flex-shrink: 0;
  box-sizing: border-box;
}
.btn-primary {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 32px;
  padding: 6px 18px;
  background: #3b82f6;
  color: white;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  font-weight: 500;
  box-sizing: border-box;
  transition: background 0.15s;
}
.btn-primary:hover { background: #2563eb; }
.btn-primary:disabled { background: #93c5fd; cursor: not-allowed; }
.btn-primary.disabled:hover { background: #93c5fd; }
.btn-secondary {
  display: inline-flex; align-items: center; justify-content: center;
  min-height: 32px;
  padding: 6px 14px;
  background: #fff;
  color: #374151;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  line-height: 1;
  font-weight: 500;
  box-sizing: border-box;
  transition: background 0.12s, border-color 0.12s;
}
.btn-secondary:hover { background: #f3f4f6; border-color: #9ca3af; }
.btn-secondary:disabled { opacity: 0.5; cursor: not-allowed; }
.test-msg { margin-top: 2px; padding: 6px 10px; border-radius: 6px; font-size: 12px; }
.test-msg.ok { background: #f0fdf4; color: #166534; }
.test-msg.err { background: #fff1f2; color: #991b1b; }
</style>
