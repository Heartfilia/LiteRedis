import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

const STORAGE_KEY = 'liteRedis_savedSearches_v1'

// Use the real Pinia workspace, real saved-search UI and Wails adapter. Only the
// native backend is mocked; assert the SelectDB/ScanKeys calls at that boundary.
async function activate(page, { connID = 'conn-dev', db = 0 } = {}) {
  await page.evaluate(async ({ connID, db }) => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    const { useSettingsStore } = await import('/src/stores/settings.js')
    const settings = useSettingsStore()
    settings.loaded = true
    settings.keyScanCount = 200
    const connections = useConnectionsStore()
    connections.stopHeartbeat()
    await connections.loadConnections()
    connections.connectedIds.add(connID)
    const workspace = useWorkspaceStore()
    workspace.activeConnID = connID
    workspace.activeConnName = connections.connections.find(conn => conn.id === connID)?.name || connID
    workspace.currentDB = db
    workspace.openTabs = [connID]
    window.__savedSearchCalls = []
    const backend = window.__savedSearchBackend || window.go.main.App
    window.__savedSearchBackend = backend
    window.go.main.App = new Proxy(backend, {
      get(target, method) {
        if (method === 'SelectDB' || method === 'ScanKeys') return async (...args) => {
          window.__savedSearchCalls.push({ method, args })
          return method === 'SelectDB' ? { success: true } : { keys: [], next_cursor: 0, has_more: false }
        }
        return target[method]
      },
    })
  }, { connID, db })
  const panel = page.locator('.saved-searches')
  await expect(panel).toBeVisible()
  if (!await panel.evaluate(element => element.open)) await panel.locator('summary').click()
  return panel
}

async function open(page, options) {
  await installWailsMock(page)
  await page.goto('/')
  return activate(page, options)
}

async function save(panel, name = 'Order lookup', pattern = 'order:*') {
  await panel.locator('form input').nth(0).fill(name)
  await panel.locator('form input').nth(1).fill(pattern)
  await panel.locator('form button').click()
  await expect(panel.locator('.saved-search-run').filter({ hasText: name })).toBeVisible()
}

async function calls(page) {
  return page.evaluate(() => window.__savedSearchCalls)
}

async function stored(page) {
  return page.evaluate(key => JSON.parse(localStorage.getItem(key) || '[]'), STORAGE_KEY)
}

// Retained interaction cases for reopening this paused feature.
test.describe.skip('saved-search UI is temporarily not exposed', () => {
test('save retains name, connection, DB and glob through reload without executing a query', async ({ page }) => {
  let panel = await open(page, { db: 4 })
  await save(panel, 'Order lookup', 'order:[12]*')
  expect(await calls(page)).toEqual([])
  expect(await stored(page)).toEqual([
    expect.objectContaining({ name: 'Order lookup', connID: 'conn-dev', db: 4, pattern: 'order:[12]*', lastUsed: 0 }),
  ])
  await expect(panel.locator('.saved-search-run')).toContainText('DB 4')
  await expect(panel.locator('.saved-search-run small')).toHaveText('order:[12]*')
  await page.reload()
  panel = await activate(page)
  await expect(panel.locator('.saved-search-run')).toContainText('Order lookup')
  await expect(panel.locator('.saved-search-run')).toContainText('DB 4')
  expect(await calls(page)).toEqual([])
  const [item] = await stored(page)
  expect(Object.keys(item).sort()).toEqual(['connID', 'db', 'id', 'lastUsed', 'name', 'pattern'])
})

test('one click selects saved DB before scanning the exact saved glob and records last use', async ({ page }) => {
  const panel = await open(page, { db: 4 })
  await save(panel, 'Orders DB4', 'order:*')
  await page.evaluate(async () => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    useWorkspaceStore().currentDB = 0
    window.__savedSearchCalls = []
  })
  await panel.locator('.saved-search-run').click()
  await expect.poll(() => calls(page)).toEqual([
    { method: 'SelectDB', args: ['conn-dev', 4] },
    { method: 'ScanKeys', args: ['conn-dev', 'order:*', 200, 0] },
  ])
  await expect.poll(async () => (await stored(page))[0].lastUsed).toBeGreaterThan(0)
  expect(await page.evaluate(async () => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    return useWorkspaceStore().currentDB
  })).toBe(4)
})

