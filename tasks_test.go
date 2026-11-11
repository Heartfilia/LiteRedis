package main

import (
	"context"
	"errors"
	"testing"
)

func TestOperationCancellationDoesNotReportSuccess(t *testing.T) {
	a := NewApp()
	run, err := a.beginOperation("copy", "source", 4, "dest", 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.beginOperation("copy", "source", 0, "", 0, 1); err == nil {
		t.Fatal("concurrent task allowed")
	}
	calls := 0
	result := run.batch([]string{"one", "two", "three"}, func(string) (bool, error, bool) {
		calls++
		run.cancel()
		return true, nil, true
	})
	if result.Success || calls != 1 || result.Affected != 1 || run.snapshot().State != "cancelled" {
		t.Fatalf("cancelled batch: %+v, calls=%d", result, calls)
	}
}

func TestOperationUnknownWriteStopsAndReadFailureDoesNot(t *testing.T) {
	a := NewApp()
	run, _ := a.beginOperation("copy", "source", 0, "dest", 0, 3)
	calls := 0
	result := run.batch([]string{"read", "write", "never"}, func(key string) (bool, error, bool) {
		calls++
		return false, errors.New("connection reset"), key == "write"
	})
	snapshot := run.snapshot()
	if result.Code != "outcome_unknown" || calls != 2 || snapshot.State != "outcome_unknown" || snapshot.Failures[0].Code != "read_failed" || snapshot.Failures[1].Code != "outcome_unknown" {
		t.Fatalf("unsafe task state: %+v", snapshot)
	}
	snapshot.Failures[0].Key = "changed"
	if run.snapshot().Failures[0].Key == "changed" {
		t.Fatal("snapshot exposes mutable state")
	}
}

func TestOperationHistoryAndFailureBounds(t *testing.T) {
	a := NewApp()
	for range 12 {
		run, _ := a.beginOperation("export", "source", 0, "", 0, 101)
		for range 101 {
			run.step("key", false, errors.New("NOPERM"), false)
		}
		run.finish(nil)
	}
	tasks := a.ListOperationTasks()
	if len(tasks) != 10 || len(tasks[0].Failures) != 100 || tasks[0].Failed != 101 || tasks[0].State != "failed" {
		t.Fatal("task limits not enforced")
	}
	run, _ := a.beginOperation("export", "source", 0, "", 0, 1)
	run.finish(context.DeadlineExceeded)
	if run.snapshot().State != "cancelled" {
		t.Fatal("timeout reported success")
	}
}
