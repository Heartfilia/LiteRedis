import { expect, test } from '@playwright/test'
import { installWailsMock } from './wails-mock.js'

async function openValue(page, value) {
  await installWailsMock(page, { keyCount: 200 })
  await page.goto('/')
  await page.evaluate((value) => {
    const original = window.go.main.App
    window.__writes = []
    window.__editResult = { success: true, edit_token: 'updated-token' }
    window.go.main.App = new Proxy(original, {
      get(target, method) {
        if (method === 'GetValue') return async (_conn, key) => ({ key, type: 'string', string_val: '', edit_token: 'original-token', ...value })
        if (method === 'SetStringChecked' || method === 'HSetChecked') return async (...args) => { window.__writes.push(args); return window.__editResult }
        return target[method]
      },
    })
  }, value)
  await page.locator('.conn-item .conn-main').first().click()
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
  await page.locator('.tree-bulk-btn[title="展开全部"]').click()
  await page.locator('.tree-scroll .node-row.leaf-row').first().click()
}

for (const reason of ['truncated', 'binary', 'changed']) test(`${reason} String preview cannot be edited or saved`, async ({ page }) => {
  await openValue(page, { string_val: 'partial value', truncated: reason === 'truncated', read_only_reason: reason })
  await expect(page.locator('.string-editor textarea')).toHaveAttribute('readonly', '')
  await expect(page.locator('.string-editor').getByRole('button', { name: '保存', exact: true })).toBeDisabled()
  expect(await page.evaluate(() => window.__writes.length)).toBe(0)
})

test('JSON formatting, tree and save retain large numeric lexemes', async ({ page }) => {
  const raw = '{"id":9223372036854775807,"n":1.2300e400,"zero":-0}'
  await openValue(page, { string_val: raw })
  const editor = page.locator('.string-editor')
  await editor.locator('.btn-json').first().click()
  expect(await editor.locator('textarea').inputValue()).toContain('9223372036854775807')
  await editor.locator('.btn-json').nth(1).click()
  await expect(editor.locator('.json-value').filter({ hasText: '9223372036854775807' })).toBeVisible()
  await expect(editor.locator('.json-value').filter({ hasText: '1.2300e400' })).toBeVisible()
  await editor.getByRole('button', { name: '保存', exact: true }).click()
  await expect.poll(() => page.evaluate(() => window.__writes.length)).toBe(1)
  const write = await page.evaluate(() => window.__writes[0])
  expect(write[1]).toBe(0)
  expect(write[3]).toContain('9223372036854775807')
  expect(write[4]).toBe('original-token')
})

test('conflicts preserve String draft and unknown outcome blocks repeat save', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  const editor = page.locator('.string-editor')
  await editor.locator('textarea').fill('my draft')
  await page.evaluate(() => { window.__editResult = { success: false, code: 'conflict' } })
  const save = editor.getByRole('button', { name: '保存', exact: true })
  await save.click()
  await expect(editor.locator('textarea')).toHaveValue('my draft')
  await expect(editor).toContainText('本次没有保存')
  await page.evaluate(() => { window.__editResult = { success: false, code: 'outcome_unknown' } })
  await save.click()
  await expect(save).toBeDisabled()
  await expect(editor.locator('textarea')).toHaveValue('my draft')
  expect(await page.evaluate(() => window.__writes.length)).toBe(2)
})

test('connection navigation can be declined without losing a String draft', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.locator('.string-editor textarea').fill('my draft')
  await page.locator('.conn-item .conn-main').nth(1).click()
  await page.locator('.confirmation-cancel').click()
  await expect(page.locator('.string-editor textarea')).toHaveValue('my draft')
  await page.locator('.conn-item .conn-main').nth(1).click()
  await page.locator('.confirmation-accept').click()
  await expect(page.locator('.string-editor')).toHaveCount(0)
})

test('Hash modal keeps conflicts visible and blocks repeat unknown writes', async ({ page }) => {
  await openValue(page, { type: 'hash', hash_val: { field: 'original' }, total_count: 1 })
  await page.locator('.hash-editor .btn-tiny').filter({ hasText: '编辑' }).first().click()
  await page.locator('.expand-textarea').fill('hash draft')
  await page.evaluate(() => { window.__editResult = { success: false, code: 'conflict' } })
  await page.locator('.btn-save-modal').click()
  await expect(page.locator('.expand-textarea')).toHaveValue('hash draft')
  await expect(page.locator('.expand-modal [role="status"]')).toContainText('本次没有保存')
  await page.evaluate(() => { window.__editResult = { success: false, code: 'outcome_unknown' } })
  await page.locator('.btn-save-modal').click()
  await expect(page.locator('.btn-save-modal')).toBeDisabled()
})

