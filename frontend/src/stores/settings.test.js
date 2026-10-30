import assert from 'node:assert/strict'
import test from 'node:test'

import { createPinia, setActivePinia } from 'pinia'

import { normalizeThemePreference, systemThemeMode, useSettingsStore } from './settings.js'

function stubSystemTheme(dark) {
  globalThis.window = {
    matchMedia: () => ({
      matches: dark,
      addEventListener: () => {},
      removeEventListener: () => {},
    }),
  }
}

function setup() {
  setActivePinia(createPinia())
  return useSettingsStore()
}

test('normalizeThemePreference: 只认三个合法值，其它回落到浅色', () => {
  assert.equal(normalizeThemePreference('light'), 'light')
  assert.equal(normalizeThemePreference('dark'), 'dark')
  assert.equal(normalizeThemePreference('auto'), 'auto')
  assert.equal(normalizeThemePreference('green'), 'green')
  assert.equal(normalizeThemePreference('AUTO'), 'light')
  assert.equal(normalizeThemePreference(undefined), 'light')
})

test('systemThemeMode: 按系统偏好返回明暗', () => {
  stubSystemTheme(true)
  assert.equal(systemThemeMode(), 'dark')
  stubSystemTheme(false)
  assert.equal(systemThemeMode(), 'light')
  delete globalThis.window
  assert.equal(systemThemeMode(), 'light')
})

test('setTheme: 明暗直接生效，auto 跟随系统当前偏好', () => {
  stubSystemTheme(true)
  const store = setup()
  store.setTheme('dark')
  assert.equal(store.themePreference, 'dark')
  assert.equal(store.themeMode, 'dark')

  store.setTheme('light')
  assert.equal(store.themeMode, 'light')

  store.setTheme('green')
  assert.equal(store.themePreference, 'green')
  assert.equal(store.themeMode, 'green')

  store.setTheme('auto')
  assert.equal(store.themePreference, 'auto')
  assert.equal(store.themeMode, 'dark')
  // 跟随系统只在浅色 / 暗色之间切换，永远不会落到其它主题
  assert.ok(['light', 'dark'].includes(store.themeMode))
})

test('toggleTheme: 只在明暗之间切，auto 也会被具体选择覆盖', () => {
  stubSystemTheme(true)
  const store = setup()
  store.setTheme('auto')
  assert.equal(store.themeMode, 'dark')

  store.toggleTheme()
  assert.equal(store.themePreference, 'light')
  assert.equal(store.themeMode, 'light')

  store.toggleTheme()
  assert.equal(store.themePreference, 'dark')
  assert.equal(store.themeMode, 'dark')
})
