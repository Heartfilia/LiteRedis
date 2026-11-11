package main

import (
	"context"
	"net"
	"strconv"
	"testing"

	"LiteRedis/backend/config"
	"github.com/alicebob/miniredis/v2"
)

func TestCopyKeysToConnectionHonorsTargetProtection(t *testing.T) {
	srv := miniredis.RunT(t)
	host, portText, _ := net.SplitHostPort(srv.Addr())
	port, _ := strconv.Atoi(portText)
	a := NewApp()
	a.ctx = context.Background()
	t.Cleanup(a.manager.DisconnectAll)
	if err := a.manager.Connect(config.ConnectionConfig{ID: "protected", Host: host, Port: port, ReadOnly: true}); err != nil {
		t.Fatal(err)
	}
	result, err := a.CopyKeysToConnection("source", "protected", []string{"k"}, true)
	if err != nil || result.Success || result.Message != errReadOnly.Error() {
		t.Fatalf("protected target not rejected: %+v %v", result, err)
	}
}