test('mock close request defaults to cancel, traps focus and accepts only explicitly', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.locator('.string-editor textarea').fill('close draft')
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__closed = 0
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'CloseApp') return async () => { window.__closed++ }
      return target[method]
    } })
    window.__wailsMock.emit('app:close-request')
    window.__wailsMock.emit('app:close-request')
  })
  await expect(page.locator('.confirmation-cancel')).toBeFocused()
  await page.keyboard.press('Shift+Tab')
  await expect(page.locator('.confirmation-accept')).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(page.locator('.confirmation-cancel')).toBeFocused()
  await page.keyboard.press('Enter')
  expect(await page.evaluate(() => window.__closed)).toBe(0)
  await expect(page.locator('.string-editor textarea')).toHaveValue('close draft')
  await page.evaluate(() => window.__wailsMock.emit('app:close-request'))
  await page.locator('.confirmation-accept').click()
  await expect.poll(() => page.evaluate(() => window.__closed)).toBe(1)
})

test('quit asks for confirmation with a connected Redis even without a draft', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__closed = 0
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'CloseApp') return async () => { window.__closed++ }
      return target[method]
    } })
    window.__wailsMock.emit('app:close-request')
  })
  await expect(page.getByRole('alertdialog')).toContainText('确定退出 LiteRedis')
  await expect(page.locator('.confirmation-cancel')).toHaveText('继续使用')
  await expect(page.locator('.confirmation-accept')).toHaveText('退出应用')
  await page.keyboard.press('Escape')
  expect(await page.evaluate(() => window.__closed)).toBe(0)
  await page.keyboard.press('Meta+q')
  await expect(page.locator('.confirmation-cancel')).toBeFocused()
  await page.locator('.confirmation-accept').click()
  await expect.poll(() => page.evaluate(() => window.__closed)).toBe(1)
})

for (const profiles of [[], [{ id: 'saved-only', name: 'Saved but disconnected', host: '127.0.0.1', port: 6379, db: 0 }]]) {
  test(`quit without live Redis skips confirmation (${profiles.length} saved profiles)`, async ({ page }) => {
    await installWailsMock(page, { connections: profiles })
    await page.goto('/')
    await expect(page.locator('.app-layout')).toBeVisible()
    await page.evaluate(() => {
      const original = window.go.main.App
      window.__closed = 0
      window.go.main.App = new Proxy(original, { get(target, method) {
        if (method === 'CloseApp') return async () => { window.__closed++ }
        return target[method]
      } })
      window.__wailsMock.emit('app:close-request')
    })
    await expect.poll(() => page.evaluate(() => window.__closed)).toBe(1)
    await expect(page.getByRole('alertdialog')).toHaveCount(0)
    await page.keyboard.press('Meta+q')
    await expect.poll(() => page.evaluate(() => window.__closed)).toBe(2)
    await expect(page.getByRole('alertdialog')).toHaveCount(0)
  })
}

test('quit after all Redis connections are lost skips the draft reminder', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.locator('.string-editor textarea').fill('disconnected draft')
  await page.evaluate(async () => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    useConnectionsStore().connectedIds.clear()
    const original = window.go.main.App
    window.__closed = 0
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'CloseApp') return async () => { window.__closed++ }
      return target[method]
    } })
  })
  await page.keyboard.press('Meta+q')
  await expect.poll(() => page.evaluate(() => window.__closed)).toBe(1)
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
})

test('background connected Redis still requires quit confirmation', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.evaluate(async () => {
    const { useWorkspaceStore } = await import('/src/stores/workspace.js')
    await useWorkspaceStore().setActiveConn(null, '')
    window.__wailsMock.emit('app:close-request')
  })
  await expect(page.getByRole('alertdialog')).toContainText('确定退出 LiteRedis')
})

test('quit dialog has clear actions and themed draft warning', async ({ page }, testInfo) => {
  await openValue(page, { string_val: 'original' })
  await page.keyboard.press('Meta+q')
  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toHaveAccessibleName('确定退出 LiteRedis？')
  await expect(dialog).toHaveAccessibleDescription('退出后将断开当前连接，下次打开可重新连接。')
  await expect(page.locator('.confirmation-cancel')).toBeFocused()
  expect(await dialog.evaluate(el => el.getBoundingClientRect().width)).toBeLessThanOrEqual(360)
  await page.screenshot({ path: testInfo.outputPath('quit-light.png'), animations: 'disabled' })
  await page.keyboard.press('Escape')
  await page.locator('.btn-theme').click()
  await page.locator('.theme-menu-item', { hasText: /^暗色/ }).click()
  await page.locator('.string-editor textarea').fill('unsaved draft')
  await page.keyboard.press('Meta+q')
  await expect(dialog).toHaveClass(/is-danger/)
  await expect(page.locator('.confirmation-overlay')).toHaveClass(/dark/)
  await expect(dialog).toContainText('未保存的编辑')
  await expect(page.locator('.confirmation-accept')).toHaveText('放弃并退出')
  await page.screenshot({ path: testInfo.outputPath('quit-dark-draft.png'), animations: 'disabled' })
  await page.keyboard.press('Escape')
  await expect(page.locator('.string-editor textarea')).toHaveValue('unsaved draft')
})

