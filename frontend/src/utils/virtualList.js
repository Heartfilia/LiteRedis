import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

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

  // 程序化设置 scrollTop 后（比如键盘跳行）立刻同步，避免等下一帧滚动事件才更新窗口
  function syncScroll() {
    scrollTop.value = containerRef.value ? containerRef.value.scrollTop : 0
  }

  return { active, start, end, topPad, bottomPad, measure, syncScroll }
}

// —— 变高行 ——
// 逐行累加得到 offsets[i+1] = 前 i 行的总高度；没量过的行先用估算高度顶上
export function buildOffsets(count, measure, estimate) {
  const total = Math.max(0, Number(count) || 0)
  const offsets = new Float64Array(total + 1)
  for (let i = 0; i < total; i++) {
    const measured = measure(i)
    const height = measured > 0 ? measured : estimate(i)
    offsets[i + 1] = offsets[i] + (height > 0 ? height : 1)
  }
  return offsets
}

// 在 offsets 里二分定位可视区间（变高行用）
export function findWindowForOffsets(offsets, scrollTop, viewport, overscan = 4) {
  const total = Math.max(0, offsets.length - 1)
  if (!total) return { start: 0, end: 0, topPad: 0, bottomPad: 0 }
  const safeScroll = Math.max(0, Number(scrollTop) || 0)
  const safeViewport = Math.max(0, Number(viewport) || 0)
  let lo = 0
  let hi = total
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (offsets[mid] <= safeScroll) lo = mid
    else hi = mid - 1
  }
  const start = Math.max(0, lo - overscan)
  const limit = safeScroll + safeViewport
  let end = start
  while (end < total && offsets[end] < limit) end++
  end = Math.min(total, Math.max(end, start) + overscan)
  return {
    start,
    end,
    topPad: offsets[start],
    bottomPad: Math.max(0, offsets[total] - offsets[end]),
  }
}

/**
 * 变高行的窗口化：行高靠实测缓存，量到新高度时用首行做锚点补偿 scrollTop，
 * 避免滚动过程中内容跳动。适合 stream entry 这种字段会换行的列表。
 */
export function useVariableVirtualizer(options) {
  const containerRef = options.containerRef
  const count = computed(() => (options.items.value || []).length)
  const keyOf = options.keyOf
  const estimate = options.estimate || (() => 58)
  const overscan = options.overscan ?? 4
  const threshold = options.threshold ?? 120

  const heights = new Map()
  const version = ref(0)
  const scrollTop = ref(0)
  const viewport = ref(0)
  let raf = 0
  let observer = null

  const active = computed(() => count.value > threshold)

  const offsets = computed(() => {
    // 依赖 items 与量化版本，行高变化时重算
    void version.value
    const items = options.items.value || []
    return buildOffsets(
      items.length,
      (i) => heights.get(keyOf(items[i])) || 0,
      (i) => estimate(items[i], i)
    )
  })

  const windowRange = computed(() =>
    active.value
      ? findWindowForOffsets(offsets.value, scrollTop.value, viewport.value, overscan)
      : { start: 0, end: count.value, topPad: 0, bottomPad: 0 }
  )

  const start = computed(() => windowRange.value.start)
  const end = computed(() => windowRange.value.end)
  const topPad = computed(() => windowRange.value.topPad)
  const bottomPad = computed(() => windowRange.value.bottomPad)

  function measureViewport() {
    const el = containerRef.value
    if (el) viewport.value = el.clientHeight || 0
  }

  // 读取已渲染行的高度；有变化就重算并做锚点补偿
  function measureRows() {
    const el = containerRef.value
    if (!el || !active.value) return
    const rows = el.querySelectorAll('[data-vrow]')
    if (!rows.length) return
    const first = rows[0]
    const firstIndex = Number(first.getAttribute('data-vindex'))
    const anchorTop = first.offsetTop
    let changed = false
    rows.forEach((row) => {
      const key = row.getAttribute('data-vkey')
      if (!key) return
      const height = row.getBoundingClientRect().height
      if (height <= 0) return
      const known = heights.get(key) || 0
      if (Math.abs(known - height) > 0.5) {
        heights.set(key, height)
        changed = true
      }
    })
    if (!changed) return
    version.value += 1
    if (!Number.isFinite(firstIndex) || firstIndex < 0) return
    nextTick(() => {
      const node = containerRef.value
      if (!node) return
      const nextTop = offsets.value[firstIndex]
      if (Number.isFinite(nextTop)) node.scrollTop += nextTop - anchorTop
    })
  }

  function onScroll() {
    if (raf) return
    raf = requestAnimationFrame(() => {
      raf = 0
      scrollTop.value = containerRef.value ? containerRef.value.scrollTop : 0
      measureRows()
    })
  }

  function bind() {
    const el = containerRef.value
    if (!el) return
    el.addEventListener('scroll', onScroll, { passive: true })
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(measureViewport)
      observer.observe(el)
    }
    measureViewport()
    measureRows()
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
  // 数据源整体换了（resetToken 变化，默认用首行 key）才清缓存并回到顶部；
  // 加载更多只是追加，不能跳回顶部
  const resetToken = options.resetToken
  let lastResetKey = null
  watch(
    () => options.items.value,
    (list) => {
      const token = resetToken ? resetToken() : list && list.length ? keyOf(list[0]) : null
      if (token !== lastResetKey) {
        lastResetKey = token
        heights.clear()
        scrollTop.value = 0
        if (containerRef.value) containerRef.value.scrollTop = 0
      }
      version.value += 1
      nextTick(() => {
        measureViewport()
        measureRows()
      })
    }
  )

  return { active, start, end, topPad, bottomPad, measureRows }
}
