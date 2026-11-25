package redis

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	goredis "github.com/redis/go-redis/v9"
)

const conflictSnapshotMaxBytes = 64 * 1024

// ConflictSnapshot is a read-only comparison preview, never an editable value or
// edit token. Value is UTF-8 text, or standard base64 when Binary is true.
// TotalBytes describes the original value; Truncated means the preview omits bytes.
// Exists describes the requested field for hashes, otherwise the key. Type is the
// current key type ("none" for a missing key), even if it no longer matches the edit.
type ConflictSnapshot struct {
	Exists     bool   `json:"exists"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Truncated  bool   `json:"truncated"`
	Binary     bool   `json:"binary"`
	TotalBytes int64  `json:"total_bytes"`
}

// All commands inside this script are read-only. Oversized Hash fields are
// metadata-only: HGET has no range form and copying the full field in Lua would
// defeat server-side memory bounds. Only bounded data crosses
// the network, even for huge fields. Do not fall back to a client-side HGET when
// EVAL or any inner command is denied. HSTRLEN/HEXISTS distinguish empty/missing.
const conflictHashScript = `
local kind = redis.call('TYPE', KEYS[1]).ok
if kind ~= 'hash' then
  return {kind, kind ~= 'none' and 1 or 0, 0, ''}
end
local size = redis.call('HSTRLEN', KEYS[1], ARGV[1])
local exists = redis.call('HEXISTS', KEYS[1], ARGV[1])
if exists == 0 then return {kind, 0, 0, ''} end
if size > 65536 then return {kind, 1, size, ''} end
return {kind, 1, size, string.sub(redis.call('HGET', KEYS[1], ARGV[1]), 1, 65536)}
`

// ReadEditConflict reads at most 64KiB of value bytes, without writing or minting
// edit tokens. String reads are a best-effort preview, not an atomic edit basis.
// A racing type change is retried once; no command ever fetches an unbounded value.
func ReadEditConflict(ctx context.Context, client goredis.UniversalClient, key, field string, isHash bool) (ConflictSnapshot, error) {
	for attempt := 0; attempt < 2; attempt++ {
		snapshot, err := readEditConflict(ctx, client, key, field, isHash)
		if err == nil || !strings.HasPrefix(err.Error(), "WRONGTYPE") {
			return snapshot, err
		}
	}
	return ConflictSnapshot{}, fmt.Errorf("key type changed while reading conflict snapshot; retry")
}

func readEditConflict(ctx context.Context, client goredis.UniversalClient, key, field string, isHash bool) (ConflictSnapshot, error) {
	kind, err := client.Type(ctx, key).Result()
	if err != nil {
		return ConflictSnapshot{}, err
	}
	snapshot := ConflictSnapshot{Exists: kind != "none", Type: kind}
	if (!isHash && kind != "string") || (isHash && kind != "hash") {
		return snapshot, nil
	}
	if isHash {
		// Check length first without transferring the field. The script repeats
		// metadata reads atomically so concurrent updates cannot mislabel its prefix.
		if _, err := client.HStrLen(ctx, key, field).Result(); err != nil {
			return ConflictSnapshot{}, err
		}
		parts, err := client.Eval(ctx, conflictHashScript, []string{key}, field).Slice()
		if err != nil {
			return ConflictSnapshot{}, err
		}
		if len(parts) != 4 {
			return ConflictSnapshot{}, fmt.Errorf("invalid conflict snapshot response")
		}
		kind, kindOK := parts[0].(string)
		exists, existsOK := parts[1].(int64)
		size, sizeOK := parts[2].(int64)
		value, valueOK := parts[3].(string)
		if !kindOK || !existsOK || !sizeOK || !valueOK || size < 0 || len(value) > conflictSnapshotMaxBytes {
			return ConflictSnapshot{}, fmt.Errorf("invalid conflict snapshot response")
		}
		return encodeConflictSnapshot(ConflictSnapshot{Type: kind, Exists: exists == 1, TotalBytes: size}, value), nil
	}
	snapshot.TotalBytes, err = client.StrLen(ctx, key).Result()
	if err != nil {
		return ConflictSnapshot{}, err
	}
	value, err := client.GetRange(ctx, key, 0, conflictSnapshotMaxBytes-1).Result()
	if err != nil {
		return ConflictSnapshot{}, err
	}
	// Redis returns empty strings for expired keys; do not mistake that for an
	// existing empty value. Also surface a racing type change without its value.
	kind, err = client.Type(ctx, key).Result()
	if err != nil {
		return ConflictSnapshot{}, err
	}
	if kind != "string" {
		return ConflictSnapshot{Type: kind, Exists: kind != "none"}, nil
	}
	finalSize, err := client.StrLen(ctx, key).Result()
	if err != nil {
		return ConflictSnapshot{}, err
	}
	if finalSize != snapshot.TotalBytes || int64(len(value)) != min(finalSize, int64(conflictSnapshotMaxBytes)) {
		return ConflictSnapshot{}, fmt.Errorf("value changed while reading conflict snapshot; retry")
	}
	return encodeConflictSnapshot(snapshot, value), nil
}

func encodeConflictSnapshot(snapshot ConflictSnapshot, value string) ConflictSnapshot {
	snapshot.Truncated = snapshot.TotalBytes > int64(len(value))
	// A bounded prefix may end inside a valid UTF-8 character. Remove only an
	// incomplete final rune; invalid bytes anywhere else are explicitly base64.
	if snapshot.Truncated {
		for n := 1; n < utf8.UTFMax && n <= len(value); n++ {
			start := len(value) - n
			if utf8.RuneStart(value[start]) {
				if !utf8.FullRuneInString(value[start:]) && utf8.ValidString(value[:start]) {
					value = value[:start]
				}
				break
			}
		}
	}
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		snapshot.Binary = true
		snapshot.Value = base64.StdEncoding.EncodeToString([]byte(value))
	} else {
		snapshot.Value = value
	}
	return snapshot
}
