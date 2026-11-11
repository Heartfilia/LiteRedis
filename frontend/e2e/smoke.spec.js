import { expect, test } from '@playwright/test'

import { installWailsMock } from './wails-mock.js'

async function connectFirst(page) {
  await page.locator('.conn-item .conn-main').first().click()
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
}

async function expandAll(page) {
  await page.locator('.tree-bulk-btn[title="展开全部"]').click()
  await expect.poll(async () => page.locator('.tree-scroll [data-vrow]').count()).toBeGreaterThan(5)
}

async function mockMemoryDoctor(page, overrides = {}) {
  await page.evaluate((overrides) => {
    const original = window.go.main.App
    window.__memoryCalls = []
    window.__workspaceDBChanges = []
    window.go.main.App = new Proxy(original, {
      get(target, method) {
        if (method === 'SelectDB') return async (...args) => { window.__workspaceDBChanges.push(args); return { success: true } }
        if (method === 'ScanMemoryReportForDB') return async (conn, db, pattern, maxKeys) => {
          window.__memoryCalls.push({ conn, db, pattern, maxKeys })
          return {
            db, pattern, scanned_keys: 2, total_keys: 2, total_bytes: 2500, avg_key_bytes: 1250,
            no_ttl_keys: 1, elapsed_ms: 12, size_source: 'memory', redis_used_memory_bytes: 10000,
            memory_share_percent: 25, memory_share_partial: pattern !== '*', prefixes: [], types: [], big_keys: [],
            ...overrides,
          }
        }
        return target[method]
      },
    })
    window.dispatchEvent(new Event('literedis:open-memory'))
  }, overrides)
  await expect(page.locator('.doctor-field-db select')).toBeVisible()
}

test('memory doctor selects an independent DB and clears stale reports', async ({ page }) => {
  await installWailsMock(page)
  await page.goto('/')
  await connectFirst(page)
  await mockMemoryDoctor(page)
  const database = page.locator('.doctor-field-db select')
  await expect(database).toHaveValue('0')
  await expect(database.locator('option')).toHaveCount(16)
  await database.selectOption('6')
  await page.getByRole('button', { name: '开始扫描', exact: true }).click()
  await expect(page.locator('.doctor-subtitle')).toContainText('db=6')
  await expect(page.locator('.doctor-metric-share')).toContainText('DB 内存占比（估算）')
  await expect(page.locator('.doctor-metric-share')).toContainText('25.00%')
  await expect(page.locator('.doctor-metrics')).toContainText('Redis 整体已用内存')
  await page.evaluate(() => Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText: async (text) => { window.__memoryMarkdown = text } },
  }))
  await page.getByRole('button', { name: '导出 Markdown', exact: true }).click()
  expect(await page.evaluate(() => window.__memoryMarkdown)).toContain('DB 内存占比（估算）: 25.00%')
  expect(await page.evaluate(() => window.__memoryMarkdown)).toContain('- DB: 6')
  expect(await page.evaluate(() => window.__memoryCalls[0])).toMatchObject({ db: 6, pattern: '*' })
  expect(await page.evaluate(() => window.__workspaceDBChanges)).toEqual([])
  await database.selectOption('7')
  await expect(page.locator('.doctor-metrics')).toHaveCount(0)
  await expect(page.getByRole('button', { name: '导出 Markdown', exact: true })).toBeDisabled()
  await page.locator('.doctor-field input').fill('user:*')
  await page.getByRole('button', { name: '开始扫描', exact: true }).click()
  await expect(page.locator('.doctor-metric-share')).toContainText('已扫描 Key 占比（估算）')
  await expect(page.locator('.doctor-body')).toContainText('占比不代表整个 DB')
})

test('memory doctor distinguishes unavailable share from empty DB zero share', async ({ page }) => {
  await installWailsMock(page)
  await page.goto('/')
  await connectFirst(page)
  await mockMemoryDoctor(page, { memory_share_percent: null, redis_used_memory_bytes: 0, memory_share_unavailable_reason: 'memory_usage_unsupported_or_renamed' })
  await page.getByRole('button', { name: '开始扫描', exact: true }).click()
  await expect(page.locator('.doctor-metric-share .metric-value')).toHaveText('—')
  await expect(page.locator('.doctor-body')).toContainText('占比不可用')
  await expect(page.locator('.doctor-body')).toContainText('3.0.7')
  await page.locator('.doctor-close').click()
  await mockMemoryDoctor(page, { memory_share_percent: 0, total_bytes: 0, scanned_keys: 0, total_keys: 0 })
  await page.getByRole('button', { name: '开始扫描', exact: true }).click()
  await expect(page.locator('.doctor-metric-share .metric-value')).toHaveText('0.00%')
})