test('disconnected saved searches do not select DB, scan or record a successful use', async ({ page }) => {
  const panel = await open(page, { db: 3 })
  await save(panel)
  await page.evaluate(async () => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    useConnectionsStore().connectedIds.delete('conn-dev')
  })
  await panel.locator('.saved-search-run').click()
  await expect(panel.getByRole('status')).toContainText(/连接|connect/i)
  await expect(panel.locator('.saved-search-run')).toBeEnabled()
  expect(await calls(page)).toEqual([])
  expect((await stored(page))[0].lastUsed).toBe(0)
})

test('cancelling the dirty-editor guard preserves draft, DB and saved-search last use', async ({ page }) => {
  const panel = await open(page, { db: 5 })
  await save(panel)
  await page.evaluate(async () => {
    const { registerEditorGuard } = await import('/src/utils/editSafety.js')
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    useWorkspaceStore().currentDB = 0
    window.__savedSearchDraft = 'unsaved editor content'
    window.__savedSearchUnregister = registerEditorGuard('saved-search-e2e-draft', () => ({
      dirty: true, saving: false,
      discard: () => { window.__savedSearchDraft = '' },
    }))
  })
  await panel.locator('.saved-search-run').click()
  await expect(page.getByRole('alertdialog')).toBeVisible()
  await page.locator('.confirmation-cancel').click()
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await expect(panel.locator('.saved-search-run')).toBeEnabled()
  expect(await calls(page)).toEqual([])
  expect((await stored(page))[0].lastUsed).toBe(0)
  expect(await page.evaluate(() => window.__savedSearchDraft)).toBe('unsaved editor content')
  expect(await page.evaluate(async () => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    return useWorkspaceStore().currentDB
  })).toBe(0)
  await page.evaluate(() => window.__savedSearchUnregister())
})

test('saved entries remain isolated per connection and run only against their owning connection', async ({ page }) => {
  let panel = await open(page)
  await save(panel, 'Dev users', 'dev:*')
  panel = await activate(page, { connID: 'conn-prod', db: 2 })
  await expect(panel.locator('.saved-search-run')).toHaveCount(0)
  await save(panel, 'Backup users', 'backup:*')
  await expect(panel.locator('.saved-search-run')).toHaveCount(1)
  await expect(panel.locator('.saved-search-run')).not.toContainText('Dev users')
  await panel.locator('.saved-search-run').click()
  await expect.poll(() => calls(page)).toEqual([
    { method: 'ScanKeys', args: ['conn-prod', 'backup:*', 200, 0] },
  ])
  panel = await activate(page, { connID: 'conn-dev' })
  await expect(panel.locator('.saved-search-run')).toHaveCount(1)
  await expect(panel.locator('.saved-search-run')).toContainText('Dev users')
  await expect(panel.locator('.saved-search-run')).not.toContainText('Backup users')
  expect((await stored(page)).map(item => item.connID).sort()).toEqual(['conn-dev', 'conn-prod'])
})

})

test('saved-search entry stays hidden and stored data is preserved', async ({ page }) => {
  await installWailsMock(page)
  const data = JSON.stringify([{ id: 'saved-1', name: 'Orders', connID: 'conn-dev', db: 4, pattern: 'order:*', lastUsed: 0 }])
  await page.addInitScript(({ key, data }) => localStorage.setItem(key, data), { key: STORAGE_KEY, data })
  await page.goto('/')
  await page.locator('.conn-item .conn-main').first().click()
  await expect(page.locator('.key-search-bar')).toBeVisible()
  await expect(page.locator('.saved-searches')).toHaveCount(0)
  await expect(page.locator('.key-search-bar')).not.toContainText('搜索条件收藏')
  expect(await page.evaluate(key => localStorage.getItem(key), STORAGE_KEY)).toBe(data)
})
