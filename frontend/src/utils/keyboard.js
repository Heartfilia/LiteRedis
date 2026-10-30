// 键盘可达性工具：让自定义可点元素也能用 Tab / Enter / 方向键操作
// 用法：
//   <div v-kbd-row @click="...">          行内元素，默认 tabindex="-1"，由容器的方向键导航
//   <div v-kbd-row="{ tabindex: 0 }">     独立元素，直接进 Tab 序列

export const vKbdRow = {
  mounted(el, binding) {
    const options = binding.value || {}
    if (!el.hasAttribute('tabindex')) {
      el.setAttribute('tabindex', String(options.tabindex ?? -1))
    }
    if (!el.hasAttribute('role')) {
      el.setAttribute('role', options.role || 'button')
    }
    el.__kbdKeydown = (event) => {
      // 只有焦点就在这一行本身时才接管，避免抢走内部输入框/复选框的按键
      if (event.target !== el) return
      if (event.key === 'Enter' || event.key === ' ' || event.key === 'Spacebar') {
        event.preventDefault()
        // 有些行的点击处理挂在内部元素上（如 .conn-main），支持指定目标
        const target = options.clickTarget ? el.querySelector(options.clickTarget) : null
        if (target) {
          target.click()
        } else {
          el.click()
        }
      }
    }
    el.addEventListener('keydown', el.__kbdKeydown)
  },
  unmounted(el) {
    if (el.__kbdKeydown) el.removeEventListener('keydown', el.__kbdKeydown)
  },
}

// 容器内使用 ↑/↓/Home/End 在行之间移动焦点（单 Tab 站点 + 方向键导航）
export function moveRovingFocus(event, container, selector) {
  if (!container) return false
  const rows = Array.from(container.querySelectorAll(selector)).filter((el) => el.offsetParent !== null)
  if (!rows.length) return false
  const current = rows.indexOf(document.activeElement)
  let next = current
  switch (event.key) {
    case 'ArrowDown':
      next = current < 0 ? 0 : Math.min(rows.length - 1, current + 1)
      break
    case 'ArrowUp':
      next = current <= 0 ? 0 : current - 1
      break
    case 'Home':
      next = 0
      break
    case 'End':
      next = rows.length - 1
      break
    case 'Tab':
      return false
    default:
      return false
  }
  event.preventDefault()
  const target = rows[next]
  // 手动滚，不用 scrollIntoView：树里 .node-row 带 content-visibility: auto，
  // 屏幕外的行没有真实布局，scrollIntoView 会滚不到位甚至不滚
  try {
    target.focus({ preventScroll: true })
  } catch (e) {
    target.focus()
  }
  revealRow(target)
  return true
}

// 把某一行滚进最近的滚动容器（左右/上下留 8px 余量）
export function revealRow(row) {
  if (!row || typeof row.getBoundingClientRect !== 'function') return
  const rect = row.getBoundingClientRect()
  const padding = 8
  let node = row.parentElement
  while (node && node !== document.body && node !== document.documentElement) {
    const style = typeof window !== 'undefined' && window.getComputedStyle ? window.getComputedStyle(node) : null
    const overflowY = style ? style.overflowY : ''
    if (/(auto|scroll)/.test(overflowY) && node.scrollHeight > node.clientHeight + 1) {
      const box = node.getBoundingClientRect()
      if (rect.top < box.top + padding) {
        node.scrollTop -= box.top + padding - rect.top
      } else if (rect.bottom > box.bottom - padding) {
        node.scrollTop += rect.bottom - (box.bottom - padding)
      }
      return
    }
    node = node.parentElement
  }
}
