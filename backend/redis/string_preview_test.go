package redis

import (
	"context"
	"strings"
	"testing"

	"LiteRedis/backend/config"
	goredis "github.com/redis/go-redis/v9"
)

func TestStringPreviewIsBoundedAndReadOnlyWhenIncomplete(t *testing.T) {
	for _, known := range []string{"", "string"} {
		t.Run("type="+known, func(t *testing.T) {
			client := testRedisClient(t)
			ctx := context.Background()
			if err := client.Set(ctx, "large", strings.Repeat("x", maxStringValueBytes+100), 0).Err(); err != nil {
				t.Fatal(err)
			}
			client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
				if cmd.Name() == "get" {
					t.Fatal("unbounded GET used for preview")
				}
				return nil
			}, beforePipeline: func(cmds []goredis.Cmder) error {
				for _, cmd := range cmds {
					if cmd.Name() == "get" {
						t.Fatal("unbounded pipeline GET used for preview")
					}
				}
				return nil
			}})
			kv, err := GetValue(ctx, client, "large", config.DefaultSettings(), 0, 0, "", "", "test", known)
			if err != nil {
				t.Fatal(err)
			}
			if !kv.Truncated || kv.ReadOnlyReason != "truncated" || len(kv.StringVal) != maxStringValueBytes || kv.EditToken != "" {
				t.Fatalf("unsafe preview: %+v", kv)
			}
		})
	}
}

func TestStringPreviewUTF8AndExactBoundary(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	for _, test := range []struct {
		name, value, reason string
		truncated           bool
	}{
		{"empty", "", "", false},
		{"exact", strings.Repeat("x", maxStringValueBytes), "", false},
		{"multibyte", strings.Repeat("x", maxStringValueBytes-1) + "你好", "truncated", true},
		{"binary", "a\xffb", "binary", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := client.Set(ctx, "s", test.value, 0).Err(); err != nil {
				t.Fatal(err)
			}
			kv, err := readStringPreview(ctx, client, "s")
			if err != nil || kv.ReadOnlyReason != test.reason || kv.Truncated != test.truncated {
				t.Fatalf("reason=%s truncated=%v err=%v", kv.ReadOnlyReason, kv.Truncated, err)
			}
			if test.reason == "" && kv.EditToken == "" {
				t.Fatal("complete value missing edit token")
			}
			if test.reason != "" && kv.EditToken != "" {
				t.Fatal("unsafe preview has edit token")
			}
		})
	}
}

func TestStringPreviewChangedSizeHasNoEditToken(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.Set(ctx, "s", "changed", 0)
	client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
		if cmd.Name() == "strlen" {
			cmd.(*goredis.IntCmd).SetVal(2)
			return true
		}
		return false
	}})
	kv, err := readStringPreview(ctx, client, "s")
	if err != nil || kv.ReadOnlyReason != "changed" || kv.EditToken != "" {
		t.Fatalf("unsafe changing preview: %+v %v", kv, err)
	}
}
