package redis

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	goredis "github.com/redis/go-redis/v9"
)

func TestStringRangeBoundedLosslessPages(t *testing.T) {
	for _, value := range []string{
		"", "plain text", strings.Repeat("x", int(StringRangePageBytes)),
		strings.Repeat("x", int(StringRangePageBytes)*2+17),
		strings.Repeat("x", int(StringRangePageBytes)-1) + "你好🌍" + strings.Repeat("y", int(StringRangePageBytes)),
		"a\x00\xffb" + strings.Repeat("z", int(StringRangePageBytes)),
	} {
		t.Run(fmt.Sprintf("bytes=%d", len(value)), func(t *testing.T) {
			client := testRedisClient(t)
			ctx := context.Background()
			if err := client.Set(ctx, "key", value, 0).Err(); err != nil {
				t.Fatal(err)
			}
			ranges := 0
			client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
				switch cmd.Name() {
				case "type", "strlen":
				case "getrange":
					ranges++
					start, ok1 := cmd.Args()[2].(int64)
					end, ok2 := cmd.Args()[3].(int64)
					if !ok1 || !ok2 || start < 0 || end-start+1 != StringRangePageBytes {
						t.Fatalf("unbounded range: %v", cmd.Args())
					}
				default:
					t.Fatalf("non-read-only/unbounded command: %s", cmd.Name())
				}
				return nil
			}})
			var joined strings.Builder
			var offset int64
			for pageNumber := 0; ; pageNumber++ {
				page, err := ReadStringRangePreview(ctx, client, "key", offset)
				if err != nil {
					t.Fatal(err)
				}
				raw := page.Value
				if page.Binary {
					decoded, err := base64.StdEncoding.DecodeString(raw)
					if err != nil || page.Encoding != "base64" {
						t.Fatalf("invalid base64: %+v %v", page, err)
					}
					raw = string(decoded)
				} else if page.Encoding != "utf-8" || !utf8.ValidString(raw) {
					t.Fatal("invalid text encoding")
				}
				if len(raw) > int(StringRangePageBytes) || page.Offset != offset || page.Start != offset || page.End != offset+int64(len(raw)) || page.NextOffset != page.End || page.TotalBytes != int64(len(value)) || page.HasMore != (page.End < page.TotalBytes) {
					t.Fatalf("bad range metadata: %+v", page)
				}
				if ranges != pageNumber+1 {
					t.Fatalf("expected exactly one bounded GETRANGE per page: %d", ranges)
				}
				joined.WriteString(raw)
				if !page.HasMore {
					break
				}
				if page.NextOffset <= offset {
					t.Fatal("non-advancing page")
				}
				offset = page.NextOffset
			}
			if joined.String() != value {
				t.Fatal("lost or duplicated raw bytes across page boundaries")
			}
		})
	}
}

func TestStringRangeRejectsMissingTypeAndChangedValues(t *testing.T) {
	for _, mutation := range []string{"missing", "wrongtype", "delete-before-range", "delete-after-range", "type-before-range", "type-after-range", "length-after-range"} {
		t.Run(mutation, func(t *testing.T) {
			client := testRedisClient(t)
			ctx := context.Background()
			client.Set(ctx, "key", "before", 0)
			if mutation == "missing" {
				client.Del(ctx, "key")
			}
			if mutation == "wrongtype" {
				client.Del(ctx, "key")
				client.RPush(ctx, "key", "item")
			}
			strlens := 0
			client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
				if cmd.Name() == "strlen" {
					strlens++
				}
				beforeRange := cmd.Name() == "getrange" && strings.Contains(mutation, "before-range")
				afterRange := cmd.Name() == "strlen" && strlens == 2 && strings.Contains(mutation, "after-range")
				if beforeRange || afterRange {
					switch {
					case strings.HasPrefix(mutation, "delete"):
						return client.Del(ctx, "key").Err()
					case strings.HasPrefix(mutation, "type"):
						client.Del(ctx, "key")
						return client.RPush(ctx, "key", "item").Err()
					default:
						return client.Set(ctx, "key", "different length", 0).Err()
					}
				}
				return nil
			}})
			if page, err := ReadStringRangePreview(ctx, client, "key", 0); err == nil || page != (StringRangePreview{}) {
				t.Fatalf("accepted unstable range: %+v %v", page, err)
			}
		})
	}
}

type stringRangeLengthClient struct {
	goredis.UniversalClient
	total int64
}

func (c stringRangeLengthClient) StrLen(context.Context, string) *goredis.IntCmd {
	return goredis.NewIntResult(c.total, nil)
}

func (c stringRangeLengthClient) GetRange(ctx context.Context, key string, start, end int64) *goredis.StringCmd {
	if c.total == maxStringRangeTotalBytes && start == c.total-1 && end-start+1 == StringRangePageBytes {
		return goredis.NewStringResult("x", nil)
	}
	return c.UniversalClient.GetRange(ctx, key, start, end)
}

func TestStringRangeValidationLimitsEOFAndCancellation(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.Set(ctx, "key", "abc", 0)
	for _, offset := range []int64{-1, MaxStringRangeOffset + 1, MaxStringRangeOffset, 4} {
		if _, err := ReadStringRangePreview(ctx, client, "key", offset); err == nil {
			t.Fatalf("accepted offset %d", offset)
		}
	}
	page, err := ReadStringRangePreview(ctx, client, "key", 3)
	if err != nil || page.HasMore || page.Start != 3 || page.End != 3 || page.Value != "" {
		t.Fatalf("EOF semantics: %+v %v", page, err)
	}
	for _, total := range []int64{-1, maxStringRangeTotalBytes + 1} {
		if _, err := ReadStringRangePreview(ctx, stringRangeLengthClient{client, total}, "key", 0); err == nil {
			t.Fatalf("accepted invalid total %d", total)
		}
	}
	// Exercise the supported 512 MiB ceiling without allocating a huge fixture.
	last, err := ReadStringRangePreview(ctx, stringRangeLengthClient{client, maxStringRangeTotalBytes}, "key", maxStringRangeTotalBytes-1)
	if err != nil || last.Value != "x" || last.End != maxStringRangeTotalBytes || last.HasMore {
		t.Fatalf("512 MiB limit not supported: %+v %v", last, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := ReadStringRangePreview(cancelled, client, "key", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
	client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
		if cmd.Name() == "getrange" {
			return errors.New("NOPERM GETRANGE denied")
		}
		return nil
	}})
	if _, err := ReadStringRangePreview(ctx, client, "key", 0); err == nil || !strings.Contains(err.Error(), "NOPERM") {
		t.Fatalf("permission error swallowed: %v", err)
	}
}
