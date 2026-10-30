// 全局快捷键：Cmd/Ctrl+R 刷新当前 key，Esc 关闭最上层的浮层
const escapeHandlers = []
let installed = false

// registerEscapeHandler 在浮层打开时登记，返回注销函数；Esc 时只触发最后登记的那个
export function registerEscapeHandler(handler) {
  if (typeof handler !== 'function') return () => {}
  const entry = { handler }
  escapeHandlers.push(entry)
  return () => {
    const index = escapeHandlers.indexOf(entry)
    if (index >= 0) escapeHandlers.splice(index, 1)
  }
}

function runTopEscapeHandler() {
  const entry = escapeHandlers[escapeHandlers.length - 1]
  if (!entry) return false
  try {
    entry.handler()
  } catch (e) {
    // 关闭回调出错不应该阻塞按键处理
  }
  return true
}

export function installGlobalShortcuts() {
  if (installed || typeof window === 'undefined') return
  installed = true
  window.addEventListener('keydown', (event) => {
    const cmdOrCtrl = event.metaKey || event.ctrlKey
    if (cmdOrCtrl && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      window.dispatchEvent(new CustomEvent('literedis:command-palette'))
      return
    }
    if (cmdOrCtrl && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'r') {
      // 不拦截的话 WebView 会尝试刷新页面
      event.preventDefault()
      window.dispatchEvent(new CustomEvent('literedis:refresh-key'))
      return
    }
    if (event.key === 'Escape') {
      if (runTopEscapeHandler()) {
        event.preventDefault()
      }
    }
  })
}
