import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

const source = {
  id: 'original', name: 'Production', group: 'Servers', sort_order: 8,
  created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-02T00:00:00Z',
  host: 'redis.internal', port: 6380, username: 'acl-user', password: ' secret ', db: 4,
  tls_enabled: true, tls_skip_verify: true, tls_ca_file: '/ca.pem', read_only: true,
  is_cluster: true, allow_cluster_scan: true, cluster_addrs: ['redis-a:6380', 'redis-b:6380'],
  proxy_enabled: false, proxy_url: 'socks5://user:pass@proxy:1080', icon_color: '#10b981',
  ssh_enabled: true, ssh_profile_id: 'jump',
  ssh: { host: 'jump.internal', port: 2222, user: 'ubuntu', password: ' ssh secret ', private_key_path: '/key', passphrase: ' phrase ' },
}

async function setup(page, { connections = [source], theme = 'light', language = 'zh' } = {}) {
  await installWailsMock(page, { connections })
  await page.addInitScript(({ connections, source }) => {
    window.__connections = structuredClone(connections)
    window.__savedConfigs = []
    window.__connectCalls = 0
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ListConnections') return async () => structuredClone(window.__connections)
      if (method === 'ListSSHProfiles') return async () => [{ id: 'jump', name: 'Jump account', ssh: source.ssh }]
      if (method === 'Connect') return async () => { window.__connectCalls++; return { success: true } }
      if (method === 'SaveConnection') return async cfg => {
        window.__savedConfigs.push(structuredClone(cfg))
        if (window.__holdSave) await new Promise(resolve => { window.__finishSave = resolve })
        if (window.__saveError) return { success: false, message: window.__saveError }
        const saved = { ...structuredClone(cfg), id: cfg.id || `copy-${window.__savedConfigs.length}`, group: (cfg.group || '').trim() }
        const index = window.__connections.findIndex(conn => conn.id === saved.id)
        if (index >= 0) window.__connections[index] = saved
        else window.__connections.unshift(saved) // Deliberately not last in the list.
        return { success: true }
      }
      return target[method]
    } })
  }, { connections, source })
  await page.goto('/')
  await expect.poll(() => page.evaluate(async () => {
    const { useSettingsStore } = await import('/src/stores/settings.js')
    return useSettingsStore().loaded
  })).toBe(true)
  await page.evaluate(async ({ theme, language }) => {
    const { useSettingsStore } = await import('/src/stores/settings.js')
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'GetSettings') return async () => ({ ...await target.GetSettings(), language })
      return target[method]
    } })
    await useSettingsStore().load()
    useSettingsStore().setTheme(theme)
  }, { theme, language })
}

async function openManager(page) {
  await page.locator('button[title="管理连接"], button[title="Manage Connections"]').click()
  await expect(page.locator('.cm-card')).toBeVisible()
}

for (const entry of ['manager', 'sidebar']) test(`copy from ${entry} preserves all configuration and opens only the independent copy`, async ({ page }) => {
  await setup(page, { theme: entry === 'manager' ? 'light' : 'dark' })
  if (entry === 'manager') await openManager(page)
  const row = page.locator(entry === 'manager' ? '.cm-sidebar .cm-conn-item' : '.conn-item .conn-main').filter({ hasText: 'Production' }).first()
  await row.click({ button: 'right' })
  await page.locator('.ctx-copy:visible').click()
  await expect(page.locator('.cm-card')).toBeVisible()
  await expect(page.locator('.connection-form input').first()).toHaveValue('Production copy')
  await expect(page.locator('.cm-conn-item.selected .conn-name')).toHaveText('Production copy')
  await expect(page.locator('#redis-ssh-profile')).toHaveValue('jump')
  const saved = await page.evaluate(() => window.__savedConfigs[0])
  const expected = { ...source, id: '', name: 'Production copy' }
  delete expected.created_at
  delete expected.updated_at
  delete expected.sort_order
  expect(saved).toEqual(expected)
  expect(await page.evaluate(() => window.__connections.find(conn => conn.id === 'original'))).toEqual(source)
  expect(await page.evaluate(() => window.__connectCalls)).toBe(0)
  // Changing and saving the copy must not modify the original record.
  await page.locator('.connection-form input').first().fill('Adjusted copy')
  await page.locator('.connection-form .btn-primary').click()
  await expect.poll(() => page.evaluate(() => window.__connections.find(conn => conn.id === 'copy-1').name)).toBe('Adjusted copy')
  expect(await page.evaluate(() => window.__connections.find(conn => conn.id === 'original'))).toEqual(source)
})

test('copy names ignore case/whitespace within the same group and increment on repeated copies', async ({ page }) => {
  await setup(page, { connections: [source,
    { ...source, id: 'collision', name: ' PRODUCTION COPY ', group: ' Servers ' },
    { ...source, id: 'other', name: 'Production copy 2', group: 'Other' },
  ] })
  await openManager(page)
  for (const name of ['Production copy 2', 'Production copy 3']) {
    await page.locator('.cm-sidebar .cm-conn-item').filter({ has: page.locator('.conn-name', { hasText: /^Production$/ }) }).click({ button: 'right' })
    await page.locator('.ctx-copy:visible').click()
    await expect(page.locator('.connection-form input').first()).toHaveValue(name)
  }
  expect(await page.evaluate(() => window.__savedConfigs.map(cfg => cfg.name))).toEqual(['Production copy 2', 'Production copy 3'])
})

test('failed copy reports the error and leaves the original selection and config untouched', async ({ page }) => {
  await setup(page)
  await openManager(page)
  const row = page.locator('.cm-sidebar .cm-conn-item').first()
  await row.click()
  await page.evaluate(() => { window.__saveError = 'disk full' })
  await row.click({ button: 'right' })
  await page.locator('.ctx-copy:visible').click()
  await expect(page.getByText('disk full', { exact: true })).toBeVisible()
  await expect(page.locator('.connection-form input').first()).toHaveValue('Production')
  expect(await page.evaluate(() => window.__connections)).toEqual([source])
  expect(await page.evaluate(() => window.__connectCalls)).toBe(0)
})

test('pending copy blocks duplicate requests for the same source', async ({ page }) => {
  await setup(page)
  await openManager(page)
  const row = page.locator('.cm-sidebar .cm-conn-item').first()
  await page.evaluate(() => { window.__holdSave = true })
  await row.click({ button: 'right' })
  await page.locator('.ctx-copy:visible').click()
  await row.click({ button: 'right' })
  await expect(page.locator('.ctx-copy:visible')).toBeDisabled()
  expect(await page.evaluate(() => window.__savedConfigs.length)).toBe(1)
  await page.evaluate(() => window.__finishSave())
  await expect(page.locator('.connection-form input').first()).toHaveValue('Production copy')
})

test('English context menu duplicates ungrouped proxy connections', async ({ page }) => {
  const proxy = { ...source, group: '', is_cluster: false, cluster_addrs: [], ssh_enabled: false, ssh_profile_id: '', ssh: null, proxy_enabled: true }
  await setup(page, { connections: [proxy], language: 'en' })
  await openManager(page)
  await page.locator('.cm-sidebar .cm-conn-item').click({ button: 'right' })
  await expect(page.locator('.ctx-copy:visible')).toHaveText('Duplicate connection')
  await page.locator('.ctx-copy:visible').click()
  await expect(page.locator('.connection-form input').first()).toHaveValue('Production copy')
  const saved = await page.evaluate(() => window.__savedConfigs[0])
  expect(saved.proxy_enabled).toBe(true)
  expect(saved.proxy_url).toBe(proxy.proxy_url)
  expect(saved.group).toBe('')
})
