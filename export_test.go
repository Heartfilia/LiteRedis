package main

import (
	"context"
	"errors"
	"testing"
)

func TestKeyExportSessionCancellationAndExclusivity(t *testing.T) {
	a := NewApp()
	a.ctx = context.Background()
	ctx, done, err := a.beginKeyExport()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.beginKeyExport(); err == nil {
		t.Fatal("concurrent exports accepted")
	}
	if !a.CancelKeyExport().Success || !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("export was not cancelled")
	}
	if _, _, err := a.beginKeyExport(); err == nil {
		t.Fatal("new export started before old task exited")
	}
	done()
	ctx, done, err = a.beginKeyExport()
	if err != nil || ctx.Err() != nil {
		t.Fatalf("cannot restart export: %v", err)
	}
	done()
}
