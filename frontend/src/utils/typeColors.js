// 类型颜色映射
export const TYPE_COLORS = {
  string: { bg: '#e8f5e9', dot: '#4CAF50', text: '#2e7d32', label: 'STRING' },
  hash:   { bg: '#e3f2fd', dot: '#2196F3', text: '#1565c0', label: 'HASH' },
  list:   { bg: '#fff3e0', dot: '#FF9800', text: '#e65100', label: 'LIST' },
  set:    { bg: '#f3e5f5', dot: '#9C27B0', text: '#6a1b9a', label: 'SET' },
  zset:   { bg: '#fce4ec', dot: '#F44336', text: '#b71c1c', label: 'ZSET' },
  stream: { bg: '#e0f7fa', dot: '#00BCD4', text: '#006064', label: 'STREAM' },
}

export function getTypeColor(type) {
  return TYPE_COLORS[type?.toLowerCase()] || { bg: '#f5f5f5', dot: '#9e9e9e', text: '#616161', label: type?.toUpperCase() || 'UNKNOWN' }
}
