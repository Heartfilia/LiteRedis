export const CONFLICT_PREVIEW_BYTES = 64 * 1024

// Bound each rendered column by UTF-8 bytes without cutting a code point.
export function conflictPreview(value) {
  const text = String(value ?? '')
  const prefix = text.slice(0, CONFLICT_PREVIEW_BYTES)
  const encoder = new TextEncoder()
  let bytes = 0
  let end = 0
  for (const character of prefix) {
    const size = encoder.encode(character).length
    if (bytes + size > CONFLICT_PREVIEW_BYTES) break
    bytes += size
    end += character.length
  }
  return { value: text.slice(0, end), truncated: end < text.length }
}
