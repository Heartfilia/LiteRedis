// 取触发元素的屏幕位置，用于弹层从该位置放大 / 收回
export function anchorRect(event) {
  const el = event?.currentTarget || event?.target
  if (!el || typeof el.getBoundingClientRect !== 'function') return null
  const rect = el.getBoundingClientRect()
  return { left: rect.left, top: rect.top, width: rect.width, height: rect.height }
}
