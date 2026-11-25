import { test, expect } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

for (const language of ['zh', 'en']) {
  test(`production badge and typed confirmation protect batch writes (${language})`, async ({ page }) => {
    await installWailsMock(page)
    await page.goto('/')
    await page.evaluate(async language => {
      const { useWorkspaceStore } = await import('/src/stores/workspace.js')
      const { useConnectionsStore } = await import('/src/stores/connections.js')
      const { setLanguage } = await import('/src/i18n/index.js')
      setLanguage(language)
      const cs = useConnectionsStore()
      cs.connections = [{ id: 'production', name: 'Orders', environment: 'production' }]
      const ws = useWorkspaceStore()
      ws.activeConnID = 'production'
      ws.currentDB = 4
      ws.selectedKeys = ['order:1', 'order:2']
      window.__productionCalls = []
      const original = window.go.main.App
      window.go.main.App = new Proxy(original, { get(target, method) {
        if (method === 'BatchDeleteKeys') return async (...args) => { window.__productionCalls.push(args); return { success: true, affected: 2 } }
        return target[method]
      } })
      window.__deleteProduction = () => ws.batchDeleteSelected()
    }, language)
    await expect(page.locator('.conn-main .environment-badge')).toHaveText('P')
    await expect(page.locator('.conn-main .environment-badge')).toHaveAttribute('title', language === 'zh' ? '连接环境: 生产' : 'Connection environment: Production')
    await expect(page.locator('.connection-context')).toHaveCount(0)
    await page.evaluate(() => { void window.__deleteProduction() })
    const dialog = page.getByRole('alertdialog')
    await expect(dialog).toContainText('Orders')
    await expect(dialog.locator('.confirmation-accept')).toBeDisabled()
    await dialog.locator('input').fill('delete')
    await expect(dialog.locator('.confirmation-accept')).toBeDisabled()
    await dialog.locator('input').fill('DELETE')
    await expect(dialog.locator('.confirmation-accept')).toBeEnabled()
    await page.keyboard.press('Escape')
    expect(await page.evaluate(() => window.__productionCalls.length)).toBe(0)
    await page.evaluate(() => { void window.__deleteProduction() })
    await dialog.locator('input').fill('DELETE')
    await dialog.locator('.confirmation-accept').click()
    await expect.poll(() => page.evaluate(() => window.__productionCalls.length)).toBe(1)
    expect(await page.evaluate(() => window.__productionCalls[0])).toEqual(['production', ['order:1', 'order:2']])
  })
}

for (const theme of ['light', 'dark']) test(`compact environment badges preserve sidebar names (${theme})`, async ({ page }) => {
  await installWailsMock(page, { connections: [
    { id: 'd', name: 'Development long connection name', environment: 'development', host: 'localhost', port: 6379 },
    { id: 't', name: 'Test connection', environment: 'staging', group: 'Group', host: 'localhost', port: 6379 },
    { id: 'p', name: 'Production connection', environment: 'production', host: 'localhost', port: 6379 },
    { id: 'u', name: 'Unspecified', host: 'localhost', port: 6379 },
  ] })
  await page.goto('/')
  await page.evaluate(async theme => { const { useSettingsStore } = await import('/src/stores/settings.js'); useSettingsStore().setTheme(theme) }, theme)
  await expect(page.locator('.conn-main .environment-badge')).toHaveCount(3)
  for (const [name, letter] of [['Development long connection name', 'D'], ['Test connection', 'T'], ['Production connection', 'P']]) {
    const row = page.locator('.conn-main').filter({ hasText: name })
    await expect(row.locator('.environment-badge')).toHaveText(letter)
    const layout = await row.evaluate(el => {
      const badge = el.querySelector('.environment-badge')
      return { width: badge.getBoundingClientRect().width, height: badge.getBoundingClientRect().height,
        before: badge.previousElementSibling.classList.contains('conn-dot'), after: badge.nextElementSibling.classList.contains('conn-name') }
    })
    expect(layout).toEqual({ width: 15, height: 15, before: true, after: true })
  }
  await expect(page.locator('.conn-main').filter({ hasText: 'Unspecified' }).locator('.environment-badge')).toHaveCount(0)
})
