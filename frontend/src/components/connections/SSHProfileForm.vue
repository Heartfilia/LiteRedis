<template>
  <div class="ssh-profile-form" :class="`theme-${settingsStore.themeMode || 'light'}`">
    <div class="ssh-form-scroll">
      <h3>{{ profile?.id ? t('connManager.sshEdit') : t('connManager.sshCreate') }}</h3>
      <p class="ssh-hint">{{ t('connManager.sshSharedHint') }}</p>
      <label for="ssh-account-name">{{ t('connManager.sshName') }} *</label>
      <input id="ssh-account-name" v-model="form.name" :disabled="busy" type="text" autocomplete="off" />
      <label for="ssh-account-host">{{ t('connManager.sshHost') }} *</label>
      <input id="ssh-account-host" v-model="form.ssh.host" :disabled="busy" type="text" placeholder="jump.example.com" />
      <label for="ssh-account-port">{{ t('connManager.sshPort') }}</label>
      <input id="ssh-account-port" v-model.number="form.ssh.port" :disabled="busy" type="number" min="1" max="65535" />
      <label for="ssh-account-user">{{ t('connManager.sshUser') }} *</label>
      <input id="ssh-account-user" v-model="form.ssh.user" :disabled="busy" type="text" placeholder="ubuntu" autocomplete="off" />
      <label for="ssh-account-password">{{ t('connManager.sshPassword') }}</label>
      <input id="ssh-account-password" v-model="form.ssh.password" :disabled="busy" type="password" autocomplete="new-password" />
      <label for="ssh-account-key">{{ t('connManager.sshPrivateKey') }}</label>
      <input id="ssh-account-key" v-model="form.ssh.private_key_path" :disabled="busy" type="text" :placeholder="t('connManager.sshPrivateKeyPlaceholder')" />
      <label for="ssh-account-passphrase">{{ t('connManager.sshPassphrase') }}</label>
      <input id="ssh-account-passphrase" v-model="form.ssh.passphrase" :disabled="busy" type="password" autocomplete="new-password" :placeholder="t('connManager.sshPassphrasePlaceholder')" />
      <p class="ssh-hint">{{ t('connManager.sshTestHint') }}</p>
      <p class="ssh-hint">{{ t('connManager.sshStorageHint') }}</p>
      <button type="button" :disabled="busy || !form.ssh.host.trim()" @click="resetFingerprint">{{ t('connManager.sshResetFingerprint') }}</button>
      <p class="ssh-hint">{{ t('connManager.sshResetFingerprintHint') }}</p>
    </div>
    <p v-if="message" class="ssh-message" :class="{ error: !ok }" role="status">{{ message }}</p>
    <div class="ssh-form-actions">
      <button v-if="profile?.id" class="ssh-delete" :disabled="busy" @click="remove">{{ t('connManager.sshDelete') }}</button>
      <button v-if="returnToRedis" :disabled="busy" @click="$emit('return')">{{ t('connManager.sshBack') }}</button>
      <button class="ssh-test" :disabled="busy" @click="test">{{ testing ? t('connManager.testing') : t('connManager.sshTest') }}</button>
      <button class="ssh-save" :disabled="busy || !dirty" @click="save">{{ saving ? t('connManager.saving') : t('connManager.save') }}</button>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { useSSHProfilesStore } from '../../stores/sshProfiles.js'
import { useSettingsStore } from '../../stores/settings.js'
import { testSSHProfile, forgetSSHHostFingerprint } from '../../api/wails.js'
import { askConfirmation } from '../../utils/confirm.js'
import { useI18n } from '../../i18n/index.js'

const props = defineProps({ profile: { type: Object, default: null }, returnToRedis: Boolean })
const emit = defineEmits(['saved', 'deleted', 'return', 'busy'])
const store = useSSHProfilesStore()
const settingsStore = useSettingsStore()
const { t } = useI18n()
const form = reactive({ id: '', name: '', ssh: { host: '', port: 22, user: '', password: '', private_key_path: '', passphrase: '' } })
const snapshot = ref('')
const message = ref('')
const ok = ref(false)
const testing = ref(false)
const saving = ref(false)
const working = ref(false)
const busy = computed(() => testing.value || saving.value || working.value)
const dirty = computed(() => JSON.stringify(form) !== snapshot.value)
async function beforeReplace() {
  if (busy.value) return false
  return !dirty.value || await askConfirmation(t('connManager.sshDiscard'))
}
defineExpose({ beforeReplace })
watch(busy, value => emit('busy', value))
watch(() => props.profile, profile => {
  form.id = profile?.id || ''
  form.name = profile?.name || ''
  Object.assign(form.ssh, { host: '', port: 22, user: '', password: '', private_key_path: '', passphrase: '' }, profile?.ssh || {})
  snapshot.value = JSON.stringify(form)
  message.value = ''
}, { immediate: true })

