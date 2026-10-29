package localapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
)

const (
	requestTimeout = 30 * time.Second
	// maxBodyBytes 单次请求体上限，避免把超大 JSON 一次塞进来
	maxBodyBytes = 64 << 10
	// maxConcurrentRequests 同时在跑的请求数上限
	maxConcurrentRequests = 8
)

// Options 本地 API 的运行开关
type Options struct {
	// RedisEnabled 允许 /api/v1/* 直接读取 Redis（默认关闭）
	RedisEnabled bool
	// AuthDisabled 关闭 token 校验（仅本机自用）
	AuthDisabled bool
}

// Server 本地 API。只监听 127.0.0.1：
//   - /api/{connID}/{key}、/info/*、/view/* 读 UI 已发布快照（原有能力）
//   - /api/v1/* 直接读 Redis（需要显式开启，见 Options.RedisEnabled）
//   - /docs、/openapi.json 提供本地调试页面
//
// 不返回任何 CORS 头：只有同源（/docs 页面）或非浏览器客户端能读响应。
type Server struct {
	registry *KeyRegistry
	token    string
	service  QueryService
	opts     Options
	slots    chan struct{}

	mu       sync.Mutex
	http     *http.Server
	listener net.Listener
	port     int
	lastErr  string // 最近一次 Start 失败原因，供设置面板展示
	tokenMu  sync.RWMutex
	optMu    sync.RWMutex
}

// Info 服务的运行时信息（设置面板展示用）
type Info struct {
	Running bool   `json:"running"`
	Port    int    `json:"port"`
	Token   string `json:"token"`
	Error   string `json:"error,omitempty"`
}

func NewServer(registry *KeyRegistry) *Server {
	return &Server{
		registry: registry,
		token:    loadPersistedToken(),
		slots:    make(chan struct{}, maxConcurrentRequests),
	}
}

// Configure 更新数据来源与运行开关（设置变更时调用）
func (s *Server) Configure(service QueryService, opts Options) {
	s.optMu.Lock()
	s.service = service
	s.opts = opts
	s.optMu.Unlock()
}

func (s *Server) options() Options {
	s.optMu.RLock()
	defer s.optMu.RUnlock()
	return s.opts
}

func (s *Server) queryService() QueryService {
	s.optMu.RLock()
	defer s.optMu.RUnlock()
	return s.service
}

// Token 返回当前 token
func (s *Server) Token() string {
	s.tokenMu.RLock()
	defer s.tokenMu.RUnlock()
	return s.token
}

// RegenerateToken 换发 token（旧 token 立即失效）并持久化，重启后保持不变
func (s *Server) RegenerateToken() error {
	token, err := generateToken()
	if err != nil {
		return err
	}
	s.tokenMu.Lock()
	s.token = token
	s.tokenMu.Unlock()
	persistToken(token)
	return nil
}

// Start 启动服务。已在运行时先停止旧实例。
func (s *Server) Start(port int) error {
	s.Stop()

	if s.Token() == "" {
		if err := s.RegenerateToken(); err != nil {
			return fmt.Errorf("generate token: %w", err)
		}
	}

	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Lock()
		s.lastErr = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	// port=0 时由系统分配，回读实际端口
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		port = tcpAddr.Port
	}

	s.mu.Lock()
	s.port = port
	s.listener = ln
	httpSrv := &http.Server{
		Handler:           http.TimeoutHandler(http.HandlerFunc(s.handle), requestTimeout, `{"error":"request timeout"}`),
		ReadHeaderTimeout: requestTimeout,
	}
	s.http = httpSrv
	s.lastErr = ""
	s.mu.Unlock()

	go func() {
		_ = httpSrv.Serve(ln)
	}()
	return nil
}

// Stop 停止服务（幂等）
func (s *Server) Stop() {
	s.mu.Lock()
	httpSrv := s.http
	listener := s.listener
	s.http = nil
	s.listener = nil
	s.lastErr = ""
	s.mu.Unlock()

	if httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}
	if listener != nil {
		_ = listener.Close()
	}
}

// IsRunning 服务是否在监听
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener != nil
}

// Port 当前监听端口
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Info 汇总运行状态
func (s *Server) Info() Info {
	s.mu.Lock()
	running := s.listener != nil
	port := s.port
	lastErr := s.lastErr
	s.mu.Unlock()
	return Info{
		Running: running,
		Port:    port,
		Token:   s.Token(),
		Error:   lastErr,
	}
}

