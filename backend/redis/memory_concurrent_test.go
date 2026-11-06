package redis

// 并发安全用例：集群内存体检下多个节点会同时写入累加器（go-redis 的
// ForEachMaster 为每个 master 起独立 goroutine），必须保证 map / 堆访问
// 有锁保护。配合 -race 运行。

import (
	"sync"
	"testing"
)

func TestMemoryAccumulatorConcurrentConsume(t *testing.T) {
	acc := newMemoryAccumulator()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				key := "p" + string(rune('a'+worker)) + ":key:" + string(rune('a'+i%26))
				acc.consume([]memoryKeySample{{
					key:     key,
					keyType: "string",
					size:    int64(worker*1000 + i),
					ttl:     -1,
				}})
				total := acc.addScanned(1)
				if total <= 0 {
					t.Errorf("scanned total should be positive, got %d", total)
				}
				acc.markTruncated()
				_ = acc.truncatedFlag()
				_ = acc.scannedTotal()
			}
		}(worker)
	}
	wg.Wait()

	report := acc.build()
	if report.ScannedKeys != 8*200 {
		t.Fatalf("scanned keys = %d, want %d", report.ScannedKeys, 8*200)
	}
	if len(report.Prefixes) == 0 || len(report.BigKeys) == 0 {
		t.Fatalf("expected aggregates, got prefixes=%d bigkeys=%d", len(report.Prefixes), len(report.BigKeys))
	}
	if !report.Truncated {
		t.Fatal("expected truncated flag to be set")
	}
}
