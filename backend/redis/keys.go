package redis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"LiteRedis/backend/config"

	"github.com/redis/go-redis/v9"
)

const (
	clusterScanSessionTTL  = 5 * time.Minute
	maxClusterScanSessions = 16
)

type clusterScanMaster struct {
	addr   string
	client *redis.Client
}

type clusterScanState struct {
	client      *redis.ClusterClient
	pattern     string
	masterAddrs []string
	masterIndex int
	cursor      uint64
	pending     []string
	seen        map[string]struct{}
	expiresAt   time.Time
}

var (
	errKeyAlreadyExists = errors.New("key already exists")
	createKeyScript     = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) ~= 0 then
  return 0
end

local key_type = ARGV[1]
if key_type == 'string' then
  redis.call('SET', KEYS[1], ARGV[2])
elseif key_type == 'hash' then
  redis.call('HSET', KEYS[1], ARGV[2], ARGV[3])
elseif key_type == 'list' then
  redis.call('RPUSH', KEYS[1], ARGV[2])
elseif key_type == 'set' then
  redis.call('SADD', KEYS[1], ARGV[2])
elseif key_type == 'zset' then
  redis.call('ZADD', KEYS[1], ARGV[4], ARGV[2])
elseif key_type == 'stream' then
  redis.call('XADD', KEYS[1], '*', ARGV[2], ARGV[3])
else
  return redis.error_reply('unsupported key type')
end

local ttl = tonumber(ARGV[5])
if ttl >= 0 then
  redis.call('EXPIRE', KEYS[1], ttl)