test('memory doctor locks DB selection during scanning and recovers after cancellation', async ({ page }) => {
  await installWailsMock(page)
  await page.goto('/')
  await connectFirst(page)
  await mockMemoryDoctor(page)
  await page.evaluate(() => {
    const original = window.go.main.App
    let rejectScan
    window.go.main.App = new Proxy(original, {
      get(target, method) {
        if (method === 'ScanMemoryReportForDB') return () => new Promise((_resolve, reject) => { rejectScan = reject })
        if (method === 'CancelMemoryScan') return async () => { rejectScan(new Error('体检已取消')); return { success: true } }
        return target[method]
      },
    })
  })
  await page.getByRole('button', { name: '开始扫描', exact: true }).click()
  await expect(page.locator('.doctor-field-db select')).toBeDisabled()
  await page.getByRole('button', { name: '取消扫描', exact: true }).click()
  await expect(page.locator('.doctor-field-db select')).toBeEnabled()
  await expect(page.locator('.doctor-state.error')).toContainText('体检已取消')
})

test('memory doctor limits cluster scans to DB zero', async ({ page }) => {
  await installWailsMock(page, { connections: [{ id: 'conn-dev', name: 'cluster', is_cluster: true, allow_cluster_scan: true, db: 0 }] })
  await page.goto('/')
  await page.locator('.conn-item .conn-main').first().click()
  await mockMemoryDoctor(page)
  await expect(page.locator('.doctor-field-db select')).toBeDisabled()
  await expect(page.locator('.doctor-field-db select option')).toHaveCount(1)
  await expect(page.locator('.doctor-db-hint')).toContainText('所有主节点（不含副本）')
})

test('connects and renders a windowed key tree', async ({ page }) => {
  await installWailsMock(page, { keyCount: 3000 })
  await page.goto('/')

  // 标签栏默认关闭，只能从左侧切换
  await expect(page.locator('.conn-tabs')).toHaveCount(0)
  await expect(page.locator('.conn-item .conn-main')).toHaveCount(2)

  await connectFirst(page)
  await expandAll(page)

  const rows = page.locator('.tree-scroll [data-vrow]')
  const scrollHeight = await page.locator('.tree-scroll').evaluate((el) => el.scrollHeight)
  expect(scrollHeight).toBeGreaterThan(5000)
  expect(await rows.count()).toBeLessThan(200)

  const firstIndex = () => page.locator('.tree-scroll [data-vrow]').first().getAttribute('data-row-index')
  const before = await firstIndex()
  await page.locator('.tree-scroll').evaluate((el) => {
    el.scrollTop = Math.floor(el.scrollHeight / 2)
  })
  await expect.poll(firstIndex).not.toBe(before)
})

test('key export shows cancellation and recovers after cancelling', async ({ page }) => {
  await installWailsMock(page, { keyCount: 200 })
  await page.goto('/')
  await connectFirst(page)
  await page.evaluate(() => {
    const original = window.go.main.App
    let rejectExport
    window.__exportCalls = 0
    window.go.main.App = new Proxy(original, {
      get(target, method) {
        if (method === 'ExportKeysToFile') {
          return () => {
            window.__exportCalls++
            return new Promise((_resolve, reject) => { rejectExport = reject })
          }
        }
        if (method === 'CancelKeyExport') {
          return async () => {
            rejectExport(new Error('context canceled'))
            return { success: true }
          }
        }
        return target[method]
      },
    })
  })
  await page.locator('.tree-bulk-btn[title="多选模式"]').click()
  await page.locator('.tree-scroll input[type="checkbox"]').first().check()
  await page.locator('.batch-btn').filter({ hasText: '⋯' }).click()
  await page.getByRole('button', { name: '导出 JSON 文件', exact: true }).click()
  const status = page.locator('.key-tree [role="status"]')
  await expect(status).toContainText('导出中')
  await status.getByRole('button', { name: '取消', exact: true }).click()
  await expect(status).toHaveCount(0)
  expect(await page.evaluate(() => window.__exportCalls)).toBe(1)
})

