package redis

import "testing"

func TestIsReadOnlyCommandAllowed(t *testing.T) {
	allowed := []string{
		"GET user:1",
		"  mget a b  ",
		"HGETALL cart:1",
		"scan 0 match * count 10",
		"INFO memory",
		"CONFIG GET maxmemory",
		"ACL WHOAMI",
		"CLIENT LIST",
		"MEMORY USAGE user:1",
		"SLOWLOG GET 10",
		"COMMAND COUNT",
		"",
	}
	for _, command := range allowed {
		if !IsReadOnlyCommandAllowed(command) {
			t.Errorf("%q should be allowed in read-only mode", command)
		}
	}

	blocked := []string{
		"SET user:1 v",
		"DEL user:1",
		"flushall",
		"CONFIG SET maxmemory 100mb",
		"CONFIG REWRITE",
		"EVAL \"return 1\" 0",
		"HSET cart:1 a b",
		"RENAME a b",
		"EXPIRE user:1 100",
		"CLUSTER RESET",
		"CONFIG",
	}
	for _, command := range blocked {
		if IsReadOnlyCommandAllowed(command) {
			t.Errorf("%q should require confirmation in read-only mode", command)
		}
	}
}
