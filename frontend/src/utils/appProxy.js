// 应用包更新代理：首次使用或地址为空时给出本机常见端口，省去手动输入一次。
export const DEFAULT_UPDATE_PROXY_URL = 'http://127.0.0.1:1081'

export function appProxyUrlOrDefault(value) {
  const trimmed = String(value ?? '').trim()
  return trimmed || DEFAULT_UPDATE_PROXY_URL
}