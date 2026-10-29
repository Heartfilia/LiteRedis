import assert from 'node:assert/strict'
import test from 'node:test'

import { escapeHtml, renderReleaseNotes } from './markdown.js'

test('escapeHtml 转义标签与引号', () => {
  assert.equal(escapeHtml('<b>"x"</b>'), '&lt;b&gt;&quot;x&quot;&lt;/b&gt;')
})

test('标题、列表、粗体、行内代码、裸链接按预期渲染', () => {
  const html = renderReleaseNotes([
    "## What's Changed",
    '* Fix **key tree** crash by @a in https://github.com/o/r/pull/1',
    '* Add `copy` menu',
  ].join('\n'))
  assert.match(html, /<h3>What&#39;s Changed<\/h3>/)
  assert.match(html, /<ul>/)
  assert.match(html, /<strong>key tree<\/strong>/)
  assert.match(html, /<code>copy<\/code>/)
  assert.match(html, /<a href="https:\/\/github.com\/o\/r\/pull\/1"/)
  assert.equal((html.match(/<ul>/g) || []).length, 1)
})

test('有序列表与分隔线', () => {
  const html = renderReleaseNotes(['1. first', '2. second', '---', 'tail'].join('\n'))
  assert.match(html, /<ol>/)
  assert.match(html, /<li>first<\/li>/)
  assert.match(html, /<hr \/>/)
  assert.match(html, /<p>tail<\/p>/)
})

test('Markdown 链接只在 http(s) 时转成 a 标签', () => {
  const ok = renderReleaseNotes('[docs](https://example.com/a)')
  assert.match(ok, /<a href="https:\/\/example.com\/a"/)
  const bad = renderReleaseNotes('[x](javascript:alert(1))')
  assert.doesNotMatch(bad, /<a /)
  assert.match(bad, /javascript/)
})

test('恶意 HTML 被转义，不会注入标签', () => {
  const html = renderReleaseNotes(['## <img src=x onerror=alert(1)>', '* <script>bad()</script>'].join('\n'))
  assert.doesNotMatch(html, /<img/)
  assert.doesNotMatch(html, /<script>/)
  assert.match(html, /&lt;script&gt;/)
})
