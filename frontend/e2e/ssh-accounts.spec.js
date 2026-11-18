import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openManager(page, { theme = 'light', language = 'zh', profiles = [] } = {}) {
  await installWailsMock(page)
  await page.goto('/')
  await expect(page.locator('.conn-item .conn-main').first()).toBeVisible()
  await expect.poll(() => page.evaluate(async () => {
    const { useSettingsStore } = await import('/src/stores/settings.js')
    return useSettingsStore().loaded
  })).toBe(true)
  await page.evaluate(async ({ theme, language, profiles }) => {
    const { useSettingsStore } = await import('/src/stores/settings.js')
    useSettingsStore().setTheme(theme)
    const original = window.go.main.App
    window.__sshProfiles = profiles
    window.__sshTests = []
    window.__redisTests = []
    window.__savedRedis = []
    window.__sshTestResult = { success: true }
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'GetSettings') return async () => ({ ...await target.GetSettings(), language })
      if (method === 'ListSSHProfiles') return async () => structuredClone(window.__sshProfiles)
      if (method === 'SaveSSHProfile') return async profile => {
        const saved = { ...profile, id: profile.id || `ssh-${window.__sshProfiles.length + 1}` }
        const index = window.__sshProfiles.findIndex(p => p.id === saved.id)
        if (index === -1) window.__sshProfiles.push(saved)
        else window.__sshProfiles[index] = saved
        return { success: true, profile: structuredClone(saved), disconnected_ids: window.__disconnectedIDs || [] }
      }
      if (method === 'TestSSHProfile') return async profile => { window.__sshTests.push(profile); return window.__sshTestResult }
      if (method === 'TestConnection') return async cfg => { window.__redisTests.push(cfg); return { success: true } }
      if (method === 'SaveConnection') return async cfg => { window.__savedRedis.push(cfg); return { success: true } }
      if (method === 'DeleteSSHProfile') return async id => {
        if (window.__sshDeleteError) return { success: false, message: window.__sshDeleteError }
        window.__sshProfiles = window.__sshProfiles.filter(p => p.id !== id)
        return { success: true }
      }
      return target[method]
    } })
    await useSettingsStore().load()
  }, { theme, language, profiles })
  await page.locator(language === 'zh' ? 'button[title="管理连接"]' : 'button[title="Manage Connections"]').click()
  await expect(page.locator('.cm-card')).toBeVisible()
}

const profile = { id: 'jump-1', name: 'Production jump', ssh: { host: 'jump.example.com', port: 2222, user: 'ubuntu', password: ' secret ', private_key_path: '', passphrase: '' } }

async function fillAccount(page, name = 'My jump') {
  await page.locator('#ssh-account-name').fill(name)
  await page.locator('#ssh-account-host').fill('jump.example.com')
  await page.locator('#ssh-account-user').fill('ubuntu')
  await page.locator('#ssh-account-password').fill('  password with spaces  ')
}

for (const theme of ['light', 'dark']) test(`SSH page manages accounts and tests authentication independently (${theme})`, async ({ page }) => {
  await openManager(page, { theme })
  await expect(page.locator('.connection-form')).toBeVisible()
  const actions = page.locator('.cm-header-actions > button')
  await expect(actions.nth(0)).toHaveText('SSH 账号管理')
  await expect(actions.nth(1)).toContainText('新建连接')
  await actions.first().click()
  await expect(page.locator('.ssh-profile-form')).toBeVisible()
  await expect(page.locator('.connection-form')).toBeHidden()
  await expect(page.locator('.ssh-profile-form')).toHaveClass(new RegExp(`theme-${theme}`))
  await fillAccount(page)
  await page.locator('.ssh-test').click()
  await expect(page.locator('.ssh-message')).toContainText('SSH 握手与认证成功')
  expect(await page.evaluate(() => window.__sshTests[0].ssh.password)).toBe('  password with spaces  ')
  expect(await page.evaluate(() => window.__redisTests.length)).toBe(0)
  expect(await page.evaluate(() => window.__sshProfiles.length)).toBe(0)
  await page.locator('.ssh-save').click()
  await expect(page.locator('.ssh-account-item')).toContainText('My jump')
  await expect(page.locator('.ssh-save')).toBeDisabled()
  await page.locator('#ssh-account-name').fill('Renamed jump')
  await page.locator('.ssh-save').click()
  await expect(page.locator('.ssh-account-item')).toContainText('Renamed jump')
  expect(await page.evaluate(() => window.__sshProfiles.length)).toBe(1)
  // Saved credentials must not leak into browser localStorage.
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain('password with spaces')
})

