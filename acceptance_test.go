package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"LiteRedis/backend/config"
	backend "LiteRedis/backend/redis"
	"github.com/redis/go-redis/v9"
)

// Opt-in only: these tests spawn disposable loopback servers, never attach to
// the user's configured Redis and never run FLUSHDB/FLUSHALL on external data.
func acceptanceRedis(t *testing.T, binary string, cluster bool) *redis.Client {
	t.Helper()
	if binary == "" {
		t.Skip("set LITEREDIS_TEST_REDIS7 / LITEREDIS_TEST_REDIS3 to redis-server binaries")
	}
	dir := t.TempDir()
	port := 0
	for {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port = ln.Addr().(*net.TCPAddr).Port
		ln.Close()
		if !cluster {
			break
		}
		if port > 55000 {
			continue
		}
		bus, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port+10000))
		if err == nil {
			bus.Close()
			break
		}
	}
	text := fmt.Sprintf("bind 127.0.0.1\nport %d\nsave \"\"\nappendonly no\ndir %s\n", port, dir)
	if cluster {
		text += "cluster-enabled yes\ncluster-config-file nodes.conf\ncluster-node-timeout 1000\n"
	}
	path := filepath.Join(dir, "redis.conf")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	log, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, path)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("127.0.0.1:%d", port), MaxRetries: -1, Protocol: 2, DialTimeout: time.Second, ReadTimeout: time.Second})
	t.Cleanup(func() {
		client.Close()
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
		log.Close()
		if t.Failed() {
			data, _ := os.ReadFile(log.Name())
			t.Logf("Redis log:\n%s", data)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for client.Ping(context.Background()).Err() != nil {
		if time.Now().After(deadline) {
			t.Fatal("Redis did not start")
		}
		time.Sleep(25 * time.Millisecond)
	}
	return client
}

func TestAcceptanceStandalone(t *testing.T) {
	for _, version := range []string{"7", "3"} {
		t.Run("Redis"+version, func(t *testing.T) {
			client := acceptanceRedis(t, os.Getenv("LITEREDIS_TEST_REDIS"+version), false)
			ctx := context.Background()
			info, err := client.Info(ctx, "server").Result()
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(info, "\n") {
				if strings.HasPrefix(line, "redis_version:") {
					t.Log(strings.TrimSpace(line))
				}
			}
			entries := []config.ExportedKey{
				{Key: "string", Type: "string", String: "new", TTL: 60},
				{Key: "hash", Type: "hash", Hash: map[string]string{"field": "new"}, TTL: -1},
				{Key: "list", Type: "list", List: []string{"a", "b"}, TTL: 60},
				{Key: "set", Type: "set", Set: []string{"a", "b"}, TTL: 60},
				{Key: "zset", Type: "zset", ZSet: []config.ZSetMember{{Member: "one", Score: 1.5}}, TTL: 60},
			}
			if version == "7" {
				entries = append(entries, config.ExportedKey{Key: "stream", Type: "stream", Stream: []config.StreamEntry{{ID: "1-0", Fields: map[string]string{"f": "v"}}}, TTL: 60})
			}
			for _, entry := range entries {
				if err := client.Set(ctx, entry.Key, "original", time.Hour).Err(); err != nil {
					t.Fatal(err)
				}
				if written, err := backend.WriteExportedKey(ctx, client, entry, false); err != nil || written {
					t.Fatalf("no-overwrite: %v %v", written, err)
				}
				if written, err := backend.WriteExportedKey(ctx, client, entry, true); err != nil || !written {
					t.Fatalf("overwrite %s: %v %v", entry.Type, written, err)
				}
				actual, err := backend.ReadKeyForExport(ctx, client, entry.Key)
				if err != nil || actual.Type != entry.Type {
					t.Fatalf("readback %+v %v", actual, err)
				}
				if entry.TTL > 0 && (actual.TTL <= 0 || actual.TTL > entry.TTL) {
					t.Fatalf("TTL=%d", actual.TTL)
				}
				if entry.TTL == -1 && actual.TTL != -1 {
					t.Fatal("persistent import has TTL")
				}
				actual.TTL = entry.TTL // Expiring TTL is checked above, payload must be exact.
				sort.Strings(actual.Set)
				sort.Strings(entry.Set)
				if !reflect.DeepEqual(actual, entry) {
					t.Fatalf("payload readback: got %+v, want %+v", actual, entry)
				}
			}
			client.Set(ctx, "cas", "original", time.Minute)
			kv, err := backend.GetValue(ctx, client, "cas", config.DefaultSettings(), 0, 0, "", "", "acceptance", "string")
			if err != nil {
				t.Fatal(err)
			}
			if result := backend.SetStringChecked(ctx, client, "cas", "saved", kv.EditToken); !result.Success {
				t.Fatalf("CAS: %+v", result)
			}
			if result := backend.SetStringChecked(ctx, client, "cas", "stale", kv.EditToken); result.Code != "conflict" {
				t.Fatalf("stale CAS: %+v", result)
			}
			if client.PTTL(ctx, "cas").Val() <= 0 {
				t.Fatal("CAS lost TTL")
			}
			if result := backend.HSetChecked(ctx, client, "hash", "field", "saved", "new", true); !result.Success {
				t.Fatalf("Hash CAS: %+v", result)
			}
			if err := backend.LSet(ctx, client, "list", 0, "saved"); err != nil {
				t.Fatal(err)
			}
			if err := backend.RenameSetMember(ctx, client, "set", "a", "c"); err != nil {
				t.Fatal(err)
			}
			if err := backend.RenameZSetMember(ctx, client, "zset", "one", "two"); err != nil {
				t.Fatal(err)
			}
			if n, err := backend.SetKeysTTL(ctx, client, []string{"list", "set"}, 30); err != nil || n != 2 {
				t.Fatalf("TTL %d %v", n, err)
			}
			if n, err := backend.DeleteKeys(ctx, client, []string{"list", "set"}); err != nil || n != 2 {
				t.Fatalf("delete %d %v", n, err)
			}
			if keys := client.Keys(ctx, "*:__literedis_import:*").Val(); len(keys) != 0 {
				t.Fatalf("stage leak %v", keys)
			}
			if version == "7" {
				if n, _, _, err := backend.CopyKeysToDB(ctx, client, []string{"cas"}, 1, true); err != nil || n != 1 {
					t.Fatalf("COPY %d %v", n, err)
				}
				copyClient := redis.NewClient(&redis.Options{Addr: client.Options().Addr, DB: 1, MaxRetries: -1})
				defer copyClient.Close()
				if value, err := copyClient.Get(ctx, "cas").Result(); err != nil || value != "saved" || copyClient.PTTL(ctx, "cas").Val() <= 0 {
					t.Fatalf("COPY readback: %q %v", value, err)
				}
			} else {
				_, _, _, err := backend.CopyKeysToDB(ctx, client, []string{"cas"}, 1, true)
				if backend.RedisErrorCode(err) != "unsupported" {
					t.Fatalf("old COPY classification %v", err)
				}
			}
		})
	}
}

func TestAcceptanceACL(t *testing.T) {
	admin := acceptanceRedis(t, os.Getenv("LITEREDIS_TEST_REDIS7"), false)
	ctx := context.Background()
	for _, denied := range []string{"eval", "hmset", "pexpire", "persist", "rename", "del-cleanup", "stage-key"} {
		t.Run(denied, func(t *testing.T) {
			admin.Set(ctx, "target", "original", time.Hour)
			args := []interface{}{"ACL", "SETUSER", "limited", "reset", "on", ">acceptance-only", "~*", "+@all"}
			if denied == "stage-key" {
				args = append(args, "resetkeys", "~target")
			} else if denied == "del-cleanup" {
				args = append(args, "-rename", "-del")
			} else {
				args = append(args, "-"+denied)
			}
			if err := admin.Do(ctx, args...).Err(); err != nil {
				t.Fatal(err)
			}
			client := redis.NewClient(&redis.Options{Addr: admin.Options().Addr, Username: "limited", Password: "acceptance-only", Protocol: 2, MaxRetries: -1})
			defer client.Close()
			written, err := backend.WriteExportedKey(ctx, client, config.ExportedKey{Key: "target", Type: "hash", Hash: map[string]string{"f": "new"}, TTL: -1}, true)
			if written || err == nil {
				t.Fatalf("ACL allowed %s: %v %v", denied, written, err)
			}
			code := backend.RedisErrorCode(err)
			t.Logf("code=%s error=%v", code, err)
			preflightDenied := denied == "eval" || denied == "stage-key"
			if preflightDenied && code != "permission_denied" || !preflightDenied && code != "partial_possible" {
				t.Fatalf("unsafe ACL error classification: %s", code)
			}
			if admin.Get(ctx, "target").Val() != "original" || admin.PTTL(ctx, "target").Val() <= 0 {
				t.Fatal("ACL failure damaged target or TTL")
			}
			keys := admin.Keys(ctx, "*:__literedis_import:*").Val()
			if denied == "del-cleanup" {
				if len(keys) != 1 || !strings.Contains(err.Error(), keys[0]) || admin.PTTL(ctx, keys[0]).Val() <= 0 {
					t.Fatal("cleanup leak not explicit/bounded")
				}
			} else if len(keys) != 0 {
				t.Fatalf("leaked stage %v", keys)
			}
			if len(keys) > 0 {
				admin.Del(ctx, keys...)
			}
		})
	}
}

// respFrame reads RESP frames verbatim, including RESP3 handshake maps. The proxy
// drops only the reply to a write after Redis has executed it, not before send.
func respFrame(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if len(line) < 3 {
		return nil, io.ErrUnexpectedEOF
	}
	if line[0] == '$' {
		n, e := strconv.Atoi(strings.TrimSpace(string(line[1:])))
		if e != nil {
			return nil, e
		}
		if n >= 0 {
			b := make([]byte, n+2)
			_, e = io.ReadFull(r, b)
			line = append(line, b...)
			return line, e
		}
	}
	if line[0] == '*' || line[0] == '%' || line[0] == '~' || line[0] == '>' {
		n, e := strconv.Atoi(strings.TrimSpace(string(line[1:])))
		if e != nil {
			return nil, e
		}
		if line[0] == '%' {
			n *= 2
		}
		for i := 0; i < n; i++ {
			b, e := respFrame(r)
			if e != nil {
				return nil, e
			}
			line = append(line, b...)
		}
	}
	return line, nil
}

func TestAcceptanceLostWriteResponse(t *testing.T) {
	admin := acceptanceRedis(t, os.Getenv("LITEREDIS_TEST_REDIS7"), false)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var writes atomic.Int32
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				up, err := net.Dial("tcp", admin.Options().Addr)
				if err != nil {
					return
				}
				defer up.Close()
				_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				_ = up.SetDeadline(time.Now().Add(10 * time.Second))
				r, u := bufio.NewReader(conn), bufio.NewReader(up)
				for {
					request, err := respFrame(r)
					if err != nil {
						return
					}
					if _, err = up.Write(request); err != nil {
						return
					}
					reply, err := respFrame(u)
					if err != nil {
						return
					}
					if strings.Contains(strings.ToLower(string(request)), "\r\nrpush\r\n") {
						writes.Add(1)
						return
					}
					if _, err = conn.Write(reply); err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-done; wg.Wait() })
	a := NewApp()
	a.ctx = context.Background()
	addr := listener.Addr().(*net.TCPAddr)
	if err := a.manager.Connect(config.ConnectionConfig{ID: "loss", Host: "127.0.0.1", Port: addr.Port, DB: 4}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.manager.DisconnectAll)
	result := a.RPush("loss", "queue", "once")
	if result.Success || result.Code != "outcome_unknown" || writes.Load() != 1 {
		t.Fatalf("lost response %+v writes=%d", result, writes.Load())
	}
	verify := redis.NewClient(&redis.Options{Addr: admin.Options().Addr, DB: 4, MaxRetries: -1})
	defer verify.Close()
	if n := verify.LLen(context.Background(), "queue").Val(); n != 1 {
		t.Fatalf("write duplicated/missing: %d", n)
	}
}

func TestAcceptanceCluster(t *testing.T) {
	binary := os.Getenv("LITEREDIS_TEST_REDIS7")
	nodes := []*redis.Client{acceptanceRedis(t, binary, true), acceptanceRedis(t, binary, true), acceptanceRedis(t, binary, true)}
	ctx := context.Background()
	for i, node := range nodes {
		slots := make([]int, 0, 5462)
		for s := i * 16384 / 3; s < (i+1)*16384/3; s++ {
			slots = append(slots, s)
		}
		if err := node.ClusterAddSlots(ctx, slots...).Err(); err != nil {
			t.Fatal(err)
		}
	}
	for _, node := range nodes[1:] {
		host, port, _ := net.SplitHostPort(node.Options().Addr)
		if err := nodes[0].ClusterMeet(ctx, host, port).Err(); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		ready := true
		for _, node := range nodes {
			info, _ := node.ClusterInfo(ctx).Result()
			ready = ready && strings.Contains(info, "cluster_state:ok")
		}
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cluster did not converge")
		}
		time.Sleep(100 * time.Millisecond)
	}
	cluster := redis.NewClusterClient(&redis.ClusterOptions{Addrs: []string{nodes[0].Options().Addr}, MaxRetries: -1, MaxRedirects: -1, Protocol: 2})
	defer cluster.Close()
	for _, key := range []string{"normal", "prefix:{tag}:key", "{}empty", "a{unclosed"} {
		if ok, err := backend.WriteExportedKey(ctx, cluster, config.ExportedKey{Key: key, Type: "string", String: "saved", TTL: 60}, true); err != nil || !ok {
			t.Fatalf("same-slot %q %v %v", key, ok, err)
		}
	}
	key := "normal"
	slot := int(nodes[0].ClusterKeySlot(ctx, key).Val())
	owner := 0
	for i := range nodes {
		if slot >= i*16384/3 && slot < (i+1)*16384/3 {
			owner = i
		}
	}
	target := (owner + 1) % 3
	id := nodes[target].Do(ctx, "CLUSTER", "MYID").Val()
	if err := nodes[owner].Do(ctx, "CLUSTER", "SETSLOT", slot, "MIGRATING", id).Err(); err != nil {
		t.Fatal(err)
	}
	if err := nodes[owner].Del(ctx, key).Err(); err != nil {
		t.Fatal(err)
	}
	err := cluster.Set(ctx, key, "must-not-follow-ASK", 0).Err()
	if backend.RedisErrorCode(err) != "cluster_redirect" || !strings.HasPrefix(err.Error(), "ASK ") {
		t.Fatalf("ASK followed/hidden: %v", err)
	}
	if err := nodes[owner].Do(ctx, "CLUSTER", "SETSLOT", slot, "STABLE").Err(); err != nil {
		t.Fatal(err)
	}
	// Move the empty slot on all masters after this client's topology is cached.
	for _, node := range nodes {
		if err := node.Do(ctx, "CLUSTER", "SETSLOT", slot, "NODE", id).Err(); err != nil {
			t.Fatal(err)
		}
	}
	err = cluster.Set(ctx, key, "must-not-follow-MOVED", 0).Err()
	if backend.RedisErrorCode(err) != "cluster_redirect" || !strings.HasPrefix(err.Error(), "MOVED ") {
		t.Fatalf("MOVED followed/hidden: %v", err)
	}
	if n := nodes[target].Exists(ctx, key).Val(); n != 0 {
		t.Fatal("redirected write executed")
	}
}
