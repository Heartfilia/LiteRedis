/**
 * 把 JSON 按展开状态压平成可见行，供窗口化渲染使用。
 * path 用 JSON Pointer 风格的转义，避免 key 里带 / 或 ~ 时撞车。
 */
export function flattenJsonRows(data, isExpanded = () => false) {
  const rows = []
  walk(data, null, '', 0, isExpanded, rows)
  return rows
}

function escapeKey(key) {
  return String(key).replace(/~/g, '~0').replace(/\//g, '~1')
}

function walk(value, label, path, depth, isExpanded, rows) {
  const isContainer = value !== null && typeof value === 'object'
  rows.push({ path, label, value, depth, isContainer })
  if (!isContainer) return
  if (!isExpanded(path, depth)) return
  if (Array.isArray(value)) {
    for (let index = 0; index < value.length; index++) {
      walk(value[index], index, path + '/' + index, depth + 1, isExpanded, rows)
    }
    return
  }
  for (const key of Object.keys(value)) {
    walk(value[key], key, path + '/' + escapeKey(key), depth + 1, isExpanded, rows)
  }
}