test('keyboard navigation keeps working inside the window', async ({ page }) => {
  await installWailsMock(page, { keyCount: 3000 })
  await page.goto('/')
  await connectFirst(page)
  await expandAll(page)

  // 点容器空白处拿焦点，再按方向键从第一行走
  await page.locator('.tree-scroll').click({ position: { x: 20, y: 5 } })
  const focusedIndex = () =>
    page.evaluate(() => {
      const row = document.activeElement && document.activeElement.closest('.tree-row')
      return row ? Number(row.dataset.rowIndex) : -1
    })
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('ArrowDown')
  await expect.poll(focusedIndex).toBe(1)

  // End 跳到末尾：窗口化后目标行还没渲染，必须滚动后仍能拿到焦点
  await page.keyboard.press('End')
  await expect.poll(focusedIndex).toBeGreaterThan(100)
  const inViewport = await page.evaluate(() => {
    const row = document.activeElement && document.activeElement.closest('.tree-row')
    if (!row) return false
    const box = row.getBoundingClientRect()
    const scroller = row.closest('.tree-scroll').getBoundingClientRect()
    return box.top >= scroller.top - 1 && box.bottom <= scroller.bottom + 1
  })
  expect(inViewport).toBe(true)
})

test('reorders tabs by dragging and closes them with the middle click', async ({ page }) => {
  await installWailsMock(page, { keyCount: 200, tabs: true })
  await page.goto('/')
  await page.locator('.conn-item .conn-main').first().click()
  await page.locator('.conn-item .conn-main').nth(1).click()

  const tabs = page.locator('.conn-tab')
  await expect(tabs).toHaveCount(2)
  expect(await tabs.nth(0).innerText()).toContain('dev')

  const target = await tabs.nth(1).boundingBox()
  await tabs.nth(0).dragTo(tabs.nth(1), {
    targetPosition: { x: Math.max(10, target.width - 6), y: target.height / 2 },
  })
  await expect(tabs.nth(0)).toContainText('prod-backup')
  await expect(tabs.nth(1)).toContainText('dev')

  await tabs.nth(0).click({ button: 'middle' })
  await expect(page.locator('.conn-tab')).toHaveCount(1)
})

test('console completes command names, subcommands and keywords', async ({ page }) => {
  await installWailsMock(page, { keyCount: 200 })
  await page.goto('/')
  await connectFirst(page)

  await page.keyboard.press('Meta+k')
  const paletteInput = page.locator('.cmdk-input')
  await expect(paletteInput).toBeVisible()
  await paletteInput.fill('命令台')
  await page.keyboard.press('Enter')

  const input = page.locator('.console-input')
  await expect(input).toBeVisible()

  // 命令名
  await input.fill('zr')
  await expect(page.locator('.console-suggest-item').first()).toContainText('ZRANGE')
  await page.keyboard.press('Tab')
  await expect(input).toHaveValue('ZRANGE ')

  // 关键字
  await input.fill('zrange key 0 -1 wi')
  await expect(page.locator('.console-suggest-item').first()).toContainText('WITHSCORES')
  await page.keyboard.press('Enter')
  await expect(input).toHaveValue('zrange key 0 -1 WITHSCORES ')

  // 子命令
  await input.fill('config ')
  await expect(page.locator('.console-suggest-item').first()).toContainText('GET')
  await page.keyboard.press('Enter')
  await expect(input).toHaveValue('config GET ')
})

