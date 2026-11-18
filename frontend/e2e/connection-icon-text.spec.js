import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

const CONNECTIONS = [
  { id: 'icon-custom', name: 'alpha', host: '127.0.0.1', port: 6379, db: 0, group: '', icon_text: 'AB12' },
  { id: 'icon-cjk', name: 'gamma', host: '127.0.0.1', port: 6379, db: 0, group: '', icon_text: '缓存一号' },
  { id: 'icon-default', name: 'beta', host: '10.0.0.2', port: 6379, db: 0, group: '' },
]

test('custom icon text uses two lines and keeps the avatar size', async ({ page }) => {
  await installWailsMock(page, { connections: CONNECTIONS })
  await page.goto('/')

  const custom = page.locator('.conn-item').filter({ hasText: 'alpha' }).locator('.conn-avatar')
  await expect(custom).toBeVisible()
  const lines = custom.locator('.icon-lines > span')
  await expect(lines).toHaveCount(2)
  await expect(lines.nth(0)).toHaveText('AB')
  await expect(lines.nth(1)).toHaveText('12')

  const box = await custom.boundingBox()
  expect(Math.round(box.width)).toBe(20)
  expect(Math.round(box.height)).toBe(20)

  // 中文比英文宽，四字两行也不能撑出 20px 底图。
  const cjk = page.locator('.conn-item').filter({ hasText: 'gamma' }).locator('.conn-avatar')
  await expect(cjk.locator('.icon-lines > span').nth(0)).toHaveText('缓存')
  await expect(cjk.locator('.icon-lines > span').nth(1)).toHaveText('一号')
  const cjkBox = await cjk.boundingBox()
  const cjkLinesBox = await cjk.locator('.icon-lines').boundingBox()
  expect(cjkLinesBox.width).toBeLessThanOrEqual(cjkBox.width)
  expect(cjkLinesBox.height).toBeLessThanOrEqual(cjkBox.height)

  const fallback = page.locator('.conn-item').filter({ hasText: 'beta' }).locator('.conn-avatar')
  await expect(fallback.locator('.icon-lines > span')).toHaveText('B')
  const customSize = await custom.evaluate(el => parseFloat(getComputedStyle(el).fontSize))
  const fallbackSize = await fallback.evaluate(el => parseFloat(getComputedStyle(el).fontSize))
  expect(customSize).toBeLessThan(fallbackSize)
})

test('connection form edits up to four icon characters and saves them', async ({ page }) => {
  await installWailsMock(page, { connections: CONNECTIONS })
  await page.goto('/')
  await page.evaluate(() => {
    window.__savedIconConfigs = []
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, {
      get(target, method) {
        if (method === 'SaveConnection') {
          return async (cfg) => {
            window.__savedIconConfigs.push(structuredClone(cfg))
            return { success: true }
          }
        }
        return target[method]
      },
    })
  })

  await page.locator('button[title="管理连接"]').click()
  await expect(page.locator('.cm-card')).toBeVisible()
  await page.locator('.cm-sidebar .cm-conn-item').filter({ hasText: 'alpha' }).click()

  const input = page.locator('.icon-text-input')
  await expect(input).toHaveValue('AB12')
  await input.fill('缓存一号库')
  await expect(input).toHaveValue('缓存一号')
  const preview = page.locator('.icon-preview-badge .icon-lines > span')
  await expect(preview).toHaveCount(2)
  await expect(preview.nth(0)).toHaveText('缓存')
  await expect(preview.nth(1)).toHaveText('一号')

  await page.getByRole('button', { name: '保存' }).click()
  const saved = await page.evaluate(() => window.__savedIconConfigs[0])
  expect(saved.icon_text).toBe('缓存一号')
})

test('collapsed sidebar keeps the custom icon lines', async ({ page }) => {
  await installWailsMock(page, { connections: CONNECTIONS })
  await page.goto('/')
  await page.locator('.conn-item').filter({ hasText: 'alpha' }).locator('.conn-main').click()
  await page.locator('.btn-collapse').click()

  const custom = page.locator('.collapsed-conn-icon').filter({ hasText: 'AB' })
  await expect(custom).toBeVisible()
  await expect(custom.locator('.icon-lines > span')).toHaveCount(2)
  await expect(custom.locator('.icon-lines > span').nth(0)).toHaveText('AB')
  await expect(custom.locator('.icon-lines > span').nth(1)).toHaveText('12')
})