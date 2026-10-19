package localapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"LiteRedis/backend/config"
)

const (
	// maxSnapshotBytes 单个快照序列化后的上限
	maxSnapshotBytes = 10 << 20 // 10MB
)

// entry 单个 key 的已加载快照（全局唯一槽位）
type entry struct {
	nonce     string // 本次提取的一次性指纹，/view 页面凭它确认"刚才那一次点击"
	connID    string
	connName  string
	kv        config.KeyValue
	db        int
	updatedAt time.Time
	sizeBytes int
}

// KeyRegistry 保存"最近一次浏览器解析"的 key 快照。
//
// 设计：全局只有一个槽位——前端把当前展示的数据推入槽位（编辑器加载时持续覆盖 +
// 点击「浏览器解析」时立即写入），网页只读槽位。切换 key/db/连接后旧链接自然失效，
// 内存上限为单份快照。快照清除由 app.go 在 key 被删 / 连接断开等 choke point
// 调用（仅当槽位匹配时清除）。
type KeyRegistry struct {
	mu   sync.Mutex
	slot *entry
}

func NewKeyRegistry() *KeyRegistry {
	return &KeyRegistry{}
}

// Publish 覆盖式写入槽位。nonce 为本次提取的一次性指纹（前端点击时生成），
// /view 页面通过比对 nonce 确认读取的就是刚才那一次点击的数据。
func (r *KeyRegistry) Publish(connID, key string, kv config.KeyValue, db int, connName, nonce string) error {
	if connID == "" {
		return fmt.Errorf("connID is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}
	// 快照以服务端视角为准，防止伪造 key/type 指向
	kv.Key = key

	data, err := json.Marshal(kv)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if len(data) > maxSnapshotBytes {
		return fmt.Errorf("snapshot too large: %d bytes (max %d)", len(data), maxSnapshotBytes)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.slot = &entry{
		nonce:     nonce,
		connID:    connID,
		connName:  connName,
		kv:        kv,
		db:        db,
		updatedAt: time.Now(),
		sizeBytes: len(data),
	}
	return nil
}

func (r *KeyRegistry) matchesLocked(connID, key string) bool {
	return r.slot != nil && r.slot.connID == connID && r.slot.kv.Key == key
}

// Unpublish 若槽位保存的是指定 key 的快照则清除
func (r *KeyRegistry) Unpublish(connID, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.matchesLocked(connID, key) {
		r.slot = nil
	}
}

// UnpublishConn 若槽位属于指定连接则清除
func (r *KeyRegistry) UnpublishConn(connID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.slot != nil && r.slot.connID == connID {
		r.slot = nil
	}
}

// Get 读取槽位（仅当 connID+key 均匹配）
func (r *KeyRegistry) Get(connID, key string) (config.KeyValue, int, time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.matchesLocked(connID, key) {
		return config.KeyValue{}, 0, time.Time{}, false
	}
	return r.slot.kv, r.slot.db, r.slot.updatedAt, true
}

// PublishedKeyInfo 快照元信息（列表用）
type PublishedKeyInfo struct {
	Key       string `json:"key"`
	Type      string `json:"type"`
	DB        int    `json:"db"`
	UpdatedAt string `json:"updated_at"`
}

// List 槽位属于该连接时返回单条元信息，否则为空
func (r *KeyRegistry) List(connID string) []PublishedKeyInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.slot == nil || r.slot.connID != connID {
		return []PublishedKeyInfo{}
	}
	return []PublishedKeyInfo{{
		Key:       r.slot.kv.Key,
		Type:      r.slot.kv.Type,
		DB:        r.slot.db,
		UpdatedAt: r.slot.updatedAt.Format(time.RFC3339),
	}}
}

// Locator 浏览器提取定位 JSON（/info、/view 端点）。字段均为字符串，
// db/position 由内部做数字兼容转换。Nonce 为本次提取的一次性指纹。
type Locator struct {
	Redis    string `json:"redis"`
	DB       string `json:"db"`
	Key      string `json:"key"`
	Type     string `json:"type"`
	Position string `json:"position"`
	Token    string `json:"token"`
	Nonce    string `json:"nonce"`
}

// ResolveLocatorDetail 解析定位并返回值 + 元信息。
// 槽位必须与定位中的连接/db/key 一致——全局槽位只保存"最近一次提取"的数据。
func (r *KeyRegistry) ResolveLocatorDetail(loc Locator) (*LocatorDetail, error) {
	if loc.Redis == "" {
		return nil, errLocator("redis is required")
	}
	if loc.Key == "" {
		return nil, errLocator("key is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.slot == nil {
		return nil, errLocator("no snapshot available: click browser-extract on a key in LiteRedis first")
	}

	// nonce 比对：槽位必须就是"刚才那一次点击"写入的数据。
	// 与连接/db/key 无关——点谁载入谁，切 redis/切 db 不影响。
	if loc.Nonce == "" || r.slot.nonce == "" || loc.Nonce != r.slot.nonce {
		return nil, errLocator("extraction link expired: the slot has been replaced by a newer extraction")
	}

	value, err := extractPlainText(r.slot.kv, loc.Position)
	if err != nil {
		return nil, err
	}
	return &LocatorDetail{
		Value:      value,
		Key:        r.slot.kv.Key,
		Type:       r.slot.kv.Type,
		DB:         r.slot.db,
		Position:   loc.Position,
		UpdatedAt:  r.slot.updatedAt,
		TotalCount: r.slot.kv.TotalCount,
		HasMore:    r.slot.kv.HasMore,
	}, nil
}

// ResolveLocator 解析定位并返回纯文本值（/info 端点用）
func (r *KeyRegistry) ResolveLocator(loc Locator) (string, error) {
	detail, err := r.ResolveLocatorDetail(loc)
	if err != nil {
		return "", err
	}
	return detail.Value, nil
}

// parseLocatorInt 定位字段数字解析（容忍空白）
func parseLocatorInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// ResolveEntry 从快照中按 selector 提取单条结构化数据（/api 端点用，保留原始响应形状）
func ResolveEntry(kv config.KeyValue, selector string) (any, error) {
	if selector == "" {
		return nil, fmt.Errorf("selector is required for %s keys", kv.Type)
	}
	switch kv.Type {
	case "string":
		return nil, fmt.Errorf("string keys do not support selector; fetch the full snapshot instead")
	case "hash":
		val, ok := kv.HashVal[selector]
		if !ok {
			return nil, fmt.Errorf("field %q not found in loaded snapshot", selector)
		}
		return map[string]string{"field": selector, "value": val}, nil
	case "list":
		idx, err := parseLocatorInt(selector)
		if err != nil {
			return nil, fmt.Errorf("list selector must be a numeric index, got %q", selector)
		}
		if idx < 0 || idx >= len(kv.ListVal) {
			return nil, fmt.Errorf("list index %d out of range (loaded 0..%d)", idx, len(kv.ListVal)-1)
		}
		return map[string]any{"index": idx, "value": kv.ListVal[idx]}, nil
	case "set":
		for _, member := range kv.SetVal {
			if member == selector {
				return map[string]string{"member": member}, nil
			}
		}
		return nil, fmt.Errorf("member %q not found in loaded snapshot", selector)
	case "zset":
		for _, m := range kv.ZSetVal {
			if m.Member == selector {
				return map[string]any{"member": m.Member, "score": m.Score}, nil
			}
		}
		return nil, fmt.Errorf("member %q not found in loaded snapshot", selector)
	case "stream":
		for _, e := range kv.StreamVal {
			if e.ID == selector {
				return map[string]any{"id": e.ID, "fields": e.Fields}, nil
			}
		}
		return nil, fmt.Errorf("stream entry %q not found in loaded snapshot", selector)
	default:
		return nil, fmt.Errorf("unsupported key type %q", kv.Type)
	}
}

// extractPlainText 按类型返回无包装的纯文本值
func extractPlainText(kv config.KeyValue, position string) (string, error) {
	switch kv.Type {
	case "string":
		return kv.StringVal, nil
	case "hash":
		if position == "" {
			return "", fmt.Errorf("position (field name) is required for hash keys")
		}
		val, ok := kv.HashVal[position]
		if !ok {
			return "", fmt.Errorf("field %q not found in loaded snapshot", position)
		}
		return val, nil
	case "list":
		idx, err := parseLocatorInt(position)
		if err != nil {
			return "", fmt.Errorf("list position must be a numeric index, got %q", position)
		}
		if idx < 0 || idx >= len(kv.ListVal) {
			return "", fmt.Errorf("list index %d out of range (loaded 0..%d)", idx, len(kv.ListVal)-1)
		}
		return kv.ListVal[idx], nil
	case "set":
		idx, err := parseLocatorInt(position)
		if err != nil {
			return "", fmt.Errorf("set position must be a numeric index, got %q", position)
		}
		if idx < 0 || idx >= len(kv.SetVal) {
			return "", fmt.Errorf("set index %d out of range (loaded 0..%d)", idx, len(kv.SetVal)-1)
		}
		return kv.SetVal[idx], nil
	case "zset":
		idx, err := parseLocatorInt(position)
		if err != nil {
			return "", fmt.Errorf("zset position must be a numeric index, got %q", position)
		}
		if idx < 0 || idx >= len(kv.ZSetVal) {
			return "", fmt.Errorf("zset index %d out of range (loaded 0..%d)", idx, len(kv.ZSetVal)-1)
		}
		return kv.ZSetVal[idx].Member, nil
	case "stream":
		// 兼容两种定位：entry ID 或已加载下标
		for _, e := range kv.StreamVal {
			if e.ID == position {
				return streamFieldsJSON(e)
			}
		}
		if idx, err := parseLocatorInt(position); err == nil {
			if idx >= 0 && idx < len(kv.StreamVal) {
				return streamFieldsJSON(kv.StreamVal[idx])
			}
			return "", fmt.Errorf("stream index %d out of range (loaded %d)", idx, len(kv.StreamVal))
		}
		return "", fmt.Errorf("stream entry %q not found in loaded snapshot", position)
	default:
		return "", fmt.Errorf("unsupported key type %q", kv.Type)
	}
}

func streamFieldsJSON(e config.StreamEntry) (string, error) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	data, err := json.Marshal(e.Fields)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
