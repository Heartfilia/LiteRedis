/**
 * browserExtract - 「浏览器解析」：把当前展示的数据写入本地提取槽位，
 * 并打开 LiteRedis 内置查看页。
 *
 * 流程：点击时生成一次性 nonce → 当前 key 快照 + nonce 写入本地槽位 →
 * http://127.0.0.1:<port>/view/<base64(定位JSON含nonce)> → 默认浏览器打开。
 *
 * 同源 http 顶层导航，无混合内容 / CORS / PNA 问题；
 * 数据来自本地内存槽位（UI 已加载内容），不请求 Redis；
 * 槽位全局唯一（与连接/db 无关），再次提取会覆盖，旧链接随之失效。
 */

import { getLocalAPIInfo, publishKeyValue } from '../api/wails.js'
import { useWorkspaceStore } from '../stores/workspace.js'
import { copyToClipboard } from '../utils/clipboard.js'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime.js'

let runtimeCache = null
let runtimeLoading = null

async function getRuntime() {
  if (runtimeCache) return runtimeCache
  if (!runtimeLoading) {
    runtimeLoading = getLocalAPIInfo()
      .then((info) => {
        runtimeCache = { port: info.port, token: info.token }
        return runtimeCache
      })
      .catch(() => {
        runtimeLoading = null
        return null
      })
  }
  return runtimeLoading
}

/** 设置保存/重新生成 token 后调用，清空缓存以便下次重新获取 */
export function refreshLocalAPIRuntime() {
  runtimeCache = null
  runtimeLoading = null
}

function base64UrlSafeEncode(str) {
  const bytes = new TextEncoder().encode(str)
  let binary = ''
  for (const b of bytes) binary += String.fromCharCode(b)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

function genNonce() {
  const buf = new Uint8Array(8)
  crypto.getRandomValues(buf)
  return Array.from(buf, (b) => b.toString(16).padStart(2, '0')).join('')
}

/**
 * 发布快照并构建本地查看页链接。失败返回 null。
 * @param {object} target {
 *   key, type, position,
 *   data: 类型对应的值字段（hash_val / list_val / set_val / zset_val / stream_val / string_val）,
 *   totalCount, hasMore,
 * }
 */
async function publishAndBuildURL(target) {
  const runtime = await getRuntime()
  if (!runtime || !runtime.port || !runtime.token) return null
  const workspaceStore = useWorkspaceStore()
  const connID = workspaceStore.activeConnID
  if (!connID || !target.key || !target.type) return null

  const nonce = genNonce()
  const kv = {
    key: target.key,
    type: target.type,
    ...(target.data || {}),
    total_count: target.totalCount,
    has_more: target.hasMore,
  }
  const published = await publishKeyValue(connID, kv, workspaceStore.currentDB, nonce)
  if (!published || published.success === false) return null

  const payload = {
    redis: connID,
    db: String(workspaceStore.currentDB ?? 0),
    key: target.key,
    type: target.type,
    position: target.position != null ? String(target.position) : '0',
    token: runtime.token,
    nonce,
  }
  return `http://127.0.0.1:${runtime.port}/view/${base64UrlSafeEncode(JSON.stringify(payload))}`
}

/**
 * 打开浏览器解析页面。返回是否成功发起打开。
 */
export async function openBrowserExtract(target) {
  const url = await publishAndBuildURL(target)
  if (!url) return false
  BrowserOpenURL(url)
  return true
}

/**
 * 复制查看页链接到剪贴板。返回是否成功。
 */
export async function copyExtractLink(target) {
  const url = await publishAndBuildURL(target)
  if (!url) return false
  return copyToClipboard(url)
}
