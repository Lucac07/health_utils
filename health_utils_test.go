package health_utils

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestEmptyCheckerIsHealthy(t *testing.T) {
	c := NewChecker()
	report := c.Run(context.Background())
	if !report.Healthy() {
		t.Fatal("empty checker should be healthy")
	}
	if len(report.Results) != 0 {
		t.Fatalf("expected no results, got %d", len(report.Results))
	}
	if failed := report.Failed(); len(failed) != 0 {
		t.Fatalf("expected no failures, got %v", failed)
	}
}

func TestAllChecksPass(t *testing.T) {
	c := NewChecker()
	c.Add("db", func(ctx context.Context) error { return nil })
	c.Add("cache", func(ctx context.Context) error { return nil })

	report := c.Run(context.Background())
	if !report.Healthy() {
		t.Fatal("expected healthy report")
	}
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}
	if report.Results[0].Name != "db" || report.Results[1].Name != "cache" {
		t.Fatalf("results out of order: %+v", report.Results)
	}
}

func TestOneCheckFails(t *testing.T) {
	boom := errors.New("boom")
	c := NewChecker()
	c.Add("db", func(ctx context.Context) error { return nil })
	c.Add("cache", func(ctx context.Context) error { return boom })

	report := c.Run(context.Background())
	if report.Healthy() {
		t.Fatal("expected unhealthy report")
	}
	if got := report.Failed(); len(got) != 1 || got[0] != "cache" {
		t.Fatalf("expected [cache], got %v", got)
	}
	if report.Results[1].Err != boom {
		t.Fatalf("expected the original error, got %v", report.Results[1].Err)
	}
}

func TestResultsOrderMatchesRegistration(t *testing.T) {
	// Checks finish in an arbitrary order, but the report must not.
	var started sync.WaitGroup
	started.Add(3)
	release := make(chan struct{})
	block := func(ctx context.Context) error {
		started.Done()
		<-release
		return nil
	}

	c := NewChecker()
	c.Add("a", block)
	c.Add("b", block)
	c.Add("c", block)

	done := make(chan Report, 1)
	go func() { done <- c.Run(context.Background()) }()

	started.Wait()
	close(release)
	report := <-done

	want := []string{"a", "b", "c"}
	for i, name := range want {
		if report.Results[i].Name != name {
			t.Fatalf("result %d: want %q, got %q", i, name, report.Results[i].Name)
		}
	}
}

func TestNilCheckIsHealthy(t *testing.T) {
	c := NewChecker()
	c.Add("optional", nil)
	report := c.Run(context.Background())
	if !report.Healthy() {
		t.Fatal("nil check should be treated as healthy")
	}
	if len(report.Results) != 1 || report.Results[0].Err != nil {
		t.Fatalf("unexpected result: %+v", report.Results)
	}
}

func TestAddCheckedRejectsEmptyName(t *testing.T) {
	c := NewChecker()
	err := c.AddChecked("", func(ctx context.Context) error { return nil })
	if !errors.Is(err, ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
	if len(c.Run(context.Background()).Results) != 0 {
		t.Fatal("rejected check should not be registered")
	}
}

func TestAddCheckedAcceptsName(t *testing.T) {
	c := NewChecker()
	if err := c.AddChecked("db", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report := c.Run(context.Background())
	if len(report.Results) != 1 || report.Results[0].Name != "db" {
		t.Fatalf("unexpected report: %+v", report.Results)
	}
}

func TestDuplicateNamesAreReportedPerRegistration(t *testing.T) {
	boom := errors.New("boom")
	c := NewChecker()
	c.Add("db", func(ctx context.Context) error { return nil })
	c.Add("db", func(ctx context.Context) error { return boom })

	report := c.Run(context.Background())
	if len(report.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(report.Results))
	}
	failed := report.Failed()
	if len(failed) != 1 || failed[0] != "db" {
		t.Fatalf("expected one failed db, got %v", failed)
	}
}

func TestCancelledContextIsPassedToChecks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewChecker()
	c.Add("ctx", func(ctx context.Context) error {
		return ctx.Err()
	})

	report := c.Run(ctx)
	if report.Healthy() {
		t.Fatal("expected failure from cancelled context")
	}
	if !errors.Is(report.Results[0].Err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", report.Results[0].Err)
	}
}
