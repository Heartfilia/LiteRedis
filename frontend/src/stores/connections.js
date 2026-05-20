import { defineStore } from 'pinia'
import { listConnections, saveConnection, reorderConnections, deleteConnection, testConnection, connect, disconnect, isConnected } from '../api/wails.js'

export const useConnectionsStore = defineStore('connections', {
  state: () => ({
    connections: [],
    connectedIds: new Set(),
    connectingIds: new Set(),
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
      try {
        this.connections = await listConnections() || []
      } catch (e) {
        this.error = e.message || String(e)
      } finally {
        this.loading = false
      }
    },

    async save(cfg) {
      const result = await saveConnection(cfg)
      if (result.success) {
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
        }
        return result
      } catch (e) {
        return { success: false, message: e.message || String(e) }
      } finally {
        this.connectingIds.delete(id)
      }
    },

    async disconnect(id) {
      await disconnect(id)
      this.connectedIds.delete(id)
      this.connectingIds.delete(id)
    },
  },
})
