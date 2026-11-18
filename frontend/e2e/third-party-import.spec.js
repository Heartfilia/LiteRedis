import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openSettings(page) {
  await installWailsMock(page)
  await page.goto('/')
  await page.locator('.btn-settings').click()
  await expect(page.locator('.settings-panel')).toBeVisible()
}

// Installs deterministic third-party import fakes and records the calls.
async function stubThirdPartyImport(page, scene) {
  await page.evaluate((scene) => {
    const calls = { preview: [], apply: [], cancel: [], list: 0 }
    window.__thirdPartyCalls = calls
    window.__thirdPartyScene = scene
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, {
      get(target, method) {
        if (method === 'PreviewThirdPartyImport') {
          return async (platform) => {
            calls.preview.push(platform)
            if (scene.mode === 'error') throw new Error('文件损坏')
            const skippedItems = scene.mode === 'all-skipped'
              ? [{ name: 'sentinel-prod', reason: '暂不支持 Sentinel 连接' }]
              : [
                  { name: 'sentinel-prod', reason: '暂不支持 Sentinel 连接' },
                  { name: 'cache-prod', reason: '地址已存在（10.0.0.5:6379），不会覆盖现有连接「prod-backup」' },
                ]
            return {
              platform,
              file_name: platform === 'ardm' ? 'connections.ano' : 'connections_20261117.zip',
              canceled: false,
              total: scene.mode === 'all-skipped' ? 3 : 4,
              importable: scene.mode === 'all-skipped' ? 0 : 2,
              skipped: skippedItems.length,
              renamed: scene.mode === 'all-skipped' ? 0 : 1,
              skipped_items: skippedItems,
              warnings: scene.mode === 'all-skipped' ? [] : ['TLS 连接保持跳过证书校验'],
              token: scene.mode === 'all-skipped' ? '' : 'token-1',
            }
          }
        }
        if (method === 'ApplyThirdPartyImport') {
          return async (token) => {
            calls.apply.push(token)
            return { added: 2, updated: 0, skipped: 2, renamed: 1, ssh_added: 0, messages: [] }
          }
        }
        if (method === 'CancelThirdPartyImport') {
          return async (token) => {
            calls.cancel.push(token)
            return { success: true }
          }
        }
        if (method === 'ListConnections') {
          return async () => {
            calls.list++
            return target.ListConnections()
          }
        }
        return target[method]
      },
    })
  }, scene)
}

async function clickImport(page, platform) {
  const row = page.locator('.third-party-platform').filter({ hasText: platform })
  await row.scrollIntoViewIfNeeded()
  await row.getByRole('button', { name: '导入配置文件' }).click()
}

test('ARDM import previews, confirms, applies once and refreshes connections', async ({ page }) => {
  await openSettings(page)
  await stubThirdPartyImport(page, { mode: 'ok' })
  await clickImport(page, 'AnotherRedisDesktopManager')

  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText('确认导入第三方连接配置')
  await expect(dialog).toContainText('文件：connections.ano')
  await expect(dialog).toContainText('可导入 2 条连接，跳过 2 条')
  await expect(dialog).toContainText('sentinel-prod')
  await expect(dialog).toContainText('暂不支持 Sentinel 连接')
  await expect(dialog).toContainText('cache-prod')
  await expect(dialog).toContainText('不会覆盖现有连接「prod-backup」')
  await expect(dialog).toContainText('TLS 连接保持跳过证书校验')
  await expect(dialog).toContainText('导入只会新增连接；相同地址的连接会跳过')

  await dialog.getByRole('button', { name: '开始导入' }).click()
  await expect(page.locator('.settings-toast')).toContainText('导入完成：新增 2 个连接')
  await expect(page.locator('.settings-toast')).toContainText('重名自动编号 1 个')
  await expect(page.locator('.settings-toast')).toContainText('跳过 2 个')

  const calls = await page.evaluate(() => window.__thirdPartyCalls)
  expect(calls.preview).toEqual(['ardm'])
  expect(calls.apply).toEqual(['token-1'])
  expect(calls.cancel).toEqual([])
  expect(calls.list).toBeGreaterThanOrEqual(1)
})

test('Tiny RDM uses the zip platform and cancelling discards the preview', async ({ page }) => {
  await openSettings(page)
  await stubThirdPartyImport(page, { mode: 'ok' })
  await clickImport(page, 'Tiny RDM')

  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText('文件：connections_20261117.zip')
  await dialog.getByRole('button', { name: '取消' }).click()
  await expect(dialog).toHaveCount(0)

  const calls = await page.evaluate(() => window.__thirdPartyCalls)
  expect(calls.preview).toEqual(['tinyrdm'])
  expect(calls.apply).toEqual([])
  expect(calls.cancel).toEqual(['token-1'])
})

test('all-skipped preview reports reasons without offering a write', async ({ page }) => {
  await openSettings(page)
  await stubThirdPartyImport(page, { mode: 'all-skipped' })
  await clickImport(page, 'AnotherRedisDesktopManager')

  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await expect(page.locator('.settings-toast')).toContainText('sentinel-prod')
  await expect(page.locator('.settings-toast')).toContainText('暂不支持 Sentinel 连接')
  const calls = await page.evaluate(() => window.__thirdPartyCalls)
  expect(calls.apply).toEqual([])
  expect(calls.cancel).toEqual([])
})

test('invalid export surfaces an error instead of a confirmation', async ({ page }) => {
  await openSettings(page)
  await stubThirdPartyImport(page, { mode: 'error' })
  await clickImport(page, 'AnotherRedisDesktopManager')

  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await expect(page.locator('.settings-toast')).toContainText('文件损坏')
  const calls = await page.evaluate(() => window.__thirdPartyCalls)
  expect(calls.apply).toEqual([])
})