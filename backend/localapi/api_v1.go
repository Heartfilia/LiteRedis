package localapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultKeyCount = 100
	maxKeyCount     = 500
)

// ConnectionInfo 供 API 展示的实例信息
type ConnectionInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	DB        int    `json:"db"`
	IsCluster bool   `json:"is_cluster"`
	ReadOnly  bool   `json:"read_only"`
	Connected bool   `json:"connected"`
}

// QueryService 由宿主应用实现，/api/v1/* 的所有 Redis 读取都走它。
// 只提供读能力：这里刻意不暴露任何写方法。
type QueryService interface {
	Connections() []ConnectionInfo
	CurrentDB(connID string) (int, bool)
	Overview(connID string) (any, error)
	Scan(connID, pattern string, count int64, cursor uint64) (any, error)
	Value(connID, key, keyType, zsetSort, streamStart string, cursor uint64, offset int, count int64) (any, error)
	Command(connID, command string) (any, error)
}

// CommandRejectedError 表示命令不在只读白名单内
type CommandRejectedError struct {
	Message string
}

func (e CommandRejectedError) Error() string { return e.Message }

type v1Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type v1Envelope struct {
	OK        bool     `json:"ok"`
	Data      any      `json:"data,omitempty"`
	Error     *v1Error `json:"error,omitempty"`
	ElapsedMS int64    `json:"elapsed_ms"`
}

func writeV1(w http.ResponseWriter, start time.Time, data any) {
	writeJSON(w, http.StatusOK, v1Envelope{
		OK:        true,
		Data:      data,
		ElapsedMS: time.Since(start).Milliseconds(),
	})
}

func writeV1Error(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, v1Envelope{
		OK:    false,
		Error: &v1Error{Code: code, Message: message},
	})
}

// handleV1 处理 /api/v1/*：只接受 POST + JSON body
func (s *Server) handleV1(w http.ResponseWriter, req *http.Request, path string) {
	if !s.options().RedisEnabled {
		writeV1Error(w, http.StatusForbidden, "disabled", "读取 Redis 的本地 API 未开启：设置 → 本地数据提取 API → 允许通过 API 读取 Redis")
		return
	}
	if req.Method != http.MethodPost {
		writeV1Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "请使用 POST，并把参数放在 JSON body 里")
		return
	}
	if !s.authorized(req) {
		writeV1Error(w, http.StatusUnauthorized, "unauthorized", "token 缺失或不正确")
		return
	}
	svc := s.queryService()
	if svc == nil {
		writeV1Error(w, http.StatusServiceUnavailable, "not_ready", "服务尚未初始化")
		return
	}
	// 并发闸门：调试接口被脚本刷时不要把应用拖垮
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		writeV1Error(w, http.StatusTooManyRequests, "busy", "并发请求过多，请稍后重试")
		return
	}

	start := time.Now()
	name := strings.TrimPrefix(path, "/api/v1/")
	switch name {
	case "connections":
		s.v1Connections(w, svc, start)
	case "overview":
		s.v1Overview(w, req, svc, start)
	case "keys":
		s.v1Keys(w, req, svc, start)
	case "key":
		s.v1Key(w, req, svc, start)
	case "command":
		s.v1Command(w, req, svc, start)
	default:
		writeV1Error(w, http.StatusNotFound, "not_found", "未知接口: "+name)
	}
}

// decodeV1Body 限制体积并解析 JSON；空 body 视为空对象
func decodeV1Body(w http.ResponseWriter, req *http.Request, dst any) bool {
	req.Body = http.MaxBytesReader(w, req.Body, maxBodyBytes)
	if err := json.NewDecoder(req.Body).Decode(dst); err != nil && err != io.EOF {
		writeV1Error(w, http.StatusBadRequest, "invalid_body", "请求体不是合法 JSON："+err.Error())
		return false
	}
	return true
}

func (s *Server) v1Connections(w http.ResponseWriter, svc QueryService, start time.Time) {
	writeV1(w, start, map[string]any{"connections": svc.Connections()})
}

type v1OverviewReq struct {
	Conn string `json:"conn"`
	DB   *int   `json:"db"`
}

func (s *Server) v1Overview(w http.ResponseWriter, req *http.Request, svc QueryService, start time.Time) {
	var body v1OverviewReq
	if !decodeV1Body(w, req, &body) {
		return
	}
	db, ok := resolveConn(w, svc, body.Conn, body.DB)
	if !ok {
		return
	}
	overview, err := svc.Overview(body.Conn)
	if err != nil {
		writeV1Error(w, http.StatusBadGateway, "redis_error", err.Error())
		return
	}
	writeV1(w, start, map[string]any{"conn": body.Conn, "db": db, "overview": overview})
}

