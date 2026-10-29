// 极简 Markdown 渲染，只覆盖 GitHub Release 说明里常见的几种语法：
// 标题、无序/有序列表、粗体、行内代码、链接、分隔线。
// 先做 HTML 转义再做替换，避免更新说明里的内容被当成标签执行。

export function escapeHtml(text) {
  return String(text ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function renderInline(raw) {
  let text = escapeHtml(raw)
  text = text.replace(/`([^`]+)`/g, '<code>$1</code>')
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  text = text.replace(
    /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g,
    '<a href="$2" target="_blank" rel="noreferrer">$1</a>'
  )
  // 裸链接（前面必须是行首/空白/左括号，避免把已有 href 里的地址再包一层）
  text = text.replace(
    /(^|[\s(])(https?:\/\/[^\s<)]+)/g,
    '$1<a href="$2" target="_blank" rel="noreferrer">$2</a>'
  )
  return text
}

export function renderReleaseNotes(markdown) {
  const lines = String(markdown ?? '').replace(/\r\n?/g, '\n').split('\n')
  const html = []
  let listType = ''
  let paragraph = []

  const closeList = () => {
    if (listType) {
      html.push(listType === 'ul' ? '</ul>' : '</ol>')
      listType = ''
    }
  }
  const flushParagraph = () => {
    if (!paragraph.length) return
    html.push('<p>' + renderInline(paragraph.join(' ')) + '</p>')
    paragraph = []
  }
  const openList = (type) => {
    if (listType === type) return
    closeList()
    html.push(type === 'ul' ? '<ul>' : '<ol>')
    listType = type
  }

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) {
      flushParagraph()
      closeList()
      continue
    }
    if (/^(-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
      flushParagraph()
      closeList()
      html.push('<hr />')
      continue
    }
    const heading = trimmed.match(/^(#{1,6})\s+(.*)$/)
    if (heading) {
      flushParagraph()
      closeList()
      const level = Math.min(6, heading[1].length + 1) // ## → h3，避免弹窗里标题过大
      html.push('<h' + level + '>' + renderInline(heading[2]) + '</h' + level + '>')
      continue
    }
    const bullet = trimmed.match(/^[-*+]\s+(.*)$/)
    if (bullet) {
      flushParagraph()
      openList('ul')
      html.push('<li>' + renderInline(bullet[1]) + '</li>')
      continue
    }
    const ordered = trimmed.match(/^\d+[.)]\s+(.*)$/)
    if (ordered) {
      flushParagraph()
      openList('ol')
      html.push('<li>' + renderInline(ordered[1]) + '</li>')
      continue
    }
    closeList()
    paragraph.push(trimmed)
  }

  flushParagraph()
  closeList()
  return html.join('\n')
}
