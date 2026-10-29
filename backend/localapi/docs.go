package localapi

import (
	_ "embed"
	"net/http"
	"strconv"
	"strings"
)

//go:embed openapi.json
var openAPISpec []byte

func (s *Server) handleOpenAPI(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(openAPISpec)
}

// handleDocs 输出本地调试页面（同源，可直接调用 /api/v1/*）
func (s *Server) handleDocs(w http.ResponseWriter) {
	page := strings.ReplaceAll(docsPage, "__TOKEN__", s.Token())
	page = strings.ReplaceAll(page, "__REDIS_ENABLED__", strconv.FormatBool(s.options().RedisEnabled))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
}

const docsPage = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>LiteRedis 本地 API 调试</title>
<style>
  :root { color-scheme: light; }
  * { box-sizing: border-box; }
  body { margin: 0; background: #f4f7fb; color: #1f2937; font: 13px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
  .wrap { max-width: 960px; margin: 0 auto; padding: 24px 20px 60px; }
  header { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 6px; }
  h1 { font-size: 18px; margin: 0; }
  .sub { color: #64748b; font-size: 12px; margin: 4px 0 18px; }
  .pill { font-size: 11px; padding: 3px 10px; border-radius: 999px; border: 1px solid #bbf7d0; background: #ecfdf5; color: #15803d; }
  .pill.off { border-color: #fecaca; background: #fef2f2; color: #b91c1c; }
  .bar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 10px 12px; margin-bottom: 18px; }
  .bar label { font-size: 12px; color: #64748b; }
  .bar input { flex: 1; min-width: 240px; padding: 6px 10px; border: 1px solid #cbd5e1; border-radius: 8px; font: 12px ui-monospace, Menlo, monospace; }
  .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 14px 16px; margin-bottom: 14px; }
  .card-head { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .method { font: 700 10.5px ui-monospace, Menlo, monospace; color: #1d4ed8; background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 6px; padding: 2px 7px; }
  .path { font: 12.5px ui-monospace, Menlo, monospace; color: #0f172a; }
  .title { font-size: 12px; color: #64748b; margin-left: auto; }
  .desc { font-size: 12px; color: #64748b; margin: 8px 0 10px; }
  textarea { width: 100%; min-height: 96px; resize: vertical; padding: 10px 12px; border: 1px solid #cbd5e1; border-radius: 8px; font: 12px/1.6 ui-monospace, Menlo, monospace; color: #0f172a; background: #fbfdff; outline: none; }
  textarea:focus { border-color: #60a5fa; box-shadow: 0 0 0 3px rgba(191, 219, 254, 0.35); }
  .actions { display: flex; gap: 8px; align-items: center; margin-top: 10px; }
  button { font-size: 12px; font-weight: 600; padding: 6px 14px; border-radius: 8px; cursor: pointer; border: 1px solid #cbd5e1; background: #fff; color: #475569; }
  button:hover { border-color: #94a3b8; color: #1e293b; }
  button.primary { background: #2563eb; border-color: #2563eb; color: #fff; }
  button.primary:hover { background: #1d4ed8; }
  button:disabled { opacity: 0.55; cursor: default; }
  .status { font-size: 11.5px; color: #64748b; margin-left: auto; }
  .status.ok { color: #15803d; }
  .status.err { color: #b91c1c; }
  pre { margin: 10px 0 0; padding: 10px 12px; border-radius: 8px; background: #0f172a; color: #dbeafe; font: 11.5px/1.65 ui-monospace, Menlo, monospace; max-height: 320px; overflow: auto; white-space: pre-wrap; word-break: break-word; }
  pre:empty { display: none; }
  .hint { font-size: 11.5px; color: #94a3b8; margin-top: 4px; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>LiteRedis 本地 API 调试</h1>
    <span id="state" class="pill">检测中…</span>
  </header>
  <div class="sub">只监听 127.0.0.1，全部接口只读。参数放在 POST 的 JSON body 里，响应统一为 { ok, data, error, elapsed_ms }。</div>

  <div class="bar">
    <label for="token">Token</label>
    <input id="token" spellcheck="false" placeholder="关闭 token 校验时可留空" />
    <button id="saveToken">记住</button>
    <a class="hint" href="/openapi.json" target="_self">openapi.json</a>
  </div>

  <div id="cards"></div>
</div>
<script>
  var TOKEN = "__TOKEN__";
  var REDIS_ENABLED = __REDIS_ENABLED__;

  var stateEl = document.getElementById('state');
  if (REDIS_ENABLED) {
    stateEl.textContent = 'Redis API 已开启';
  } else {
    stateEl.textContent = 'Redis API 未开启';
    stateEl.className = 'pill off';
  }

  var tokenInput = document.getElementById('token');
  try {
    tokenInput.value = localStorage.getItem('liteRedisApiToken') || TOKEN || '';
  } catch (e) {
    tokenInput.value = TOKEN || '';
  }
  document.getElementById('saveToken').onclick = function () {
    try { localStorage.setItem('liteRedisApiToken', tokenInput.value.trim()); } catch (e) {}
  };

  var ENDPOINTS = [
    { name: 'connections', title: '已连接的实例', desc: '列出当前已连接实例的 id / 名字 / 所在 DB，其它接口的 conn 就用这里的 id。', body: {} },
    { name: 'overview', title: '实例概览', desc: '等价于界面右上角的连接概览（INFO 摘要）。', body: { conn: '' } },
    { name: 'keys', title: '扫描 key', desc: 'SCAN 一页 key，返回 next_cursor；count 上限 500。', body: { conn: '', pattern: '*', count: 100, cursor: 0 } },
    { name: 'key', title: '读取 key', desc: '类型、TTL 和值。hash/set 用 cursor 翻页，list/zset 用 offset，stream 用 stream_start；string 过长会标记 truncated。', body: { conn: '', key: '', count: 100, offset: 0, cursor: 0 } },
    { name: 'command', title: '只读命令', desc: '写命令（SET/DEL/FLUSHALL…）会被拒绝，只放行只读命令白名单。', body: { conn: '', command: 'GET your-key' } }
  ];

  var cardsEl = document.getElementById('cards');
  ENDPOINTS.forEach(function (ep, index) {
    var card = document.createElement('div');
    card.className = 'card';

    var head = document.createElement('div');
    head.className = 'card-head';
    var method = document.createElement('span');
    method.className = 'method';
    method.textContent = 'POST';
    var path = document.createElement('span');
    path.className = 'path';
    path.textContent = '/api/v1/' + ep.name;
    var title = document.createElement('span');
    title.className = 'title';
    title.textContent = ep.title;
    head.appendChild(method);
    head.appendChild(path);
    head.appendChild(title);

    var desc = document.createElement('div');
    desc.className = 'desc';
    desc.textContent = ep.desc;

    var area = document.createElement('textarea');
    area.value = JSON.stringify(ep.body, null, 2);
    area.id = 'body-' + ep.name;

    var actions = document.createElement('div');
    actions.className = 'actions';
    var send = document.createElement('button');
    send.className = 'primary';
    send.textContent = '发送';
    var curlBtn = document.createElement('button');
    curlBtn.textContent = '复制 curl';
    var status = document.createElement('span');
    status.className = 'status';
    actions.appendChild(send);
    actions.appendChild(curlBtn);
    actions.appendChild(status);

    var out = document.createElement('pre');

    function buildHeaders() {
      var headers = { 'Content-Type': 'application/json' };
      var token = (tokenInput.value || '').trim();
      if (token) headers['Authorization'] = 'Bearer ' + token;
      return headers;
    }

    send.onclick = function () {
      send.disabled = true;
      status.className = 'status';
      status.textContent = '请求中…';
      var started = Date.now();
      fetch('/api/v1/' + ep.name, {
        method: 'POST',
        headers: buildHeaders(),
        body: area.value
      }).then(function (res) {
        return res.text().then(function (text) {
          var parsed = text;
          try { parsed = JSON.stringify(JSON.parse(text), null, 2); } catch (e) {}
          status.className = 'status ' + (res.ok ? 'ok' : 'err');
          status.textContent = res.status + ' ' + (res.statusText || '') + ' · ' + (Date.now() - started) + 'ms';
          out.textContent = parsed;
        });
      }).catch(function (err) {
        status.className = 'status err';
        status.textContent = '请求失败';
        out.textContent = String(err);
      }).then(function () {
        send.disabled = false;
      });
    };

    curlBtn.onclick = function () {
      var token = (tokenInput.value || '').trim();
      var curl = 'curl -s -X POST http://' + location.host + '/api/v1/' + ep.name +
        " -H 'Content-Type: application/json'" +
        (token ? " -H 'Authorization: Bearer " + token + "'" : '') +
        " -d '" + area.value.replace(/s+/g, ' ').replace(/'/g, '') + "'";
      navigator.clipboard && navigator.clipboard.writeText(curl);
      status.className = 'status ok';
      status.textContent = '已复制 curl';
    };

    card.appendChild(head);
    card.appendChild(desc);
    card.appendChild(area);
    card.appendChild(actions);
    card.appendChild(out);
    cardsEl.appendChild(card);
  });
</script>
</body>
</html>`
