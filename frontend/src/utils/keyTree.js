/**
 * 将扁平 key 列表构建成前缀树（O(N)，按 `:` 分割）
 */
export function buildKeyTree(keys, sep = ':') {
  const root = {}
  const LEAF_MARK = '__leaf__'

  for (const key of keys) {
    const parts = key.name.split(sep)
    let cur = root
    for (let i = 0; i < parts.length; i++) {
      const rawPart = parts[i]
      const part = rawPart === '' ? '[empty]' : rawPart
      const isLast = i === parts.length - 1

      if (isLast) {
        const leafKey = rawPart === '' ? `${LEAF_MARK}:${i}` : `${LEAF_MARK}:${rawPart}`
        cur[leafKey] = {
          label: part,
          fullPath: key.name,
          isLeaf: true,
          keyType: key.type,
          ttl: key.ttl,
          children: {},
          count: 1,
        }
        continue
      }

      if (!cur[part]) {
        cur[part] = {
          label: part,
          fullPath: parts.slice(0, i + 1).join(sep),
          isLeaf: false,
          keyType: null,
          ttl: null,
          children: {},
          count: 0,
        }
      }
      cur[part].count++
      cur = cur[part].children
    }
  }

  function toArray(obj) {
    return Object.values(obj)
      .sort((a, b) => {
        // 目录节点排前，同类型按字母
        if (!a.isLeaf && b.isLeaf) return -1
        if (a.isLeaf && !b.isLeaf) return 1
        return a.label.localeCompare(b.label)
      })
      .map(n => ({ ...n, children: toArray(n.children) }))
  }

  return toArray(root)
}

/**
 * 把树按展开状态压平成可见行（深度优先），用于窗口化渲染。
 * isExpanded(node, depth) 决定目录节点的子节点要不要展开。
 */
export function flattenVisibleRows(nodes = [], isExpanded = () => true, depth = 0, output = []) {
  for (const node of nodes) {
    output.push({ node, depth })
    if (!node) continue
    const children = node.children || []
    if (node.isLeaf || !children.length) continue
    if (!isExpanded(node, depth)) continue
    flattenVisibleRows(children, isExpanded, depth + 1, output)
  }
  return output
}
/**
 * 找到某个 key 在树里的祖先目录链（用于展开并定位）。
 * 返回 [] 表示它在根层级，返回 null 表示当前树里没有这个 key。
 */
export function findKeyAncestors(nodes = [], fullPath = '') {
  if (!fullPath) return null
  function walk(list, chain) {
    for (const node of list) {
      if (!node) continue
      if (node.isLeaf) {
        if (node.fullPath === fullPath) return chain
        continue
      }
      const found = walk(node.children || [], [...chain, node.fullPath])
      if (found) return found
    }
    return null
  }
  return walk(nodes, [])
}