test('command palette finds keys in the current database', async ({ page }) => {
  await installWailsMock(page, { keyCount: 200 })
  await page.goto('/')
  await connectFirst(page)

  await page.keyboard.press('Meta+k')
  const input = page.locator('.cmdk-input')
  await input.fill('user:1')
  await expect(page.locator('.cmdk-group-title', { hasText: '匹配的 key' })).toBeVisible()
  await expect(page.locator('.cmdk-item-label', { hasText: 'user:1:field:1' }).first()).toBeVisible()
})
test('big JSON values render through a windowed tree', async ({ page }) => {
  await installWailsMock(page, { keyCount: 60 })
  await page.goto('/')
  await connectFirst(page)
  await expandAll(page)

  // 打开一个 string key，切到树形视图
  await page.locator('.tree-scroll .node-row.leaf-row').first().click()
  const treeBtn = page.locator('.string-editor .btn-json', { hasText: '树形' })
  await expect(treeBtn).toBeVisible()
  await treeBtn.click()

  const jsonTree = page.locator('.json-viewer .json-tree')
  await expect(jsonTree).toBeVisible()
  await expect(page.locator('.json-tree .json-row').first()).toBeVisible()

  // 3000 个字段全部展开，但 DOM 里只保留可视区附近的行
  const rendered = await page.locator('.json-tree [data-vrow]').count()
  expect(rendered).toBeGreaterThan(5)
  expect(rendered).toBeLessThan(200)

  const firstKey = () => page.locator('.json-tree [data-vrow]').first().getAttribute('data-vkey')
  const before = await firstKey()
  await jsonTree.evaluate((el) => {
    el.scrollTop = Math.floor(el.scrollHeight / 2)
  })
  await expect.poll(firstKey).not.toBe(before)
})
test('theme menu switches between light and dark', async ({ page }) => {
  await installWailsMock(page, { keyCount: 20 })
  await page.goto('/')

  const layout = page.locator('.app-layout')
  await expect(layout).toHaveClass(/theme-light/)

  const btnBox = await page.locator('.btn-theme').boundingBox()
  await page.locator('.btn-theme').click()
  const menu = page.locator('.theme-menu')
  await expect(menu).toBeVisible()
  await expect(menu.locator('.theme-menu-item')).toHaveCount(5)

  // 菜单从按钮上方弹出：先记录一次位置，动画结束后不能跳到别处
  const opening = await menu.boundingBox()
  await page.waitForTimeout(260)
  const settled = await menu.boundingBox()
  // 入场动画有 6px 位移 + 缩放，允许小幅差异；之前的位置跳变是上百像素
  // 底边应该贴着按钮上沿（留一点间距），而不是跑到别的地方
  const gap = btnBox.y - (settled.y + settled.height)
  expect(gap).toBeGreaterThan(0)
  expect(gap).toBeLessThan(20)

  await menu.locator('.theme-menu-item', { hasText: /^暗色/ }).click()
  await expect(layout).toHaveClass(/theme-dark/)
  await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'dark')
  await expect(menu).toHaveCount(0)

  // 极客模式：暗色为底 + 终端皮肤（html 上挂 skin-geek 滤镜）
  await page.locator('.btn-theme').click()
  await page.locator('.theme-menu-item', { hasText: /^极客/ }).click()
  await expect(layout).toHaveClass(/skin-geek/)
  await expect(layout).toHaveClass(/theme-dark/)
  await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'geek')
  const skinOnHtml = await page.evaluate(() => document.documentElement.classList.contains('skin-geek'))
  expect(skinOnHtml).toBe(true)

  // 护眼模式：整屏进入绿色调
  await page.locator('.btn-theme').click()
  await page.locator('.theme-menu-item', { hasText: '护眼' }).click()
  await expect(layout).toHaveClass(/theme-green/)
  await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'green')
  await expect(page.locator('.theme-tint')).toBeVisible()

  await page.locator('.btn-theme').click()
  await page.locator('.theme-menu-item', { hasText: /^浅色/ }).click()
  await expect(layout).toHaveClass(/theme-light/)
})

test.describe('follow system theme', () => {
  test.use({ colorScheme: 'dark' })

  test('auto theme uses the system preference', async ({ page }) => {
    await installWailsMock(page, { keyCount: 20 })
    await page.goto('/')

    await page.locator('.btn-theme').click()
    await page.locator('.theme-menu-item', { hasText: /^跟随系统/ }).click()
    await expect(page.locator('.app-layout')).toHaveClass(/theme-dark/)
    await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'auto')
  })
})
test('collapse button follows connection state and restores on network loss', async ({ page }) => {
  await installWailsMock(page, { keyCount: 60 })
  await page.goto('/')

  // 没有已连接的 Redis：折叠按钮收起不可见
  await expect(page.locator('.btn-collapse')).toBeHidden()

  await page.locator('.conn-item .conn-main').first().click()
  await expandAll(page)
  const collapseBtn = page.locator('.btn-collapse')
  await expect(collapseBtn).toBeVisible()

  // 打开一个 key，让刷新有目标
  await page.locator('.tree-scroll .node-row.leaf-row').first().click()
  await page.waitForTimeout(600)

  await collapseBtn.click()
  await expect(page.locator('.sidebar-collapsed-layer.visible')).toBeVisible()

  // 模拟断网后刷新当前 key，触发连接失败
  await page.evaluate(() => window.__wailsMock.setNetworkUp(false))
  await page.keyboard.press('Meta+r')

  // 自动展开回到等待连接的初始状态，折叠按钮随之消失
  await expect(page.locator('.sidebar-collapsed-layer.visible')).toHaveCount(0)
  await expect(page.locator('.btn-collapse')).toBeHidden()
})
