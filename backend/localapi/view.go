package localapi

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"
)

// LocatorDetail 定位解析结果（含展示所需的元信息）
type LocatorDetail struct {
	Value      string
	Key        string
	Type       string
	DB         int
	Position   string
	UpdatedAt  time.Time
	TotalCount int64
	HasMore    bool
}

// ResolveLocatorDetail 的实现位于 registry.go（全局槽位语义）。

type locatorError struct{ msg string }

func (e locatorError) Error() string { return e.msg }

func errLocator(format string, args ...any) error {
	return locatorError{msg: fmt.Sprintf(format, args...)}
}

// ---------- 内置 JSON 查看页 ----------

type viewPageData struct {
	RawValue template.JS
}

var viewPageTpl = template.Must(template.New("view").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LiteRedis · 数据提取</title>
<style>
  * { box-sizing: border-box; }
  html, body { height: 100%; }
  body {
    margin: 0; display: flex; flex-direction: column;
    font-family: ui-monospace, Menlo, Consolas, "PingFang SC", monospace;
    background: #0f172a; color: #cbd5e1; font-size: 13px;
  }
  header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 10px 16px; border-bottom: 1px solid rgba(51, 65, 85, 0.8);
    background: rgba(15, 23, 42, 0.95); flex: none;
  }
  .brand { font-weight: 700; color: #e2e8f0; }
  .brand span { color: #64748b; font-weight: 400; font-size: 12px; margin-left: 8px; }
  .btn {
    padding: 6px 14px; border: 1px solid rgba(51, 65, 85, 0.9); border-radius: 8px;
    background: rgba(30, 41, 59, 0.9); font-size: 13px; cursor: pointer; color: #cbd5e1;
  }
  .btn:hover { border-color: #3b82f6; color: #93c5fd; }
  .btn.copied { border-color: #16a34a; color: #4ade80; }
  #content {
    flex: 1; overflow: auto; padding: 14px 18px; margin: 0;
    line-height: 1.65; white-space: pre-wrap; word-break: break-all;
  }
  /* 可折叠树 */
  .node { padding-left: 18px; }
  .row { display: flex; align-items: baseline; gap: 6px; cursor: pointer; border-radius: 4px; }
  .row:hover { background: rgba(51, 65, 85, 0.45); }
  .arrow { display: inline-block; width: 14px; color: #64748b; transition: transform 0.12s; }
  .collapsed > .row > .arrow { transform: rotate(-90deg); }
  .collapsed > .children { display: none; }
  .children { }
  .k { color: #7dd3fc; }
  .punct { color: #64748b; }
  .summary { color: #64748b; font-size: 12px; }
  .v-str { color: #a5d6a7; }
  .v-num { color: #fbbf24; }
  .v-bool { color: #f472b6; }
  .v-null { color: #64748b; font-style: italic; }
  .plain { white-space: pre-wrap; word-break: break-all; }
</style>
</head>
<body>
<header>
  <div class="brand">LiteRedis<span>本地数据提取 · 数据来自界面当前已加载内容</span></div>
  <button id="copyBtn" class="btn">复制内容</button>
</header>
<pre id="content" class="plain"></pre>

<script>
const RAW = {{.RawValue}};

function esc(s) { return String(s); }
function span(cls, text) {
  const el = document.createElement('span');
  if (cls) el.className = cls;
  el.textContent = text;
  return el;
}

function valueSpan(value) {
  if (value === null) return span('v-null', 'null');
  switch (typeof value) {
    case 'string': return span('v-str', '"' + value + '"');
    case 'number': return span('v-num', String(value));
    case 'boolean': return span('v-bool', String(value));
    default: return span('', String(value));
  }
}

function appendComma(parent, isLast) {
  if (!isLast) parent.appendChild(span('punct', ','));
}

function buildTree(container, key, value, isLast) {
  const isObj = value !== null && typeof value === 'object';
  if (!isObj) {
    const leaf = document.createElement('div');
    leaf.className = 'node leaf';
    if (key !== null) {
      leaf.appendChild(span('k', '"' + key + '"'));
      leaf.appendChild(span('punct', ': '));
    }
    leaf.appendChild(valueSpan(value));
    appendComma(leaf, isLast);
    container.appendChild(leaf);
    return;
  }

  const isArray = Array.isArray(value);
  const entries = isArray ? value.map((v, i) => [String(i), v]) : Object.entries(value);

  const node = document.createElement('div');
  node.className = 'node';
  const row = document.createElement('div');
  row.className = 'row';
  const arrow = span('arrow', '▾');
  row.appendChild(arrow);
  if (key !== null) {
    row.appendChild(span('k', '"' + key + '"'));
    row.appendChild(span('punct', ': '));
  }
  row.appendChild(span('punct', isArray ? '[ ' : '{ '));
  row.appendChild(span('summary', entries.length + ' 项'));
  row.appendChild(span('punct', isArray ? ' ]' : ' }'));
  appendComma(row, isLast);
  node.appendChild(row);

  const children = document.createElement('div');
  children.className = 'children';
  entries.forEach(([k, v], i) => buildTree(children, k, v, i === entries.length - 1));
  node.appendChild(children);

  row.addEventListener('click', () => node.classList.toggle('collapsed'));
  container.appendChild(node);
}

const content = document.getElementById('content');
let parsed = null;
try { parsed = JSON.parse(RAW); } catch (e) { /* 非 JSON，按纯文本展示 */ }

if (parsed !== null && typeof parsed === 'object') {
  content.className = '';
  const root = document.createElement('div');
  root.className = 'tree';
  buildTree(root, null, parsed, true);
  content.appendChild(root);
} else {
  content.textContent = RAW;
}

const btn = document.getElementById('copyBtn');
btn.addEventListener('click', async () => {
  let ok = false;
  try { await navigator.clipboard.writeText(RAW); ok = true; }
  catch (e) {
    const ta = document.createElement('textarea');
    ta.value = RAW;
    document.body.appendChild(ta);
    ta.select();
    try { ok = document.execCommand('copy'); } catch (e2) {}
    document.body.removeChild(ta);
  }
  if (ok) {
    btn.textContent = '✓ 已复制';
    btn.classList.add('copied');
    setTimeout(() => { btn.textContent = '复制内容'; btn.classList.remove('copied'); }, 1500);
  }
});
</script>
</body>
</html>
`))

// ---------- 错误页 ----------

type viewErrorData struct {
	Icon    string
	Title   string
	Message string
	Hint    string
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}

var viewErrorTpl = template.Must(template.New("viewError").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LiteRedis · 无法提取数据</title>
<style>
  * { box-sizing: border-box; }
  body {
    margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
    font-family: -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
    background: #0f172a; color: #cbd5e1; font-size: 14px;
  }
  .card {
    max-width: 460px; width: calc(100% - 48px);
    background: rgba(30, 41, 59, 0.85); border: 1px solid rgba(51, 65, 85, 0.9);
    border-radius: 16px; padding: 36px 32px; text-align: center;
  }
  .icon { font-size: 44px; margin-bottom: 14px; }
  .title { font-size: 18px; font-weight: 700; color: #e2e8f0; margin-bottom: 10px; }
  .message { line-height: 1.7; color: #94a3b8; word-break: break-all; }
  .hint {
    margin-top: 18px; padding-top: 16px; border-top: 1px solid rgba(51, 65, 85, 0.7);
    font-size: 12px; color: #64748b; line-height: 1.7;
  }
</style>
</head>
<body>
<div class="card">
  <div class="icon">{{.Icon}}</div>
  <div class="title">{{.Title}}</div>
  <div class="message">{{.Message}}</div>
  <div class="hint">{{.Hint}}</div>
</div>
</body>
</html>
`))

// renderViewError 渲染友好的错误页
func renderViewError(w http.ResponseWriter, status int, icon, title, message, hint string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = viewErrorTpl.Execute(w, viewErrorData{Icon: icon, Title: title, Message: message, Hint: hint})
}

// handleView 处理 /view/{base64}：解码定位 JSON，渲染内置查看页。
// 页面与数据同源（顶层导航 http→http），不存在混合内容/CORS 问题。
func (s *Server) handleView(w http.ResponseWriter, encoded string) {
	const genericHint = "请回到 LiteRedis 重新生成提取链接后重试"
	if encoded == "" {
		renderViewError(w, http.StatusBadRequest, "🔗", "链接不完整",
			"提取链接缺少定位内容，可能复制时被截断。", genericHint)
		return
	}
	data, err := decodeLocatorBase64(encoded)
	if err != nil {
		renderViewError(w, http.StatusBadRequest, "🔗", "链接无法解析",
			"提取链接格式有误（Base64 解码失败），请重新从 LiteRedis 复制完整链接。", genericHint)
		return
	}
	var loc Locator
	if err := json.Unmarshal(data, &loc); err != nil {
		renderViewError(w, http.StatusBadRequest, "🔗", "链接无法解析",
			"提取链接内容不是有效的定位数据，请重新从 LiteRedis 复制。", genericHint)
		return
	}

	token := s.Token()
	if token == "" || subtle.ConstantTimeCompare([]byte(loc.Token), []byte(token)) != 1 {
		renderViewError(w, http.StatusUnauthorized, "🔑", "访问凭证无效",
			"链接中的访问凭证与当前 LiteRedis 不匹配（可能在设置中重新生成过 Token，或链接来自旧版本）。", genericHint)
		return
	}

	detail, err := s.registry.ResolveLocatorDetail(loc)
	if err != nil {
		switch {
		case containsStr(err.Error(), "no snapshot available"):
			renderViewError(w, http.StatusNotFound, "📭", "暂无可提取数据",
				"还没有从 LiteRedis 提取过任何数据。",
				"在 LiteRedis 中对某个 Key 点击「浏览器解析」后，这里会展示对应的数据。")
		case containsStr(err.Error(), "expired"):
			renderViewError(w, http.StatusNotFound, "⏳", "提取链接已过期",
				"槽位中已被更新的提取内容覆盖（全局只保留最近一次提取的数据）。",
				"回到 LiteRedis 对需要的 Key 重新点击「浏览器解析」即可。")
		case containsStr(err.Error(), "not published"):
			renderViewError(w, http.StatusNotFound, "📭", "数据尚未加载",
				"该数据还没有在 LiteRedis 界面中加载过，或对应的连接已断开。",
				"请在 LiteRedis 中打开对应的 Key（需要提取哪一条就打开哪个 Key），然后重新生成链接重试。")
		case containsStr(err.Error(), "no published snapshots"):
			renderViewError(w, http.StatusNotFound, "🔌", "连接不可用",
				"定位指向的 Redis 连接与当前提取槽位不一致（可能连接已切换或断开）。",
				"请在 LiteRedis 中连接并重新点击「浏览器解析」。")
		case containsStr(err.Error(), "db mismatch"):
			renderViewError(w, http.StatusNotFound, "🔀", "数据库不一致",
				err.Error(), "请在 LiteRedis 中切换到对应数据库后重新生成提取链接。")
		case containsStr(err.Error(), "type mismatch"):
			renderViewError(w, http.StatusNotFound, "🔀", "类型不一致",
				err.Error(), "请在 LiteRedis 中重新打开该 Key 后重新生成提取链接。")
		default:
			renderViewError(w, http.StatusNotFound, "⚠️", "定位失败",
				err.Error(), genericHint)
		}
		return
	}

	// json.Marshal 会转义 < > & 与 U+2028/2029，嵌入 <script> 安全
	rawJSON, err := json.Marshal(detail.Value)
	if err != nil {
		writeText(w, http.StatusInternalServerError, "render failed")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = viewPageTpl.Execute(w, viewPageData{
		RawValue: template.JS(rawJSON),
	})
}
