import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

// 纯计算：给定总行数 / 滚动位置 / 视口高度 / 行高，算出要渲染的行区间和上下留白
export function computeWindow(count, scrollTop, viewport, rowHeight, overscan = 8) {
  const total = Math.max(0, Number(count) || 0)
  if (!rowHeight || rowHeight <= 0 || total === 0) {
    return { start: 0, end: total, topPad: 0, bottomPad: 0 }
  }
  const safeScroll = Math.max(0, Number(scrollTop) || 0)
  const safeViewport = Math.max(0, Number(viewport) || 0)
  const start = Math.max(0, Math.floor(safeScroll / rowHeight) - overscan)
  const end = Math.min(total, Math.ceil((safeScroll + safeViewport) / rowHeight) + overscan)
  return {
    start,
    end: Math.max(start, end),
    topPad: start * rowHeight,
    bottomPad: Math.max(0, (total - Math.max(start, end)) * rowHeight),
  }
}

// 只对「行高一致」的长列表做窗口化：超过 threshold 行时只渲染可视区附近的行，
// 用上下两个占位块维持滚动高度，避免一次性创建上万行 DOM。
export function useRowVirtualizer(options) {
  const containerRef = options.containerRef
  const count = options.count
  const threshold = options.threshold ?? 120
  const overscan = options.overscan ?? 8

  const rowHeight = ref(32)
  const scrollTop = ref(0)
  const viewport = ref(0)
  let raf = 0
  let observer = null

  function measure() {
    const el = containerRef.value
    if (!el) return
    viewport.value = el.clientHeight || 0
    const row = el.querySelector('[data-vrow]')
    if (row) {
      const h = row.getBoundingClientRect().height
      if (h > 4) rowHeight.value = h
    }
  }

  function onScroll() {
    if (raf) return
    raf = requestAnimationFrame(() => {
      raf = 0
      scrollTop.value = containerRef.value ? containerRef.value.scrollTop : 0
    })
  }

  const active = computed(() => count.value > threshold && rowHeight.value > 0)
  const windowRange = computed(() =>
    active.value
      ? computeWindow(count.value, scrollTop.value, viewport.value, rowHeight.value, overscan)
      : { start: 0, end: count.value, topPad: 0, bottomPad: 0 }
  )
  const start = computed(() => windowRange.value.start)
  const end = computed(() => windowRange.value.end)
  const topPad = computed(() => windowRange.value.topPad)
  const bottomPad = computed(() => windowRange.value.bottomPad)

  function bind() {
    const el = containerRef.value
    if (!el) return
    el.addEventListener('scroll', onScroll, { passive: true })
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(measure)
      observer.observe(el)
    }
    measure()
  }

  function unbind() {
    const el = containerRef.value
    if (el) el.removeEventListener('scroll', onScroll)
    if (observer) {
      observer.disconnect()
      observer = null
    }
    if (raf) {
      cancelAnimationFrame(raf)
      raf = 0
    }
  }

  onMounted(bind)
  onBeforeUnmount(unbind)
  watch(containerRef, (el, old) => {
    if (old) unbind()
    if (el) bind()
  })

  return { active, start, end, topPad, bottomPad, measure }
}