// Apply 按新设置重启服务（enabled/port 变化时调用）
func (s *Server) Apply(settings config.AppSettings) error {
	s.Stop()
	if !settings.LocalAPIEnabled {
		return nil
	}
	return s.Start(config.EffectiveLocalAPIPort(settings.LocalAPIPort))
}

var (
	defaultTokenFilePath string
	tokenFilePath        string
)

func init() {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	_ = os.MkdirAll(filepath.Join(dir, "LiteRedis"), 0700)
	defaultTokenFilePath = filepath.Join(dir, "LiteRedis", "api_token")
	tokenFilePath = defaultTokenFilePath
}

// setTokenFilePathForTest 供测试注入临时路径；传空恢复默认
func setTokenFilePathForTest(path string) {
	if path == "" {
		path = defaultTokenFilePath
	}
	tokenFilePath = path
}

func loadPersistedToken() string {
	data, err := os.ReadFile(tokenFilePath)
	if err != nil {
		return ""
	}
	token := strings.TrimSpace(string(data))
	// 基本合法性：64 位 hex
	if len(token) != 32 {
		return ""
	}
	for _, c := range token {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return ""
		}
	}
	return token
}

func persistToken(token string) {
	_ = config.AtomicWriteFile(tokenFilePath, []byte(token+"\n"), 0600)
}

func generateToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Server) handle(w http.ResponseWriter, req *http.Request) {
	// 只服务本机回环地址：拒绝 Host 不是 127.0.0.1/localhost/[::1] 的请求，
	// 顺带防住 DNS rebinding（把恶意域名解析到本机后绕过同源限制）
	if !isLoopbackHost(req.Host) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "host not allowed"})
		return
	}
	// 浏览器跨源请求一律拒绝；/docs 页面自身同源，可以正常调用
	if origin := req.Header.Get("Origin"); origin != "" && !isSameOrigin(origin, req.Host) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin request blocked"})
		return
	}

	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := req.URL.Path // 已解码；key 含 %2F 等编码字符时由客户端编码为 %252F 的场景极罕见，直接使用解码值
	switch {
	case path == "/health":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	case path == "/docs" || path == "/docs/":
		s.handleDocs(w)
		return
	case path == "/openapi.json":
		s.handleOpenAPI(w)
		return
	case strings.HasPrefix(path, "/api/v1/"):
		s.handleV1(w, req, path)
		return
	}

	if req.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	switch {
	case strings.HasPrefix(path, "/info/"):
		s.handleInfo(w, path[len("/info/"):])
		return
	case strings.HasPrefix(path, "/view/"):
		s.handleView(w, path[len("/view/"):])
		return
	case path == "/api" || path == "/api/":
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing connID"})
		return
	}

	if !s.authorized(req) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if !strings.HasPrefix(path, "/api/") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	// /api/{connID}/{key...}：connID 不含 /，key 取剩余全部路径（可含 /）
	rest := path[len("/api/"):]
	connID := rest
	key := ""
	if idx := strings.Index(rest, "/"); idx >= 0 {
		connID = rest[:idx]
		key = rest[idx+1:]
	}
	if connID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "missing connID"})
		return
	}
	if key == "" {
		// GET /api/{connID}：列出已发布的 key
		writeJSON(w, http.StatusOK, map[string]any{"conn_id": connID, "keys": s.registry.List(connID)})
		return
	}

	// r.URL.Path 已是解码后的路径，无需再 unescape（二次解码会破坏含 % 的 key）
	decodedKey := key

	kv, db, updatedAt, ok := s.registry.Get(connID, decodedKey)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not published (not loaded in UI or connection lost)"})
		return
	}

	if selector := req.URL.Query().Get("selector"); selector != "" {
		entry, resolveErr := ResolveEntry(kv, selector)
		if resolveErr != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": resolveErr.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"conn_id":    connID,
			"key":        decodedKey,
			"db":         db,
			"type":       kv.Type,
			"selector":   selector,
			"updated_at": updatedAt.Format(time.RFC3339),
			"entry":      entry,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conn_id":     connID,
		"key":         decodedKey,
		"db":          db,
		"type":        kv.Type,
		"ttl":         kv.TTL,
		"updated_at":  updatedAt.Format(time.RFC3339),
		"total_count": kv.TotalCount,
		"has_more":    kv.HasMore,
		"data":        typeSpecificData(kv),
	})
}

