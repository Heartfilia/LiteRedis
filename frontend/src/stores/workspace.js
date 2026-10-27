import { defineStore } from 'pinia'
import { scanKeys, getValue, getKeyInfo, deleteKey, renameKey, setTTL, selectDB, dbSize, createKey, batchDeleteKeys, batchSetKeysTTL, batchRenamePrefix } from '../api/wails.js'
import { buildKeyTree } from '../utils/keyTree.js'
import { useSettingsStore } from './settings.js'
import { useConnectionsStore } from './connections.js'
import { isConnectionErrorMessage, formatConnectionLostMessage } from '../utils/connection.js'

function ensureConnSearchEntry(entry) {
  if (Array.isArray(entry)) {
    return {
      pinned: [],
      history: entry.filter(item => typeof item === 'string' && item.trim()),
    }
  }

  if (!entry || typeof entry !== 'object') {
    return { pinned: [], history: [] }
  }

  const pinned = Array.isArray(entry.pinned) ? entry.pinned.filter(item => typeof item === 'string' && item.trim()) : []
  const pinnedSet = new Set(pinned)
  const history = Array.isArray(entry.history)
    ? entry.history.filter(item => typeof item === 'string' && item.trim() && !pinnedSet.has(item))
    : []

  const pinnedUnchanged =
    Array.isArray(entry.pinned) &&
    entry.pinned.length === pinned.length &&
    entry.pinned.every((item, idx) => item === pinned[idx])
  const historyUnchanged =
    Array.isArray(entry.history) &&
    entry.history.length === history.length &&
    entry.history.every((item, idx) => item === history[idx])

  if (pinnedUnchanged && historyUnchanged) {
    return entry
  }

  return { pinned, history }
}

function normalizePersistedSearchHistory(raw) {
  if (!raw || typeof raw !== 'object') return {}
  const normalized = {}
  for (const [connId, entry] of Object.entries(raw)) {
    normalized[connId] = ensureConnSearchEntry(entry)
  }
  return normalized
}

function collectExpandableNodePaths(treeData = [], output = []) {
  for (const node of treeData) {
    if (!node || node.isLeaf) continue
    if (node.fullPath) {
      output.push(node.fullPath)
    }
    if (Array.isArray(node.children) && node.children.length) {
      collectExpandableNodePaths(node.children, output)
    }
  }
  return output
}

const SHORTCUT_STORAGE_KEY = 'liteRedis_shortcuts'

// 只持久化收藏；最近访问是本次运行期间的临时数据，关掉应用即清空
function loadShortcuts() {
  try {
    const raw = localStorage.getItem(SHORTCUT_STORAGE_KEY)
    if (!raw) return { favorites: {}, recent: {} }
    const parsed = JSON.parse(raw)
    return { favorites: parsed.favorites || {}, recent: {} }
  } catch (e) {
    return { favorites: {}, recent: {} }
  }
}

