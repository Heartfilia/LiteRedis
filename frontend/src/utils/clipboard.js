/**
 * 剪贴板工具
 * 优先使用 Clipboard API，降级到 execCommand（Wails WebView 兼容）
 */
export async function copyToClipboard(text) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(String(text))
      return true
    }
    // fallback: execCommand
    const ta = document.createElement('textarea')
    ta.value = String(text)
    ta.style.cssText = 'position:fixed;opacity:0;top:0;left:0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}
