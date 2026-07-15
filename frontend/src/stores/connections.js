import { defineStore } from 'pinia'
import { listConnections, saveConnection, reorderConnections, deleteConnection, testConnection, connect, disconnect as disconnectAPI, isConnected, pingConnection } from '../api/wails.js'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../utils/connection.js'

export const useConnectionsStore = defineStore('connections', {
  state: () => ({
    connections: [],
    connectedIds: new Set(),
    connectingIds: new Set(),
    heartbeatFailures: {},
    heartbeatTimer: null,
    heartbeatRunning: false,
    heartbeatInFlight: false,
    heartbeatGeneration: 0,
    heartbeatVisible: true,
    globalToast: '',
    globalToastOk: true,
    loading: false,
    error: null,
  }),

  getters: {
    isConnected: (state) => (id) => state.connectedIds.has(id),
    isConnecting: (state) => (id) => state.connectingIds.has(id),
    groupedConnections: (state) => {
      const groups = {}
      for (const conn of state.connections) {
        const g = conn.group || ''
        if (!groups[g]) groups[g] = []
        groups[g].push(conn)
      }
      return groups
    },
  },

  actions: {
    async loadConnections() {
      this.loading = true
      this.error = null
      try {
        this.connections = await listConnections() || []
      } catch (e) {
        this.error = e.message || String(e)
        this.showGlobalToast(this.error, false)
      } finally {
        this.loading = false
      }
    },

    async save(cfg) {
      const result = await saveConnection(cfg)
      if (result.success) {
        if (result.disconnected && cfg.id) {
          this.connectedIds.delete(cfg.id)
          this.connectingIds.delete(cfg.id)
          const nextFailures = { ...this.heartbeatFailures }
          delete nextFailures[cfg.id]
          this.heartbeatFailures = nextFailures
          this.showGlobalToast(result.message || 'Connection settings changed; reconnect required', true)
        }
        await this.loadConnections()
      }
      return result
    },

    async reorder(items) {
      const result = await reorderConnections(items)
      if (result.success) {
        await this.loadConnections()
      }
      return result
    },

    async moveConnection(id, targetId = '', targetGroup = '', placement = 'before') {
      const current = [...this.connections]
      const sourceIndex = current.findIndex(conn => conn.id === id)
      if (sourceIndex === -1) return { success: false, message: 'connection not found' }

      const [moved] = current.splice(sourceIndex, 1)
      moved.group = targetGroup

      let insertIndex = current.length
      if (targetId) {
        const targetIndex = current.findIndex(conn => conn.id === targetId)
        if (targetIndex !== -1) {
          insertIndex = targetIndex + (placement === 'after' ? 1 : 0)
        }
      } else {
        const lastGroupIndex = [...current].map((conn, index) => ({ conn, index }))
          .filter(({ conn }) => (conn.group || '') === targetGroup)
          .at(-1)?.index
        if (typeof lastGroupIndex === 'number') {
          insertIndex = lastGroupIndex + 1
        }
      }

      current.splice(insertIndex, 0, moved)

      const payload = current.map((conn, index) => ({
        id: conn.id,
        group: conn.group || '',
        sort_order: index,
      }))

      return await this.reorder(payload)
    },

    async remove(id) {
      const result = await deleteConnection(id)
      if (result.success) {
        this.connectedIds.delete(id)
        this.connectingIds.delete(id)
        await this.loadConnections()
      }
      return result
    },

    reportConnectionSuccess(id) {
      if (!id || (this.heartbeatFailures[id] || 0) === 0) return
      this.heartbeatFailures = { ...this.heartbeatFailures, [id]: 0 }
    },

    async reportConnectionFailure(id, error, threshold = 1) {
      if (!id || !isConnectionErrorMessage(error)) return null
      const failCount = (this.heartbeatFailures[id] || 0) + 1
      this.heartbeatFailures = { ...this.heartbeatFailures, [id]: failCount }
      const message = formatConnectionLostMessage(error)
      if (failCount < Math.max(1, threshold)) {
        return { success: false, disconnected: false, message }
      }
      this.connectedIds.delete(id)
      this.connectingIds.delete(id)
      try {
        await disconnectAPI(id)
      } catch (e) {}
      this.heartbeatFailures = { ...this.heartbeatFailures, [id]: 0 }
      return {
        success: false,
        disconnected: true,
        message,
      }
    },

    async handleConnectionFailure(id, error) {
      return await this.reportConnectionFailure(id, error, 1)
    },

    showGlobalToast(message, ok = true) {
      this.globalToast = message || ''
      this.globalToastOk = ok
    },

    clearGlobalToast() {
      this.globalToast = ''
    },

    async test(cfg) {
      // 前端兜底超时：防止 Go 的 TestConnection 在网络异常时永久卡住
      return await Promise.race([
        testConnection(cfg),
        new Promise((_, reject) =>
          setTimeout(() => reject(new Error('Connection test timeout after 15 seconds')), 15000)
        ),
      ])
    },

    async connect(id) {
      if (this.connectingIds.has(id)) {
        return { success: false, message: 'connecting' }
      }
      this.connectingIds.add(id)
      try {
        // 前端兜底超时：防止 Go 的 Connect 在网络异常时永久卡住
        const result = await Promise.race([
          connect(id),
          new Promise((_, reject) =>
            setTimeout(() => reject(new Error('Connection timeout after 15 seconds')), 15000)
          ),
        ])
        if (result.success) {
          this.connectedIds.add(id)
          this.heartbeatFailures = { ...this.heartbeatFailures, [id]: 0 }
        }
        return result
      } catch (e) {
        return { success: false, message: e.message || String(e) }
      } finally {
        this.connectingIds.delete(id)
      }
    },

    async disconnect(id) {
      await disconnectAPI(id)
      this.connectedIds.delete(id)
      this.connectingIds.delete(id)
      if (this.heartbeatFailures[id] !== undefined) {
        const nextFailures = { ...this.heartbeatFailures }
        delete nextFailures[id]
        this.heartbeatFailures = nextFailures
      }
    },

    setHeartbeatVisibility(visible) {
      this.heartbeatVisible = visible
    },

    stopHeartbeat() {
      this.heartbeatGeneration++
      if (this.heartbeatTimer) {
        clearTimeout(this.heartbeatTimer)
        this.heartbeatTimer = null
      }
      this.heartbeatRunning = false
      this.heartbeatInFlight = false
    },

    startHeartbeat(workspaceStore) {
      if (this.heartbeatRunning) return
      this.heartbeatRunning = true
      const generation = ++this.heartbeatGeneration
      this._scheduleHeartbeat(workspaceStore, generation)
    },

    _heartbeatIsCurrent(generation) {
      return this.heartbeatRunning && this.heartbeatGeneration === generation
    },

    _scheduleHeartbeat(workspaceStore, generation) {
      if (!this._heartbeatIsCurrent(generation) || this.heartbeatTimer || this.heartbeatInFlight) return
      this.heartbeatTimer = setTimeout(async () => {
        this.heartbeatTimer = null
        if (!this._heartbeatIsCurrent(generation)) return
        this.heartbeatInFlight = true
        try {
          if (this.heartbeatVisible) {
            await this._runHeartbeatCycle(workspaceStore, generation)
          }
        } finally {
          if (this._heartbeatIsCurrent(generation)) {
            this.heartbeatInFlight = false
            this._scheduleHeartbeat(workspaceStore, generation)
          }
        }
      }, 20000)
    },

    async _runHeartbeatCycle(workspaceStore, generation) {
      const ids = [...this.connectedIds]
      for (const id of ids) {
        if (!this._heartbeatIsCurrent(generation)) return
        if (!this.connectedIds.has(id) || this.connectingIds.has(id)) continue
        try {
          const result = await pingConnection(id)
          if (!this._heartbeatIsCurrent(generation) || !this.connectedIds.has(id)) continue
          if (result?.success) {
            const hadFailures = (this.heartbeatFailures[id] || 0) > 0
            this.reportConnectionSuccess(id)
            if (hadFailures && workspaceStore?.activeConnID === id) {
              this.showGlobalToast('当前 Redis 连接已恢复', true)
              await workspaceStore.refreshAfterReconnect(id)
            }
            continue
          }
          await this._recordHeartbeatFailure(id, result?.message || 'Redis connection lost', workspaceStore, generation)
        } catch (e) {
          if (!this._heartbeatIsCurrent(generation) || !this.connectedIds.has(id)) continue
          await this._recordHeartbeatFailure(id, e, workspaceStore, generation)
        }
      }
    },

    async _recordHeartbeatFailure(id, error, workspaceStore, generation) {
      if (!this._heartbeatIsCurrent(generation) || !this.connectedIds.has(id)) return
      const failure = await this.reportConnectionFailure(id, error, 2)
      if (!this._heartbeatIsCurrent(generation)) return
      if (failure?.disconnected && workspaceStore?.activeConnID === id && failure.message) {
        workspaceStore.applyConnectionLostState(id, failure.message)
        this.showGlobalToast('当前 Redis 连接已断开', false)
      }
    },
  },
})
