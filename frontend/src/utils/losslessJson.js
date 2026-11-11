// 只调整 token 间空白：保留数字、重复属性及字符串转义的原始文本。
export class JsonNumber {
  constructor(raw) { this.raw = raw }
  toString() { return this.raw }
}

function tokensOf(text) {
  if (text.length > 2 * 1024 * 1024) throw new Error('JSON exceeds preview limit')
  JSON.parse(text) // 只校验语法，绝不使用其数值结果进行保存或展示。
  return text.match(/"(?:[^"\\]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*"|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?|true|false|null|[{}[\],:]/g) || []
}

export function formatJsonLossless(text) {
  const tokens = tokensOf(text)
  let depth = 0
  let out = ''
  const newline = () => '\n' + '  '.repeat(depth)
  tokens.forEach((token, i) => {
    if (token === '{' || token === '[') {
      out += token
      if (tokens[i + 1] !== (token === '{' ? '}' : ']')) {
        if (++depth > 256) throw new Error('JSON nesting exceeds preview limit')
        out += newline()
      }
    } else if (token === '}' || token === ']') {
      if (tokens[i - 1] !== (token === '}' ? '{' : '[')) { depth--; out += newline() }
      out += token
    } else if (token === ',') out += ',' + newline()
    else if (token === ':') out += ': '
    else out += token
    if (out.length > 8 * 1024 * 1024) throw new Error('Formatted JSON exceeds preview limit')
  })
  return out
}

export function parseJsonLossless(text) {
  const tokens = tokensOf(text)
  let index = 0
  function parse(depth = 0) {
    if (depth > 256) throw new Error('JSON nesting exceeds preview limit')
    const token = tokens[index++]
    if (token === '{') {
      const value = Object.create(null)
      while (tokens[index] !== '}') {
        const key = JSON.parse(tokens[index++])
        if (Object.hasOwn(value, key)) throw new Error('Duplicate keys: use text preview to preserve all properties')
        index++ // colon
        value[key] = parse(depth + 1)
        if (tokens[index] !== ',') break
        index++
      }
      index++
      return value
    }
    if (token === '[') {
      const value = []
      while (tokens[index] !== ']') {
        value.push(parse(depth + 1))
        if (tokens[index] !== ',') break
        index++
      }
      index++
      return value
    }
    if (/^-?\d/.test(token)) return new JsonNumber(token)
    return JSON.parse(token)
  }
  return parse()
}

export function stringifyJsonLossless(value) {
  if (value instanceof JsonNumber) return value.raw
  if (Array.isArray(value)) return '[' + value.map(stringifyJsonLossless).join(',') + ']'
  if (value !== null && typeof value === 'object') return '{' + Object.keys(value).map((key) => JSON.stringify(key) + ':' + stringifyJsonLossless(value[key])).join(',') + '}'
  return JSON.stringify(value)
}
