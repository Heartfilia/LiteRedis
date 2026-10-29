package localapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeService struct {
	conns      []ConnectionInfo
	currentDB  int
	overview   any
	scanResult any
	value      any
	commandRes any
	commandErr error
	lastCount  int64
	lastOffset int
	lastKey    string
	lastType   string
}

func (f *fakeService) Connections() []ConnectionInfo { return f.conns }

func (f *fakeService) CurrentDB(connID string) (int, bool) {
	for _, c := range f.conns {
		if c.ID == connID && c.Connected {
			return f.currentDB, true
		}
	}
	return 0, false
}

func (f *fakeService) Overview(connID string) (any, error) { return f.overview, nil }

func (f *fakeService) Scan(connID, pattern string, count int64, cursor uint64) (any, error) {
	f.lastCount = count
	return f.scanResult, nil
}

func (f *fakeService) Value(connID, key, keyType, zsetSort, streamStart string, cursor uint64, offset int, count int64) (any, error) {
	f.lastCount = count
	f.lastOffset = offset
	f.lastKey = key
	f.lastType = keyType
	return f.value, nil
}

func (f *fakeService) Command(connID, command string) (any, error) {
	if f.commandErr != nil {
		return nil, f.commandErr
	}
	return f.commandRes, nil
}

func newV1Server(t *testing.T, opts Options, svc QueryService) (*Server, *httptest.Server) {
	t.Helper()
	srv, ts := newTestServer(t)
	srv.Configure(svc, opts)
	return srv, ts
}

func postV1(t *testing.T, url, token string, body any, headers map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp, decoded
}

func TestV1DisabledByDefault(t *testing.T) {
	_, ts := newV1Server(t, Options{}, &fakeService{})
	resp, body := postV1(t, ts.URL+"/api/v1/connections", "any", map[string]any{}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "disabled" {
		t.Fatalf("expected disabled error, got %v", body)
	}
}

func TestV1RequiresTokenWhenAuthEnabled(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Connected: true}}}
	_, ts := newV1Server(t, Options{RedisEnabled: true}, svc)

	resp, _ := postV1(t, ts.URL+"/api/v1/connections", "", map[string]any{}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	resp, body := postV1(t, ts.URL+"/api/v1/connections", "bad-token", map[string]any{}, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", resp.StatusCode)
	}
	_ = body
}

func TestV1AuthDisabledAllowsRequest(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Name: "dev", Connected: true}}}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	resp, body := postV1(t, ts.URL+"/api/v1/connections", "", map[string]any{}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d (%v)", resp.StatusCode, body)
	}
	if body["ok"] != true {
		t.Fatalf("expected ok envelope, got %v", body)
	}
	data, _ := body["data"].(map[string]any)
	conns, _ := data["connections"].([]any)
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %v", data)
	}
}

func TestV1RejectsForeignOrigin(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Connected: true}}}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	resp, _ := postV1(t, ts.URL+"/api/v1/connections", "", map[string]any{}, map[string]string{"Origin": "https://evil.example"})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-origin, got %d", resp.StatusCode)
	}

	resp, _ = postV1(t, ts.URL+"/api/v1/connections", "", map[string]any{}, map[string]string{"Origin": ts.URL})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected same-origin request to pass, got %d", resp.StatusCode)
	}
}

func TestV1RejectsNonLoopbackHost(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Connected: true}}}
	srv, _ := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/connections", strings.NewReader("{}"))
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	srv.handle(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign host, got %d", rec.Code)
	}
}

func TestV1RejectsGet(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Connected: true}}}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)
	resp, err := http.Get(ts.URL + "/api/v1/connections")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestV1KeyValidationAndPaging(t *testing.T) {
	svc := &fakeService{
		conns: []ConnectionInfo{{ID: "c1", Connected: true}},
		value: map[string]any{"key": "k", "type": "hash", "has_more": false},
	}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	resp, body := postV1(t, ts.URL+"/api/v1/key", "", map[string]any{"conn": "c1"}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing key, got %d (%v)", resp.StatusCode, body)
	}

	resp, body = postV1(t, ts.URL+"/api/v1/key", "", map[string]any{"conn": "c1", "key": "k", "count": 5000, "offset": -3}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d (%v)", resp.StatusCode, body)
	}
	if svc.lastCount != maxKeyCount {
		t.Fatalf("count should clamp to %d, got %d", maxKeyCount, svc.lastCount)
	}
	if svc.lastOffset != 0 {
		t.Fatalf("negative offset should clamp to 0, got %d", svc.lastOffset)
	}
}

func TestV1KeysDefaults(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Connected: true}}, scanResult: map[string]any{"keys": []any{}}}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	resp, body := postV1(t, ts.URL+"/api/v1/keys", "", map[string]any{"conn": "c1"}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d (%v)", resp.StatusCode, body)
	}
	if svc.lastCount != defaultKeyCount {
		t.Fatalf("default count should be %d, got %d", defaultKeyCount, svc.lastCount)
	}
	data, _ := body["data"].(map[string]any)
	if data["pattern"] != "*" {
		t.Fatalf("pattern should default to *, got %v", data["pattern"])
	}
}

func TestV1DBMismatchRejected(t *testing.T) {
	svc := &fakeService{conns: []ConnectionInfo{{ID: "c1", Connected: true}}, currentDB: 3}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	resp, body := postV1(t, ts.URL+"/api/v1/overview", "", map[string]any{"conn": "c1", "db": 0}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d (%v)", resp.StatusCode, body)
	}

	resp, _ = postV1(t, ts.URL+"/api/v1/overview", "", map[string]any{"conn": "c1", "db": 3}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 when db matches, got %d", resp.StatusCode)
	}
}

func TestV1CommandRejectsWriteCommands(t *testing.T) {
	svc := &fakeService{}
	svc.conns = []ConnectionInfo{{ID: "c1", Connected: true}}
	svc.commandErr = CommandRejectedError{Message: "只允许只读命令，已拒绝: SET a b"}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)

	resp, body := postV1(t, ts.URL+"/api/v1/command", "", map[string]any{"conn": "c1", "command": "SET a b"}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for write command, got %d (%v)", resp.StatusCode, body)
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "readonly_only" {
		t.Fatalf("expected readonly_only code, got %v", body)
	}
}

func TestV1UnknownConn(t *testing.T) {
	svc := &fakeService{}
	_, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, svc)
	resp, body := postV1(t, ts.URL+"/api/v1/overview", "", map[string]any{"conn": "nope"}, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%v)", resp.StatusCode, body)
	}
}

func TestDocsPageServesTokenAndState(t *testing.T) {
	srv, ts := newV1Server(t, Options{RedisEnabled: true, AuthDisabled: true}, &fakeService{})
	resp, err := http.Get(ts.URL + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("unexpected content type: %s", ct)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("docs response must not set CORS headers")
	}
	_ = srv
}

func TestOpenAPISpecIsValidJSON(t *testing.T) {
	_, ts := newV1Server(t, Options{}, &fakeService{})
	resp, err := http.Get(ts.URL + "/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var spec map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	if spec["openapi"] == nil {
		t.Fatalf("missing openapi version: %v", spec)
	}
}
