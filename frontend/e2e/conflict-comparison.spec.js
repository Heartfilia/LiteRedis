import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openValue(page, value = {}) {
  await installWailsMock(page, { keyCount: 200 })
  await page.goto('/')
  await page.evaluate(value => {
    const original = window.go.main.App
    window.__conflictReads = []
    window.__conflictValue = { exists: true, type: value.type || 'string', value: 'server latest', total_bytes: 13 }
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'GetValue') return async (_conn, key) => ({ key, type: 'string', string_val: 'original', edit_token: 'original-token', ...value })
      if (method === 'SetStringChecked' || method === 'HSetChecked') return async () => ({ success: false, code: 'conflict' })
      if (method === 'GetEditConflict') return (...args) => {
        window.__conflictReads.push(args)
        if (window.__delayConflict) return new Promise(resolve => { window.__finishConflict = resolve })
        if (window.__failConflict) return Promise.reject(new Error('failed'))
        return Promise.resolve(window.__conflictValue)
      }
      return target[method]
    } })
  }, value)
  await page.locator('.conn-item .conn-main').first().click()
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
  await page.locator('.tree-bulk-btn[title="展开全部"]').click()
  await page.locator('.tree-scroll .node-row.leaf-row').first().click()
}

async function stringConflict(page) {
  await openValue(page)
  await page.locator('.string-editor textarea').fill('my draft')
  await page.locator('.string-editor').getByRole('button', { name: '保存', exact: true }).click()
}

test('String conflict is opt-in, shows three values and preserves draft after close', async ({ page }) => {
  await stringConflict(page)
  await expect(page.locator('.conflict-dialog')).toHaveCount(0)
  expect(await page.evaluate(() => window.__conflictReads.length)).toBe(0)
  await page.locator('.string-editor').getByRole('button', { name: '查看冲突对比' }).click()
  await expect(page.locator('[data-conflict-column="original"] pre')).toHaveText('original')
  await expect(page.locator('[data-conflict-column="draft"] pre')).toHaveText('my draft')
  await expect(page.locator('[data-conflict-column="current"] pre')).toHaveText('server latest')
  expect((await page.evaluate(() => window.__conflictReads[0])).slice(3)).toEqual(['', false])
  const close = page.locator('.conflict-dialog').getByRole('button', { name: '关闭' })
  await close.focus()
  await page.keyboard.press('Shift+Tab')
  await expect(page.locator('.conflict-dialog footer button')).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(close).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.locator('.conflict-dialog')).toHaveCount(0)
  await expect(page.locator('.string-editor textarea')).toHaveValue('my draft')
})

test('Hash modal comparison keeps editable modal draft and original untouched', async ({ page }) => {
  await openValue(page, { type: 'hash', hash_val: { field: 'hash original' }, total_count: 1 })
  await page.locator('.hash-editor .btn-tiny').filter({ hasText: '编辑' }).first().click()
  await page.locator('.expand-textarea').fill('hash draft')
  await page.locator('.btn-save-modal').click()
  await expect(page.locator('.conflict-dialog')).toHaveCount(0)
  await page.locator('.expand-textarea').fill('revised hash draft')
  await page.locator('.expand-modal').getByRole('button', { name: '查看冲突对比' }).click()
  await expect(page.locator('[data-conflict-column="original"] pre')).toHaveText('hash original')
  await expect(page.locator('[data-conflict-column="draft"] pre')).toHaveText('revised hash draft')
  await expect(page.locator('[data-conflict-column="current"] pre')).toHaveText('server latest')
  expect((await page.evaluate(() => window.__conflictReads[0])).slice(3)).toEqual(['field', true])
  await page.locator('.conflict-dialog').getByRole('button', { name: '关闭' }).click()
  await expect(page.locator('.expand-textarea')).toHaveValue('revised hash draft')
})

test('closed comparison ignores stale response on reopen', async ({ page }) => {
  await stringConflict(page)
  await page.evaluate(() => { window.__delayConflict = true })
  const open = page.locator('.string-editor').getByRole('button', { name: '查看冲突对比' })
  await open.click()
  await expect.poll(() => page.evaluate(() => window.__conflictReads.length)).toBe(1)
  await page.locator('.conflict-dialog').getByRole('button', { name: '关闭' }).click()
  await page.evaluate(() => { window.__delayConflict = false })
  await open.click()
  await expect(page.locator('[data-conflict-column="current"] pre')).toHaveText('server latest')
  await page.evaluate(() => window.__finishConflict({ exists: true, type: 'string', value: 'STALE', total_bytes: 5 }))
  await expect(page.locator('[data-conflict-column="current"] pre')).toHaveText('server latest')
})

test('Hash add conflict compares an absent original and retains the add draft', async ({ page }) => {
  await openValue(page, { type: 'hash', hash_val: { field: 'original' }, total_count: 1 })
  await page.locator('.hash-editor .btn-add').click()
  await page.locator('.add-row input').nth(0).fill('new-field')
  await page.locator('.add-row input').nth(1).fill('new draft')
  await page.locator('.add-row-btn-primary').click()
  await page.locator('.hash-editor').getByRole('button', { name: '查看冲突对比' }).click()
  await expect(page.locator('[data-conflict-column="original"] pre')).toHaveCount(0)
  await expect(page.locator('[data-conflict-column="draft"] pre')).toHaveText('new draft')
  await expect(page.locator('[data-conflict-column="current"] pre')).toHaveText('server latest')
  expect((await page.evaluate(() => window.__conflictReads[0])).slice(3)).toEqual(['new-field', true])
  await page.locator('.conflict-dialog').getByRole('button', { name: '关闭' }).click()
  await expect(page.locator('.add-row input').nth(1)).toHaveValue('new draft')
})

for (const mode of ['missing', 'binary', 'truncated', 'type', 'failed']) test(`comparison clearly marks ${mode} without overwriting draft`, async ({ page }) => {
  await stringConflict(page)
  await page.evaluate(mode => {
    window.__failConflict = mode === 'failed'
    window.__conflictValue = {
      exists: mode !== 'missing', type: mode === 'type' ? 'hash' : 'string',
      binary: mode === 'binary', truncated: mode === 'truncated',
      value: mode === 'truncated' ? '中'.repeat(40000) : 'server latest', total_bytes: 120000,
    }
  }, mode)
  await page.locator('.string-editor').getByRole('button', { name: '查看冲突对比' }).click()
  const current = page.locator('[data-conflict-column="current"]')
  if (mode === 'truncated') {
    await expect(current).toContainText('64 KiB')
    expect(new TextEncoder().encode(await current.locator('pre').innerText()).length).toBeLessThanOrEqual(65536)
  } else if (mode === 'type') await expect(current).toContainText('string → hash')
  else if (mode === 'failed') await expect(current.locator('[role="alert"]')).toBeVisible()
  else if (mode === 'binary') await expect(current).toContainText('Base64')
  else await expect(current.locator('pre')).toHaveCount(0)
  await expect(page.locator('.string-editor textarea')).toHaveValue('my draft')
})