test('Redis SSH dropdown creates an account, returns and preserves the unsaved Redis form', async ({ page }) => {
  await openManager(page)
  await page.locator('.connection-form input').first().fill('Unsaved Redis')
  await page.locator('.ssh-section input[type=checkbox]').check()
  const select = page.locator('#redis-ssh-profile')
  await expect(select.locator('option').last()).toContainText('新建 SSH 账号')
  await select.selectOption('__create_ssh__')
  await expect(page.locator('.ssh-profile-form')).toBeVisible()
  await fillAccount(page)
  await page.locator('.ssh-save').click()
  await expect(page.locator('.connection-form')).toBeVisible()
  await expect(page.locator('.connection-form input').first()).toHaveValue('Unsaved Redis')
  await expect(select).toHaveValue('ssh-1')
  await expect(page.locator('.ssh-section')).toContainText('ubuntu@jump.example.com:22')
  await expect(page.locator('.ssh-section input[type=password]')).toHaveCount(0)
  await page.locator('.connection-form .form-actions button').nth(1).click()
  expect(await page.evaluate(() => window.__redisTests[0].ssh_profile_id)).toBe('ssh-1')
  await page.locator('.connection-form .btn-primary').click()
  expect(await page.evaluate(() => window.__savedRedis[0].ssh_profile_id)).toBe('ssh-1')
  expect(await page.evaluate(() => window.__savedRedis[0].ssh)).toBeNull()
})

test('existing accounts can be selected, managed and reused in Redis settings', async ({ page }) => {
  await openManager(page, { profiles: [profile] })
  await page.locator('.ssh-section input[type=checkbox]').check()
  await page.locator('#redis-ssh-profile').selectOption(profile.id)
  await expect(page.locator('.ssh-manage')).toHaveCount(0)
  await page.locator('.btn-mode').click()
  await page.locator('.ssh-account-item').click()
  await expect(page.locator('#ssh-account-name')).toHaveValue(profile.name)
  await expect(page.locator('#ssh-account-port')).toHaveValue('2222')
  await page.locator('#ssh-account-host').fill('new-jump.example.com')
  await page.locator('.ssh-save').click()
  await page.locator('.btn-mode').click()
  await expect(page.locator('.connection-form')).toBeVisible()
  await expect(page.locator('.ssh-section')).toContainText('new-jump.example.com')
  await page.locator('.btn-mode').click()
  await page.evaluate(() => { window.__sshTestResult = { success: false, message: 'SSH authentication failed' } })
  await page.locator('.ssh-test').click()
  await expect(page.locator('.ssh-message.error')).toContainText('SSH authentication failed')
  await expect(page.locator('.ssh-save')).toBeDisabled()
})

for (const theme of ['light', 'dark']) test(`SSH dropdown and return button have clear sizing and visual distinction (${theme})`, async ({ page }) => {
  await openManager(page, { theme, profiles: [profile] })
  await page.locator('.ssh-section input[type=checkbox]').check()
  const select = page.locator('#redis-ssh-profile')
  await select.selectOption(profile.id)
  const dbIndex = page.locator('.connection-form input[type=number][max="15"]')
  await expect(select).toHaveCSS('height', await dbIndex.evaluate(el => getComputedStyle(el).height))
  await expect(select).toHaveCSS('appearance', 'none')
  await expect(page.locator('.ssh-panel button')).toHaveCount(0)
  await select.focus()
  await expect(select).toBeFocused()
  await expect(select).toHaveCSS('outline-style', 'solid')
  await page.locator('.btn-mode').click()
  await expect(page.locator('.btn-back-redis')).toContainText('Redis 连接管理')
  await page.mouse.move(0, 0)
  await expect(page.locator('.btn-back-redis')).toHaveCSS('background-color', theme === 'dark' ? 'rgb(110, 231, 183)' : 'rgb(4, 120, 87)')
  await page.locator('.btn-back-redis').click()
  await expect(select).toBeVisible()
  await expect(select).toHaveValue(profile.id)
})

test('SSH deletion rejects referenced accounts and confirms before deleting unused accounts', async ({ page }) => {
  await openManager(page, { profiles: [profile] })
  await page.locator('.btn-mode').click()
  await page.locator('.ssh-account-item').click()
  await page.evaluate(() => { window.__sshDeleteError = 'SSH account is used by Redis connection "prod"; unlink it first' })
  await page.locator('.ssh-delete').click()
  await page.locator('.confirmation-accept').click()
  await expect(page.locator('.ssh-message.error')).toContainText('unlink it first')
  await expect(page.locator('.ssh-account-item')).toHaveCount(1)
  await page.evaluate(() => { window.__sshDeleteError = '' })
  await page.locator('.ssh-delete').click()
  await page.locator('.confirmation-cancel').click()
  await expect(page.locator('.ssh-account-item')).toHaveCount(1)
  await page.locator('.ssh-delete').click()
  await page.locator('.confirmation-accept').click()
  await expect(page.locator('.ssh-account-item')).toHaveCount(0)
})

