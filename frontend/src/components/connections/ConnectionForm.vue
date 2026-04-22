<template>
  <div class="connection-form">
    <h3>{{ isEdit ? '编辑连接' : '新建连接' }}</h3>

    <div class="form-group">
      <label>连接名称 *</label>
      <input v-model="form.name" type="text" placeholder="My Redis" />
    </div>

    <div class="form-group">
      <label>分组（可选）</label>
      <input
        v-model="form.group"
        type="text"
        placeholder="留空则不分组，同名即同组"
        list="group-datalist"
      />
      <datalist id="group-datalist">
        <option v-for="g in existingGroups" :key="g" :value="g" />
      </datalist>
    </div>

    <!-- 集群模式切换 -->
    <div class="form-group toggle-row">
      <label>集群模式 (Cluster)</label>
      <input type="checkbox" v-model="form.isCluster" />
    </div>

    <!-- 普通模式地址 -->
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
        <input v-model="form.password" type="password" placeholder="(无密码则留空)" />
      </div>
      <div class="form-group w80">
        <label>DB Index</label>
        <input v-model.number="form.db" type="number" min="0" max="15" placeholder="0" />
      </div>
    </template>

    <!-- 集群模式地址列表 -->
    <template v-else>
      <div class="form-group">
        <label>集群节点地址（每行一个，格式 host:port）</label>
        <textarea v-model="clusterAddrsText" rows="4" placeholder="127.0.0.1:7000&#10;127.0.0.1:7001&#10;127.0.0.1:7002" />
      </div>
      <div class="form-group">
        <label>Password</label>
        <input v-model="form.password" type="password" placeholder="(无密码则留空)" />
      </div>
    </template>

    <!-- SSH 配置 -->
    <div class="ssh-section">
      <div class="form-group toggle-row">
        <label>启用 SSH 隧道</label>
        <input type="checkbox" v-model="form.sshEnabled" />
      </div>

      <template v-if="form.sshEnabled">
        <div class="ssh-panel">
          <div class="form-row">
            <div class="form-group flex1">
              <label>SSH Host *</label>
              <input v-model="form.ssh.host" type="text" placeholder="jump.example.com" />
            </div>
            <div class="form-group w80">
              <label>Port</label>
              <input v-model.number="form.ssh.port" type="number" placeholder="22" min="1" max="65535" />
            </div>
          </div>
          <div class="form-row">
            <div class="form-group flex1">
              <label>SSH User *</label>
              <input v-model="form.ssh.user" type="text" placeholder="ubuntu" />
            </div>
            <div class="form-group flex1">
              <label>SSH Password</label>
              <input v-model="form.ssh.password" type="password" placeholder="密码" />
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- 操作按钮 -->
    <div class="form-actions">
      <button class="btn-secondary" @click="$emit('cancel')">取消</button>
      <button class="btn-secondary" :disabled="testing" @click="handleTest">
        {{ testing ? '测试中...' : '测试连接' }}
      </button>
      <button class="btn-primary" :disabled="saving" @click="handleSave">
        {{ saving ? '保存中...' : '保存' }}
      </button>
    </div>

    <div v-if="testMsg" :class="['test-msg', testOk ? 'ok' : 'err']">{{ testMsg }}</div>
    <div v-if="saveMsg" class="test-msg err">{{ saveMsg }}</div>
  </div>
</template>

<script setup>
import { ref, reactive, watch, computed } from 'vue'
import { useConnectionsStore } from '../../stores/connections.js'
const props = defineProps({
  connection: { type: Object, default: null },
})
const emit = defineEmits(['cancel', 'saved'])

const connectionsStore = useConnectionsStore()

const isEdit = computed(() => !!props.connection?.id)
const existingGroups = computed(() => Object.keys(connectionsStore.groupedConnections).filter(g => g))

const defaultForm = () => ({
  id: '',
  name: '',
  group: '',
  host: '127.0.0.1',
  port: 6379,
  password: '',
  db: 0,
  isCluster: false,
  clusterAddrs: [],
  sshEnabled: false,
  ssh: { host: '', port: 22, user: '', password: '' },
})

