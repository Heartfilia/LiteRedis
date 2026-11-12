import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function expectLoadedKeys(page, count) {
  await expect.poll(() => page.evaluate(async () => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    return useWorkspaceStore().activeSession.keys.length
  })).toBe(count)
}

async function openPagedSearch(page, keep = false) {
  await installWailsMock(page, {
    connections: [{ id: 'conn-dev', name: 'cluster', is_cluster: true, allow_cluster_scan: true, db: 0 }],
  })
  await page.goto('/')
  await page.locator('.conn-item .conn-main').first().click()
  await page.evaluate(async (keep) => {
    const original = window.go.main.App
    window.__scanCalls = []
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ScanKeys') return async (conn, pattern, count, cursor) => {
        window.__scanCalls.push({ conn, pattern, count, cursor })
        if (cursor) await new Promise((resolve, reject) => { window.__finishPage = resolve; window.__failPage = reject })
        const offset = cursor ? Number(cursor) * 200 : 0
        const size = cursor === 2 ? 195 : 200
        return {
          keys: Array.from({ length: size }, (_, i) => ({
            name: cursor === 2 && i === 194 ? 'hermes:thor:aweme_ios_2280_did' : `hermes:thor:aweme_ios_${String(offset + i).padStart(4, '0')}`,
            type: 'string', ttl: -1,
          })),
          next_cursor: cursor === 2 ? 0 : (cursor || 0) + 1,
          has_more: cursor !== 2,
        }
      }
      return target[method]
    } })
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    const workspace = useWorkspaceStore()
    workspace.keepPrevSearch = keep
    await workspace.search('hermes:thor:aweme_ios*')
  }, keep)
  await expect(page.locator('.node-row').first()).toBeVisible()
}

test('cluster fuzzy search retains the scrolled list while loading all three pages', async ({ page }) => {
  await openPagedSearch(page)
  const list = page.locator('.tree-scroll')
  await list.evaluate(el => { el.scrollTop = el.scrollHeight })
  await expect.poll(() => list.evaluate(el => el.scrollTop)).toBeGreaterThan(1000)
  const before = await list.evaluate(el => el.scrollTop)
  await list.evaluate(el => { window.__originalList = el })
  await page.locator('.btn-load-more').click()
  await expect(page.locator('.btn-load-more')).toBeDisabled()
  expect(await page.locator('.btn-load-more').evaluate(el => getComputedStyle(el).animationName)).toBe('none')
  expect(await page.locator('.btn-load-more').evaluate(el => getComputedStyle(el, '::before').content)).toBe('none')
  await expect(list).toBeVisible()
  expect(await list.evaluate(el => el === window.__originalList)).toBe(true)
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
  await page.evaluate(() => window.__finishPage())
  await expectLoadedKeys(page, 400)
  expect(await list.evaluate(el => el.scrollTop)).toBeCloseTo(before, 0)
  await list.evaluate(el => { el.scrollTop = el.scrollHeight })
  await page.locator('.btn-load-more').click()
  await page.evaluate(() => window.__finishPage())
  await expectLoadedKeys(page, 595)
  await expect(page.locator('.btn-load-more')).toHaveCount(0)
  await list.evaluate(el => { el.scrollTop = el.scrollHeight })
  await expect(page.locator('.tree-scroll')).toContainText('aweme_ios_2280_did')
  expect(await page.evaluate(() => window.__scanCalls.map(c => c.cursor))).toEqual([0, 1, 2])
  await list.evaluate(el => { el.scrollTop = 0 })
  await expect(page.locator('.tree-scroll .node-row').first()).toContainText('hermes')
})

test('repeated SCAN keys are merged once and an empty continuation page keeps existing results', async ({ page }) => {
  await openPagedSearch(page)
  await page.evaluate(() => {
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ScanKeys') return async (_conn, _pattern, _count, cursor) => cursor === 1
        ? { keys: [{ name: 'hermes:thor:aweme_ios_0000', type: 'string', ttl: 60 }], has_more: true, next_cursor: 2 }
        : { keys: [], has_more: false, next_cursor: 0 }
      return target[method]
    } })
  })
  await page.locator('.btn-load-more').click()
  await expectLoadedKeys(page, 200)
  expect(await page.evaluate(async () => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    return useWorkspaceStore().activeSession.keys.find(key => key.name.endsWith('_0000')).ttl
  })).toBe(60)
  await page.locator('.btn-load-more').click()
  await expect(page.locator('.btn-load-more')).toHaveCount(0)
  await expectLoadedKeys(page, 200)
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
})

for (const keep of [false, true]) {
  test(`pagination failure preserves keys, shows an error and permits retry (merged=${keep})`, async ({ page }) => {
    await openPagedSearch(page, keep)
    const list = page.locator(keep ? '.merged-scroll' : '.tree-scroll')
    await page.locator('.btn-load-more').click()
    await expect(list.locator('.node-row').first()).toBeVisible()
    await page.evaluate(() => window.__failPage(new Error('cluster scan cursor expired or invalid; restart the scan')))
    await expect(page.getByRole('alert')).toContainText('cluster scan cursor expired')
    await expect(list.locator('.node-row').first()).toBeVisible()
    await expectLoadedKeys(page, 200)
    await page.locator('.btn-load-more').click()
    await expect(page.getByRole('alert')).toHaveCount(0)
    await page.evaluate(() => window.__finishPage())
    await expectLoadedKeys(page, 400)
  })
}

for (const theme of ['light', 'dark']) {
  test(`initial key loading keeps text stationary and animates only its spinner (${theme})`, async ({ page }) => {
    await openPagedSearch(page)
    await page.evaluate(async (theme) => {
      const { useSettingsStore } = await import('/src/stores/settings.js')
      useSettingsStore().setTheme(theme)
      const original = window.go.main.App
      window.go.main.App = new Proxy(original, { get(target, method) {
        if (method === 'ScanKeys') return () => new Promise(resolve => { window.__finishInitialScan = resolve })
        return target[method]
      } })
      const { useWorkspaceStore } = await import('/src/stores/workspace.js')
      void useWorkspaceStore().search('pending:*')
    }, theme)
    const loading = page.locator('.tree-content [role="status"]')
    await expect(loading).toContainText('加载')
    const styles = await loading.evaluate(el => {
      const parent = getComputedStyle(el)
      const spinner = getComputedStyle(el, '::before')
      return { animation: parent.animationName, transform: parent.transform, display: parent.display, spinnerAnimation: spinner.animationName, spinnerDisplay: spinner.display }
    })
    expect(styles.animation).toBe('none')
    expect(styles.transform).toBe('none')
    expect(styles.display).toBe('flex')
    expect(styles.spinnerAnimation).toContain('loadingSpin')
    expect(styles.spinnerDisplay).toBe('block')
    await page.evaluate(() => window.__finishInitialScan({ keys: [], has_more: false, next_cursor: 0 }))
    await expect(loading).toHaveCount(0)
  })
}