test('SSH drafts survive page switching and require confirmation before replacing or closing', async ({ page }) => {
  await openManager(page, { profiles: [profile] })
  await page.locator('.btn-mode').click()
  await fillAccount(page, 'Draft account')
  await page.locator('.btn-mode').click()
  await page.locator('.btn-mode').click()
  await expect(page.locator('#ssh-account-name')).toHaveValue('Draft account')
  await page.locator('.ssh-account-item').click()
  await page.locator('.confirmation-cancel').click()
  await expect(page.locator('#ssh-account-name')).toHaveValue('Draft account')
  await page.locator('.cm-header .btn-close').click()
  await page.locator('.confirmation-cancel').click()
  await expect(page.locator('.cm-card')).toBeVisible()
  await page.locator('.cm-header .btn-close').click()
  await page.locator('.confirmation-accept').click()
  await expect(page.locator('.cm-card')).toHaveCount(0)
})

test('SSH management is translated in English', async ({ page }) => {
  await openManager(page, { language: 'en' })
  await page.locator('.btn-mode').click()
  await expect(page.locator('.cm-title')).toHaveText('SSH accounts')
  await expect(page.locator('.ssh-test')).toHaveText('Test SSH')
  await expect(page.locator('.ssh-profile-form')).not.toContainText('connManager.')
})

test('SSH tests keep the form open while pending and do not save or run Redis commands', async ({ page }) => {
  await openManager(page)
  await page.locator('.btn-mode').click()
  await fillAccount(page)
  await page.evaluate(() => {
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'TestSSHProfile') return () => new Promise(resolve => { window.__finishSSHTest = resolve })
      return target[method]
    } })
  })
  await page.locator('.ssh-test').click()
  await expect(page.locator('.ssh-test')).toBeDisabled()
  await expect(page.locator('.btn-mode')).toBeDisabled()
  await expect(page.locator('.cm-header .btn-close')).toBeDisabled()
  await page.keyboard.press('Escape')
  await expect(page.locator('.ssh-profile-form')).toBeVisible()
  await page.evaluate(() => window.__finishSSHTest({ success: false, message: 'SSH handshake timed out' }))
  await expect(page.locator('.ssh-message.error')).toContainText('SSH handshake timed out')
  await expect(page.locator('#ssh-account-name')).toHaveValue('My jump')
  await expect(page.locator('.btn-mode')).toBeEnabled()
  expect(await page.evaluate(() => window.__sshProfiles.length)).toBe(0)
  expect(await page.evaluate(() => window.__redisTests.length)).toBe(0)
})

test('saving a changed shared SSH account invalidates affected frontend connection state', async ({ page }) => {
  await openManager(page, { profiles: [profile] })
  await page.evaluate(async () => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    useConnectionsStore().connectedIds.add('conn-dev')
    window.__disconnectedIDs = ['conn-dev']
  })
  await page.locator('.btn-mode').click()
  await page.locator('.ssh-account-item').click()
  await page.locator('#ssh-account-password').fill('new credentials')
  await page.locator('.ssh-save').click()
  await expect(page.locator('.ssh-message')).toContainText('请重新连接')
  expect(await page.evaluate(async () => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    return useConnectionsStore().isConnected('conn-dev')
  })).toBe(false)
})

test('migrated Redis connections show their SSH account and can disable the tunnel', async ({ page }) => {
  await openManager(page, { profiles: [profile] })
  await page.evaluate(async profile => {
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ListConnections') return async () => [{ id: 'conn-dev', name: 'Migrated Redis', host: 'redis.internal', port: 6379, ssh_enabled: true, ssh_profile_id: profile.id, ssh: profile.ssh }]
      return target[method]
    } })
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    await useConnectionsStore().loadConnections()
  }, profile)
  await page.locator('.cm-sidebar .cm-conn-item').filter({ hasText: 'Migrated Redis' }).click()
  await expect(page.locator('#redis-ssh-profile')).toHaveValue(profile.id)
  await expect(page.locator('.ssh-section')).toContainText('ubuntu@jump.example.com:2222')
  await page.locator('.ssh-section input[type=checkbox]').uncheck()
  await page.locator('.connection-form .btn-primary').click()
  expect(await page.evaluate(() => window.__savedRedis[0].ssh_enabled)).toBe(false)
  expect(await page.evaluate(() => window.__savedRedis[0].ssh_profile_id)).toBe('')
  expect(await page.evaluate(() => window.__savedRedis[0].ssh)).toBeNull()
})