test('Cmd+W closes only the current key and leaves the connection and tree open', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.locator('.string-editor textarea').focus()
  await page.keyboard.press('Meta+w')
  await expect(page.locator('.string-editor')).toHaveCount(0)
  await expect(page.locator('.key-editor')).toContainText('选择')
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
  await page.keyboard.press('Meta+w')
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await page.locator('.tree-scroll .node-row.leaf-row').first().click()
  await expect(page.locator('.string-editor')).toBeVisible()
})

test('Cmd+W and native Close Key protect dirty drafts and never confirm an open quit dialog', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.locator('.string-editor textarea').fill('keep draft')
  await page.keyboard.press('Meta+w')
  await page.keyboard.press('Escape')
  await expect(page.locator('.string-editor textarea')).toHaveValue('keep draft')
  await page.keyboard.press('Meta+q')
  await expect(page.getByRole('alertdialog')).toContainText('退出 LiteRedis')
  await page.keyboard.press('Meta+w')
  await expect(page.getByRole('alertdialog')).toContainText('退出 LiteRedis')
  await page.keyboard.press('Escape')
  await page.evaluate(() => window.__wailsMock.emit('app:close-key'))
  await page.locator('.confirmation-accept').click()
  await expect(page.locator('.string-editor')).toHaveCount(0)
  await expect(page.locator('.tree-scroll .node-row').first()).toBeVisible()
})

test('mock close request is blocked during save and keeps unknown-result draft', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__closed = 0
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'CloseApp') return async () => { window.__closed++ }
      if (method === 'SetStringChecked') return () => new Promise(resolve => { window.__finishSave = resolve })
      return target[method]
    } })
  })
  const input = page.locator('.string-editor textarea')
  await input.fill('pending draft')
  await page.locator('.string-editor').getByRole('button', { name: '保存', exact: true }).click()
  await expect(input).toBeDisabled()
  await page.keyboard.press('Meta+w')
  await expect(page.getByRole('alertdialog')).toContainText('正在保存')
  await page.locator('.confirmation-cancel').click()
  await expect(input).toBeDisabled()
  await page.evaluate(async () => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    useConnectionsStore().connectedIds.clear()
  })
  await page.evaluate(() => window.__wailsMock.emit('app:close-request'))
  await expect(page.getByRole('alertdialog')).toContainText('正在保存')
  await page.locator('.confirmation-cancel').click()
  expect(await page.evaluate(() => window.__closed)).toBe(0)
  await page.evaluate(() => window.__finishSave({ success: false, code: 'outcome_unknown' }))
  await expect(input).toBeEnabled()
  await page.evaluate(async () => {
    const { useConnectionsStore } = await import('/src/stores/connections.js')
    useConnectionsStore().connectedIds.add('conn-dev')
  })
  await page.evaluate(() => window.__wailsMock.emit('app:close-request'))
  await page.keyboard.press('Escape')
  await expect(input).toHaveValue('pending draft')
  expect(await page.evaluate(() => window.__closed)).toBe(0)
})

test('task panel reports partial progress, cancellation and bounded failure copy', async ({ page }) => {
  await installWailsMock(page)
  await page.goto('/')
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__taskCancelled = ''
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ListOperationTasks') return async () => [{ id: 'task-1', kind: 'copy', conn_id: 'conn-dev', db: 4, target_conn_id: 'conn-prod', target_db: 7, total: 10, processed: 3, succeeded: 2, skipped: 0, failed: 1, state: 'running', failures: [{ key: 'failed-key', code: 'permission_denied', message: 'NOPERM' }] }]
      if (method === 'CancelOperationTask') return async id => { window.__taskCancelled = id; return true }
      return target[method]
    } })
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: async text => { window.__taskCopy = text } } })
  })
  const panel = page.locator('.operation-tasks')
  await expect(panel).toContainText('DB 4')
  await expect(panel).toContainText('DB 7')
  await panel.getByRole('button', { name: '停止后续操作' }).click()
  expect(await page.evaluate(() => window.__taskCancelled)).toBe('task-1')
  await panel.getByRole('button', { name: '复制失败清单' }).click()
  expect(await page.evaluate(() => window.__taskCopy)).toContain('failed-key')
})

