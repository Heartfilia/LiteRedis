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

// 超大值只读取预览窗口，不因源值过大而把整个值传到客户端。
func readStringPreview(ctx context.Context, client redis.UniversalClient, key string) (config.KeyValue, error) {
	pipe := client.Pipeline()
	kind := pipe.Type(ctx, key)
	ttl := pipe.TTL(ctx, key)
	size := pipe.StrLen(ctx, key)
	// 一次多取 1 字节，普通小值可直接命中，超大值也只传输很小的窗口。
	head := pipe.GetRange(ctx, key, 0, maxStringPreviewBytes)
	if _, err := pipe.Exec(ctx); err != nil {
		return config.KeyValue{}, err
	}
	if kind.Val() != "string" || ttl.Val() == -2 {
		return config.KeyValue{}, fmt.Errorf("key no longer exists or its type changed; refresh before editing")
	}
	kv := config.KeyValue{Key: key, Type: "string", TTL: ttlSecondsOf(ttl.Val()), TotalCount: size.Val()}
	raw := head.Val()
	truncated := false
	switch {
	case size.Val() > maxStringValueBytes:
		// 超大 key：只展示前 2000 个字符，不拉取剩余内容，也不生成编辑令牌。
		truncated = true
		raw = truncatePreviewRunes(raw, maxStringPreviewRunes)
	case int64(len(raw)) < size.Val():
		// 超过初次读取窗口、但仍允许完整编辑的值，再补一次有上限的读取。
		full, err := client.GetRange(ctx, key, 0, maxStringValueBytes).Result()
		if err != nil {
			return config.KeyValue{}, err
		}
		raw = full
	}
	kv.Truncated = truncated
	if truncated {
		kv.ReadOnlyReason = "truncated"
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
