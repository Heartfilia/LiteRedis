import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openTemplates(page, { production = false, readOnly = false } = {}) {
  await installWailsMock(page)
  await page.goto('/')
  await page.locator('.conn-item .conn-main').first().click()
  await page.evaluate(async ({ production, readOnly }) => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    const cs = useConnectionsStore()
    const ws = useWorkspaceStore()
    const connection = cs.connections.find(item => item.id === ws.activeConnID)
    connection.environment = production ? 'production' : ''
    connection.read_only = readOnly
    window.__templateTarget = { id: connection.id, name: connection.name }
    window.__templateCommands = []
    window.__templateForced = []
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ExecuteRedisCommand') return async (...args) => {
        window.__templateCommands.push(args)
        return readOnly ? { success: false, blocked: true } : { success: true, output: 'OK' }
      }
      if (method === 'ExecuteRedisCommandForce') return async (...args) => {
        window.__templateForced.push(args)
        return { success: true, output: 'OK' }
      }
      return target[method]
    } })
  }, { production, readOnly })
  await page.keyboard.press('Meta+k')
  await page.locator('.cmdk-input').fill('命令台')
  await page.keyboard.press('Enter')
  await expect(page.locator('.console-input')).toBeVisible()
  await page.locator('.console-templates summary').click()
  await expect(page.locator('.command-templates')).toBeVisible()
}

async function saveAndSelect(page, body = 'GET {{key}}') {
  const panel = page.locator('.command-templates')
  await panel.locator('.template-form input').fill('Read safely')
  await panel.locator('.template-form textarea').fill(body)
  await panel.getByRole('button', { name: '保存模板', exact: true }).click()
  await expect(panel).toContainText('模板已保存')
  await panel.locator('li').getByRole('button', { name: '准备并填入命令台' }).click()
  return panel
}

test('template save, safely quoted preview and prepare never execute or persist parameter values', async ({ page }) => {
  await openTemplates(page)
  const panel = await saveAndSelect(page)
  const value = 'secret key "quoted" \\ tail'
  const expected = 'GET "' + value.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"'
  await panel.locator('.template-prepare input').fill(value)
  await expect(panel.locator('.template-prepare pre')).toHaveText(expected)
  await expect(panel).toContainText('参数值不保存')
  await expect(panel).toContainText('DB 0')
  await panel.locator('.template-prepare').getByRole('button', { name: '准备并填入命令台' }).click()
  await expect(page.locator('.console-input')).toHaveValue(expected)
  expect(await page.evaluate(() => window.__templateCommands)).toEqual([])
  const persisted = await page.evaluate(() => localStorage.getItem('liteRedis_commandTemplates_v1'))
  expect(persisted).toContain('GET {{key}}')
  expect(persisted).not.toContain('secret key')
  expect(persisted).not.toContain('quoted')
  await page.locator('.console-input').press('Enter')
  await expect.poll(() => page.evaluate(() => window.__templateCommands.length)).toBe(1)
  expect(await page.evaluate(() => window.__templateCommands[0][1])).toBe(expected)
})

test('prepared production write still requires typed confirmation at execution', async ({ page }) => {
  await openTemplates(page, { production: true })
  const panel = await saveAndSelect(page, 'SET {{key}} {{value}}')
  await panel.locator('.template-prepare input').nth(0).fill('orders:key')
  await panel.locator('.template-prepare input').nth(1).fill('draft value')
  await panel.locator('.template-prepare').getByRole('button', { name: '准备并填入命令台' }).click()
  expect(await page.evaluate(() => window.__templateCommands.length)).toBe(0)
  await page.locator('.console-input').press('Enter')
  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.locator('.confirmation-accept')).toBeDisabled()
  expect(await page.evaluate(() => window.__templateCommands.length)).toBe(0)
  await dialog.locator('input').fill('CONFIRM')
  await dialog.locator('.confirmation-accept').click()
  await expect.poll(() => page.evaluate(() => window.__templateCommands.length)).toBe(1)
  expect(await page.evaluate(() => window.__templateCommands[0][1])).toBe('SET "orders:key" "draft value"')
})

test('prepared read-only write follows existing blocked-command confirmation, never forces automatically', async ({ page }) => {
  await openTemplates(page, { readOnly: true })
  const panel = await saveAndSelect(page, 'SET {{key}} {{value}}')
  await panel.locator('.template-prepare input').nth(0).fill('protected:key')
  await panel.locator('.template-prepare input').nth(1).fill('draft')
  await panel.locator('.template-prepare').getByRole('button', { name: '准备并填入命令台' }).click()
  expect(await page.evaluate(() => window.__templateCommands.length)).toBe(0)
  await page.locator('.console-input').press('Enter')
  await expect(page.locator('.confirmation-cancel')).toBeVisible()
  expect(await page.evaluate(() => window.__templateForced)).toEqual([])
  await page.locator('.confirmation-cancel').click()
  expect(await page.evaluate(() => window.__templateCommands.length)).toBe(1)
  expect(await page.evaluate(() => window.__templateForced)).toEqual([])
})

test('template fill protects existing unexecuted console input', async ({ page }) => {
  await openTemplates(page)
  const panel = await saveAndSelect(page)
  await panel.locator('.template-prepare input').fill('new key')
  await page.locator('.console-input').fill('GET old-key')
  await panel.locator('.template-prepare').getByRole('button', { name: '准备并填入命令台' }).click()
  await expect(page.locator('.confirmation-cancel')).toBeVisible()
  await page.locator('.confirmation-cancel').click()
  await expect(page.locator('.console-input')).toHaveValue('GET old-key')
  expect(await page.evaluate(() => window.__templateCommands)).toEqual([])
})