// handleInfo 处理 /info/{base64}：解码定位 JSON（token 内嵌校验），
// 从注册表解析出纯文本值直接返回，不做额外包装。
// 该端点由第三方站点（json.litetools.top）服务端请求，无法携带 HTTP 鉴权头，
// 故 token 走 payload 内嵌校验。
// writeText 输出纯文本响应
func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func (s *Server) handleInfo(w http.ResponseWriter, encoded string) {
	writeText := func(status int, body string) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}

	if encoded == "" {
		writeText(http.StatusBadRequest, "missing locator payload")
		return
	}
	data, err := decodeLocatorBase64(encoded)
	if err != nil {
		writeText(http.StatusBadRequest, "invalid locator encoding")
		return
	}
	var loc Locator
	if err := json.Unmarshal(data, &loc); err != nil {
		writeText(http.StatusBadRequest, "invalid locator payload")
		return
	}

	token := s.Token()
	if token == "" || subtle.ConstantTimeCompare([]byte(loc.Token), []byte(token)) != 1 {
		writeText(http.StatusUnauthorized, "unauthorized")
		return
	}

	value, err := s.registry.ResolveLocator(loc)
	if err != nil {
		writeText(http.StatusNotFound, err.Error())
		return
	}
	writeText(http.StatusOK, value)
}

// decodeLocatorBase64 兼容 URL-safe 与标准 Base64、容忍缺失 padding。
// 同时容忍误粘贴的外层参数后缀（如 "&app=literedis"）：& 不是合法 base64 字符，安全截断。
func decodeLocatorBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '&'); idx >= 0 {
		s = s[:idx]
	}
	if s == "" {
		return nil, fmt.Errorf("empty payload")
	}
	normalized := strings.NewReplacer("+", "-", "/", "_").Replace(s)
	// 补齐 padding
	if m := len(normalized) % 4; m != 0 {
		normalized += strings.Repeat("=", 4-m)
	}
	return base64.URLEncoding.DecodeString(normalized)
}

// authorized 校验访问权限：关闭 token 校验时直接放行（仅限本机自用）
func (s *Server) authorized(req *http.Request) bool {
	if s.options().AuthDisabled {
		return true
	}
	return s.checkAuth(req)
}

// isLoopbackHost 判断 Host 是否为本机回环地址（可带端口）
func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// isSameOrigin 判断 Origin 是否就是本次请求自己的地址（/docs 页面调用自己）
func isSameOrigin(origin, hostport string) bool {
	origin = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(origin)), "/")
	if origin == "" {
		return false
	}
	return origin == "http://"+strings.ToLower(hostport)
}

// checkAuth 支持 Authorization: Bearer <token> 或 ?token=
func (s *Server) checkAuth(req *http.Request) bool {
	token := s.Token()
	if token == "" {
		return false
	}
	provided := ""
	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		provided = strings.TrimPrefix(auth, "Bearer ")
	}
	if provided == "" {
		provided = req.URL.Query().Get("token")
	}
	if provided == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}

// typeSpecificData 返回类型对应的值字段（与 config.KeyValue 字段一一对应）
func typeSpecificData(kv config.KeyValue) any {
	switch kv.Type {
	case "string":
		return map[string]any{"string_val": kv.StringVal}
	case "hash":
		if kv.HashVal == nil {
			kv.HashVal = map[string]string{}
		}
		return map[string]any{"hash_val": kv.HashVal}
	case "list":
		if kv.ListVal == nil {
			kv.ListVal = []string{}
		}
		return map[string]any{"list_val": kv.ListVal}
	case "set":
		if kv.SetVal == nil {
			kv.SetVal = []string{}
		}
		return map[string]any{"set_val": kv.SetVal}
	case "zset":
		if kv.ZSetVal == nil {
			kv.ZSetVal = []config.ZSetMember{}
		}
		return map[string]any{"zset_val": kv.ZSetVal}
	case "stream":
		if kv.StreamVal == nil {
			kv.StreamVal = []config.StreamEntry{}
		}
		return map[string]any{"stream_val": kv.StreamVal}
	default:
		return nil
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
