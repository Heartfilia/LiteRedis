// 字节数格式化
export function formatBytes(bytes) {
  const value = Number(bytes)
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const exp = Math.min(units.length - 1, Math.floor(Math.log(value) / Math.log(1024)))
  const scaled = value / Math.pow(1024, exp)
  const digits = exp === 0 || scaled >= 100 ? 0 : 1
  return scaled.toFixed(digits) + ' ' + units[exp]
}

// 数量格式化：1234 -> 1.2K
export function formatCount(count) {
  const value = Number(count)
  if (!Number.isFinite(value) || value <= 0) return '0'
  if (value < 1000) return String(Math.round(value))
  if (value < 1000000) return (value / 1000).toFixed(1) + 'K'
  return (value / 1000000).toFixed(2) + 'M'
}
