import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openSettings(page, { language = 'zh', theme = 'light', apiEnabled = false } = {}) {
  await installWailsMock(page)
  await page.goto('/')
  await page.evaluate(async ({ language, theme, apiEnabled }) => {
    const { useSettingsStore } = await import('/src/stores/settings.js')
    const settings = useSettingsStore()
    settings.setTheme(theme)
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'GetSettings') return async () => ({ ...await target.GetSettings(), language, local_api_enabled: apiEnabled })
      if (method === 'GetAppVersion') return async () => 'dev'
      return target[method]
    } })
  }, { language, theme, apiEnabled })
  await page.locator('.btn-settings').click()
  await expect(page.locator('.settings-panel')).toBeVisible()
}

for (const language of ['zh', 'en']) {
  for (const theme of ['light', 'dark']) {
    test(`settings documents shortcuts and ends with installation, hiding developer tools (${language}, ${theme})`, async ({ page }) => {
      // Even an enabled API and a dev build must not expose the developer group by default.
      await openSettings(page, { language, theme, apiEnabled: true })
      const panel = page.locator('.settings-panel')
      await expect(panel).toHaveClass(new RegExp(`theme-${theme}`))
      await expect(panel.locator('.local-api-item')).toHaveCount(0)
      const titles = panel.locator('.settings-body .section-title')
      await expect(titles.last()).toHaveText(language === 'zh' ? '安装说明' : 'Installation guide')
      await expect(titles.nth(await titles.count() - 2)).toHaveText(language === 'zh' ? '快捷键说明' : 'Keyboard shortcuts')
      await expect(panel.locator('.settings-body > :last-child')).toContainText(language === 'zh' ? '查看安装说明' : 'View installation guide')

      const shortcuts = panel.getByRole('region', { name: language === 'zh' ? '快捷键说明' : 'Keyboard shortcuts' })
      await shortcuts.scrollIntoViewIfNeeded()
      await expect(shortcuts.locator('.shortcut-group')).toHaveCount(3)
      await expect(shortcuts.locator('.shortcut-row')).toHaveCount(10)
      await expect(shortcuts.locator('.shortcut-row dt')).toHaveText([
        'Cmd / Ctrl+K', 'Cmd / Ctrl+R', 'Cmd / Ctrl+W', 'Cmd / Ctrl+Q', 'Esc',
        '↑ / ↓', 'Enter', 'Tab', 'Enter', '↑ / ↓',
      ])
      await expect(shortcuts).toContainText(language === 'zh' ? 'macOS 使用 Cmd' : 'Use Cmd')
      await expect(shortcuts.locator('.shortcut-row').nth(2)).toContainText(language === 'zh' ? '仅关闭当前 key' : 'Close only the current key')
      await expect(shortcuts.locator('.shortcut-row').nth(3)).toContainText(language === 'zh' ? '无连接直接退出' : 'quit directly otherwise')
      await expect(shortcuts.locator('.shortcut-row').last()).toContainText(language === 'zh' ? '浏览历史命令' : 'browse command history')
      await expect(shortcuts).not.toContainText('settings.')
      // Keycaps use separate light/dark palettes and stay inside the scrollable panel.
      const keycap = shortcuts.locator('kbd').first()
      expect(await keycap.evaluate(el => getComputedStyle(el).color)).toBe(theme === 'dark' ? 'rgb(226, 232, 240)' : 'rgb(51, 65, 85)')
      expect(await panel.locator('.settings-body').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
    })
  }
}

test('developer API remains gated and appears after installation at the very bottom when unlocked', async ({ page }) => {
  await openSettings(page)
  const panel = page.locator('.settings-panel')
  const title = panel.locator('.settings-title-trigger')
  await title.click({ clickCount: 4 })
  await expect(panel.locator('.local-api-item')).toHaveCount(0)
  await title.click()
  const titles = panel.locator('.settings-body .section-title')
  await expect(titles.last()).toHaveText('开发者')
  await expect(titles.nth(await titles.count() - 2)).toHaveText('安装说明')
  await expect(panel.locator('.settings-body > :last-child')).toHaveClass(/local-api-item/)
  const api = panel.locator('.local-api-item')
  await expect(api).toContainText('API')
  await api.locator('.check-input').first().check()
  await expect(api.locator('.local-api-port')).toBeVisible()
  await api.locator('.check-input').nth(1).check()
  await expect(api).toContainText('关闭 Token 校验')
  // Reopening settings in the same session retains developer mode.
  await panel.locator('.btn-close').click()
  await page.locator('.btn-settings').click()
  await expect(page.locator('.settings-body .section-title').last()).toHaveText('开发者')
  // It is not persisted: a fresh app session hides all developer/API UI again.
  await page.reload()
  await page.locator('.btn-settings').click()
  await expect(page.locator('.local-api-item')).toHaveCount(0)
  await expect(page.locator('.settings-body .section-title').last()).toHaveText('安装说明')
})

test('installation guide still opens and settings still closes with Escape', async ({ page }) => {
  await openSettings(page)
  await page.getByRole('button', { name: '查看安装说明' }).click()
  await expect(page.locator('.guide-overlay')).toBeVisible()
  await page.locator('.guide-close').click()
  await expect(page.locator('.guide-overlay')).toHaveCount(0)
  await expect(page.locator('.settings-panel')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.locator('.settings-panel')).toHaveCount(0)
})
