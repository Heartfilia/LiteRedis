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
