package localapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"LiteRedis/backend/config"
)

func TestPublishAndGet(t *testing.T) {
	r := NewKeyRegistry()
	kv := config.KeyValue{Key: "k", Type: "string", StringVal: "v1"}
	if err := r.Publish("c1", "k", kv, 3, "conn-one", ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	got, db, updatedAt, ok := r.Get("c1", "k")
	if !ok {
		t.Fatal("expected snapshot to exist")
	}
	if got.StringVal != "v1" || db != 3 || updatedAt.IsZero() {
		t.Fatalf("unexpected snapshot: %+v db=%d updatedAt=%v", got, db, updatedAt)
	}
}

// 发布时服务端强制覆盖 kv.Key，防止伪造指向
func TestPublishOverridesKeyField(t *testing.T) {
	r := NewKeyRegistry()
	kv := config.KeyValue{Key: "evil", Type: "string", StringVal: "v"}
	if err := r.Publish("c1", "real", kv, 0, "", ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	got, _, _, ok := r.Get("c1", "real")
	if !ok || got.Key != "real" {
		t.Fatalf("expected key overridden to %q, got %+v", "real", got)
	}
}

func TestPublishOverwriteReplacesSize(t *testing.T) {
	r := NewKeyRegistry()
	big := strings.Repeat("x", 1000)
	for i := 0; i < 10; i++ {
		if err := r.Publish("c1", "k", config.KeyValue{Key: "k", Type: "string", StringVal: big}, 0, "", ""); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	// 多次覆盖同一 key 不应累计 totalSize——20 次足够撑爆 50MB 上限
	for i := 0; i < 20; i++ {
		if err := r.Publish("c1", "k", config.KeyValue{Key: "k", Type: "string", StringVal: big}, 0, "", ""); err != nil {
			t.Fatalf("overwrite %d unexpectedly hit total limit: %v", i, err)
		}
	}
}

func TestUnpublishAndUnpublishConn(t *testing.T) {
	r := NewKeyRegistry()
	_ = r.Publish("c1", "a", config.KeyValue{Key: "a", Type: "string", StringVal: "va"}, 0, "", "")
	_ = r.Publish("c1", "b", config.KeyValue{Key: "b", Type: "string", StringVal: "vb"}, 0, "", "")
	_ = r.Publish("c2", "a", config.KeyValue{Key: "a", Type: "string", StringVal: "c2a"}, 0, "", "")

	// 槽位保存的是最后一次发布的 c2/a
	if _, _, _, ok := r.Get("c2", "a"); !ok {
		t.Fatal("slot should hold c2/a")
	}
	if _, _, _, ok := r.Get("c1", "a"); ok {
		t.Fatal("slot is single: c1/a should have been replaced")
	}

	// Unpublish 不匹配的 conn/key 不清除
	r.Unpublish("c1", "a")
	if _, _, _, ok := r.Get("c2", "a"); !ok {
		t.Fatal("mismatched Unpublish should not clear slot")
	}
	r.Unpublish("c2", "a")
	if _, _, _, ok := r.Get("c2", "a"); ok {
		t.Fatal("matched Unpublish should clear slot")
	}

	// UnpublishConn 匹配才清除
	_ = r.Publish("c1", "b", config.KeyValue{Key: "b", Type: "string"}, 0, "", "")
	r.UnpublishConn("c2")
	if _, _, _, ok := r.Get("c1", "b"); !ok {
		t.Fatal("UnpublishConn(c2) should not clear c1 slot")
	}
	r.UnpublishConn("c1")
	if _, _, _, ok := r.Get("c1", "b"); ok {
		t.Fatal("UnpublishConn(c1) should clear slot")
	}
}

func TestPublishRejectsOversizedSnapshot(t *testing.T) {
	r := NewKeyRegistry()
	huge := strings.Repeat("x", maxSnapshotBytes+1)
	err := r.Publish("c1", "k", config.KeyValue{Key: "k", Type: "string", StringVal: huge}, 0, "", "")
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size rejection, got %v", err)
	}
}

func TestListReturnsMetadata(t *testing.T) {
	r := NewKeyRegistry()
	_ = r.Publish("c1", "a", config.KeyValue{Key: "a", Type: "hash", HashVal: map[string]string{"f": "v"}}, 2, "named-conn", "")
	infos := r.List("c1")
	if len(infos) != 1 {
		t.Fatalf("infos = %d, want 1", len(infos))
	}
	info := infos[0]
	if info.Key != "a" || info.Type != "hash" || info.DB != 2 {
		t.Fatalf("unexpected info: %+v", info)
	}
	if _, err := time.Parse(time.RFC3339, info.UpdatedAt); err != nil {
		t.Fatalf("updated_at not RFC3339: %v", err)
	}
	if infos := r.List("missing"); len(infos) != 0 {
		t.Fatalf("unknown conn should return empty list, got %v", infos)
	}
}

func TestResolveEntry(t *testing.T) {
	kv := config.KeyValue{
		Key:  "k",
		Type: "hash",
		HashVal: map[string]string{
			"field1": "v1",
			"a/b":    "slash-value",
		},
	}
	entry, err := ResolveEntry(kv, "field1")
	if err != nil {
		t.Fatalf("hash selector: %v", err)
	}
	if m, ok := entry.(map[string]string); !ok || m["value"] != "v1" {
		t.Fatalf("hash entry = %#v", entry)
	}
	// selector 含 / 也应正常匹配（走 query 参数，不经路径切分）
	if _, err := ResolveEntry(kv, "a/b"); err != nil {
		t.Fatalf("selector with slash should work: %v", err)
	}
	if _, err := ResolveEntry(kv, "missing"); err == nil {
		t.Fatal("missing field should error")
	}
}

func TestResolveEntryListIndex(t *testing.T) {
	kv := config.KeyValue{Key: "k", Type: "list", ListVal: []string{"a", "b", "c"}}
	entry, err := ResolveEntry(kv, "1")
	if err != nil {
		t.Fatalf("list selector: %v", err)
	}
	if m, ok := entry.(map[string]any); !ok || m["value"] != "b" {
		t.Fatalf("list entry = %#v", entry)
	}
	if _, err := ResolveEntry(kv, "-1"); err == nil {
		t.Fatal("negative index should error")
	}
	if _, err := ResolveEntry(kv, "99"); err == nil {
		t.Fatal("out-of-range index should error")
	}
	if _, err := ResolveEntry(kv, "abc"); err == nil {
		t.Fatal("non-numeric selector should error")
	}
}

func TestResolveEntrySetZSetStream(t *testing.T) {
	setKV := config.KeyValue{Key: "k", Type: "set", SetVal: []string{"m1", "m2"}}
	if entry, err := ResolveEntry(setKV, "m1"); err != nil {
		t.Fatalf("set selector: %v", err)
	} else if m := entry.(map[string]string); m["member"] != "m1" {
		t.Fatalf("set entry = %#v", entry)
	}

	zsetKV := config.KeyValue{Key: "k", Type: "zset", ZSetVal: []config.ZSetMember{{Member: "z1", Score: 1.5}}}
	if entry, err := ResolveEntry(zsetKV, "z1"); err != nil {
		t.Fatalf("zset selector: %v", err)
	} else if m := entry.(map[string]any); m["score"] != 1.5 {
		t.Fatalf("zset entry = %#v", entry)
	}

	streamKV := config.KeyValue{Key: "k", Type: "stream",
		StreamVal: []config.StreamEntry{{ID: "1-1", Fields: map[string]string{"f": "v"}}}}
	if entry, err := ResolveEntry(streamKV, "1-1"); err != nil {
		t.Fatalf("stream selector: %v", err)
	} else if m := entry.(map[string]any); m["id"] != "1-1" {
		t.Fatalf("stream entry = %#v", entry)
	}
}

func TestResolveEntryStringUnsupported(t *testing.T) {
	kv := config.KeyValue{Key: "k", Type: "string", StringVal: "v"}
	if _, err := ResolveEntry(kv, "any"); err == nil {
		t.Fatal("string with selector should error")
	}
	if _, err := ResolveEntry(kv, ""); err == nil {
		t.Fatal("empty selector should error")
	}
}

func TestSnapshotJSONShape(t *testing.T) {
	kv := config.KeyValue{Key: "k", Type: "hash", HashVal: map[string]string{"f": "v"}, TotalCount: 1}
	data, err := json.Marshal(kv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"hash_val"`) {
		t.Fatalf("unexpected JSON: %s", data)
	}
}

func TestResolveLocatorByIDAndName(t *testing.T) {
	r := NewKeyRegistry()
	_ = r.Publish("conn-id-1", "hkey", config.KeyValue{
		Key: "hkey", Type: "hash", HashVal: map[string]string{"f1": "v1"},
	}, 3, "my-redis", "n1")

	cases := []struct {
		name    string
		loc     Locator
		want    string
		wantErr bool
	}{
		{"by id", Locator{Nonce: "n1", Redis: "conn-id-1", DB: "3", Key: "hkey", Type: "hash", Position: "f1"}, "v1", false},
		{"by name", Locator{Nonce: "n1", Redis: "my-redis", DB: "3", Key: "hkey", Type: "hash", Position: "f1"}, "v1", false},
		{"db numeric", Locator{Nonce: "n1", Redis: "conn-id-1", DB: "3", Key: "hkey", Type: "hash", Position: "f1"}, "v1", false},
		{"conn/db/key ignored when nonce matches", Locator{Nonce: "n1", Redis: "whatever", DB: "9", Key: "whatever", Position: "f1"}, "v1", false},
		{"unknown conn without nonce match", Locator{Nonce: "other", Redis: "nope", Key: "hkey"}, "", true},
		{"missing redis", Locator{Key: "hkey"}, "", true},
		{"bad nonce", Locator{Nonce: "bad", Redis: "conn-id-1", Key: "hkey"}, "", true},
	}
	for _, tc := range cases {
		got, err := r.ResolveLocator(tc.loc)
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: err = %v, wantErr = %v", tc.name, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResolveLocatorPlainTextPerType(t *testing.T) {
	r := NewKeyRegistry()
	publishAndResolve := func(key, typ, position string, kv config.KeyValue, want string) {
		t.Helper()
		if err := r.Publish("c", key, kv, 0, "", "n"); err != nil {
			t.Fatalf("publish %s: %v", key, err)
		}
		got, err := r.ResolveLocator(Locator{Nonce: "n", Redis: "c", Key: key, Type: typ, Position: position})
		if err != nil || got != want {
			t.Fatalf("%s: got %q err %v, want %q", key, got, err, want)
		}
	}

	// string：position 忽略
	publishAndResolve("s", "string", "0", config.KeyValue{Key: "s", Type: "string", StringVal: "hello"}, "hello")
	// list 下标
	publishAndResolve("l", "list", "1", config.KeyValue{Key: "l", Type: "list", ListVal: []string{"a", "b"}}, "b")
	// set 下标
	publishAndResolve("set", "set", "0", config.KeyValue{Key: "set", Type: "set", SetVal: []string{"m1", "m2"}}, "m1")
	// zset 下标 → member（不含 score）
	publishAndResolve("z", "zset", "0", config.KeyValue{Key: "z", Type: "zset",
		ZSetVal: []config.ZSetMember{{Member: "z1", Score: 9.5}, {Member: "z2", Score: 1}}}, "z1")
	// stream by ID
	publishAndResolve("st", "stream", "1-1", config.KeyValue{Key: "st", Type: "stream",
		StreamVal: []config.StreamEntry{{ID: "1-1", Fields: map[string]string{"f": "v"}}}}, `{"f":"v"}`)
	// stream by index
	publishAndResolve("st2", "stream", "0", config.KeyValue{Key: "st2", Type: "stream",
		StreamVal: []config.StreamEntry{{ID: "1-1", Fields: map[string]string{"f": "v"}}}}, `{"f":"v"}`)

	// 越界：先发布再解析
	_ = r.Publish("c", "l2", config.KeyValue{Key: "l2", Type: "list", ListVal: []string{"a"}}, 0, "", "n")
	if _, err := r.ResolveLocator(Locator{Nonce: "n", Redis: "c", Key: "l2", Type: "list", Position: "99"}); err == nil {
		t.Fatal("list out-of-range should error")
	}
	// nonce 匹配即槽位数据（key/db/连接字段不影响定位）
	got, err := r.ResolveLocator(Locator{Nonce: "n", Redis: "whatever", Key: "other", Type: "whatever", Position: "0"})
	if err != nil || got != "a" {
		t.Fatalf("nonce match should serve slot data: %q %v", got, err)
	}
	// nonce 不匹配 → 过期
	if _, err := r.ResolveLocator(Locator{Nonce: "stale", Redis: "c", Key: "l2", Type: "list", Position: "0"}); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("stale nonce should report expired, got %v", err)
	}
}