end
return 1
`)

	clusterScanToken atomic.Uint64
	clusterScans     = struct {
		sync.Mutex
		states map[uint64]*clusterScanState
	}{states: make(map[uint64]*clusterScanState)}
)

// ScanKeys 扫描 key（支持 cursor 分页），通过 Pipeline 批量获取 TYPE 和 TTL
func ScanKeys(ctx context.Context, client redis.UniversalClient, pattern string, count int64, cursor uint64) (config.ScanResult, error) {
	result := config.ScanResult{Keys: []config.RedisKey{}}
	if pattern == "" {
		pattern = "*"
	}
	if count <= 0 {
		count = 100
	}
	if clusterClient, ok := client.(*redis.ClusterClient); ok {
		return scanClusterKeys(ctx, clusterClient, pattern, count, cursor)
	}

	var keyNames []string
	var nextCursor uint64

	// 使用 SCAN 避免阻塞，循环直到获取到数据或 cursor 回到 0
	for {
		keys, newCursor, err := client.Scan(ctx, cursor, pattern, count).Result()
		if err != nil {
			return result, err
		}
		keyNames = append(keyNames, keys...)
		cursor = newCursor
		if cursor == 0 || int64(len(keyNames)) >= count {
			nextCursor = cursor
			break
		}
		// 如果一批返回空但 cursor 不为 0，继续 scan
		if len(keys) == 0 {
			continue
		}
	}

	if len(keyNames) == 0 {
		return result, nil
	}

	// Pipeline 批量获取 TYPE + TTL
	pipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keyNames))
	ttlCmds := make([]*redis.DurationCmd, len(keyNames))

	for i, k := range keyNames {
		typeCmds[i] = pipe.Type(ctx, k)
		ttlCmds[i] = pipe.TTL(ctx, k)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return result, err
	}

	keys := make([]config.RedisKey, len(keyNames))
	for i, name := range keyNames {
		ttlDur := ttlCmds[i].Val()
		var ttlSec int64
		if ttlDur < 0 {
			ttlSec = int64(ttlDur) // -1 或 -2
		} else {
			ttlSec = int64(ttlDur / time.Second)
		}
		keys[i] = config.RedisKey{
			Name: name,
			Type: typeCmds[i].Val(),
			TTL:  ttlSec,
		}
	}

	result.Keys = keys
	result.NextCursor = nextCursor
	result.HasMore = nextCursor != 0
	return result, nil
}

func scanClusterKeys(ctx context.Context, client *redis.ClusterClient, pattern string, count int64, cursor uint64) (config.ScanResult, error) {
	result := config.ScanResult{Keys: []config.RedisKey{}}
	masters, err := clusterMasterClients(ctx, client)
	if err != nil {
		return result, err
	}

	var retryState *clusterScanState
	retryCursor := cursor
	var state *clusterScanState
	if cursor == 0 {
		state = &clusterScanState{
			client:      client,
			pattern:     pattern,
			masterAddrs: clusterMasterAddresses(masters),
			seen:        make(map[string]struct{}),
		}
	} else {
		state = takeClusterScanState(cursor)
		if state == nil {
			return result, fmt.Errorf("cluster scan cursor expired or invalid; restart the scan")
		}
		if state.client != client || state.pattern != pattern || !sameClusterMasters(state.masterAddrs, masters) {
			return result, fmt.Errorf("cluster scan context changed; restart the scan")
		}
		retryState = cloneClusterScanState(state)
	}

	pageKeys, complete, err := scanClusterPage(ctx, state, masters, count)
	if err != nil {
		restoreClusterScanState(retryCursor, retryState)
		return result, err
	}
	keys, err := loadClusterKeyMetadata(ctx, client, pageKeys)
	if err != nil {
		restoreClusterScanState(retryCursor, retryState)
		return result, err
	}
	result.Keys = keys
	if !complete {
		result.NextCursor = storeClusterScanState(state)
		result.HasMore = true
	}
	return result, nil
}

func clusterMasterClients(ctx context.Context, client *redis.ClusterClient) ([]clusterScanMaster, error) {
	masters := make([]clusterScanMaster, 0)
	var mu sync.Mutex
	err := client.ForEachMaster(ctx, func(_ context.Context, master *redis.Client) error {
		mu.Lock()
		masters = append(masters, clusterScanMaster{addr: master.Options().Addr, client: master})
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(masters, func(i, j int) bool { return masters[i].addr < masters[j].addr })
	return masters, nil
}

func clusterMasterAddresses(masters []clusterScanMaster) []string {
	addresses := make([]string, len(masters))
	for i, master := range masters {
		addresses[i] = master.addr
	}
	return addresses
}

func sameClusterMasters(addresses []string, masters []clusterScanMaster) bool {
	if len(addresses) != len(masters) {
		return false
	}
	for i, address := range addresses {
		if address != masters[i].addr {
			return false
		}
	}
	return true
}

func scanClusterPage(ctx context.Context, state *clusterScanState, masters []clusterScanMaster, count int64) ([]string, bool, error) {
	page := make([]string, 0, count)
	for int64(len(page)) < count {
		for len(state.pending) > 0 && int64(len(page)) < count {
			key := state.pending[0]
			state.pending = state.pending[1:]
			if _, duplicate := state.seen[key]; duplicate {
				continue
			}
			state.seen[key] = struct{}{}
			page = append(page, key)
		}
		if int64(len(page)) >= count {
			break
		}
		if state.masterIndex >= len(masters) {
			state.pending = nil
			return page, true, nil
		}

		batch, nextCursor, err := masters[state.masterIndex].client.Scan(ctx, state.cursor, state.pattern, count).Result()
		if err != nil {
			return nil, false, err
		}
		state.cursor = nextCursor
		if nextCursor == 0 {
			state.masterIndex++
			state.cursor = 0
		}
		state.pending = append(state.pending, batch...)
	}

	if len(state.pending) > 0 {
		state.pending = append([]string(nil), state.pending...)
	}
	complete := state.masterIndex >= len(masters) && len(state.pending) == 0
	return page, complete, nil
}

func loadClusterKeyMetadata(ctx context.Context, client *redis.ClusterClient, keyNames []string) ([]config.RedisKey, error) {
	if len(keyNames) == 0 {
		return []config.RedisKey{}, nil
	}
	pipe := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(keyNames))
	ttlCmds := make([]*redis.DurationCmd, len(keyNames))
	for i, key := range keyNames {
		typeCmds[i] = pipe.Type(ctx, key)
		ttlCmds[i] = pipe.TTL(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	keys := make([]config.RedisKey, len(keyNames))
	for i, name := range keyNames {
		ttlDur := ttlCmds[i].Val()
		ttlSec := int64(ttlDur)
		if ttlDur >= 0 {
			ttlSec = int64(ttlDur / time.Second)
		}
		keys[i] = config.RedisKey{Name: name, Type: typeCmds[i].Val(), TTL: ttlSec}
	}
	return keys, nil
}

func cloneClusterScanState(state *clusterScanState) *clusterScanState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.masterAddrs = append([]string(nil), state.masterAddrs...)
	clone.pending = append([]string(nil), state.pending...)
	clone.seen = make(map[string]struct{}, len(state.seen))
	for key := range state.seen {
		clone.seen[key] = struct{}{}
	}
	return &clone
}

func storeClusterScanState(state *clusterScanState) uint64 {
	clusterScans.Lock()
	defer clusterScans.Unlock()
	pruneClusterScanStatesLocked(time.Now())
	for len(clusterScans.states) >= maxClusterScanSessions {
		var oldestToken uint64
		var oldestExpiry time.Time
		for token, candidate := range clusterScans.states {
			if oldestToken == 0 || candidate.expiresAt.Before(oldestExpiry) {
				oldestToken = token
				oldestExpiry = candidate.expiresAt
			}
		}
		delete(clusterScans.states, oldestToken)
	}
	token := clusterScanToken.Add(1)
	if token == 0 {
		token = clusterScanToken.Add(1)
	}
	state.expiresAt = time.Now().Add(clusterScanSessionTTL)
	clusterScans.states[token] = state
	return token
}

func takeClusterScanState(token uint64) *clusterScanState {
	clusterScans.Lock()
	defer clusterScans.Unlock()
	pruneClusterScanStatesLocked(time.Now())
	state := clusterScans.states[token]
	delete(clusterScans.states, token)
	return state
}

func restoreClusterScanState(token uint64, state *clusterScanState) {
	if token == 0 || state == nil {
		return
	}
	clusterScans.Lock()
	defer clusterScans.Unlock()
	state.expiresAt = time.Now().Add(clusterScanSessionTTL)
	clusterScans.states[token] = state
}

func pruneClusterScanStatesLocked(now time.Time) {
	for token, state := range clusterScans.states {
		if !state.expiresAt.After(now) {
			delete(clusterScans.states, token)
		}
	}
}

// GetKeyInfo 获取单个 key 的元信息
func GetKeyInfo(ctx context.Context, client redis.UniversalClient, key string) (config.RedisKey, error) {
	pipe := client.Pipeline()
	typeCmd := pipe.Type(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return config.RedisKey{}, err
	}

	ttlDur := ttlCmd.Val()
	var ttlSec int64
	if ttlDur < 0 {
		ttlSec = int64(ttlDur)
	} else {
		ttlSec = int64(ttlDur / time.Second)
	}

	keyType := typeCmd.Val()
	var count int64
	switch keyType {
	case "list":
		count, _ = client.LLen(ctx, key).Result()
	case "hash":
		count, _ = client.HLen(ctx, key).Result()
	case "set":
		count, _ = client.SCard(ctx, key).Result()
	case "zset":
		count, _ = client.ZCard(ctx, key).Result()
	case "stream":
		count, _ = client.XLen(ctx, key).Result()
	default:
		count = 0
	}

	return config.RedisKey{
		Name:  key,
		Type:  keyType,
		TTL:   ttlSec,
		Count: count,
	}, nil
}

// DeleteKey 删除 key
func DeleteKey(ctx context.Context, client redis.UniversalClient, key string) error {
	return client.Del(ctx, key).Err()
}

// RenameKey 重命名 key
func RenameKey(ctx context.Context, client redis.UniversalClient, oldKey, newKey string) error {
	return client.Rename(ctx, oldKey, newKey).Err()
}

// SetTTL 设置 key TTL（-1 表示永久，即 PERSIST）
func SetTTL(ctx context.Context, client redis.UniversalClient, key string, ttlSec int64) error {
	if ttlSec < 0 {
		return client.Persist(ctx, key).Err()
	}
	return client.Expire(ctx, key, time.Duration(ttlSec)*time.Second).Err()
}

// CreateKey 创建指定类型的 key，并按需设置初始值和 TTL。
func CreateKey(ctx context.Context, client redis.UniversalClient, req config.CreateKeyRequest) error {
	key := strings.TrimSpace(req.Key)
	keyType := strings.ToLower(strings.TrimSpace(req.Type))
	if key == "" {
		return fmt.Errorf("key is required")
	}
	if keyType == "" {
		return fmt.Errorf("type is required")
	}

	var value1, value2 string
	switch keyType {
	case "string":
		value1 = req.StringValue
	case "hash":
		value1 = strings.TrimSpace(req.Field)
		if value1 == "" {
			return fmt.Errorf("field is required for hash")
		}
		value2 = req.Value
	case "list":
		value1 = req.ListValue
	case "set":
		value1 = strings.TrimSpace(req.Member)
		if value1 == "" {
			return fmt.Errorf("member is required for set")
		}
	case "zset":
		value1 = strings.TrimSpace(req.Member)
		if value1 == "" {
			return fmt.Errorf("member is required for zset")
		}
	case "stream":
		value1 = strings.TrimSpace(req.Field)
		if value1 == "" {
			return fmt.Errorf("field is required for stream")
		}
		value2 = req.Value
	default:
		return fmt.Errorf("unsupported type: %s", keyType)
	}

	created, err := createKeyScript.Run(
		ctx,
		client,
		[]string{key},
		keyType,
		value1,
		value2,
		strconv.FormatFloat(req.Score, 'g', -1, 64),
		strconv.FormatInt(req.TTL, 10),
	).Int()
	if err != nil {
		return err
	}
	if created == 0 {
		return fmt.Errorf("%w: %q", errKeyAlreadyExists, key)
	}
	return nil
}

// DBSize 获取当前 DB key 总数
func DBSize(ctx context.Context, client redis.UniversalClient) (int64, error) {
	if clusterClient, ok := client.(*redis.ClusterClient); ok {
		var total atomic.Int64
		err := clusterClient.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
			size, err := master.DBSize(ctx).Result()
			if err != nil {
				return err
			}
			total.Add(size)
			return nil
		})
		if err != nil {
			return 0, err
		}
		return total.Load(), nil
	}
	return client.DBSize(ctx).Result()
}
