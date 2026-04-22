import { defineStore } from 'pinia'
import { listConnections, saveConnection, deleteConnection, testConnection, connect, disconnect, isConnected } from '../api/wails.js'

export const useConnectionsStore = defineStore('connections', {
  state: () => ({
    connections: [],
    connectedIds: new Set(),
    loading: false,
    error: null,
  }),

  getters: {
    isConnected: (state) => (id) => state.connectedIds.has(id),
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

    async remove(id) {
      // 断开连接
      this.connectedIds.delete(id)
      const result = await deleteConnection(id)
      if (result.success) {
        await this.loadConnections()
      }
      return result
    },

    async test(cfg) {
      return await testConnection(cfg)
    },

    async connect(id) {
      const result = await connect(id)
      if (result.success) {
        this.connectedIds.add(id)
      }
      return result
    },

    async disconnect(id) {
      await disconnect(id)
      this.connectedIds.delete(id)
    },
  },
})
