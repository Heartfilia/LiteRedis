/**
 * 将扁平 key 列表构建成前缀树（O(N)，按 `:` 分割）
 */
export function buildKeyTree(keys, sep = ':') {
  const root = {}

  for (const key of keys) {
    const parts = key.name.split(sep)
    let cur = root
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
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
      if (i === parts.length - 1) {
        cur[part].isLeaf = true
        cur[part].keyType = key.type
        cur[part].ttl = key.ttl
      }
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