type v1KeysReq struct {
	Conn    string `json:"conn"`
	DB      *int   `json:"db"`
	Pattern string `json:"pattern"`
	Count   int64  `json:"count"`
	Cursor  uint64 `json:"cursor"`
}

func (s *Server) v1Keys(w http.ResponseWriter, req *http.Request, svc QueryService, start time.Time) {
	var body v1KeysReq
	if !decodeV1Body(w, req, &body) {
		return
	}
	db, ok := resolveConn(w, svc, body.Conn, body.DB)
	if !ok {
		return
	}
	pattern := strings.TrimSpace(body.Pattern)
	if pattern == "" {
		pattern = "*"
	}
	count := body.Count
	if count <= 0 {
		count = defaultKeyCount
	}
	if count > maxKeyCount {
		count = maxKeyCount
	}
	result, err := svc.Scan(body.Conn, pattern, count, body.Cursor)
	if err != nil {
		writeV1Error(w, http.StatusBadGateway, "redis_error", err.Error())
		return
	}
	writeV1(w, start, map[string]any{"conn": body.Conn, "db": db, "pattern": pattern, "result": result})
}

type v1KeyReq struct {
	Conn        string `json:"conn"`
	DB          *int   `json:"db"`
	Key         string `json:"key"`
	Type        string `json:"type"`
	Cursor      uint64 `json:"cursor"`
	Offset      int    `json:"offset"`
	Count       int64  `json:"count"`
	ZSetSort    string `json:"zset_sort"`
	StreamStart string `json:"stream_start"`
}

func (s *Server) v1Key(w http.ResponseWriter, req *http.Request, svc QueryService, start time.Time) {
	var body v1KeyReq
	if !decodeV1Body(w, req, &body) {
		return
	}
	if strings.TrimSpace(body.Key) == "" {
		writeV1Error(w, http.StatusBadRequest, "missing_key", "缺少 key")
		return
	}
	db, ok := resolveConn(w, svc, body.Conn, body.DB)
	if !ok {
		return
	}
	count := body.Count
	if count <= 0 {
		count = defaultKeyCount
	}
	if count > maxKeyCount {
		count = maxKeyCount
	}
	offset := body.Offset
	if offset < 0 {
		offset = 0
	}
	value, err := svc.Value(body.Conn, body.Key, body.Type, body.ZSetSort, body.StreamStart, body.Cursor, offset, count)
	if err != nil {
		writeV1Error(w, http.StatusBadGateway, "redis_error", err.Error())
		return
	}
	writeV1(w, start, map[string]any{"conn": body.Conn, "db": db, "key": body.Key, "count": count, "value": value})
}

type v1CommandReq struct {
	Conn    string `json:"conn"`
	DB      *int   `json:"db"`
	Command string `json:"command"`
}

func (s *Server) v1Command(w http.ResponseWriter, req *http.Request, svc QueryService, start time.Time) {
	var body v1CommandReq
	if !decodeV1Body(w, req, &body) {
		return
	}
	command := strings.TrimSpace(body.Command)
	if command == "" {
		writeV1Error(w, http.StatusBadRequest, "missing_command", "缺少 command")
		return
	}
	db, ok := resolveConn(w, svc, body.Conn, body.DB)
	if !ok {
		return
	}
	result, err := svc.Command(body.Conn, command)
	if err != nil {
		var rejected CommandRejectedError
		if errors.As(err, &rejected) {
			writeV1Error(w, http.StatusForbidden, "readonly_only", rejected.Message)
			return
		}
		writeV1Error(w, http.StatusBadGateway, "redis_error", err.Error())
		return
	}
	writeV1(w, start, map[string]any{"conn": body.Conn, "db": db, "command": command, "result": result})
}

// resolveConn 校验连接存在且已连接，并核对 body 里的 db 与当前 DB 是否一致
func resolveConn(w http.ResponseWriter, svc QueryService, conn string, want *int) (int, bool) {
	if strings.TrimSpace(conn) == "" {
		writeV1Error(w, http.StatusBadRequest, "missing_conn", "缺少 conn：先用 /api/v1/connections 查已连接的实例 ID")
		return 0, false
	}
	db, ok := svc.CurrentDB(conn)
	if !ok {
		writeV1Error(w, http.StatusNotFound, "conn_not_found", "连接不存在或尚未连接: "+conn)
		return 0, false
	}
	if want != nil && *want != db {
		writeV1Error(w, http.StatusConflict, "db_mismatch",
			fmt.Sprintf("该连接当前在 DB %d，请先在界面切到 DB %d（或省略 db 参数）", db, *want))
		return 0, false
	}
	return db, true
}
