// 连接图标内的字符：默认为名称首字，也可自定义最多 4 个字符。
// 图标底图尺寸不变，多字符按每行 2 个换行显示。

export const ICON_TEXT_MAX = 4
const LINE_LENGTH = 2

export function normalizeIconText(value) {
  const cleaned = String(value ?? '').replace(/[\r\n\t]/g, '').trim()
  return Array.from(cleaned).slice(0, ICON_TEXT_MAX).join('')
}

// 每 2 个字符一行：AB -> ['AB']，ABC -> ['AB', 'C']，ABCD -> ['AB', 'CD']
export function iconTextLines(value) {
  const chars = Array.from(normalizeIconText(value))
  const lines = []
  for (let i = 0; i < chars.length; i += LINE_LENGTH) {
    lines.push(chars.slice(i, i + LINE_LENGTH).join(''))
  }
  return lines
}

export function connectionIconText(conn) {
  const custom = normalizeIconText(conn?.iconText ?? conn?.icon_text ?? '')
  if (custom) return custom
  const fallback = String(conn?.name || conn?.host || '?').trim()
  const first = Array.from(fallback)[0]
  return first ? first.toUpperCase() : '?'
}

export function connectionIconLines(conn) {
  return iconTextLines(connectionIconText(conn))
}

// 两个及以上字符时缩小字号，避免撑出图标底图。
export function connectionIconCompact(conn) {
  return Array.from(connectionIconText(conn)).length > 1
}