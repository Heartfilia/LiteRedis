import assert from 'node:assert/strict'
import test from 'node:test'

import { appProxyUrlOrDefault, DEFAULT_UPDATE_PROXY_URL } from './appProxy.js'

test('应用包更新代理默认指向本机 1081 端口', () => {
  assert.equal(DEFAULT_UPDATE_PROXY_URL, 'http://127.0.0.1:1081')
  assert.equal(appProxyUrlOrDefault(''), DEFAULT_UPDATE_PROXY_URL)
  assert.equal(appProxyUrlOrDefault('   '), DEFAULT_UPDATE_PROXY_URL)
  assert.equal(appProxyUrlOrDefault(null), DEFAULT_UPDATE_PROXY_URL)
  assert.equal(appProxyUrlOrDefault(undefined), DEFAULT_UPDATE_PROXY_URL)
})

test('保留用户已经配置的代理地址', () => {
  assert.equal(appProxyUrlOrDefault('socks5://127.0.0.1:1080'), 'socks5://127.0.0.1:1080')
  assert.equal(appProxyUrlOrDefault('  http://proxy.internal:8080  '), 'http://proxy.internal:8080')
})