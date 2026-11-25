import { defineStore } from 'pinia'
import { parseCommandTemplate } from '../utils/commandTemplates.js'

export const COMMAND_TEMPLATES_STORAGE_KEY = 'liteRedis_commandTemplates_v1'
export const COMMAND_TEMPLATES_LIMIT = 50
export const COMMAND_TEMPLATE_NAME_LIMIT = 80

function normalizeItem(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const { id, name, template } = value
  if (typeof id !== 'string' || !id.trim() || id.length > 200) return null
  if (typeof name !== 'string' || !name.trim() || name.trim().length > COMMAND_TEMPLATE_NAME_LIMIT) return null
  try { parseCommandTemplate(template) } catch { return null }
  // Parameter values, connections and credentials must never be persisted.
  return { id, name: name.trim(), template }
}

export const useCommandTemplatesStore = defineStore('commandTemplates', {
  state: () => ({ items: [], loaded: false, error: '' }),
  actions: {
    _fail(code, error) {
      this.error = String(error?.message || error || code)
      return { success: false, code, message: this.error }
    },
    load() {
      try {
        const raw = globalThis.localStorage.getItem(COMMAND_TEMPLATES_STORAGE_KEY)
        const values = raw === null ? [] : JSON.parse(raw)
        if (!Array.isArray(values)) return this._fail('invalid_storage', 'Invalid command template storage')
        const ids = new Set()
        this.items = values.map(normalizeItem).filter(item => {
          if (!item || ids.has(item.id)) return false
          ids.add(item.id)
          return true
        }).slice(0, COMMAND_TEMPLATES_LIMIT)
        this.loaded = true
        this.error = ''
        return { success: true }
      } catch (error) { return this._fail('storage_read_failed', error) }
    },
    _persist(items, item) {
      try {
        const clean = items.map(normalizeItem)
        if (clean.length > COMMAND_TEMPLATES_LIMIT || clean.some(value => !value)) return this._fail('invalid_input', 'Invalid command template')
        globalThis.localStorage.setItem(COMMAND_TEMPLATES_STORAGE_KEY, JSON.stringify(clean))
        this.items = clean
        this.error = ''
        return { success: true, ...(item ? { item: { ...item } } : {}) }
      } catch (error) { return this._fail('storage_write_failed', error) }
    },
    save(input) {
      if (!this.loaded) { const loaded = this.load(); if (!loaded.success) return loaded }
      const original = input?.id ? this.items.find(item => item.id === input.id) : null
      if (input?.id && !original) return this._fail('not_found', 'Command template not found')
      if (!original && this.items.length >= COMMAND_TEMPLATES_LIMIT) return this._fail('limit_reached', 'At most 50 command templates')
      const id = original?.id || globalThis.crypto?.randomUUID?.() || `template-${Date.now()}-${Math.random().toString(36).slice(2)}`
      const item = normalizeItem({ ...input, id })
      if (!item) return this._fail('invalid_input', 'Invalid command template')
      return this._persist(original ? this.items.map(current => current.id === id ? item : current) : [...this.items, item], item)
    },
    remove(id) {
      if (!this.loaded) { const loaded = this.load(); if (!loaded.success) return loaded }
      if (!this.items.some(item => item.id === id)) return this._fail('not_found', 'Command template not found')
      return this._persist(this.items.filter(item => item.id !== id))
    },
  },
})
