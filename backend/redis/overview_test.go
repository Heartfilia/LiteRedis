package redis

import (
	"context"
	"reflect"
	"testing"
)

func TestSplitCommandPreservesQuotedAndEscapedArguments(t *testing.T) {
	tests := []struct {
		input string
		want  []interface{}
	}{
		{input: `SET key ""`, want: []interface{}{"SET", "key", ""}},
		{input: `SET '' value`, want: []interface{}{"SET", "", "value"}},
		{input: `SET key "hello world"`, want: []interface{}{"SET", "key", "hello world"}},
		{input: `SET key hello\ world`, want: []interface{}{"SET", "key", "hello world"}},
		{input: `ECHO "a\"b"`, want: []interface{}{"ECHO", `a"b`}},
		{input: `ECHO """suffix"`, want: []interface{}{"ECHO", "suffix"}},
	}
	for _, test := range tests {
		got, err := splitCommand(test.input)
		if err != nil {
			t.Errorf("splitCommand(%q): %v", test.input, err)
			continue
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("splitCommand(%q) = %#v, want %#v", test.input, got, test.want)
		}
	}
}

func TestSplitCommandRejectsUnterminatedQuote(t *testing.T) {
	if _, err := splitCommand(`SET key "unterminated`); err == nil {
		t.Fatal("unterminated quote was accepted")
	}
}

func TestExecuteRedisCommandCanStoreEmptyString(t *testing.T) {
	ctx := context.Background()
	client := testRedisClient(t)
	result := ExecuteRedisCommand(ctx, client, `SET empty ""`)
	if !result.Success {
		t.Fatalf("SET empty string failed: %s", result.Error)
	}
	value, err := client.Get(ctx, "empty").Result()
	if err != nil || value != "" {
		t.Fatalf("stored value = %q, err=%v", value, err)
	}
}

func TestMergeClusterNodeInfosAggregates(t *testing.T) {
	nodes := []map[string]string{
		{
			"redis_version":               "7.2.0",
			"used_memory":                 "1000",
			"instantaneous_ops_per_sec":   "120",
			"connected_clients":           "5",
			"uptime_in_seconds":           "86400",
			"used_memory_human":           "1000B", // 应被忽略，不进入合并结果
			"role":                        "master",
		},
		{
			"redis_version":             "7.0.0", // 只取第一个节点
			"used_memory":               "2000",
			"instantaneous_ops_per_sec": "80",
			"connected_clients":         "3",
			"uptime_in_seconds":         "172800",
		},
	}

	got := mergeClusterNodeInfos(nodes)
	if got["role"] != "cluster" {
		t.Errorf("role = %q, want cluster", got["role"])
	}
	if got["redis_version"] != "7.2.0" {
		t.Errorf("redis_version = %q, want first node's 7.2.0", got["redis_version"])
	}
	if got["used_memory"] != "3000" {
		t.Errorf("used_memory = %q, want summed 3000", got["used_memory"])
	}
	if got["instantaneous_ops_per_sec"] != "200" {
		t.Errorf("ops = %q, want summed 200", got["instantaneous_ops_per_sec"])
	}
	if got["connected_clients"] != "8" {
		t.Errorf("clients = %q, want summed 8", got["connected_clients"])
	}
	if got["uptime_in_seconds"] != "172800" {
		t.Errorf("uptime = %q, want max 172800", got["uptime_in_seconds"])
	}
	if _, ok := got["used_memory_human"]; ok {
		t.Error("used_memory_human should not be set so the summary bytes are used")
	}
}

func TestMergeClusterNodeInfosHandlesMissingFields(t *testing.T) {
	got := mergeClusterNodeInfos([]map[string]string{{}})
	if got["role"] != "cluster" {
		t.Errorf("role = %q, want cluster", got["role"])
	}
	if got["used_memory"] != "0" {
		t.Errorf("used_memory = %q, want 0", got["used_memory"])
	}
}
