package redis

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"LiteRedis/backend/config"
	"github.com/redis/go-redis/v9"
)

// EditToken 是编辑冲突检测用的内容摘要，不是认证或安全校验值。
func stringEditToken(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

// 只读取预览上限+1 字节，不因源值过大而把整个值传到客户端。
func readStringPreview(ctx context.Context, client redis.UniversalClient, key string) (config.KeyValue, error) {
	pipe := client.Pipeline()
	kind := pipe.Type(ctx, key)
	ttl := pipe.TTL(ctx, key)
	size := pipe.StrLen(ctx, key)
	value := pipe.GetRange(ctx, key, 0, maxStringValueBytes)
	if _, err := pipe.Exec(ctx); err != nil {
		return config.KeyValue{}, err
	}
	if kind.Val() != "string" || ttl.Val() == -2 {
		return config.KeyValue{}, fmt.Errorf("key no longer exists or its type changed; refresh before editing")
	}
	kv := config.KeyValue{Key: key, Type: "string", TTL: ttlSecondsOf(ttl.Val()), TotalCount: size.Val()}
	raw := value.Val()
	kv.Truncated = size.Val() > maxStringValueBytes || len(raw) > maxStringValueBytes
	if kv.Truncated {
		kv.ReadOnlyReason = "truncated"
		raw = raw[:min(len(raw), maxStringValueBytes)]
		// 仅移除边界处被切开的 UTF-8 编码，不遍历删除含二进制的整个前缀。
		for i := 0; i < 3 && len(raw) > 0 && !utf8.ValidString(raw); i++ {
			if utf8.RuneStart(raw[len(raw)-1]) && utf8.FullRuneInString(raw[len(raw)-1:]) {
				break
			}
			raw = raw[:len(raw)-1]
		}
	} else if int64(len(raw)) != size.Val() {
		kv.ReadOnlyReason = "changed"
	}
	if !utf8.ValidString(raw) {
		if kv.ReadOnlyReason == "" {
			kv.ReadOnlyReason = "binary"
		}
		kv.StringVal = strings.ToValidUTF8(raw, "\uFFFD")
	} else {
		kv.StringVal = raw
		if kv.ReadOnlyReason == "" {
			kv.EditToken = stringEditToken(raw)
		}
	}
	return kv, nil
}
