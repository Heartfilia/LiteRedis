/**
 * Wails API 封装
 * 所有后端方法通过 window.go.main.App.XXX() 调用
 */

function callGo(method, ...args) {
  return window.go.main.App[method](...args)
}

// ===== 连接管理 =====
export const listConnections = () => callGo('ListConnections')
export const saveConnection = (cfg) => callGo('SaveConnection', cfg)
export const deleteConnection = (id) => callGo('DeleteConnection', id)
export const testConnection = (cfg) => callGo('TestConnection', cfg)
export const connect = (id) => callGo('Connect', id)
export const disconnect = (id) => callGo('Disconnect', id)
export const isConnected = (id) => callGo('IsConnected', id)
export const selectDB = (id, db) => callGo('SelectDB', id, db)

// ===== Key 操作 =====
export const scanKeys = (connID, pattern, count, cursor) => callGo('ScanKeys', connID, pattern, count, cursor)
export const getKeyInfo = (connID, key) => callGo('GetKeyInfo', connID, key)
export const deleteKey = (connID, key) => callGo('DeleteKey', connID, key)
export const renameKey = (connID, oldKey, newKey) => callGo('RenameKey', connID, oldKey, newKey)
export const setTTL = (connID, key, ttl) => callGo('SetTTL', connID, key, ttl)
export const dbSize = (connID) => callGo('DBSize', connID)

// ===== Value CRUD =====
export const getValue = (connID, key, cursor, offset) => callGo('GetValue', connID, key, cursor, offset)
export const searchValue = (connID, key, keyType, pattern) => callGo('SearchValue', connID, key, keyType, pattern)
export const setString = (connID, key, value, ttl) => callGo('SetString', connID, key, value, ttl)
export const hSet = (connID, key, field, value) => callGo('HSet', connID, key, field, value)
export const hDel = (connID, key, field) => callGo('HDel', connID, key, field)
export const lPush = (connID, key, value) => callGo('LPush', connID, key, value)
export const rPush = (connID, key, value) => callGo('RPush', connID, key, value)
export const lSet = (connID, key, index, value) => callGo('LSet', connID, key, index, value)
export const lRem = (connID, key, count, value) => callGo('LRem', connID, key, count, value)
export const sAdd = (connID, key, member) => callGo('SAdd', connID, key, member)
export const sRem = (connID, key, member) => callGo('SRem', connID, key, member)
export const zAdd = (connID, key, member, score) => callGo('ZAdd', connID, key, member, score)
export const zRem = (connID, key, member) => callGo('ZRem', connID, key, member)

// ===== 设置 =====
export const getSettings = () => callGo('GetSettings')
export const saveSettings = (s) => callGo('SaveSettings', s)
