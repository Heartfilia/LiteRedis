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
  await expect(menu.locator('.theme-menu-item')).toHaveCount(4)

  // 菜单从按钮上方弹出：先记录一次位置，动画结束后不能跳到别处
  const opening = await menu.boundingBox()
  await page.waitForTimeout(260)
  const settled = await menu.boundingBox()
  // 入场动画有 6px 位移 + 缩放，允许小幅差异；之前的位置跳变是上百像素
  // 底边应该贴着按钮上沿（留一点间距），而不是跑到别的地方
  const gap = btnBox.y - (settled.y + settled.height)
  expect(gap).toBeGreaterThan(0)
  expect(gap).toBeLessThan(20)

  await menu.locator('.theme-menu-item', { hasText: '暗色' }).click()
  await expect(layout).toHaveClass(/theme-dark/)
  await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'dark')
  await expect(menu).toHaveCount(0)

  // 护眼模式：整屏进入绿色调
  await page.locator('.btn-theme').click()
  await page.locator('.theme-menu-item', { hasText: '护眼' }).click()
  await expect(layout).toHaveClass(/theme-green/)
  await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'green')
  await expect(page.locator('.theme-tint')).toBeVisible()

  await page.locator('.btn-theme').click()
  await page.locator('.theme-menu-item', { hasText: '浅色' }).click()
  await expect(layout).toHaveClass(/theme-light/)
})

test.describe('follow system theme', () => {
  test.use({ colorScheme: 'dark' })

  test('auto theme uses the system preference', async ({ page }) => {
    await installWailsMock(page, { keyCount: 20 })
    await page.goto('/')

    await page.locator('.btn-theme').click()
    await page.locator('.theme-menu-item', { hasText: '跟随系统' }).click()
    await expect(page.locator('.app-layout')).toHaveClass(/theme-dark/)
    await expect(page.locator('.btn-theme')).toHaveAttribute('data-theme-pref', 'auto')
  })
})
