package redis

import (
	"context"
	"errors"
	"math"
	"net"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// 注入 INFO / MEMORY 响应，其他命令仍走真实 miniredis 连接。
type memoryReportHook struct {
	process func(goredis.Cmder) bool
}

func (h memoryReportHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }
func (h memoryReportHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		if h.process(cmd) {
			return cmd.Err()
		}
		return next(ctx, cmd)
	}
}
func (h memoryReportHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		var pending []goredis.Cmder
		var firstErr error
		for _, cmd := range cmds {
			if h.process(cmd) {
				if firstErr == nil {
					firstErr = cmd.Err()
				}
			} else {
				pending = append(pending, cmd)
			}
		}
		if len(pending) > 0 {
			if err := next(ctx, pending); err != nil {
				return err
			}
		}
		return firstErr
	}
}

func TestScanMemoryShare(t *testing.T) {
	for _, test := range []struct {
		name, pattern, info, fallback                                                          string
		limit                                                                                  int64
		partial, unavailable, infoDenied, sizeFailure, typeFailure, countDenied, countMismatch bool
	}{
		{name: "whole DB", pattern: "*"},
		{name: "filtered", pattern: "user:*", partial: true},
		{name: "truncated", pattern: "*", limit: 1, partial: true},
		{name: "INFO denied", pattern: "*", infoDenied: true, unavailable: true},
		{name: "invalid INFO", pattern: "*", info: "used_memory:nope\r\n", unavailable: true},
		{name: "zero denominator", pattern: "*", info: "used_memory:0\r\n", unavailable: true},
		{name: "inconsistent samples", pattern: "*", info: "used_memory:1\r\n", unavailable: true},
		{name: "length fallback", pattern: "*", fallback: "length", unavailable: true},
		{name: "debug fallback", pattern: "*", fallback: "debug", unavailable: true},
		{name: "partial memory failure", pattern: "*", sizeFailure: true, partial: true},
		{name: "partial type failure", pattern: "*", typeFailure: true, partial: true},
		{name: "DBSIZE denied", pattern: "*", countDenied: true, partial: true},
		{name: "changing DB size", pattern: "*", countMismatch: true, partial: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			client := testRedisClient(t)
			seedMemoryFixtures(t, ctx, client)
			client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
				switch cmd.Name() {
				case "dbsize":
					if test.countDenied {
						cmd.SetErr(errors.New("NOPERM DBSIZE denied"))
						return true
					}
					if test.countMismatch {
						cmd.(*goredis.IntCmd).SetVal(10)
						return true
					}
				case "info":
					if test.infoDenied {
						cmd.SetErr(errors.New("NOPERM INFO denied"))
					} else {
						info := test.info
						if info == "" {
							info = "# Memory\r\nused_memory:10000\r\n"
						}
						cmd.(*goredis.StringCmd).SetVal(info)
					}
					return true
				case "memory":
					if test.fallback != "" || (test.sizeFailure && cmd.Args()[2] == "plain") {
						cmd.SetErr(errors.New("NOPERM MEMORY denied"))
					} else {
						cmd.(*goredis.IntCmd).SetVal(100)
					}
					return true
				case "debug":
					if test.fallback == "debug" {
						cmd.(*goredis.Cmd).SetVal("serializedlength:20")
						return true
					}
				case "type":
					if test.typeFailure && cmd.Args()[1] == "plain" {
						cmd.SetErr(errors.New("NOPERM TYPE denied"))
						return true
					}
				}
				return false
			}})
			report, err := ScanMemory(ctx, client, test.pattern, test.limit, nil)
			if err != nil {
				t.Fatal(err)
			}
			if report.MemorySharePartial != test.partial {
				t.Fatalf("partial = %v, report=%+v", report.MemorySharePartial, report)
			}
			if test.unavailable {
				if report.MemorySharePercent != nil {
					t.Fatal("unavailable share displayed as a number")
				}
				return
			}
			if report.RedisUsedMemoryBytes != 10000 || report.MemorySharePercent == nil {
				t.Fatalf("missing share: %+v", report)
			}
			want := float64(report.TotalBytes) / 10000 * 100
			if math.Abs(*report.MemorySharePercent-want) > 0.00001 {
				t.Fatalf("share = %v, want %v", *report.MemorySharePercent, want)
			}
		})
	}
}

