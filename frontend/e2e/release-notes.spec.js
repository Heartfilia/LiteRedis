import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

for (const language of ['zh', 'en']) {
  test(`installed release notes are offline, localized and reopenable (${language})`, async ({ page }) => {
    await installWailsMock(page)
    await page.addInitScript(({ language }) => {
      const app = window.go.main.App
      window.__notesRequests = 0
      window.go.main.App = new Proxy(app, { get(target, method) {
        if (method === 'GetSettings') return async () => ({ ...await target.GetSettings(), language, last_seen_version: '0.8.2' })
        if (method === 'GetAppVersion') return async () => '0.8.2'
        if (method === 'GetReleaseNotes') return async () => { window.__notesRequests++; throw new Error('offline') }
        return target[method]
      } })
    }, { language })
    await page.goto('/')
    await page.locator('.btn-settings').click()
    const open = page.getByRole('button', { name: language === 'zh' ? '查看新功能' : 'View new features' })
    await open.click()
    await expect(page.locator('.wn-version')).toHaveText('v0.8.2')
    await expect(page.locator('.wn-notes')).toContainText('SSH')
    await expect(page.locator('.wn-notes')).not.toContainText(language === 'zh' ? '网页跳转' : '/releases/latest')
    const other = language === 'zh' ? 'en' : 'zh'
    await page.evaluate(async other => {
      const { setLanguage } = await import('/src/i18n/index.js')
      setLanguage(other)
    }, other)
    const body = await page.locator('.wn-notes').innerText()
    expect(/[\u4e00-\u9fff]/.test(body)).toBe(other === 'zh')
    await page.keyboard.press('Escape')
    await expect(page.locator('.settings-panel')).toBeVisible()
    await page.getByRole('button', { name: other === 'zh' ? '查看新功能' : 'View new features' }).click()
    await expect(page.locator('.wn-notes')).toBeVisible()
    expect(await page.evaluate(() => window.__notesRequests)).toBe(0)
  })
}

test('unknown installed versions never display pending changes', async ({ page }) => {
  await installWailsMock(page)
  await page.goto('/')
  await page.evaluate(() => {
    window.go.main.App = new Proxy(window.go.main.App, { get(target, method) {
      if (method === 'GetAppVersion') return async () => '9.9.9'
      return target[method]
    } })
  })
  await page.locator('.btn-settings').click()
  await page.getByRole('button', { name: '查看新功能' }).click()
  await expect(page.locator('.wn-version')).toHaveText('v9.9.9')
  await expect(page.locator('.wn-notes')).toContainText('暂无内置更新说明')
  await expect(page.locator('.wn-notes')).not.toContainText('302')
})