const form = reactive(defaultForm())
const clusterAddrsText = ref('')

watch(() => props.connection, (conn) => {
  if (conn) {
    Object.assign(form, {
      ...defaultForm(),
      ...conn,
      group: conn.group || '',
      ssh: conn.ssh ? { ...conn.ssh } : { host: '', port: 22, user: '', password: '' },
    })
    clusterAddrsText.value = (conn.clusterAddrs || []).join('\n')
  } else {
    Object.assign(form, defaultForm())
    clusterAddrsText.value = ''
  }
}, { immediate: true })

const testing = ref(false)
const saving = ref(false)
const testMsg = ref('')
const testOk = ref(false)
const saveMsg = ref('')

function buildCfg() {
  const cfg = {
    id: form.id,
    name: form.name,
    group: form.group || '',
    host: form.host,
    port: form.port || 6379,
    password: form.password,
    db: form.db || 0,
    is_cluster: form.isCluster,
    cluster_addrs: form.isCluster
      ? clusterAddrsText.value.split('\n').map(s => s.trim()).filter(Boolean)
      : [],
    ssh_enabled: form.sshEnabled,
    ssh: form.sshEnabled ? {
      host: form.ssh.host,
      port: form.ssh.port || 22,
      user: form.ssh.user,
      password: form.ssh.password,
    } : null,
  }
  return cfg
}

async function handleTest() {
  testMsg.value = ''
  testing.value = true
  try {
    const result = await connectionsStore.test(buildCfg())
    testOk.value = result.success
    testMsg.value = result.success ? '✓ ' + (result.message || '连接成功') : '✗ ' + (result.message || '连接失败')
  } catch (e) {
    testOk.value = false
    testMsg.value = '✗ ' + (e.message || String(e))
  } finally {
    testing.value = false
  }
}

async function handleSave() {
  saveMsg.value = ''
  if (!form.name.trim()) { saveMsg.value = '请填写连接名称'; return }
  if (!form.isCluster && !form.host.trim()) { saveMsg.value = '请填写 Host'; return }

  saving.value = true
  try {
    const result = await connectionsStore.save(buildCfg())
    if (result.success) {
      emit('saved')
    } else {
      saveMsg.value = result.message || '保存失败'
    }
  } catch (e) {
    saveMsg.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.connection-form {
  padding: 16px;
  max-width: 480px;
}
h3 { margin: 0 0 16px; font-size: 16px; color: #333; }
.form-group { margin-bottom: 12px; }
.form-group label { display: block; font-size: 12px; color: #666; margin-bottom: 4px; }
input[type=text], input[type=password], input[type=number], textarea {
  width: 100%; padding: 6px 8px; border: 1px solid #ddd; border-radius: 4px;
  font-size: 13px; box-sizing: border-box; outline: none;
}
input:focus, textarea:focus { border-color: #4e9af1; }
textarea { resize: vertical; font-family: monospace; }
.toggle-row { display: flex; align-items: center; justify-content: space-between; }
.toggle-row input[type=checkbox] { width: auto; }
.form-row { display: flex; gap: 8px; }
.flex1 { flex: 1; }
.w100 { width: 100px; }
.w80 { width: 80px; }
.ssh-section { border-top: 1px solid #eee; padding-top: 12px; margin-top: 4px; }
.ssh-panel { background: #f9f9f9; padding: 12px; border-radius: 4px; margin-top: 8px; }
.form-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
.btn-primary { padding: 6px 16px; background: #4e9af1; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 13px; }
.btn-primary:hover { background: #3a85e0; }
.btn-primary:disabled { background: #aaa; cursor: default; }
.btn-secondary { padding: 6px 16px; background: #f5f5f5; color: #333; border: 1px solid #ddd; border-radius: 4px; cursor: pointer; font-size: 13px; }
.btn-secondary:hover { background: #eee; }
.btn-secondary:disabled { opacity: 0.6; cursor: default; }
.test-msg { margin-top: 8px; padding: 6px 10px; border-radius: 4px; font-size: 12px; }
.test-msg.ok { background: #e8f5e9; color: #2e7d32; }
.test-msg.err { background: #fce4ec; color: #b71c1c; }
</style>