export const useWorkspaceStore = defineStore('workspace', {
  state: () => {
    // 从 localStorage 加载持久化的搜索历史
    let persistedHistory = {}
    try {
      const raw = localStorage.getItem('liteRedis_searchHistory')
      if (raw) {
        const parsed = JSON.parse(raw)
        persistedHistory = normalizePersistedSearchHistory(parsed)
      }
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
      // 多选模式：批量删除 / 批量设置 TTL
      selectionMode: false,
      selectedKeys: [],
      // Shift 连选的锚点
      selectionAnchor: null,
      // 收藏持久化到 localStorage按；最近访问只存在内存里，关闭应用即清空
      shortcuts: loadShortcuts(),
      keyValue: null,
      keyValueLoading: false,
      keyValueRefreshing: false,
      keyValueError: null,   // 加载失败的错误信息

      // 竞态控制：记录当前"有效"请求的 key，旧请求结果自动丢弃
      _loadingKey: null,
      _requestEpoch: 0,
      _requestSequence: 0,
      _requestTokens: {},
      _selectionSequence: 0,

      // 最近一次需要自动展开层级的搜索 session id（有有效数据且 pattern 不是 * 时设置）
      _autoExpandSessionId: null,

      // 每个连接的状态快照，key 为 connID
      connStates: {},

      // 每个连接展开的目录节点，key 为 connID，value 为 { [fullPath]: boolean }
      expandedNodes: {},

      // 每个连接的搜索历史，key 为 connID，value 为 { history: [], pinned: [] }
      connSearchHistory: persistedHistory,

      // 编辑器内搜索状态缓存，key 为 'connID:keyType:key'，value 为 { query, fuzzy }
      editorSearchStates: {},
    }
  },

  persistHistory: true,

  getters: {
    activeSession: (state) => state.searchSessions.find(s => s.id === state.activeSessionId),
    selectedKeySet: (state) => new Set(state.selectedKeys),

    shortcutScope: (state) => String(state.activeConnID || '') + '|' + String(state.currentDB || 0),

    favoriteKeys() {
      return this.shortcuts.favorites[this.shortcutScope] || []
    },

    favoriteKeySet() {
      return new Set(this.favoriteKeys)
    },

    recentKeyList(state) {
      const list = this.shortcuts.recent[this.shortcutScope] || []
      return list.filter(key => key !== state.selectedKey)
    },

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
    _beginRequest(scope, options = {}) {
      const sequence = ++this._requestSequence
      const token = {
        scope,
        sequence,
        epoch: this._requestEpoch,
        connID: this.activeConnID,
        db: this.currentDB,
        key: options.key ?? null,
        requireSelectedKey: options.requireSelectedKey === true,
      }
      this._requestTokens[scope] = sequence
      return token
    },

    _isRequestCurrent(token) {
      if (!token) return false
      if (token.epoch !== this._requestEpoch) return false
      if (this._requestTokens[token.scope] !== token.sequence) return false
      if (this.activeConnID !== token.connID || this.currentDB !== token.db) return false
      if (token.requireSelectedKey && this.selectedKey !== token.key) return false
      return true
    },

    _finishRequest(token) {
      if (token && this._requestTokens[token.scope] === token.sequence) {
        delete this._requestTokens[token.scope]
      }
    },

    _invalidateRequestScope(scope) {
      delete this._requestTokens[scope]
    },

    _invalidateRequests() {
      this._requestEpoch++
      this._requestTokens = {}
      this._loadingKey = null
    },

    async _handleConnectionFailure(error, options = {}) {
      if (!this.activeConnID || !isConnectionErrorMessage(error)) return null
      const connectionsStore = useConnectionsStore()
      await connectionsStore.handleConnectionFailure(this.activeConnID, error)
      const message = formatConnectionLostMessage(error)
      if (options.clearKeyValue !== false) {
        this.keyValue = null
      }
      if (options.setKeyError !== false) {
        this.keyValueError = message
      }
      this.keyValueLoading = false
      this.keyValueRefreshing = false
      this._loadingKey = null
      return {
        success: false,
        disconnected: true,
        message,
      }
    },

    applyConnectionLostState(connId, message) {
      if (!connId || this.activeConnID !== connId) return
      this._invalidateRequests()
      this.searchSessions = this.searchSessions.map(session => ({
        ...session,
        loading: false,
        error: message || session.error || null,
      }))
      this.keyValue = null
      this.keyValueLoading = false
      this.keyValueRefreshing = false
      this.keyValueError = message
      this._loadingKey = null
    },

    async refreshAfterReconnect(connId) {
      if (!connId || this.activeConnID !== connId) return
      const request = this._beginRequest('reconnect')
      this.keyValueError = null
      const connectionsStore = useConnectionsStore()
      const conn = connectionsStore.connections.find(item => item.id === connId)
      const clusterExactOnly = !!conn?.is_cluster && !conn?.allow_cluster_scan

      // A new Redis client starts on the configured default DB. Restore the
      // workspace DB before issuing any reads so UI state and backend agree.
      if (conn && !conn.is_cluster) {
        const targetDB = Number.isInteger(Number(this.currentDB)) ? Number(this.currentDB) : Number(conn.db || 0)
        const result = await selectDB(connId, targetDB)
        if (!this._isRequestCurrent(request)) return
        if (!result?.success) {
          const failure = await this._handleConnectionFailure(result?.message, {
            clearKeyValue: false,
            setKeyError: false,
          })
          this.keyValueError = failure?.message || result?.message || 'Failed to restore Redis DB'
          this._finishRequest(request)
          return result
        }
      }

      if (this.activeSession?.pattern) {
        const pattern = this.activeSession.pattern
        if (clusterExactOnly && pattern === '*') {
          this.searchSessions = []
          this.activeSessionId = null
        } else if (pattern !== '*') {
          await this.search(pattern)
        } else {
          await this.search('*')
        }
      } else if (!clusterExactOnly) {
        await this.search('*')
      }

      if (!this._isRequestCurrent(request)) return

      if (this.selectedKey) {
        await this.selectKey(this.selectedKey)
      }

      if (!this._isRequestCurrent(request)) return
      await this.fetchTotalKeys()
      this._finishRequest(request)
    },

    _patternToRegExp(pattern) {
      const normalized = pattern && pattern.trim() ? pattern.trim() : '*'
      const escaped = normalized.replace(/[.+^${}()|[\]\\]/g, '\\$&')
      const regexBody = escaped.replace(/\*/g, '.*').replace(/\?/g, '.')
      return new RegExp(`^${regexBody}$`)
    },

    // 从当前搜索结果里找 key 的类型，省掉后端一次 TYPE 往返
    _findKeyType(keyName) {
      if (!keyName) return ''
      for (const session of this.searchSessions) {
        const hit = (session.keys || []).find(item => item?.name === keyName)
        if (hit?.type) return hit.type
      }
      return ''
    },

    _sessionMatchesKey(session, keyName) {
      if (!session || !keyName) return false
      const pattern = session.pattern || '*'
      try {
        return this._patternToRegExp(pattern).test(keyName)
      } catch (e) {
        return pattern === '*' || pattern === keyName
      }
    },

    _upsertKeyIntoSessions(keyInfo) {
      if (!keyInfo?.name) return false
      let inserted = false
      this.searchSessions = this.searchSessions.map(session => {
        if (!this._sessionMatchesKey(session, keyInfo.name)) {
          return session
        }
        const nextKeys = [...session.keys]
        const existingIdx = nextKeys.findIndex(item => item.name === keyInfo.name)
        if (existingIdx === -1) {
          nextKeys.push(keyInfo)
          inserted = true
        } else {
          nextKeys[existingIdx] = { ...nextKeys[existingIdx], ...keyInfo }
          inserted = true
        }
        return {
          ...session,
          keys: nextKeys,
          treeData: buildKeyTree(nextKeys),
        }
      })
      return inserted
    },

    // 只改一个节点的元信息（TTL / 类型），直接就地打补丁：
    // 重建整棵树会让所有节点对象变新，导致整棵树重新渲染
    _patchTreeNode(nodes, fullPath, patch) {
      for (const node of nodes || []) {
        if (node.fullPath === fullPath) {
          Object.assign(node, patch)
          return true
        }
        if (!node.isLeaf && node.children?.length) {
          if (this._patchTreeNode(node.children, fullPath, patch)) return true
        }
      }
      return false
    },

    _updateKeyMetadata(keyName, patch) {
      if (!keyName || !patch || typeof patch !== 'object') return false
      let updated = false
      this.searchSessions = this.searchSessions.map(session => {
        let sessionUpdated = false
        const nextKeys = session.keys.map(item => {
          if (item?.name !== keyName) return item
          updated = true
          sessionUpdated = true
          return { ...item, ...patch }
        })
        if (!sessionUpdated) return session
        const patched = this._patchTreeNode(session.treeData, keyName, patch)
        return {
          ...session,
          keys: nextKeys,
          // 没找到节点（理论上不会）才退回重建
          treeData: patched ? session.treeData : buildKeyTree(nextKeys),
        }
      })
      return updated
    },

    setActiveConn(id, name, initDB = 0) {
      this._invalidateRequests()
      // 切换连接必须清空多选，否则批量操作会作用到另一个连接的同名 key
      this.selectionMode = false
      this.selectedKeys = []
      this.selectionAnchor = null
      // 保存当前连接的状态快照
      if (this.activeConnID) {
        this.connStates[this.activeConnID] = {
          currentDB: this.currentDB,
          totalKeys: this.totalKeys,
          keepPrevSearch: this.keepPrevSearch,
          searchSessions: this.searchSessions,
          activeSessionId: this.activeSessionId,
          selectedKey: this.selectedKey,
          keyValue: this.keyValue,
          keyValueError: this.keyValueError,
          keyValueLoading: false,
          expandedNodes: { ...(this.expandedNodes[this.activeConnID] || {}) },
        }
      }

      this.activeConnID = id
      this.activeConnName = name
      this._loadingKey = null

      // 若该连接有历史快照，恢复之
      if (id && this.connStates[id]) {
        const s = this.connStates[id]
        this.currentDB = s.currentDB
        this.totalKeys = s.totalKeys ?? 0
        this.keepPrevSearch = s.keepPrevSearch ?? false
        this.searchSessions = s.searchSessions
        this.activeSessionId = s.activeSessionId
        this.selectedKey = s.selectedKey
        this.keyValue = s.keyValue
        this.keyValueError = s.keyValueError
        this.keyValueLoading = false
        this.keyValueRefreshing = false
        this.expandedNodes[id] = { ...(s.expandedNodes || this.expandedNodes[id] || {}) }
        return true
      }

      // 首次激活：使用默认初始状态
      this.currentDB = initDB
      this.totalKeys = 0
      this.searchSessions = []
      this.activeSessionId = null
      this.selectedKey = null
      this.keyValue = null
      this.keyValueError = null
      this.keyValueLoading = false
      this.keyValueRefreshing = false
      if (id && !this.expandedNodes[id]) {
        this.expandedNodes[id] = {}
      }
      return false
    },

    isNodeExpanded(fullPath, depth = 0) {
      const connMap = this.expandedNodes[this.activeConnID] || {}
      if (Object.prototype.hasOwnProperty.call(connMap, fullPath)) {
        return !!connMap[fullPath]
      }
      if (this.keepPrevSearch) return true
      const session = this.activeSession
      // 搜索了具体 pattern 且有有效数据时，默认全部展开
      if (
        session &&
        session.id === this._autoExpandSessionId &&
        session.keys?.length > 0 &&
        session.pattern &&
        session.pattern !== '*'
      ) {
        return true
      }
      return depth < 1
    },

    setNodeExpanded(fullPath, expanded) {
      if (!this.activeConnID || !fullPath) return
      const connMap = this.expandedNodes[this.activeConnID] || {}
      this.expandedNodes[this.activeConnID] = {
        ...connMap,
        [fullPath]: expanded,
      }
      // 用户手动干预展开状态后，不再自动全部展开
      this._autoExpandSessionId = null
    },

    setVisibleTreeExpanded(treeData, expanded) {
      if (!this.activeConnID || !Array.isArray(treeData) || treeData.length === 0) return
      const paths = collectExpandableNodePaths(treeData)
      if (!paths.length) return
      const connMap = this.expandedNodes[this.activeConnID] || {}
      const nextMap = { ...connMap }
      for (const path of paths) {
        nextMap[path] = expanded
      }
      this.expandedNodes[this.activeConnID] = nextMap
      this._autoExpandSessionId = null
    },

    async fetchTotalKeys() {
      if (!this.activeConnID) return
      const request = this._beginRequest('totalKeys')
      try {
        const total = await dbSize(request.connID)
        if (!this._isRequestCurrent(request)) return
        this.totalKeys = total
      } catch (e) {
        if (!this._isRequestCurrent(request)) return
        await this._handleConnectionFailure(e)
      } finally {
        this._finishRequest(request)
      }
    },

    _loadSearchHistory() {
      try {
        const raw = localStorage.getItem('liteRedis_searchHistory')
        if (raw) {
          this.connSearchHistory = normalizePersistedSearchHistory(JSON.parse(raw))
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

    getConnSearchHistory(connId) {
      const normalized = ensureConnSearchEntry(this.connSearchHistory[connId])
      if (this.connSearchHistory[connId] !== normalized) {
        this.connSearchHistory[connId] = normalized
      }
      return normalized
    },

    _recordSearchHistory(pattern) {
      if (!this.activeConnID || !pattern) return
      const p = pattern.trim()
      if (!p || p === '*') return
      const settingsStore = useSettingsStore()
      const limit = settingsStore.loaded ? settingsStore.searchHistoryLimit : 10
      const entry = this.getConnSearchHistory(this.activeConnID)
      if (entry.pinned.includes(p)) {
        this._saveSearchHistory()
        return
      }
      let list = entry.history.filter(item => item !== p)
      list.unshift(p)
      if (list.length > limit) list = list.slice(0, limit)
      this.connSearchHistory[this.activeConnID] = {
        pinned: [...entry.pinned],
        history: list,
      }
      this._saveSearchHistory()
    },

    togglePinnedSearchHistory(connId, pattern) {
      if (!connId || !pattern) return
      const p = pattern.trim()
      if (!p || p === '*') return
      const settingsStore = useSettingsStore()
      const limit = settingsStore.loaded ? settingsStore.searchHistoryLimit : 10
      const entry = this.getConnSearchHistory(connId)
      const isPinned = entry.pinned.includes(p)

      if (isPinned) {
        const pinned = entry.pinned.filter(item => item !== p)
        const history = [p, ...entry.history.filter(item => item !== p)].slice(0, limit)
        this.connSearchHistory[connId] = { pinned, history }
      } else {
        const pinned = [p, ...entry.pinned.filter(item => item !== p)]
        const history = entry.history.filter(item => item !== p)
        this.connSearchHistory[connId] = { pinned, history }
      }

      this._saveSearchHistory()
    },

    trimAllSearchHistory(limit) {
      const normalizedLimit = Number(limit)
      if (!Number.isFinite(normalizedLimit) || normalizedLimit < 1) return
      const next = {}
      for (const [connId, history] of Object.entries(this.connSearchHistory || {})) {
        const entry = ensureConnSearchEntry(history)
        next[connId] = {
          pinned: [...entry.pinned],
          history: entry.history.slice(0, normalizedLimit),
        }
      }
      this.connSearchHistory = next
      this._saveSearchHistory()
    },

    async search(pattern) {
      if (!this.activeConnID) return
      // 新的搜索会替换当前列表，旧选中项已不可见，必须清空，
      // 否则批量删除会作用到看不见的 key 上
      this.selectionMode = false
      this.selectedKeys = []
      this.selectionAnchor = null
      this._recordSearchHistory(pattern)
      const settingsStore = useSettingsStore()
      const sessionId = `${Date.now()}-${++this._requestSequence}`
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
      const request = this._beginRequest(`keys:${sessionId}`)

      try {
        const count = settingsStore.loaded ? settingsStore.keyScanCount : 0
        const result = await scanKeys(request.connID, pattern || '*', count, 0)
        if (!this._isRequestCurrent(request)) return
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        const keys = result.keys || []
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            keys,
            treeData: buildKeyTree(keys),
            loading: false,
            cursor: result.next_cursor || 0,
            hasMore: result.has_more || false,
          }
        }
        // 搜索了具体 pattern 且有有效数据时，标记为自动展开
        if ((pattern || '*') !== '*' && keys.length > 0) {
          this._autoExpandSessionId = sessionId
        }
      } catch (e) {
        if (!this._isRequestCurrent(request)) return
        await this._handleConnectionFailure(e, { clearKeyValue: false, setKeyError: false })
        if (!this._isRequestCurrent(request)) return
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            loading: false,
            error: e.message || String(e),
          }
        }
      } finally {
        this._finishRequest(request)
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
      const request = this._beginRequest(`keys:${sessionId}`)

      try {
        const result = await scanKeys(request.connID, session.pattern, count, session.cursor)
        if (!this._isRequestCurrent(request)) return
        const currentIdx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (currentIdx !== -1) {
          const mergedKeys = [...this.searchSessions[currentIdx].keys, ...(result.keys || [])]
          this.searchSessions[currentIdx] = {
            ...this.searchSessions[currentIdx],
            keys: mergedKeys,
            treeData: buildKeyTree(mergedKeys),
            loading: false,
            cursor: result.next_cursor || 0,
            hasMore: result.has_more || false,
          }
        }
      } catch (e) {
        if (!this._isRequestCurrent(request)) return
        await this._handleConnectionFailure(e, { clearKeyValue: false, setKeyError: false })
        if (!this._isRequestCurrent(request)) return
        const currentIdx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (currentIdx !== -1) {
          this.searchSessions[currentIdx] = {
            ...this.searchSessions[currentIdx],
            loading: false,
            error: e.message || String(e),
          }
        }
      } finally {
        this._finishRequest(request)
      }
    },

    /**
     * 精确 key 搜索：不走 SCAN，直接用 GetKeyInfo 构造只含这一个 key 的 session，
     * 并同时触发右侧 value 加载。key 不存在时 session 显示为空列表。
     */
    async searchExact(key) {
      if (!this.activeConnID) return
      this._recordSearchHistory(key)
      const selectionSequence = this._selectionSequence
      const sessionId = `${Date.now()}-${++this._requestSequence}`
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
      const request = this._beginRequest(`keys:${sessionId}`)

      try {
        const info = await getKeyInfo(request.connID, key)
        if (!this._isRequestCurrent(request)) return
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
          this._autoExpandSessionId = sessionId
          if (this._selectionSequence === selectionSequence) {
            await this.selectKey(key)
          }
        }
      } catch (e) {
        if (!this._isRequestCurrent(request)) return
        await this._handleConnectionFailure(e, { clearKeyValue: false, setKeyError: false })
        if (!this._isRequestCurrent(request)) return
        const idx = this.searchSessions.findIndex(s => s.id === sessionId)
        if (idx !== -1) {
          this.searchSessions[idx] = {
            ...this.searchSessions[idx],
            loading: false,
            error: e.message || String(e),
          }
        }
      } finally {
        this._finishRequest(request)
      }
    },

    closeSession(id) {
      this._invalidateRequestScope(`keys:${id}`)
      this.searchSessions = this.searchSessions.filter(s => s.id !== id)
      if (this.activeSessionId === id) {
        this.activeSessionId = this.searchSessions.at(-1)?.id ?? null
      }
      if (!this.searchSessions.length) {
        this._invalidateRequestScope('keyValue')
        this.selectedKey = null
        this.keyValue = null
        this.keyValueError = null
        this.keyValueLoading = false
        this.keyValueRefreshing = false
        this._loadingKey = null
      }
    },

    removeSession(sessionId) {
      this._invalidateRequestScope(`keys:${sessionId}`)
      this.searchSessions = this.searchSessions.filter(s => s.id !== sessionId)
      if (this.activeSessionId === sessionId) {
        this.activeSessionId = this.searchSessions.at(-1)?.id ?? null
      }
      if (!this.searchSessions.length) {
        this._invalidateRequestScope('keyValue')
        this.selectedKey = null
        this.keyValue = null
        this.keyValueError = null
        this.keyValueLoading = false
        this.keyValueRefreshing = false
        this._loadingKey = null
      }
    },

    /**
     * 选中一个 key，立即更新 selectedKey（UI 即时响应），
     * 异步加载 value。使用"令牌"机制丢弃过时的响应：
     * 若用户在本次请求返回前又点击了其他 key，本次结果自动丢弃。
     */
    async selectKey(key, options = {}) {
      const preserveCurrentValue = options?.preserveCurrentValue === true
      this._selectionSequence++
      // 立即切换选中状态，让 UI 即时响应
      this.selectedKey = key
      if (key) this.recordRecentKey(key)
      this.keyValueError = null
      if (!preserveCurrentValue) {
        this.keyValue = null
      }
      this.keyValueLoading = !preserveCurrentValue
      this.keyValueRefreshing = preserveCurrentValue
      const request = this._beginRequest('keyValue', { key, requireSelectedKey: true })
      this._loadingKey = key

      try {
        const result = await getValue(request.connID, key, 0, 0, '', '', this._findKeyType(key))

        if (!this._isRequestCurrent(request)) return

        this.keyValue = result
        this.keyValueError = null
      } catch (e) {
        if (!this._isRequestCurrent(request)) return

        if (!preserveCurrentValue) {
          this.keyValue = null
        }
        const disconnected = await this._handleConnectionFailure(e)
        this.keyValueError = disconnected?.message || e.message || String(e)
      } finally {
        if (this._isRequestCurrent(request)) {
          this.keyValueLoading = false
          this.keyValueRefreshing = false
          this._loadingKey = null
        }
        this._finishRequest(request)
      }
    },

    _persistShortcuts() {
      try {
        // 只写收藏，最近访问不落盘
        localStorage.setItem(SHORTCUT_STORAGE_KEY, JSON.stringify({ favorites: this.shortcuts.favorites }))
      } catch (e) {}
    },

    toggleFavorite(key) {
      if (!key || !this.activeConnID) return
      const scope = this.shortcutScope
      const current = this.shortcuts.favorites[scope] || []
      const next = current.includes(key) ? current.filter(item => item !== key) : [...current, key]
      this.shortcuts = {
        ...this.shortcuts,
        favorites: { ...this.shortcuts.favorites, [scope]: next },
      }
      this._persistShortcuts()
    },

    recordRecentKey(key) {
      if (!key || !this.activeConnID) return
      const scope = this.shortcutScope
      const current = this.shortcuts.recent[scope] || []
      if (current[0] === key) return
      const next = [key, ...current.filter(item => item !== key)].slice(0, 11)
      this.shortcuts = {
        ...this.shortcuts,
        recent: { ...this.shortcuts.recent, [scope]: next },
      }
      this._persistShortcuts()
    },

    toggleSelectionMode() {
      this.selectionMode = !this.selectionMode
      this.selectedKeys = []
      this.selectionAnchor = null
    },

    toggleKeySelection(key) {
      if (!key) return
      const index = this.selectedKeys.indexOf(key)
      if (index === -1) {
        this.selectedKeys = [...this.selectedKeys, key]
      } else {
        this.selectedKeys = this.selectedKeys.filter(item => item !== key)
      }
      this.selectionAnchor = key
    },

    // 整目录勾选：全部已选则取消，否则全选
    toggleKeysSelection(keys) {
      const list = Array.isArray(keys) ? keys.filter(Boolean) : []
      if (!list.length) return
      const selected = this.selectedKeySet
      const allSelected = list.every(key => selected.has(key))
      const next = new Set(this.selectedKeys)
      for (const key of list) {
        if (allSelected) next.delete(key)
        else next.add(key)
      }
      this.selectedKeys = Array.from(next)
      this.selectionAnchor = list[list.length - 1]
    },

    // Shift 连选：按当前可见顺序把锚点到目标之间的 key 全部加入选中
    selectKeyRangeTo(key) {
      if (!key) return
      const order = this.visibleLeafKeys()
      const anchorIndex = this.selectionAnchor ? order.indexOf(this.selectionAnchor) : -1
      const targetIndex = order.indexOf(key)
      if (anchorIndex === -1 || targetIndex === -1) {
        this.toggleKeySelection(key)
        return
      }
      const [start, end] = anchorIndex < targetIndex ? [anchorIndex, targetIndex] : [targetIndex, anchorIndex]
      const next = new Set(this.selectedKeys)
      for (let i = start; i <= end; i++) next.add(order[i])
      this.selectedKeys = Array.from(next)
    },

    // 当前屏幕上可见的叶子 key 顺序，用于 Shift 连选
    visibleLeafKeys() {
      const order = []
      if (useSettingsStore().keyDisplayMode === 'flat') {
        for (const session of this.displaySessions) {
          for (const item of session.keys || []) {
            if (item?.name) order.push(item.name)
          }
        }
        return order
      }
      const walk = (nodes, depth) => {
        for (const node of nodes || []) {
          if (!node) continue
          if (node.isLeaf) {
            order.push(node.fullPath)
            continue
          }
          if (this.isNodeExpanded(node.fullPath, depth)) {
            walk(node.children, depth + 1)
          }
        }
      }
      for (const session of this.displaySessions) {
        walk(session.treeData, 0)
      }
      return order
    },

    // 目标连接的缓存快照失效，下次切过去重新拉取（复制数据后避免看到旧列表）
    invalidateConnState(id) {
      if (!id) return
      if (this.connStates[id]) {
        const next = { ...this.connStates }
        delete next[id]
        this.connStates = next
      }
    },

    clearSelection() {
      this.selectedKeys = []
      this.selectionAnchor = null
    },

    _dropKeysFromSessions(keys) {
      const drop = new Set(keys)
      this.searchSessions = this.searchSessions.map(session => {
        const nextKeys = session.keys.filter(item => !drop.has(item?.name))
        return { ...session, keys: nextKeys, treeData: buildKeyTree(nextKeys) }
      })
      if (this.selectedKey && drop.has(this.selectedKey)) {
        this.selectedKey = null
        this.keyValue = null
        this.keyValueError = null
      }
    },

    async batchDeleteSelected() {
      const keys = [...this.selectedKeys]
      if (!keys.length || !this.activeConnID) return { success: false }
      const connID = this.activeConnID
      try {
        const result = await batchDeleteKeys(connID, keys)
        if (!result?.success) {
          useConnectionsStore().showGlobalToast(result?.message || 'Batch delete failed', false)
          return result || { success: false }
        }
        if (connID !== this.activeConnID) return result
        this._dropKeysFromSessions(keys)
        this.totalKeys = Math.max(0, (this.totalKeys || 0) - (result.affected || 0))
        this.selectedKeys = []
        return result
      } catch (e) {
        useConnectionsStore().showGlobalToast(e?.message || String(e), false)
        return { success: false, message: e?.message || String(e) }
      }
    },

    async batchSetTTLSelected(ttlSec) {
      const keys = [...this.selectedKeys]
      if (!keys.length || !this.activeConnID) return { success: false }
      const connID = this.activeConnID
      try {
        const result = await batchSetKeysTTL(connID, keys, ttlSec)
        if (!result?.success) {
          useConnectionsStore().showGlobalToast(result?.message || 'Batch TTL failed', false)
          return result || { success: false }
        }
        if (connID !== this.activeConnID) return result
        const nextTTL = ttlSec < 0 ? -1 : ttlSec
        const target = new Set(keys)
        this.searchSessions = this.searchSessions.map(session => {
          const nextKeys = session.keys.map(item => (target.has(item?.name) ? { ...item, ttl: nextTTL } : item))
          return { ...session, keys: nextKeys, treeData: buildKeyTree(nextKeys) }
        })
        if (this.keyValue && target.has(this.keyValue.key)) {
          this.keyValue = { ...this.keyValue, ttl: nextTTL }
        }
        this.selectedKeys = []
        return result
      } catch (e) {
        useConnectionsStore().showGlobalToast(e?.message || String(e), false)
        return { success: false, message: e?.message || String(e) }
      }
    },

    async batchRenameSelectedPrefix(oldPrefix, newPrefix) {
      const keys = [...this.selectedKeys]
      if (!keys.length || !this.activeConnID) return { success: false }
      const connID = this.activeConnID
      try {
        const result = await batchRenamePrefix(connID, keys, oldPrefix, newPrefix)
        if (!result?.success) {
          useConnectionsStore().showGlobalToast(result?.message || 'Batch rename failed', false)
          return result || { success: false }
        }
        if (connID !== this.activeConnID) return result
        this.selectedKeys = []
        // key 名称变了，重新拉一次当前搜索，保证列表与实际一致
        const pattern = this.activeSession?.pattern
        if (pattern) await this.search(pattern)
        return result
      } catch (e) {
        useConnectionsStore().showGlobalToast(e?.message || String(e), false)
        return { success: false, message: e?.message || String(e) }
      }
    },

    async deleteCurrentKey() {
      if (!this.selectedKey || !this.activeConnID) return
      const deletedKey = this.selectedKey
      const request = this._beginRequest('deleteKey', { key: deletedKey, requireSelectedKey: true })
      let result
      try {
        result = await deleteKey(request.connID, deletedKey)
      } catch (e) {
        if (!this._isRequestCurrent(request)) {
          this._finishRequest(request)
          return { success: false, stale: true }
        }
        const disconnected = await this._handleConnectionFailure(e)
        if (disconnected) {
          this._finishRequest(request)
          return disconnected
        }
        result = { success: false, message: e?.message || String(e) }
      }
      if (!this._isRequestCurrent(request)) {
        this._finishRequest(request)
        return { ...result, stale: true }
      }
      if (!result?.success) {
        const disconnected = await this._handleConnectionFailure(result?.message)
        if (disconnected) {
          this._finishRequest(request)
          return disconnected
        }
        useConnectionsStore().showGlobalToast(result?.message || 'Failed to delete key', false)
        this._finishRequest(request)
        return result || { success: false, message: 'Failed to delete key' }
      }
      this.searchSessions = this.searchSessions.map(session => {
        const nextKeys = session.keys.filter(key => key?.name !== deletedKey)
        return {
          ...session,
          keys: nextKeys,
          treeData: buildKeyTree(nextKeys),
        }
      })
      this.totalKeys = Math.max(0, (this.totalKeys || 0) - 1)
      this.selectedKey = null
      this.keyValue = null
      this.keyValueError = null
      this.keyValueLoading = false
      this.keyValueRefreshing = false
      if (this._loadingKey === deletedKey) {
        this._loadingKey = null
      }
      this._finishRequest(request)
      return result
    },

    async renameCurrentKey(newKey) {
      if (!this.selectedKey || !this.activeConnID) return
      const oldKey = this.selectedKey
      const request = this._beginRequest('renameKey', { key: oldKey, requireSelectedKey: true })
      const result = await renameKey(request.connID, oldKey, newKey)
      if (!this._isRequestCurrent(request)) return { ...result, stale: true }
      if (!result.success && isConnectionErrorMessage(result.message)) {
        const failure = await this._handleConnectionFailure(result.message)
        this._finishRequest(request)
        return failure
      }
      if (result.success) {
        if (this.activeSession) {
          await this.search(this.activeSession.pattern)
          if (!this._isRequestCurrent(request)) {
            this._finishRequest(request)
            return { ...result, stale: true }
          }
        }
        this.selectedKey = newKey
        await this.selectKey(newKey)
      }
      this._finishRequest(request)
      return result
    },

    async updateTTL(ttlSec) {
      if (!this.selectedKey || !this.activeConnID) return
      const key = this.selectedKey
      const request = this._beginRequest('setTTL', { key, requireSelectedKey: true })
      const result = await setTTL(request.connID, key, ttlSec)
      if (!this._isRequestCurrent(request)) return { ...result, stale: true }
      if (!result.success && isConnectionErrorMessage(result.message)) {
        const failure = await this._handleConnectionFailure(result.message)
        this._finishRequest(request)
        return failure
      }
      if (result.success && this.keyValue) {
        this.keyValue.ttl = ttlSec
        this._updateKeyMetadata(this.selectedKey, {
          ttl: Number.isFinite(ttlSec) && ttlSec > 0 ? ttlSec : -1,
        })
      }
      this._finishRequest(request)
      return result
    },

    async createKey(req) {
      if (!this.activeConnID) return { success: false, message: 'No active connection' }
      const request = this._beginRequest('createKey')
      const result = await createKey(request.connID, req)
      if (!this._isRequestCurrent(request)) return { ...result, stale: true }
      if (!result.success && isConnectionErrorMessage(result.message)) {
        const failure = await this._handleConnectionFailure(result.message, { clearKeyValue: false, setKeyError: false })
        this._finishRequest(request)
        return failure
      }
      if (!result.success) {
        this._finishRequest(request)
        return result
      }

      this.totalKeys = (this.totalKeys || 0) + 1
      const keyInfo = {
        name: req.key,
        type: req.type,
        ttl: Number.isFinite(req.ttl) && req.ttl > 0 ? req.ttl : -1,
      }
      const inserted = this._upsertKeyIntoSessions(keyInfo)
      if (!inserted) {
        await this.searchExact(req.key)
        this._finishRequest(request)
        return result
      }
      await this.selectKey(req.key)
      this._finishRequest(request)
      return result
    },

    getEditorSearchState(key, keyType) {
      if (!this.activeConnID || !key) return null
      const cacheKey = `${this.activeConnID}:${keyType}:${key}`
      return this.editorSearchStates[cacheKey] || null
    },

    setEditorSearchState(key, keyType, state) {
      if (!this.activeConnID || !key) return
      const cacheKey = `${this.activeConnID}:${keyType}:${key}`
      this.editorSearchStates[cacheKey] = state
    },

    async switchDB(db) {
      if (!this.activeConnID) return
      // 切库同样不能保留其他 DB 的选中项
      this.selectionMode = false
      this.selectedKeys = []
      this.selectionAnchor = null
      const request = this._beginRequest('switchDB')
      const result = await selectDB(request.connID, db)
      if (!this._isRequestCurrent(request)) return { ...result, stale: true }
      if (!result.success && isConnectionErrorMessage(result.message)) {
        const failure = await this._handleConnectionFailure(result.message)
        this._finishRequest(request)
        return failure
      }
      if (result.success) {
        this._invalidateRequests()
        this.currentDB = db
        this.searchSessions = []
        this.activeSessionId = null
        this.selectedKey = null
        this.keyValue = null
        this.keyValueError = null
        this.keyValueLoading = false
        this.keyValueRefreshing = false
        this._loadingKey = null
        await this.fetchTotalKeys()
      }
      this._finishRequest(request)
      return result
    },

    clearSearchHistory(connId) {
      if (!connId) return
      delete this.connSearchHistory[connId]
      this._saveSearchHistory()
    },
  },
})
