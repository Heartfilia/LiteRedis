/**
 * Wails API 封装
 * 所有后端方法通过 window.go.main.App.XXX() 调用
 */

import { guardedWrite } from '../utils/writeSafety.js'

const writes = new Set(['DeleteKey', 'RenameKey', 'SetTTL', 'SetString', 'CreateKey', 'HSet', 'HDel', 'LPush', 'RPush', 'LSet', 'LRem', 'SAdd', 'SRem', 'RenameSetMember', 'ZAdd', 'ZRem', 'RenameZSetMember', 'BatchDeleteKeys', 'BatchSetKeysTTL', 'BatchRenamePrefix', 'ExecuteRedisCommand', 'ExecuteRedisCommandForce'])
const wholeConnectionWrites = new Set(['RenameKey', 'BatchDeleteKeys', 'BatchSetKeysTTL', 'BatchRenamePrefix', 'ExecuteRedisCommand', 'ExecuteRedisCommandForce'])
const consoleVerificationRead = /^(?:GET|MGET|TYPE|TTL|PTTL|EXISTS|STRLEN|LLEN|LRANGE|LINDEX|SCARD|SMEMBERS|SISMEMBER|HLEN|HGET|HMGET|HGETALL|ZCARD|ZSCORE|ZRANGE|XLEN|XRANGE|INFO|DBSIZE|PING)\s/i

async function callGo(method, ...args) {
  const invoke = () => window.go.main.App[method](...args)
  if (method === 'ImportKeysFromFile' || method === 'CopyKeysToDatabase' || method === 'CopyKeysToConnection') {
    const target = method === 'CopyKeysToConnection' ? args[1] : args[0]
    return guardedWrite(method, [target], invoke, true)
  }
  if (method === 'PublishMessage') {
    const result = await guardedWrite(method, args, async () => {
      try { return { success: true, receivers: await invoke() } }
      catch (error) {
        const message = String(error?.message || error)
        const code = /^(invalid|rejected|permission_denied|unsupported|cluster_redirect):/.exec(message)?.[1] || 'outcome_unknown'
        return { success: false, code, message }
      }
    }, true)
    if (!result.success) throw new Error(result.message)
    return result.receivers
  }
  // Let users verify uncertain writes in the Console without first unlocking.
  if (method.startsWith('ExecuteRedisCommand') && consoleVerificationRead.test(args[1] + ' ')) {
    try { return await invoke() } catch { return { success: false, code: 'read_failed', error: '读取失败，请检查连接' } }
  }
  if (method === 'SetStringChecked' || method === 'HSetChecked') return guardedWrite(method, [args[0], args[2]], invoke)
  if (method === 'CreateKey') return guardedWrite(method, [args[0], args[1].key.trim()], invoke)
  if (writes.has(method)) return guardedWrite(method, args, invoke, wholeConnectionWrites.has(method))
  const result = await invoke()
  return result
}

// ===== 连接管理 =====
export const listConnections = () => callGo('ListConnections')
export const saveConnection = (cfg) => callGo('SaveConnection', cfg)
export const reorderConnections = (items) => callGo('ReorderConnections', items)
export const deleteConnection = (id) => callGo('DeleteConnection', id)
export const testConnection = (cfg) => callGo('TestConnection', cfg)
export const forgetSSHHostFingerprint = (host, port) => callGo('ForgetSSHHostFingerprint', host, port)
export const connect = (id) => callGo('Connect', id)
export const disconnect = (id) => callGo('Disconnect', id)
export const isConnected = (id) => callGo('IsConnected', id)
export const pingConnection = (id) => callGo('PingConnection', id)
export const selectDB = (id, db) => callGo('SelectDB', id, db)
export const getConnectionOverview = (id) => callGo('GetConnectionOverview', id)
export const getConnectionDB = (connID) => callGo('GetConnectionDB', connID)
export const scanMemoryReport = (connID, pattern, maxKeys) => callGo('ScanMemoryReport', connID, pattern, maxKeys)
export const scanMemoryReportForDB = (connID, db, pattern, maxKeys) => callGo('ScanMemoryReportForDB', connID, db, pattern, maxKeys)
export const cancelMemoryScan = () => callGo('CancelMemoryScan')
export const getSlowLog = (connID, count) => callGo('GetSlowLog', connID, count)

// ===== 发布订阅 =====
export const startPubSub = (connID, channels, patterns) => callGo('StartPubSub', connID, channels, patterns)
export const stopPubSub = (connID) => callGo('StopPubSub', connID)
export const getPubSubStatus = (connID) => callGo('GetPubSubStatus', connID)
export const publishMessage = (connID, channel, payload) => callGo('PublishMessage', connID, channel, payload)
export const exportKeysToFile = (connID, keys, format) => callGo('ExportKeysToFile', connID, keys, format)
export const cancelKeyExport = () => callGo('CancelKeyExport')
export const copyKeysToConnection = (srcConnID, dstConnID, keys, overwrite) => callGo('CopyKeysToConnection', srcConnID, dstConnID, keys, overwrite)
export const copyKeysToDatabase = (connID, keys, dstDB, overwrite) => callGo('CopyKeysToDatabase', connID, keys, dstDB, overwrite)
export const getDatabases = (connID) => callGo('GetDatabases', connID)
export const importKeysFromFile = (connID, overwrite) => callGo('ImportKeysFromFile', connID, overwrite)
export const executeRedisCommand = (id, command) => callGo('ExecuteRedisCommand', id, command)
export const executeRedisCommandForce = (id, command) => callGo('ExecuteRedisCommandForce', id, command)

