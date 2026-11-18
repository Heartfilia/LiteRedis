import { defineStore } from 'pinia'
import { listSSHProfiles, saveSSHProfile, deleteSSHProfile } from '../api/wails.js'
import { useConnectionsStore } from './connections.js'
import { confirmDiscardEdits } from '../utils/editSafety.js'

export const useSSHProfilesStore = defineStore('sshProfiles', {
  state: () => ({ profiles: [], loading: false, error: '' }),
  actions: {
    async load() {
      this.loading = true
      this.error = ''
      try {
        this.profiles = await listSSHProfiles() || []
      } catch (e) {
        this.error = e.message || String(e)
      } finally {
        this.loading = false
      }
    },
    async save(profile) {
      if (profile.id && !await confirmDiscardEdits()) return { success: false, cancelled: true }
      const result = await saveSSHProfile(profile)
      if (result.success) {
        const connections = useConnectionsStore()
        for (const id of result.disconnected_ids || []) {
          connections.connectedIds.delete(id)
          connections.connectingIds.delete(id)
          delete connections.heartbeatFailures[id]
        }
        await Promise.all([this.load(), connections.loadConnections()])
      }
      return result
    },
    async remove(id) {
      const result = await deleteSSHProfile(id)
      if (result.success) await this.load()
      return result
    },
  },
})
