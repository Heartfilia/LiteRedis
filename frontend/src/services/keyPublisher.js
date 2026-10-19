/**
 * keyPublisher - 将前端数据上报给 Go 侧内存提取槽位。
 *
 * 当前为"点击即载入"模式：仅在点击「浏览器解析」/「复制链接」时上报一次，
 * 携带一次性 nonce，槽位全局唯一（与连接/db 无关）。
 */

import { publishKeyValue } from '../api/wails.js'

let debugEnabled = false

export function setPublisherDebug(enabled) {
  debugEnabled = !!enabled
}

function log(...args) {
  if (debugEnabled) console.debug('[keyPublisher]', ...args)
}

export { publishKey }

/**
 * 上报一个 key 的快照到提取槽位，返回 Promise。
 * @param {string} connID
 * @param {object} kv { key, type, string_val?/hash_val?/..., total_count?, has_more? }
 * @param {number} db
 * @param {string} nonce 本次提取的一次性指纹
 */
function publishKey(connID, kv, db = 0, nonce = '') {
  if (!connID || !kv || !kv.key || !kv.type) return Promise.resolve(null)
  return publishKeyValue(connID, kv, db, nonce)
    .then((result) => {
      log('published', kv.key)
      return result
    })
    .catch((e) => {
      log('publish failed', kv.key, e)
      return null
    })
}
