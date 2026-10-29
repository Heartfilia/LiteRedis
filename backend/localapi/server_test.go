package localapi

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"LiteRedis/backend/config"
)

const testNonce = "test-nonce-123"

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	setTokenFilePathForTest(filepath.Join(t.TempDir(), "api_token"))
	t.Cleanup(func() { setTokenFilePathForTest("") })
	registry := NewKeyRegistry()
	srv := NewServer(registry)
	if err := srv.RegenerateToken(); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(srv.handle))
	t.Cleanup(ts.Close)
	return srv, ts
}

func publishTestSnapshot(t *testing.T, srv *Server) {
	t.Helper()
	err := srv.registry.Publish("c1", "mykey", config.KeyValue{
		Key:        "mykey",
		Type:       "hash",
		HashVal:    map[string]string{"field1": "value1", "a/b": "slash"},
		TotalCount: 2,
	}, 3, "test-conn", testNonce)
	if err != nil {
		t.Fatal(err)
	}
}

func authGet(url string, token string) (*http.Response, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

func TestHealthNoAuth(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}
}

func TestUnauthorizedWithoutToken(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/c1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUnauthorizedWrongToken(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	resp, err := authGet(ts.URL+"/api/c1/mykey", "wrong-token")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestListPublishedKeys(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	resp, err := authGet(ts.URL+"/api/c1", srv.Token())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		ConnID string             `json:"conn_id"`
		Keys   []PublishedKeyInfo `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ConnID != "c1" || len(body.Keys) != 1 || body.Keys[0].Key != "mykey" || body.Keys[0].Type != "hash" || body.Keys[0].DB != 3 {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestFullSnapshot(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	resp, err := authGet(ts.URL+"/api/c1/mykey", srv.Token())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		ConnID     string         `json:"conn_id"`
		Key        string         `json:"key"`
		DB         int            `json:"db"`
		Type       string         `json:"type"`
		HasMore    bool           `json:"has_more"`
		TotalCount int64          `json:"total_count"`
		Data       map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.ConnID != "c1" || body.Key != "mykey" || body.DB != 3 || body.Type != "hash" {
		t.Fatalf("unexpected meta: %+v", body)
	}
	hashVal, _ := body.Data["hash_val"].(map[string]any)
	if hashVal == nil || hashVal["field1"] != "value1" {
		t.Fatalf("unexpected data: %+v", body.Data)
	}
}

func TestSelectorSingleEntry(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	resp, err := authGet(ts.URL+"/api/c1/mykey?selector=field1", srv.Token())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Entry map[string]any `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Entry["value"] != "value1" {
		t.Fatalf("unexpected entry: %+v", body.Entry)
	}
}

func TestSelectorWithSpecialChars(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	resp, err := authGet(ts.URL+"/api/c1/mykey?selector=a%2Fb", srv.Token())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Entry map[string]any `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Entry["value"] != "slash" {
		t.Fatalf("selector with slash should resolve: %+v", body.Entry)
	}
}

func TestSelectorNotFound(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	resp, err := authGet(ts.URL+"/api/c1/mykey?selector=missing", srv.Token())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestUnknownKeyOrConn404(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)

	resp, _ := authGet(ts.URL+"/api/c1/nokey", srv.Token())
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown key status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	resp, _ = authGet(ts.URL+"/api/unknown/mykey", srv.Token())
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown conn status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestKeyContainingSlash(t *testing.T) {
	srv, ts := newTestServer(t)
	if err := srv.registry.Publish("c1", "app/user:1", config.KeyValue{
		Key: "app/user:1", Type: "string", StringVal: "hello",
	}, 0, "", testNonce); err != nil {
		t.Fatal(err)
	}
	// key 含 / 时整体路径应被视为 key
	resp, err := authGet(ts.URL+"/api/c1/app/user:1", srv.Token())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Key  string `json:"key"`
		Type string `json:"type"`
		Data map[string]any
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Key != "app/user:1" || body.Type != "string" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/c1/mykey", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+srv.Token())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

func TestStartAndStopRealServer(t *testing.T) {
	setTokenFilePathForTest(filepath.Join(t.TempDir(), "api_token"))
	t.Cleanup(func() { setTokenFilePathForTest("") })
	registry := NewKeyRegistry()
	srv := NewServer(registry)
	if err := srv.Start(0); err != nil { // port 0 由系统分配，测试端口可用性逻辑
		t.Fatalf("start: %v", err)
	}
	if !srv.IsRunning() || srv.Port() == 0 {
		t.Fatal("server should be running with a port")
	}
	if srv.Token() == "" {
		t.Fatal("token should be generated on start")
	}
	srv.Stop()
	if srv.IsRunning() {
		t.Fatal("server should be stopped")
	}
	// 幂等
	srv.Stop()
}

func encodeLocator(t *testing.T, loc Locator) string {
	t.Helper()
	data, err := json.Marshal(loc)
	if err != nil {
		t.Fatal(err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(data)
}

func TestInfoEndpointPlainText(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)

	url := ts.URL + "/info/" + encodeLocator(t, Locator{
		Redis: "c1", DB: "3", Key: "mykey", Type: "hash", Position: "field1", Token: srv.Token(), Nonce: testNonce,
	})
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "value1" {
		t.Fatalf("body = %q, want value1", body)
	}
}

func TestInfoEndpointNameFallbackAndSpecialChars(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)

	// 按连接名解析 + position 含 /（URL-safe base64 内不受影响）
	url := ts.URL + "/info/" + encodeLocator(t, Locator{
		Redis: "test-conn", DB: "3", Key: "mykey", Type: "hash", Position: "a/b", Token: srv.Token(), Nonce: testNonce,
	})
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "slash" {
		t.Fatalf("body = %q, want slash", body)
	}
}

func TestInfoEndpointErrors(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)

	// 无 token
	resp, _ := http.Get(ts.URL + "/info/" + encodeLocator(t, Locator{Redis: "c1", Key: "mykey"}))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// 错 token
	resp, _ = http.Get(ts.URL + "/info/" + encodeLocator(t, Locator{Redis: "c1", Key: "mykey", Token: "bad"}))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad token status = %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()

	// 坏 base64（@ 在 URL path 中合法但不是 base64 字符）
	resp, _ = http.Get(ts.URL + "/info/@@@@@")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad base64 status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	// 空路径
	resp, _ = http.Get(ts.URL + "/info/")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty payload status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	// 未知 key
	resp, _ = http.Get(ts.URL + "/info/" + encodeLocator(t, Locator{Redis: "c1", DB: "3", Key: "nokey", Type: "hash", Position: "f", Token: srv.Token(), Nonce: testNonce}))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown key status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestInfoEndpointToleratesStandardBase64Padding(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	// 标准 base64（含 +/= 字符）也应可解码
	data, _ := json.Marshal(Locator{Redis: "c1", DB: "3", Key: "mykey", Type: "hash", Position: "field1", Token: srv.Token(), Nonce: testNonce})
	encoded := base64.StdEncoding.EncodeToString(data)
	resp, err := http.Get(ts.URL + "/info/" + encoded)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestInfoEndpointToleratesAppParamSuffix(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)
	// 用户误在外层链接参数直接拼到本地 URL（无 ? 分隔）也应正常解析
	url := ts.URL + "/info/" + encodeLocator(t, Locator{
		Redis: "c1", DB: "3", Key: "mykey", Type: "hash", Position: "field1", Token: srv.Token(), Nonce: testNonce,
	}) + "&app=literedis"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "value1" {
		t.Fatalf("body = %q, want value1", body)
	}
}

func TestTokenPersistsAcrossRestart(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "api_token")
	setTokenFilePathForTest(tokenFile)
	t.Cleanup(func() { setTokenFilePathForTest("") })

	r := NewKeyRegistry()
	s1 := NewServer(r)
	// 首次启动：生成并持久化
	if err := s1.Start(0); err != nil {
		t.Fatalf("start: %v", err)
	}
	first := s1.Token()
	if first == "" {
		t.Fatal("token should be generated")
	}
	s1.Stop()

	// 模拟重启：新 Server 实例应加载同一 token
	s2 := NewServer(NewKeyRegistry())
	if s2.Token() != first {
		t.Fatalf("token should persist across restart: got %q want %q", s2.Token(), first)
	}
	// 重新生成后持久化新值
	if err := s2.RegenerateToken(); err != nil {
		t.Fatal(err)
	}
	if s2.Token() == first {
		t.Fatal("regenerated token should differ")
	}
	s3 := NewServer(NewKeyRegistry())
	if s3.Token() != s2.Token() {
		t.Fatal("regenerated token should persist")
	}
}

func TestStartFailureRecordsError(t *testing.T) {
	setTokenFilePathForTest(filepath.Join(t.TempDir(), "api_token"))
	t.Cleanup(func() { setTokenFilePathForTest("") })

	// 占住端口
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { blocker.Close() })
	port := blocker.Addr().(*net.TCPAddr).Port

	srv := NewServer(NewKeyRegistry())
	if err := srv.Start(port); err == nil {
		t.Fatal("start on occupied port should fail")
	}
	info := srv.Info()
	if info.Running {
		t.Fatal("server should not be running")
	}
	if info.Error == "" {
		t.Fatal("error should be recorded in Info")
	}
	if !strings.Contains(info.Error, strconv.Itoa(port)) {
		t.Fatalf("error should contain port, got: %s", info.Error)
	}

	// 释放端口后重试应成功且清空错误
	if err := blocker.Close(); err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(port); err != nil {
		t.Fatalf("restart after port released: %v", err)
	}
	if info := srv.Info(); info.Error != "" {
		t.Fatalf("error should be cleared after success, got: %s", info.Error)
	}
	srv.Stop()
}

func TestCrossOriginBlockedWithoutCors(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)

	// 任何跨源请求（包括原来的 json.litetools.top）都直接 403，且不再回 CORS 头
	for _, origin := range []string{"https://json.litetools.top", "https://evil.example.com"} {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("origin %s: status = %d, want 403", origin, resp.StatusCode)
		}
		if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
			t.Fatalf("origin %s: unexpected ACAO %q", origin, got)
		}
	}

	// 同源（调试页面自己）放行，但同样不下发 CORS 头
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	req.Header.Set("Origin", ts.URL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("same-origin must not set ACAO, got %q", got)
	}

	// 预检不再返回任何 CORS 许可
	req, _ = http.NewRequest(http.MethodOptions, ts.URL+"/health", nil)
	req.Header.Set("Origin", ts.URL)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "" {
		t.Fatalf("preflight must not allow methods, got %q", got)
	}
}

func TestViewPageRenders(t *testing.T) {
	srv, ts := newTestServer(t)
	publishTestSnapshot(t, srv)

	// 正常渲染
	url := ts.URL + "/view/" + encodeLocator(t, Locator{
		Redis: "c1", DB: "3", Key: "mykey", Type: "hash", Position: "field1", Token: srv.Token(), Nonce: testNonce,
	})
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, want := range []string{"value1", "LiteRedis", "复制内容"} {
		if !strings.Contains(html, want) {
			t.Errorf("page should contain %q", want)
		}
	}
	// 基础信息表已移除，不应再出现 meta 行
	if strings.Contains(html, "<table") {
		t.Error("meta table should be removed from view page")
	}
	// 值应被 JSON 编码嵌入脚本
	if !strings.Contains(html, `"value1"`) {
		t.Error("page should embed JSON-encoded value")
	}

	// 无 token → 401
	badURL := ts.URL + "/view/" + encodeLocator(t, Locator{Redis: "c1", DB: "3", Key: "mykey", Type: "hash", Position: "field1"})
	resp2, err := http.Get(badURL)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token status = %d, want 401", resp2.StatusCode)
	}
}

func TestResolveLocatorDetailMeta(t *testing.T) {
	r := NewKeyRegistry()
	_ = r.Publish("c1", "k", config.KeyValue{
		Key: "k", Type: "list", ListVal: []string{"a", "b"}, TotalCount: 100, HasMore: true,
	}, 5, "named", testNonce)
	detail, err := r.ResolveLocatorDetail(Locator{Redis: "c1", DB: "5", Key: "k", Type: "list", Position: "1", Nonce: testNonce})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Value != "b" || detail.Type != "list" || detail.DB != 5 || detail.TotalCount != 100 || !detail.HasMore {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if detail.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be set")
	}
}

func TestViewErrorPages(t *testing.T) {
	srv, ts := newTestServer(t)

	// 无任何快照：连接不可用页
	url := ts.URL + "/view/" + encodeLocator(t, Locator{
		Redis: "c1", DB: "3", Key: "missing", Type: "string", Position: "0", Token: srv.Token(), Nonce: testNonce,
	})
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, want := range []string{"暂无可提取数据", "LiteRedis", "class=\"card\""} {
		if !strings.Contains(html, want) {
			t.Errorf("error page should contain %q", want)
		}
	}

	// 槽位有数据但 nonce 不匹配：链接过期页
	publishTestSnapshot(t, srv)
	resp3, err := http.Get(ts.URL + "/view/" + encodeLocator(t, Locator{
		Redis: "c1", DB: "3", Key: "mykey", Type: "hash", Position: "field1", Token: srv.Token(), Nonce: "stale",
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	body3, _ := io.ReadAll(resp3.Body)
	if !strings.Contains(string(body3), "提取链接已过期") {
		t.Errorf("should render 'expired' page, got: %s", body3)
	}

	// token 错误：凭证无效页
	badURL := ts.URL + "/view/" + encodeLocator(t, Locator{
		Redis: "c1", DB: "3", Key: "missing", Type: "string", Position: "0", Token: "bad",
	})
	resp2, err := http.Get(badURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), "访问凭证无效") {
		t.Errorf("token error page should contain friendly title, got: %s", body2)
	}
}
