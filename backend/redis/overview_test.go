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
