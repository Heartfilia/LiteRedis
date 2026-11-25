export const COMMAND_TEMPLATE_LIMIT = 4096

// A template is one console command. Placeholders must occupy an entire,
// unquoted argument; static arguments retain the console's quoting syntax.
export function parseCommandTemplate(template) {
  if (typeof template !== 'string' || !template.trim() || template.length > COMMAND_TEMPLATE_LIMIT || /[\r\n\0]/.test(template)) {
    throw new Error('Invalid command template')
  }
  const tokens = []
  let start = -1
  let quote = ''
  let escaped = false
  for (let i = 0; i < template.length; i++) {
    const char = template[i]
    if (start < 0 && /\s/.test(char)) continue
    if (start < 0) start = i
    if (escaped) { escaped = false; continue }
    if (char === '\\') { escaped = true; continue }
    if (quote) { if (char === quote) quote = ''; continue }
    if (char === '"' || char === "'") { quote = char; continue }
    if (/\s/.test(char)) { tokens.push(template.slice(start, i)); start = -1 }
  }
  if (quote || escaped) throw new Error('Unclosed quote or escape')
  if (start >= 0) tokens.push(template.slice(start))
  const parameters = []
  const parts = tokens.map((raw, index) => {
    const match = raw.match(/^\{\{([A-Za-z_][A-Za-z0-9_]{0,63})\}\}$/)
    if (match) {
      if (index === 0) throw new Error('Command name cannot be a parameter')
      if (!parameters.includes(match[1])) parameters.push(match[1])
      return { parameter: match[1] }
    }
    if (raw.includes('{{') || raw.includes('}}')) throw new Error('Parameters must be standalone unquoted arguments')
    return { raw }
  })
  return { parts, parameters }
}

export function quoteRedisArgument(value) {
  if (typeof value !== 'string' || /[\r\n\0]/.test(value)) throw new Error('Multiline or NUL parameters are not supported')
  return '"' + value.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"'
}

export function prepareCommandTemplate(template, values = {}) {
  const { parts, parameters } = parseCommandTemplate(template)
  for (const parameter of parameters) {
    if (!Object.prototype.hasOwnProperty.call(values, parameter) || typeof values[parameter] !== 'string') throw new Error('Missing parameter')
  }
  return parts.map(part => part.parameter ? quoteRedisArgument(values[part.parameter]) : part.raw).join(' ')
}
