package redis

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf8"

	goredis "github.com/redis/go-redis/v9"
)

const (
	// StringRangePageBytes is the maximum raw byte count returned per page.
	StringRangePageBytes int64 = 64 * 1024
	// MaxStringRangeOffset is the largest integer exactly representable by JS.
	MaxStringRangeOffset int64 = 1<<53 - 1
	// Redis strings support at most 512 MiB; reject implausible metadata without
	// allocating a buffer or attempting an unbounded read.
	maxStringRangeTotalBytes int64 = 512 * 1024 * 1024
)

// StringRangePreview is a read-only byte range, not an editable value or token.
// Offset and Start are inclusive; End and NextOffset are exclusive raw-byte
// positions, independent of the encoded Value length. Empty EOF pages have
// Start == End == TotalBytes. Encoding is "utf-8" or "base64"; Binary identifies
// base64 pages (including otherwise-text pages split inside a UTF-8 rune).
// Reads are NOT snapshots: length/type changes are detected best-effort, but
// same-length writes and mutations between separate page requests may go unseen.
type StringRangePreview struct {
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	TotalBytes int64  `json:"total_bytes"`
	HasMore    bool   `json:"has_more"`
	Value      string `json:"value"`
	Binary     bool   `json:"binary"`
	Encoding   string `json:"encoding"`
	Start      int64  `json:"start"`
	End        int64  `json:"end"`
}

// ReadStringRangePreview reads exactly one bounded GETRANGE with metadata checks
// on either side. It never GETs the entire value, writes, or generates a token.
// Offset equal to EOF returns an empty page; offset beyond EOF is an error.
func ReadStringRangePreview(ctx context.Context, client goredis.UniversalClient, key string, offset int64) (StringRangePreview, error) {
	if key == "" || offset < 0 || offset > MaxStringRangeOffset {
		return StringRangePreview{}, fmt.Errorf("invalid string range parameters: offset must be a non-negative safe JS integer")
	}
	if err := ctx.Err(); err != nil {
		return StringRangePreview{}, err
	}
	kind, err := client.Type(ctx, key).Result()
	if err != nil {
		return StringRangePreview{}, err
	}
	if kind != "string" {
		return StringRangePreview{}, fmt.Errorf("string range key missing or not a string (type %s)", kind)
	}
	total, err := client.StrLen(ctx, key).Result()
	if err != nil {
		return StringRangePreview{}, err
	}
	if total < 0 || total > maxStringRangeTotalBytes {
		return StringRangePreview{}, fmt.Errorf("string range length exceeds Redis 512 MiB limit")
	}
	if offset > total {
		return StringRangePreview{}, fmt.Errorf("string range offset %d beyond EOF %d", offset, total)
	}
	// Offset has already been bounded by the validated total, so addition cannot
	// overflow. Redis GETRANGE uses an inclusive end; our response uses exclusive.
	value, err := client.GetRange(ctx, key, offset, offset+StringRangePageBytes-1).Result()
	if err != nil {
		return StringRangePreview{}, err
	}
	finalTotal, err := client.StrLen(ctx, key).Result()
	if err != nil {
		return StringRangePreview{}, err
	}
	finalKind, err := client.Type(ctx, key).Result()
	if err != nil {
		return StringRangePreview{}, err
	}
	if finalKind != "string" || finalTotal != total || int64(len(value)) != min(StringRangePageBytes, total-offset) {
		return StringRangePreview{}, fmt.Errorf("string changed or disappeared while reading range; retry (read is not a snapshot)")
	}
	next := offset + int64(len(value))
	page := StringRangePreview{
		Offset: offset, Start: offset, End: next, NextOffset: next,
		TotalBytes: total, HasMore: next < total, Value: value, Encoding: "utf-8",
	}
	// Never trim partial UTF-8: preserving every raw byte is essential to avoid
	// skipping or duplicating bytes on subsequent pages.
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
		page.Binary = true
		page.Encoding = "base64"
		page.Value = base64.StdEncoding.EncodeToString([]byte(value))
	}
	return page, nil
}
