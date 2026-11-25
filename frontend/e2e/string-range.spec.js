import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openLargeString(page) {
  await installWailsMock(page, { keyCount: 200 })
  await page.goto('/')
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__rangeReads = []
    window.__rangeWrites = []
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'GetValue') return async (_conn, key) => ({
        key, type: 'string', string_val: 'bounded initial preview',
        truncated: true, read_only_reason: 'truncated', total_count: 131089,
      })
      if (method === 'GetStringRangePreview') return (...args) => {
        window.__rangeReads.push(args)
        const offset = args[3]
        const end = Math.min(offset + 65536, 131089)
        const result = {
          offset, start: offset, end, next_offset: end, total_bytes: 131089,
          has_more: end < 131089, value: `range at ${offset}`, binary: false, encoding: 'utf-8',
        }
        if (window.__delayRange) return new Promise(resolve => { window.__finishRange = resolve })
        if (window.__binaryRange) return Promise.resolve({ ...result, value: 'AP9h', binary: true, encoding: 'base64' })
        return Promise.resolve(result)
      }
      if (method === 'SetString' || method === 'SetStringChecked') return async (...args) => {
        window.__rangeWrites.push(args)
        return { success: true }
      }
      return target[method]
    } })
  })
  await page.locator('.conn-item .conn-main').first().click()
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
  await page.locator('.tree-bulk-btn[title="展开全部"]').click()
  await page.locator('.tree-scroll .node-row.leaf-row').first().click()
  await expect(page.locator('.range-preview summary')).toBeVisible()
}

test('range reads are opt-in, bounded metadata and byte offsets drive paging', async ({ page }) => {
  await openLargeString(page)
  expect(await page.evaluate(() => window.__rangeReads)).toEqual([])
  const range = page.locator('.range-preview')
  await range.locator('summary').click()
  await expect(range.locator('pre')).toHaveText('range at 0')
  await expect(range.locator('.range-position')).toHaveText('[0, 65536) / 131089 bytes · utf-8')
  await expect(range).toContainText('不是快照')
  await range.getByRole('button', { name: '下一段', exact: true }).click()
  await expect(range.locator('pre')).toHaveText('range at 65536')
  await expect(range.locator('.range-position')).toContainText('[65536, 131072)')
  await range.getByRole('button', { name: '上一段', exact: true }).click()
  await expect(range.locator('pre')).toHaveText('range at 0')
  await range.locator('input').fill('131072')
  await range.getByRole('button', { name: '跳转读取', exact: true }).click()
  await expect(range.locator('.range-position')).toContainText('[131072, 131089)')
  await expect(range.getByRole('button', { name: '下一段', exact: true })).toBeDisabled()
  expect((await page.evaluate(() => window.__rangeReads)).map(args => args[3])).toEqual([0, 65536, 0, 131072])
  await expect(page.locator('.string-editor textarea')).toHaveAttribute('readonly', '')
  await expect(page.locator('.string-editor').getByRole('button', { name: '保存', exact: true })).toBeDisabled()
  expect(await page.evaluate(() => window.__rangeWrites)).toEqual([])
})

test('binary and split UTF-8 ranges are explicitly Base64 and never editable', async ({ page }) => {
  await openLargeString(page)
  await page.evaluate(() => { window.__binaryRange = true })
  const range = page.locator('.range-preview')
  await range.locator('summary').click()
  await expect(range.locator('pre')).toHaveText('AP9h')
  await expect(range).toContainText('Base64 无损显示')
  await expect(range.locator('.range-position')).toContainText('base64')
  await expect(range.locator('textarea')).toHaveCount(0)
  await expect(range.getByRole('button', { name: '保存', exact: true })).toHaveCount(0)
  await expect(page.locator('.string-editor').getByRole('button', { name: '保存', exact: true })).toBeDisabled()
  expect(await page.evaluate(() => window.__rangeWrites)).toEqual([])
})

for (const change of ['key', 'db']) test(`late range response is discarded after ${change} changes`, async ({ page }) => {
  await openLargeString(page)
  await page.evaluate(() => { window.__delayRange = true })
  await page.locator('.range-preview summary').click()
  await expect.poll(() => page.evaluate(() => window.__rangeReads.length)).toBe(1)
  if (change === 'key') {
    await page.locator('.tree-scroll .node-row.leaf-row').nth(1).click()
  } else {
    await page.evaluate(async () => {
      const { useWorkspaceStore } = await import('/src/stores/workspace.js')
      useWorkspaceStore().currentDB = 7
    })
  }
  // Resolve only after Vue has processed navigation/store watchers.
  await expect(page.locator('.range-preview pre')).toHaveCount(0)
  await page.evaluate(() => window.__finishRange({
    offset: 0, start: 0, end: 5, next_offset: 5, total_bytes: 5,
    has_more: false, value: 'STALE', binary: false, encoding: 'utf-8',
  }))
  await expect(page.locator('.range-preview pre')).toHaveCount(0)
  await expect(page.locator('.string-editor')).not.toContainText('STALE')
})
