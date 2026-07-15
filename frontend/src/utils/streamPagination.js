export function mergeStreamEntries(currentEntries, nextEntries) {
  const merged = new Map()
  for (const entry of currentEntries || []) merged.set(entry.id, entry)
  for (const entry of nextEntries || []) merged.set(entry.id, entry)
  return [...merged.values()]
}
