export function isConnectionErrorMessage(error) {
  if (!error) return false
  const msg = String(error).toLowerCase()
  return msg.includes('connection reset')
    || msg.includes('broken pipe')
    || msg.includes('eof')
    || msg.includes('network')
    || msg.includes('timeout')
    || msg.includes('refused')
    || msg.includes('closed')
    || msg.includes('dial')
    || msg.includes('i/o timeout')
    || msg.includes('no route to host')
    || msg.includes('connection not found')
    || msg.includes('not connected')
    || msg.includes('use of closed network connection')
    || msg.includes('client is closed')
    || msg.includes('ssh')
}

export function formatConnectionLostMessage(message, fallback = 'Redis 连接已断开，请检查网络或连接状态后重试') {
  const detail = String(message || '').trim()
  if (!detail) return fallback
  return `${fallback}\n${detail}`
}