func TestEmptyDBHasZeroMemoryShare(t *testing.T) {
	client := testRedisClient(t)
	client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
		if cmd.Name() == "info" {
			cmd.(*goredis.StringCmd).SetVal("used_memory:10000\r\n")
			return true
		}
		return false
	}})
	report, err := ScanMemory(context.Background(), client, "*", 100, nil)
	if err != nil || report.MemorySharePercent == nil || *report.MemorySharePercent != 0 || report.MemorySharePartial {
		t.Fatalf("empty DB: %+v %v", report, err)
	}
}

func TestMemoryScanDeduplicatesKeys(t *testing.T) {
	client := testRedisClient(t)
	ctx := context.Background()
	seedMemoryFixtures(t, ctx, client)
	page := 0
	client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
		if cmd.Name() != "scan" {
			return false
		}
		page++
		if page == 1 {
			cmd.(*goredis.ScanCmd).SetVal([]string{"user:1", "user:1"}, 1)
		} else {
			cmd.(*goredis.ScanCmd).SetVal([]string{"user:1", "user:2"}, 0)
		}
		return true
	}})
	acc := newMemoryAccumulator()
	if _, err := scanMemoryNode(ctx, client, "*", 100, sizeModeMemory, acc, nil); err != nil {
		t.Fatal(err)
	}
	report := acc.build()
	if report.ScannedKeys != 2 || len(report.BigKeys) != 2 {
		t.Fatalf("duplicate keys counted: %+v", report)
	}
}

func TestMemoryScanLimitReservationIsGlobal(t *testing.T) {
	acc := newMemoryAccumulator()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); acc.addScanned(acc.reserveKeys(200, 250)) }()
	}
	wg.Wait()
	if acc.scannedTotal() != 250 || !acc.truncatedFlag() {
		t.Fatalf("global scan limit exceeded: %d", acc.scannedTotal())
	}
}

func TestClusterMemoryShareUsesOnlyAllMasters(t *testing.T) {
	ctx := context.Background()
	first, second, replica := miniredis.RunT(t), miniredis.RunT(t), miniredis.RunT(t)
	first.Set("a", "one")
	second.Set("b", "two")
	replica.Set("replicated", "not counted")
	newCluster := func(denySecond bool) *goredis.ClusterClient {
		return goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs: []string{first.Addr(), second.Addr()},
			ClusterSlots: func(context.Context) ([]goredis.ClusterSlot, error) {
				return []goredis.ClusterSlot{
					{Start: 0, End: 8191, Nodes: []goredis.ClusterNode{{Addr: first.Addr()}, {Addr: replica.Addr()}}},
					{Start: 8192, End: 16383, Nodes: []goredis.ClusterNode{{Addr: second.Addr()}}},
				}, nil
			},
			NewClient: func(opts *goredis.Options) *goredis.Client {
				_, nodePort, _ := net.SplitHostPort(opts.Addr)
				_, secondPort, _ := net.SplitHostPort(second.Addr())
				client := goredis.NewClient(opts)
				client.AddHook(memoryReportHook{process: func(cmd goredis.Cmder) bool {
					switch cmd.Name() {
					case "info":
						if denySecond && nodePort == secondPort {
							cmd.SetErr(errors.New("NOPERM INFO denied"))
						} else {
							cmd.(*goredis.StringCmd).SetVal("used_memory:10000\r\n")
						}
						return true
					case "memory":
						cmd.(*goredis.IntCmd).SetVal(100)
						return true
					}
					return false
				}})
				return client
			},
		})
	}
	cluster := newCluster(false)
	defer cluster.Close()
	report, err := ScanMemory(ctx, cluster, "*", 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.RedisUsedMemoryBytes != 20000 || report.ScannedKeys != 2 || report.MemorySharePercent == nil || *report.MemorySharePercent != 1 || report.MemorySharePartial {
		t.Fatalf("cluster report = %+v", report)
	}
	denied := newCluster(true)
	defer denied.Close()
	report, err = ScanMemory(ctx, denied, "*", 100, nil)
	if err != nil || report.RedisUsedMemoryBytes != 0 || report.MemorySharePercent != nil {
		t.Fatalf("partial master denominator exposed: %+v %v", report, err)
	}
}