test('List push keeps the draft after a lost response and never blindly repeats it', async ({ page }) => {
  await openValue(page, { type: 'list', list_val: ['existing'], total_count: 1 })
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__pushes = 0
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'RPush') return async () => { window.__pushes++; throw new Error('EOF after write') }
      return target[method]
    } })
  })
  const editor = page.locator('.list-editor')
  await editor.locator('.btn-add').click()
  await editor.locator('.add-row input').fill('do not duplicate')
  await editor.locator('.add-row-btn-primary').click()
  await expect(page.locator('.write-warning')).toContainText('写入结果未知')
  await expect(editor.locator('.add-row input')).toHaveValue('do not duplicate')
  await editor.locator('.add-row-btn-primary').click()
  expect(await page.evaluate(() => window.__pushes)).toBe(1)
  await page.locator('.write-warning button').click()
  await page.locator('.confirmation-cancel').click()
  await editor.locator('.add-row-btn-primary').click()
  expect(await page.evaluate(() => window.__pushes)).toBe(1)
  await page.locator('.write-warning button').click()
  await page.locator('.confirmation-accept').click()
  await expect(page.locator('.write-warning')).toHaveCount(0)
})

test('Console unknown write retains command while read-only verification remains available', async ({ page }) => {
  await installWailsMock(page)
  await page.goto('/')
  await page.locator('.conn-item .conn-main').first().click()
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__commands = []
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ExecuteRedisCommand') return async (_id, command) => {
        window.__commands.push(command)
        return command.startsWith('GET') ? { success: true, output: '1' } : { success: false, code: 'outcome_unknown', error: 'response lost' }
      }
      return target[method]
    } })
  })
  await page.keyboard.press('Meta+k')
  await page.locator('.cmdk-input').fill('命令台')
  await page.keyboard.press('Enter')
  const input = page.locator('.console-input')
  await expect(input).toBeVisible()
  await input.fill('INCR counter')
  await input.press('Enter')
  await expect(page.locator('.write-warning')).toBeVisible()
  await expect(input).toHaveValue('INCR counter')
  await page.locator('.overview-close').click()
  await page.locator('.confirmation-cancel').click()
  await expect(input).toHaveValue('INCR counter')
  await input.press('Enter')
  expect(await page.evaluate(() => window.__commands)).toEqual(['INCR counter'])
  await input.fill('GET counter')
  await input.press('Enter')
  await expect.poll(() => page.evaluate(() => window.__commands.length)).toBe(2)
})

test('List inline save failure preserves the editable draft', async ({ page }) => {
  await openValue(page, { type: 'list', list_val: ['original'], total_count: 1 })
  await page.evaluate(() => {
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'LSet') return async () => ({ success: false, code: 'permission_denied', message: 'NOPERM' })
      return target[method]
    } })
  })
  await page.locator('.list-editor .item-val').dblclick()
  const input = page.locator('.list-editor .list-item input')
  await input.fill('list draft')
  await input.press('Enter')
  await expect(input).toHaveValue('list draft')
  await page.locator('.conn-item .conn-main').nth(1).click()
  await page.locator('.confirmation-cancel').click()
  await expect(input).toHaveValue('list draft')
})

test('ZSet score save failure keeps the score draft', async ({ page }) => {
  await openValue(page, { type: 'zset', zset_val: [{ member: 'one', score: 1 }], total_count: 1 })
  await page.evaluate(() => {
    const original = window.go.main.App
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ZAdd') return async () => ({ success: false, code: 'permission_denied', message: 'NOPERM' })
      return target[method]
    } })
  })
  await page.locator('.zset-editor .score-text').click()
  const input = page.locator('.zset-editor .score-cell input')
  await input.fill('42')
  await input.press('Enter')
  await expect(input).toHaveValue('42')
})

test('import entry confirms target DB, skip mode and safe limits before invoking file dialog', async ({ page }) => {
  await openValue(page, { string_val: 'original' })
  await page.evaluate(() => {
    const original = window.go.main.App
    window.__imports = 0
    window.go.main.App = new Proxy(original, { get(target, method) {
      if (method === 'ImportKeysFromFile') return async () => { window.__imports++; return { success: false, message: '已取消' } }
      return target[method]
    } })
  })
  const button = page.locator('.tree-bulk-btn[title="从 JSON 文件导入 key"]')
  await button.click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toContainText('dev / DB 0')
  await expect(dialog).toContainText('不覆盖')
  await expect(dialog).toContainText('5MiB / 1 万项')
  await page.keyboard.press('Escape')
  expect(await page.evaluate(() => window.__imports)).toBe(0)
  await button.click()
  await page.locator('.confirmation-accept').click()
  await expect.poll(() => page.evaluate(() => window.__imports)).toBe(1)
})