function validate(needName = false) {
  if ((needName && !form.name.trim()) || !form.ssh.host.trim() || !form.ssh.user.trim() || !Number.isInteger(form.ssh.port) || form.ssh.port < 1 || form.ssh.port > 65535) {
    ok.value = false
    message.value = t('connManager.sshRequired')
    return false
  }
  return true
}
function payload() { return { id: form.id, name: form.name.trim(), ssh: { ...form.ssh } } }
async function test() {
  if (busy.value || !validate()) return
  testing.value = true
  message.value = ''
  try {
    const result = await testSSHProfile(payload())
    ok.value = !!result.success
    message.value = result.success ? t('connManager.sshTestOk') : result.message || t('connManager.testFailed')
  } catch (e) {
    ok.value = false
    message.value = e.message || String(e)
  } finally { testing.value = false }
}
async function save() {
  if (busy.value || !dirty.value || !validate(true)) return
  saving.value = true
  message.value = ''
  try {
    const result = await store.save(payload())
    if (result.cancelled) return
    ok.value = !!result.success
    if (result.success) {
      form.id = result.profile.id
      snapshot.value = JSON.stringify(form)
      emit('saved', result.profile, result.disconnected_ids || [])
      await nextTick()
      message.value = result.disconnected_ids?.length ? t('connManager.sshReconnect') : t('connManager.saveOk')
    } else { message.value = result.message || t('connManager.saveFailed') }
  } catch (e) {
    ok.value = false
    message.value = e.message || String(e)
  } finally { saving.value = false }
}
async function remove() {
  if (busy.value) return
  working.value = true
  try {
    if (!await askConfirmation(t('connManager.sshDeleteConfirm'))) return
    const result = await store.remove(form.id)
    ok.value = !!result.success
    message.value = result.message || ''
    if (result.success) emit('deleted')
  } catch (e) { ok.value = false; message.value = e.message || String(e) }
  finally { working.value = false }
}
async function resetFingerprint() {
  if (busy.value || !validate()) return
  working.value = true
  try {
    if (!await askConfirmation(t('connManager.sshResetConfirm'))) return
    const result = await forgetSSHHostFingerprint(form.ssh.host.trim(), form.ssh.port)
    ok.value = !!result.success
    message.value = result.message || t('connManager.saveOk')
  } catch (e) { ok.value = false; message.value = e.message || String(e) }
  finally { working.value = false }
}
</script>

<style scoped>
.ssh-profile-form { display: flex; flex-direction: column; height: 100%; min-height: 0; max-width: 480px; background: #fff; border: 1px solid #e2e8f0; border-radius: var(--radius-lg); overflow: hidden; }
.ssh-form-scroll { flex: 1; min-height: 0; overflow-y: auto; padding: 14px 16px; }
h3 { margin: 0 0 12px; font-size: 15px; color: #0f172a; }
label { display: block; font-size: 12px; color: #64748b; margin: 12px 0 4px; }
input { width: 100%; box-sizing: border-box; padding: 7px 9px; border: 1px solid #cbd5e1; border-radius: var(--radius-sm); color: #1f2937; background: #fff; font-size: 13px; }
input:focus { outline: 2px solid rgba(59, 130, 246, .25); border-color: #3b82f6; }
.ssh-hint { color: #64748b; font-size: 12px; line-height: 1.6; }
.ssh-form-actions { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 8px; padding: 10px 16px; border-top: 1px solid #e2e8f0; }
button { padding: 7px 10px; border: 1px solid #cbd5e1; border-radius: var(--radius-sm); color: #334155; background: #f8fafc; cursor: pointer; font-size: 12px; }
button:disabled { opacity: .5; cursor: not-allowed; }
.ssh-save { background: #3b82f6; color: white; border-color: #3b82f6; }
.ssh-delete { color: #b91c1c; margin-right: auto; }
.ssh-message { margin: 0; padding: 8px 16px; font-size: 12px; line-height: 1.6; color: #166534; background: #f0fdf4; overflow-wrap: anywhere; }
.ssh-message.error { color: #991b1b; background: #fff1f2; }
.theme-dark { background: #0f172a; border-color: #334155; }
.theme-dark h3 { color: #e2e8f0; }
.theme-dark label, .theme-dark .ssh-hint { color: #94a3b8; }
.theme-dark input, .theme-dark button:not(.ssh-save) { color: #e2e8f0; background: #1e293b; border-color: #475569; }
.theme-dark .ssh-delete { color: #fca5a5 !important; }
.theme-dark .ssh-form-actions { border-color: #334155; }
.theme-dark .ssh-message { color: #86efac; background: #14332b; }
.theme-dark .ssh-message.error { color: #fca5a5; background: #3f1d29; }
</style>
