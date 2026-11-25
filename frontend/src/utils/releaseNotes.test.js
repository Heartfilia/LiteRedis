import assert from 'node:assert/strict'
import test from 'node:test'

import { PENDING_RELEASE_NOTES, RELEASE_NOTES } from '../changelog.js'
import { getLocalReleaseNotes, validateReleaseNotes } from './releaseNotes.js'

const releaseUrl = 'https://github.com/Heartfilia/LiteRedis/releases/tag/v0.8.2'

test('正式版本精确匹配，去除 v 前缀并返回对应语言及发布页', () => {
  assert.deepEqual(getLocalReleaseNotes(' v0.8.2 '), {
    version: '0.8.2',
    body: RELEASE_NOTES['0.8.2'].zh,
    url: releaseUrl,
  })
  assert.deepEqual(getLocalReleaseNotes('0.8.2', 'en'), {
    version: '0.8.2',
    body: RELEASE_NOTES['0.8.2'].en,
    url: releaseUrl,
  })
  assert.equal(getLocalReleaseNotes('V0.8.2', 'en-US').body, RELEASE_NOTES['0.8.2'].en)
  assert.equal(getLocalReleaseNotes('0.8.2', 'zh-CN').body, RELEASE_NOTES['0.8.2'].zh)
})

test('空版本和 dev 使用待发布日志，保留 dev 标识且没有正式发布链接', () => {
  for (const version of [undefined, null, '', '  ', 'dev']) {
    assert.deepEqual(getLocalReleaseNotes(version), {
      version: 'dev',
      body: PENDING_RELEASE_NOTES.zh,
      url: '',
    })
  }
  assert.deepEqual(getLocalReleaseNotes('DEV', 'en'), {
    version: 'DEV',
    body: PENDING_RELEASE_NOTES.en,
    url: '',
  })
})

test('local 包预览待发布日志并保留原本 local 版本标识', () => {
  for (const version of ['v0.8.2-local', '0.8.2-local.abc123', '0.8.2-local+abc123']) {
    assert.deepEqual(getLocalReleaseNotes(version, 'en'), {
      version,
      body: PENDING_RELEASE_NOTES.en,
      url: '',
    })
    assert.equal(validateReleaseNotes(version), false)
  }
})

test('未知正式版本不可回退到待发布或最新版本，预览正式版也需精确匹配', () => {
  for (const version of ['0.8.4', 'v9.9.9', '0.8', '0.8.2-beta.1', 'constructor', '__proto__']) {
    assert.equal(getLocalReleaseNotes(version), null)
    assert.equal(getLocalReleaseNotes(version, 'en'), null)
    assert.equal(validateReleaseNotes(version), false)
  }
  assert.equal(Object.hasOwn(RELEASE_NOTES, '0.8.4'), false)
})

test('历史中文字符串保持原文，缺少英文翻译时返回 null', () => {
  for (const version of ['0.7.0', '0.7.1']) {
    assert.equal(typeof RELEASE_NOTES[version], 'string')
    assert.equal(getLocalReleaseNotes(version).body, RELEASE_NOTES[version])
    assert.equal(getLocalReleaseNotes(version, 'en'), null)
    assert.equal(validateReleaseNotes(version), false)
  }
  assert.equal(getLocalReleaseNotes('0.8.2', 'fr'), null)
  assert.equal(getLocalReleaseNotes('dev', 'fr'), null)
})

test('已发布变化按0.8.2和0.8.3分版，待发布日志不混入已发更新链路', () => {
  for (const language of ['zh', 'en']) {
    const body = RELEASE_NOTES['0.8.2'][language]
    assert.match(body, /0\.7\.6/)
    assert.match(body, /SSH/)
    assert.match(body, /AnotherRedisDesktopManager/)
    assert.match(body, /Tiny RDM/)
    assert.match(body, /4/)
    assert.match(body, /127\.0\.0\.1:1081/)
    assert.match(body, /2000/)
    assert.doesNotMatch(body, /API|302|软件说明|Software Information/)
    assert.match(RELEASE_NOTES['0.8.3'][language], /API/)
    assert.doesNotMatch(PENDING_RELEASE_NOTES[language], /302|API quotas/)
    assert.equal(validateReleaseNotes('0.8.3'), true)
  }
})

test('CI 仅允许明确正式版本的非空中英文说明，不允许待发布日志', () => {
  assert.equal(validateReleaseNotes('v0.8.2'), true)
  assert.equal(validateReleaseNotes('0.8.2'), true)
  for (const version of [undefined, null, '', 'dev', 'pending', 'PENDING_RELEASE_NOTES']) {
    assert.equal(validateReleaseNotes(version), false)
  }
})

test('CI 拒绝任何一门语言为空或缺失的条目，查询也不回退到其他语言', () => {
  const key = '0.0.0-test'
  try {
    for (const notes of [
      { zh: '中文', en: '' },
      { zh: '中文', en: '   ' },
      { zh: '中文' },
      { zh: '', en: 'English' },
      { zh: '   ', en: 'English' },
      { en: 'English' },
      {},
      null,
    ]) {
      RELEASE_NOTES[key] = notes
      assert.equal(validateReleaseNotes(key), false)
    }
    RELEASE_NOTES[key] = { zh: '中文', en: '  ' }
    assert.equal(getLocalReleaseNotes(key, 'en'), null)
    RELEASE_NOTES[key] = { zh: '  ', en: 'English' }
    assert.equal(getLocalReleaseNotes(key, 'zh'), null)
  } finally {
    delete RELEASE_NOTES[key]
  }
})
