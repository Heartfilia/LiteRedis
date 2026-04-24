import { defineStore } from 'pinia'
import { scanKeys, getValue, getKeyInfo, deleteKey, renameKey, setTTL, selectDB, dbSize } from '../api/wails.js'
import { buildKeyTree } from '../utils/keyTree.js'
import { useSettingsStore } from './settings.js'

export const useWorkspaceStore = defineStore('workspace', {
  state: () => {
    // 从 localStorage 加载持久化的搜索历史
    let persistedHistory = {}
    try {
      const raw = localStorage.getItem('liteRedis_searchHistory')
      if (raw) persistedHistory = JSON.parse(raw)
    } catch (e) {}

    return {
      activeConnID: null,
      activeConnName: '',
      currentDB: 0,
      totalKeys: 0,

      keepPrevSearch: false,
      searchSessions: [],
      activeSessionId: null,

      selectedKey: null,
      keyValue: null,
      keyValueLoading: false,
      keyValueError: null,   // 加载失败的错误信息

      // 竞态控制：记录当前"有效"请求的 key，旧请求结果自动丢弃
      _loadingKey: null,

      // 每个连接的状态快照，key 为 connID
      connStates: {},

      // 每个连接的搜索历史，key 为 connID，value 为 pattern 数组（最多10条）
      connSearchHistory: persistedHistory,
    }
  },

  persistHistory: true,

  getters: {
    activeSession: (state) => state.searchSessions.find(s => s.id === state.activeSessionId),
    displaySessions: (state) => {
      if (state.keepPrevSearch) {
        // 合并模式：只展示非 * 的搜索结果（* 作为"全量刷新"不显示在合并列表里）
        return state.searchSessions.filter(s => (s.pattern || '*') !== '*')
      }
      const s = state.searchSessions[0]
      return s ? [s] : []
    },
  },

  actions: {
    setActiveConn(id, name, initDB = 0) {
      // 保存当前连接的状态快照
      if (this.activeConnID) {
        this.connStates[this.activeConnID] = {
          currentDB: this.currentDB,
          searchSessions: this.searchSessions,
          activeSessionId: this.activeSessionId,
          selectedKey: this.selectedKey,
          keyValue: this.keyValue,
          keyValueError: this.keyValueError,
          keyValueLoading: false,
        }
      }

      this.activeConnID = id
      this.activeConnName = name
      this._loadingKey = null

      // 若该连接有历史快照，恢复之
      if (id && this.connStates[id]) {
        const s = this.connStates[id]
        this.currentDB = s.currentDB
        this.searchSessions = s.searchSessions
        this.activeSessionId = s.activeSessionId
        this.selectedKey = s.selectedKey
        this.keyValue = s.keyValue
        this.keyValueError = s.keyValueError
        this.keyValueLoading = false
        return true
      }

      // 首次激活：使用默认初始状态
      this.currentDB = initDB
      this.searchSessions = []
      this.activeSessionId = null
      this.selectedKey = null
      this.keyValue = null
      this.keyValueError = null
      this.keyValueLoading = false
      return false
    },

    async fetchTotalKeys() {
      if (!this.activeConnID) return
      try {
        this.totalKeys = await dbSize(this.activeConnID)
      } catch (e) {
        this.totalKeys = 0
      }
    },

    _loadSearchHistory() {
      try {
        const raw = localStorage.getItem('liteRedis_searchHistory')
        if (raw) {
          this.connSearchHistory = JSON.parse(raw)
        }
      } catch (e) {
        this.connSearchHistory = {}
      }
    },

    _saveSearchHistory() {
      try {
        localStorage.setItem('liteRedis_searchHistory', JSON.stringify(this.connSearchHistory))
      } catch (e) {}
    },

    _recordSearchHistory(pattern) {
      if (!this.activeConnID || !pattern) return
      const p = pattern.trim()
      if (!p || p === '*') return
      let list = this.connSearchHistory[this.activeConnID] || []
      list = list.filter(item => item !== p)
      list.unshift(p)
      if (list.length > 10) list = list.slice(0, 10)
      this.connSearchHistory[this.activeConnID] = list
      this._saveSearchHistory()
    },

    async search(pattern) {
      if (!this.activeConnID) return
      this._recordSearchHistory(pattern)
      const settingsStore = useSettingsStore()
      const sessionId = Date.now().toString()
      const session = {
        id: sessionId,
        pattern: pattern || '*',
        keys: [],
        treeData: [],
        loading: true,
        error: null,
        cursor: 0,
        hasMore: false,
      }

      if (!this.keepPrevSearch) {
        this.searchSessions = [session]
      } else {
        // keepPrevSearch = true 时，如果 pattern 是 * 则不保留（直接替换全部）
        if ((pattern || '*') === '*') {
          this.searchSessions = [session]
        } else {
          this.searchSessions = this.searchSessions.filter(s => s.pattern !== session.pattern)
          this.searchSessions.push(session)
        }
      }
      this.activeSessionId = sessionId

      try {
        const count = settingsStore.loaded ? settingsStore.keyScanCount : 0
        const result = await scanKeys(this.activeConnID, pattern || '*', count, 0)
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            keys: result.keys || [],
            treeData: buildKeyTree(result.keys || []),
            loading: false,
            cursor: result.next_cursor || 0,
            hasMore: result.has_more || false,
          }
        }
      } catch (e) {
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            loading: false,
            error: e.message || String(e),
          }
        }
      }
    },

    async loadMoreKeys(sessionId) {
      const session = this.searchSessions.find(s => s.id === sessionId)
      if (!session || !session.hasMore || session.loading) return
      const settingsStore = useSettingsStore()
      const count = settingsStore.loaded ? settingsStore.keyScanCount : 0

      const idx = this.searchSessions.findIndex(s => s.id === sessionId)
      if (idx !== -1) {
        this.searchSessions[idx] = { ...this.searchSessions[idx], loading: true }
      }

      try {
        const result = await scanKeys(this.activeConnID, session.pattern, count, session.cursor)
        if (idx !== -1) {
          const mergedKeys = [...this.searchSessions[idx].keys, ...(result.keys || [])]
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            keys: mergedKeys,
            treeData: buildKeyTree(mergedKeys),
            loading: false,
            cursor: result.next_cursor || 0,
            hasMore: result.has_more || false,
          }
        }
      } catch (e) {
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            loading: false,
            error: e.message || String(e),
          }
        }
      }
    },

    /**
     * 精确 key 搜索：不走 SCAN，直接用 GetKeyInfo 构造只含这一个 key 的 session，
     * 并同时触发右侧 value 加载。key 不存在时 session 显示为空列表。
     */
    async searchExact(key) {
      if (!this.activeConnID) return
      this._recordSearchHistory(key)
      const sessionId = Date.now().toString()
      const session = {
        id: sessionId,
        pattern: key,
        keys: [],
        treeData: [],
        loading: true,
        error: null,
      }
      if (!this.keepPrevSearch) {
        this.searchSessions = [session]
      } else {
        this.searchSessions = this.searchSessions.filter(s => s.pattern !== key)
        this.searchSessions.push(session)
      }
      this.activeSessionId = sessionId

      try {
        const info = await getKeyInfo(this.activeConnID, key)
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        // info.name 是 key 名，info.type === 'none' 表示 key 不存在
        const exists = info && info.name && info.type && info.type !== 'none'
        if (idx !== -1) {
          const keys = exists ? [info] : []
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            keys,
            treeData: buildKeyTree(keys),
            loading: false,
          }
        }
        if (exists) {
          await this.selectKey(key)
        }
      } catch (e) {
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            loading: false,
            error: e.message || String(e),
          }
        }
      }
    },

    closeSession(id) {
      this.searchSessions = this.searchSessions.filter(s => s.id !== id)
      if (this.activeSessionId === id) {
        this.activeSessionId = this.searchSessions.at(-1)?.id ?? null
      }
    },

    removeSession(sessionId) {
      this.searchSessions = this.searchSessions.filter(s => s.id !== sessionId)
    },

    /**
     * 选中一个 key，立即更新 selectedKey（UI 即时响应），
     * 异步加载 value。使用"令牌"机制丢弃过时的响应：
     * 若用户在本次请求返回前又点击了其他 key，本次结果自动丢弃。
     */
    async selectKey(key) {
      // 立即切换选中状态，让 UI 即时响应
      this.selectedKey = key
      this.keyValue = null
      this.keyValueError = null
      this.keyValueLoading = true
      this._loadingKey = key   // 设置本次令牌

      try {
        const result = await getValue(this.activeConnID, key, 0, 0)

        // 竞态检查：如果用户在等待过程中又点击了别的 key，丢弃本次结果
        if (this._loadingKey !== key) return

        this.keyValue = result
        this.keyValueError = null
      } catch (e) {
        // 同样检查竞态
        if (this._loadingKey !== key) return

        this.keyValue = null
        this.keyValueError = e.message || String(e)
      } finally {
        // 仅当本次令牌仍有效时才清除 loading
        if (this._loadingKey === key) {
          this.keyValueLoading = false
          this._loadingKey = null
        }
      }
    },

    async deleteCurrentKey() {
      if (!this.selectedKey || !this.activeConnID) return
      await deleteKey(this.activeConnID, this.selectedKey)
      if (this.activeSession) {
        await this.search(this.activeSession.pattern)
      }
      this.selectedKey = null
      this.keyValue = null
      this.keyValueError = null
    },

    async renameCurrentKey(newKey) {
      if (!this.selectedKey || !this.activeConnID) return
      const result = await renameKey(this.activeConnID, this.selectedKey, newKey)
      if (result.success) {
        if (this.activeSession) {
          await this.search(this.activeSession.pattern)
        }
        this.selectedKey = newKey
        await this.selectKey(newKey)
      }
      return result
    },

    async updateTTL(ttlSec) {
      if (!this.selectedKey || !this.activeConnID) return
      const result = await setTTL(this.activeConnID, this.selectedKey, ttlSec)
      if (result.success && this.keyValue) {
        this.keyValue.ttl = ttlSec
      }
      return result
    },

    async switchDB(db) {
      if (!this.activeConnID) return
      const result = await selectDB(this.activeConnID, db)
      if (result.success) {
        this.currentDB = db
        this.searchSessions = []
        this.activeSessionId = null
        this.selectedKey = null
        this.keyValue = null
        this.keyValueError = null
        this.keyValueLoading = false
        this._loadingKey = null
        await this.fetchTotalKeys()
      }
      return result
    },

    clearSearchHistory(connId) {
      if (!connId) return
      delete this.connSearchHistory[connId]
      this._saveSearchHistory()
    },
  },
})
