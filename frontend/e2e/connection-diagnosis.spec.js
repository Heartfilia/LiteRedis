import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

const SECRET = 'RAW-SECRET-DO-NOT-COPY'
const connection = {
  id: 'diagnosis-config', name: 'Diagnosis target', host: 'redis.internal', port: 6379,
  db: 3, password: SECRET, username: 'private-account', group: '',
}
const report = {
  success: false,
  // Defensive display/copy must never use arbitrary raw backend fields.
  raw_error: `redis://private-account:${SECRET}@redis.internal:6379`,
  stages: [
    { stage: 'config', status: 'success', code: 'ok', duration_ms: 1 },
    { stage: 'dns', status: 'skipped', code: 'remote_dns', duration_ms: 0 },
    { stage: 'tcp', status: 'success', code: 'ok', duration_ms: 4 },
    { stage: 'proxy', status: 'skipped', code: 'not_enabled', duration_ms: 0 },
    { stage: 'ssh', status: 'skipped', code: 'not_enabled', duration_ms: 0 },
    { stage: 'tls', status: 'skipped', code: 'not_enabled', duration_ms: 0 },
    { stage: 'redis_auth_ping', status: 'failed', code: 'auth_failed', duration_ms: 6, message: SECRET },
  ],
}

async function openForm(page, language = 'zh') {
  await installWailsMock(page, { connections: [connection] })
  await page.addInitScript(({ report }) => {
    window.__diagnoseCalls = []
    window.__diagnoseSideEffects = []
    window.__diagnoseClipboard = []
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: {
      writeText: async text => { window.__diagnoseClipboard.push(text) },
    } })
    const backend = window.go.main.App
    window.go.main.App = new Proxy(backend, { get(target, method) {
      if (method === 'CancelConnectionDiagnosis') return async requestID => { window.__cancelledDiagnosis = requestID; return true }
      if (method === 'DiagnoseConnectionWithID') return async (requestID, cfg) => {
        window.__diagnosisRequestID = requestID;
        window.__diagnoseCalls.push(structuredClone(cfg))
        if (window.__holdDiagnosis) await new Promise(resolve => { window.__resolveDiagnosis = resolve })
        window.__diagnoseReturned = true
        return structuredClone(report)
      }
      if (['SaveConnection', 'Connect', 'SelectDB', 'TestConnection'].includes(method)) return async (...args) => {
        window.__diagnoseSideEffects.push({ method, args })
        return { success: true }
      }
      return target[method]
    } })
  }, { report })
  await page.goto('/')
  await expect.poll(() => page.evaluate(async () => {
    const { useSettingsStore } = await import('/src/stores/settings.js')
    return useSettingsStore().loaded
  })).toBe(true)
  await page.evaluate(async language => {
    const { setLanguage } = await import('/src/i18n/index.js')
    setLanguage(language)
  }, language)
  await page.locator('button[title="管理连接"], button[title="Manage Connections"]').click()
  await expect(page.locator('.cm-card')).toBeVisible()
  await page.locator('.cm-sidebar .cm-conn-item').filter({ hasText: connection.name }).click()
  await expect(page.locator('.connection-form input').first()).toHaveValue(connection.name)
  return page.locator('.connection-diagnosis')
}

for (const language of ['zh', 'en']) {
  test(`connection form diagnosis localizes stages and does not save, connect or expose raw secrets (${language})`, async ({ page }) => {
    const panel = await openForm(page, language)
    await panel.locator('button').first().click()
    await expect(panel.locator('li')).toHaveCount(7)
    const config = panel.locator('[data-stage="config"]')
    const dns = panel.locator('[data-stage="dns"]')
    const auth = panel.locator('[data-stage="redis_auth_ping"]')
    await expect(config).toHaveAttribute('data-status', 'success')
    await expect(config).toContainText(language === 'zh' ? '成功' : 'Success')
    await expect(dns).toHaveAttribute('data-status', 'skipped')
    await expect(dns).toContainText(language === 'zh' ? '未执行' : 'Not executed')
    await expect(dns).not.toContainText(language === 'zh' ? '成功' : 'Success')
    await expect(auth).toHaveAttribute('data-status', 'failed')
    await expect(auth).toContainText(language === 'zh' ? '失败' : 'Failed')
    await expect(auth).toContainText(language === 'zh' ? '认证失败' : 'Check credentials')
    await expect(panel).not.toContainText('diagnose.')
    await expect(panel).not.toContainText(SECRET)
    const calls = await page.evaluate(() => window.__diagnoseCalls)
    expect(calls).toHaveLength(1)
    expect(calls[0]).toEqual(expect.objectContaining({ id: connection.id, host: connection.host, db: 3, password: SECRET }))
    expect(await page.evaluate(() => window.__diagnoseSideEffects)).toEqual([])
  })
}

test('copy contains only the diagnostic report, never config credentials or raw backend errors', async ({ page }) => {
  const panel = await openForm(page)
  await panel.locator('button').first().click()
  await expect(panel.locator('li')).toHaveCount(7)
  await panel.getByRole('button', { name: '复制脱敏诊断' }).click()
  await expect.poll(() => page.evaluate(() => window.__diagnoseClipboard.length)).toBe(1)
  const copied = await page.evaluate(() => window.__diagnoseClipboard[0])
  expect(copied.split('\n')).toHaveLength(7)
  expect(copied).toContain('DNS')
  expect(copied).toContain('remote_dns')
  expect(copied).toContain('auth_failed')
  for (const forbidden of [SECRET, connection.host, connection.name, connection.username, 'raw_error', 'redis://']) {
    expect(copied).not.toContain(forbidden)
  }
  expect(await page.evaluate(() => window.__diagnoseSideEffects)).toEqual([])
})

test('changing form inputs discards a late report from an older configuration snapshot', async ({ page }) => {
  const panel = await openForm(page)
  await page.evaluate(() => { window.__holdDiagnosis = true })
  await panel.locator('button').first().click()
  await expect.poll(() => page.evaluate(() => typeof window.__resolveDiagnosis)).toBe('function')
  await expect(panel.locator('button').first()).toBeDisabled()
  await page.locator('.connection-form input[placeholder="127.0.0.1"]').fill('changed.internal')
  await expect(panel.locator('button').first()).toBeDisabled()
  await expect.poll(() => page.evaluate(() => window.__cancelledDiagnosis === window.__diagnosisRequestID)).toBe(true)
  await page.evaluate(() => window.__resolveDiagnosis())
  await expect.poll(() => page.evaluate(() => window.__diagnoseReturned)).toBe(true)
  // A frame boundary flushes the completed request and reactive render without
  // depending on Vite's internal optimized dependency URL.
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => resolve())))
  await expect(panel.locator('ol')).toHaveCount(0)
  await expect(panel.getByRole('button', { name: '复制脱敏诊断' })).toHaveCount(0)
  expect(await page.evaluate(() => window.__diagnoseCalls[0].host)).toBe('redis.internal')
  expect(await page.evaluate(() => window.__diagnoseSideEffects)).toEqual([])
})
