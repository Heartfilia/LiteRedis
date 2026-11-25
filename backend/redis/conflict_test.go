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

func TestConflictSnapshotBoundedTextAndBinary(t *testing.T) {
	for _, hash := range []bool{false, true} {
		for _, tc := range []struct {
			name, value       string
			binary, truncated bool
		}{
			{"empty", "", false, false},
			{"exact", strings.Repeat("x", conflictSnapshotMaxBytes), false, false},
			{"large", strings.Repeat("x", conflictSnapshotMaxBytes+10000), false, true},
			{"utf8 boundary", strings.Repeat("x", conflictSnapshotMaxBytes-1) + "你好", false, true},
			{"binary", "a\x00\xffb", true, false},
			{"large binary", "\xff" + strings.Repeat("b", conflictSnapshotMaxBytes+100), true, true},
		} {
			t.Run(fmt.Sprintf("hash=%v/%s", hash, tc.name), func(t *testing.T) {
				client := testRedisClient(t)
				ctx := context.Background()
				if hash {
					if err := client.HSet(ctx, "key", "field", tc.value, "unrelated", "untouched").Err(); err != nil {
						t.Fatal(err)
					}
				} else if err := client.Set(ctx, "key", tc.value, 0).Err(); err != nil {
					t.Fatal(err)
				}
				client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
					switch cmd.Name() {
					case "type", "strlen", "hstrlen":
					case "getrange":
						if fmt.Sprint(cmd.Args()[2]) != "0" || fmt.Sprint(cmd.Args()[3]) != "65535" {
							t.Fatal("unbounded range")
						}
					case "eval":
						if cmd.Args()[1] != conflictHashScript {
							t.Fatal("unexpected Lua")
						}
					default:
						t.Fatalf("unexpected command %s", cmd.Name())
					}
					return nil
				}})
				s, err := ReadEditConflict(ctx, client, "key", "field", hash)
				if err != nil {
					t.Fatal(err)
				}
				if !s.Exists || s.Binary != (tc.binary && !(hash && tc.truncated)) || s.Truncated != tc.truncated || s.TotalBytes != int64(len(tc.value)) {
					t.Fatalf("invalid metadata: %+v", s)
				}
				value := s.Value
				if s.Binary {
					decoded, err := base64.StdEncoding.DecodeString(value)
					if err != nil {
						t.Fatal(err)
					}
					value = string(decoded)
				} else if !utf8.ValidString(value) {
					t.Fatal("invalid UTF-8 display")
				}
				if len(value) > conflictSnapshotMaxBytes || !strings.HasPrefix(tc.value, value) {
					t.Fatalf("invalid bounded prefix: %d", len(value))
				}
				if !tc.truncated && value != tc.value {
					t.Fatal("incomplete value")
				}
			})
		}
	}
}

func TestConflictSnapshotMissingAndChangedType(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	for _, hash := range []bool{false, true} {
		s, err := ReadEditConflict(ctx, client, "missing", "f", hash)
		if err != nil || s.Exists || s.Type != "none" {
			t.Fatalf("missing key: %+v %v", s, err)
		}
	}
	client.HSet(ctx, "hash", "empty", "")
	for _, field := range []string{"empty", "missing"} {
		s, err := ReadEditConflict(ctx, client, "hash", field, true)
		if err != nil || s.Exists != (field == "empty") || s.Type != "hash" || s.Value != "" || s.TotalBytes != 0 {
			t.Fatalf("empty vs missing: %+v %v", s, err)
		}
	}
	client.RPush(ctx, "changed", "item")
	for _, hash := range []bool{false, true} {
		s, err := ReadEditConflict(ctx, client, "changed", "f", hash)
		if err != nil || !s.Exists || s.Type != "list" || s.Value != "" {
			t.Fatalf("changed type: %+v %v", s, err)
		}
	}
}

func TestConflictHashPermissionDeniedNeverFallsBack(t *testing.T) {
	for _, denied := range []string{"hstrlen", "eval"} {
		for _, denial := range []string{"NOPERM denied", "ERR unknown command"} {
			t.Run(denied+denial, func(t *testing.T) {
				client := testRedisClient(t)
				ctx := context.Background()
				client.HSet(ctx, "key", "field", strings.Repeat("x", conflictSnapshotMaxBytes*2))
				calls := 0
				client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
					calls++
					if cmd.Name() == "hget" {
						t.Fatal("unbounded fallback HGET")
					}
					if cmd.Name() == denied {
						return errors.New(denial)
					}
					return nil
				}})
				_, err := ReadEditConflict(ctx, client, "key", "field", true)
				wantCalls := 2
				if denied == "eval" {
					wantCalls = 3
				}
				if err == nil || !strings.Contains(err.Error(), denial) || calls != wantCalls {
					t.Fatalf("err=%v calls=%d", err, calls)
				}
			})
		}
	}
}

func TestConflictSnapshotReadOnlyAndRacingType(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.Set(ctx, "key", "before", 0)
	changed := false
	client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
		if cmd.Name() == "strlen" && !changed {
			changed = true
			client.Del(ctx, "key")
			client.RPush(ctx, "key", "changed")
		}
		return nil
	}})
	s, err := ReadEditConflict(ctx, client, "key", "", false)
	if err != nil || s.Type != "list" || !s.Exists || s.Value != "" {
		t.Fatalf("racing type: %+v %v", s, err)
	}
}

func TestConflictSnapshotRejectsRacingLength(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	client.Set(ctx, "key", "small", 0)
	client.AddHook(exportCommandHook{before: func(cmd goredis.Cmder) error {
		if cmd.Name() == "getrange" {
			return client.Set(ctx, "key", "different length", 0).Err()
		}
		return nil
	}})
	if _, err := ReadEditConflict(ctx, client, "key", "", false); err == nil {
		t.Fatal("changing length claimed to be a complete snapshot")
	}
}
