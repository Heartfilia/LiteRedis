import { defineStore } from 'pinia'

export const SAVED_SEARCHES_STORAGE_KEY = 'liteRedis_savedSearches_v1'
export const SAVED_SEARCHES_LIMIT = 100
export const SAVED_SEARCH_NAME_LIMIT = 80
export const SAVED_SEARCH_PATTERN_LIMIT = 1024

function normalizeItem(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const { id, name, connID, db, pattern, lastUsed } = value
  if (typeof id !== 'string' || !id.trim() || typeof connID !== 'string' || !connID.trim()) return null
  if (typeof name !== 'string' || !name.trim() || [...name.trim()].length > SAVED_SEARCH_NAME_LIMIT) return null
  if (!Number.isSafeInteger(db) || db < 0) return null
  // Preserve the exact Redis glob, including meaningful whitespace.
  if (typeof pattern !== 'string' || !pattern.length || [...pattern].length > SAVED_SEARCH_PATTERN_LIMIT) return null
  if (!Number.isSafeInteger(lastUsed) || lastUsed < 0) return null
  // Deliberately whitelist fields: never persist connection credentials or results.
  return { id: id.trim(), name: name.trim(), connID: connID.trim(), db, pattern, lastUsed }
}

function newID() {
  return globalThis.crypto?.randomUUID?.() || `search-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export const useSavedSearchesStore = defineStore('savedSearches', {
  state: () => ({ items: [], loaded: false, error: '' }),
  getters: {
    forConnection: (state) => (connID, db) => state.items.filter(item =>
      item.connID === connID && (db === undefined || item.db === db)),
  },
  actions: {
    _fail(code, message) {
      this.error = message
      return { success: false, code, message }
    },
    load() {
      let raw
      try {
        raw = globalThis.localStorage.getItem(SAVED_SEARCHES_STORAGE_KEY)
      } catch (error) {
        return this._fail('storage_read_failed', `Unable to read saved searches: ${error.message || error}`)
      }
      let values
      try {
        values = raw === null ? [] : JSON.parse(raw)
        if (!Array.isArray(values)) throw new Error('Expected an array')
      } catch {
        this.items = []
        this.loaded = true
        return this._fail('invalid_storage', 'Saved searches contain invalid JSON or an invalid structure')
      }
      const seen = new Set()
      const items = []
      for (const value of values) {
        const item = normalizeItem(value)
        if (!item || seen.has(item.id)) continue
        seen.add(item.id)
        items.push(item)
        if (items.length === SAVED_SEARCHES_LIMIT) break
      }
      this.items = items
      this.loaded = true
      this.error = ''
      return { success: true }
    },
    _ensureLoaded() {
      return this.loaded ? { success: true } : this.load()
    },
    _persist(items, item) {
      try {
        // Re-whitelist all records even if external code modified reactive state.
        const clean = items.map(normalizeItem)
        if (clean.some(value => !value)) return this._fail('invalid_input', 'Invalid saved search')
        globalThis.localStorage.setItem(SAVED_SEARCHES_STORAGE_KEY, JSON.stringify(clean))
        this.items = clean
        this.error = ''
        return { success: true, ...(item ? { item: { ...item } } : {}) }
      } catch (error) {
        return this._fail('storage_write_failed', `Unable to save searches: ${error.message || error}`)
      }
    },
    add(input) {
      const loaded = this._ensureLoaded()
      if (!loaded.success) return loaded
      if (this.items.length >= SAVED_SEARCHES_LIMIT) return this._fail('limit_reached', 'At most 100 saved searches are allowed')
      let id = newID()
      while (this.items.some(item => item.id === id)) id = `${id}-new`
      const item = normalizeItem({ ...input, id, lastUsed: 0 })
      if (!item) return this._fail('invalid_input', 'Invalid saved search name, connection, database or pattern')
      return this._persist([...this.items, item], item)
    },
    update(id, patch) {
      const loaded = this._ensureLoaded()
      if (!loaded.success) return loaded
      const original = this.items.find(item => item.id === id)
      if (!original) return this._fail('not_found', 'Saved search not found')
      const item = normalizeItem({ ...original, ...patch, id: original.id, lastUsed: original.lastUsed })
      if (!item) return this._fail('invalid_input', 'Invalid saved search name, connection, database or pattern')
      return this._persist(this.items.map(current => current.id === id ? item : current), item)
    },
    remove(id) {
      const loaded = this._ensureLoaded()
      if (!loaded.success) return loaded
      if (!this.items.some(item => item.id === id)) return this._fail('not_found', 'Saved search not found')
      return this._persist(this.items.filter(item => item.id !== id))
    },
    touch(id) {
      const loaded = this._ensureLoaded()
      if (!loaded.success) return loaded
      const original = this.items.find(item => item.id === id)
      if (!original) return this._fail('not_found', 'Saved search not found')
      const item = { ...original, lastUsed: Date.now() }
      return this._persist(this.items.map(current => current.id === id ? item : current), item)
    },
  },
})