// ===== 本地数据提取 API =====
export const publishKeyValue = (connID, kv, db, nonce) => callGo('PublishKeyValue', connID, kv, db, nonce)
export const getLocalAPIInfo = () => callGo('GetLocalAPIInfo')
export const regenerateLocalAPIToken = () => callGo('RegenerateLocalAPIToken')

// ===== Key 操作 =====
export const scanKeys = (connID, pattern, count, cursor) => callGo('ScanKeys', connID, pattern, count, cursor)
export const getKeyInfo = (connID, key) => callGo('GetKeyInfo', connID, key)
export const deleteKey = (connID, key) => callGo('DeleteKey', connID, key)
export const batchDeleteKeys = (connID, keys) => callGo('BatchDeleteKeys', connID, keys)
export const batchSetKeysTTL = (connID, keys, ttl) => callGo('BatchSetKeysTTL', connID, keys, ttl)
export const batchRenamePrefix = (connID, keys, oldPrefix, newPrefix) => callGo('BatchRenamePrefix', connID, keys, oldPrefix, newPrefix)
export const renameKey = (connID, oldKey, newKey) => callGo('RenameKey', connID, oldKey, newKey)
export const setTTL = (connID, key, ttl) => callGo('SetTTL', connID, key, ttl)
export const dbSize = (connID) => callGo('DBSize', connID)

// ===== Value CRUD =====
export const getValue = (connID, key, cursor, offset, zsetSort = '', streamStart = '', keyType = '') =>
  callGo('GetValue', connID, key, cursor, offset, zsetSort, streamStart, keyType)
export const searchValue = (connID, key, keyType, pattern, exact = false, cursor = 0) =>
  callGo('SearchValue', connID, key, keyType, pattern, exact, cursor)
export const setString = (connID, key, value) => callGo('SetString', connID, key, value)
export const setStringChecked = (connID, db, key, value, token) => callGo('SetStringChecked', connID, db, key, value, token)
export const hSetChecked = (connID, db, key, field, value, expected, existed) => callGo('HSetChecked', connID, db, key, field, value, expected, existed)
export const closeApp = () => callGo('CloseApp')
export const frontendReady = () => callGo('FrontendReady')
export const listOperationTasks = () => callGo('ListOperationTasks')
export const cancelOperationTask = (id) => callGo('CancelOperationTask', id)
export const createKey = (connID, req) => callGo('CreateKey', connID, req)
export const hSet = (connID, key, field, value) => callGo('HSet', connID, key, field, value)
export const hDel = (connID, key, field) => callGo('HDel', connID, key, field)
export const lPush = (connID, key, value) => callGo('LPush', connID, key, value)
export const rPush = (connID, key, value) => callGo('RPush', connID, key, value)
export const lSet = (connID, key, index, value) => callGo('LSet', connID, key, index, value)
export const lRem = (connID, key, count, value) => callGo('LRem', connID, key, count, value)
export const sAdd = (connID, key, member) => callGo('SAdd', connID, key, member)
export const sRem = (connID, key, member) => callGo('SRem', connID, key, member)
export const renameSetMember = (connID, key, source, destination) =>
  callGo('RenameSetMember', connID, key, source, destination)
export const zAdd = (connID, key, member, score) => callGo('ZAdd', connID, key, member, score)
export const zRem = (connID, key, member) => callGo('ZRem', connID, key, member)
export const renameZSetMember = (connID, key, source, destination) =>
  callGo('RenameZSetMember', connID, key, source, destination)

// ===== 设置 =====
export const getSettings = () => callGo('GetSettings')
export const saveSettings = (s) => callGo('SaveSettings', s)
export const getAppVersion = () => callGo('GetAppVersion')
export const checkLatestRelease = () => callGo('CheckLatestRelease')
export const startUpdate = () => callGo('StartUpdate')
export const getReleaseNotes = (version) => callGo('GetReleaseNotes', version)
export const markVersionSeen = (version) => callGo('MarkVersionSeen', version)
export const completeOnboarding = () => callGo('CompleteOnboarding')

// ===== 命令历史 =====
export const getCommandHistory = () => callGo('GetCommandHistory')
export const pushCommandHistory = (command) => callGo('PushCommandHistory', command)
export const clearCommandHistory = () => callGo('ClearCommandHistory')

// ===== 配置导出 / 导入 =====
export const exportConnectionsToFile = () => callGo('ExportConnectionsToFile')
export const importConnectionsFromFile = () => callGo('ImportConnectionsFromFile')

// ===== 诊断 =====
export const getDiagnostics = () => callGo('GetDiagnostics')
export const exportDebugLog = () => callGo('ExportDebugLog')
