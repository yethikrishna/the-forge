package selftest

import (
	"context"
	"testing"
	"time"
)

func TestNewRunner(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	if r == nil {
		t.Fatal("expected non-nil runner")
	}
	if r.version != "test" {
		t.Errorf("expected version test, got %s", r.version)
	}
}

func TestRunnerDefaultChecks(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	if len(r.checks) == 0 {
		t.Error("expected default checks to be registered")
	}
}

func TestAddCheck(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	initial := len(r.checks)

	r.AddCheck(func(ctx context.Context) *Check {
		return &Check{
			ID:       "custom",
			Name:     "Custom Check",
			Category: CatAgent,
			Status:   StatusPass,
			Message:  "custom check passed",
		}
	})

	if len(r.checks) != initial+1 {
		t.Errorf("expected %d checks, got %d", initial+1, len(r.checks))
	}
}

func TestRunAllChecks(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	r.SetTimeout(60 * time.Second)

	report := r.Run(context.Background())
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if len(report.Checks) == 0 {
		t.Error("expected checks in report")
	}

	if report.Summary == nil {
		t.Fatal("expected summary in report")
	}

	if report.Summary.Total != len(report.Checks) {
		t.Errorf("summary total %d != checks count %d", report.Summary.Total, len(report.Checks))
	}

	// Verify total = pass + warn + fail + skip + timeout
	total := report.Summary.Pass + report.Summary.Warn + report.Summary.Fail +
		report.Summary.Skip + report.Summary.Timeout
	if total != report.Summary.Total {
		t.Errorf("status counts %d != total %d", total, report.Summary.Total)
	}
}

func TestCheckTimeout(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	r.SetTimeout(100 * time.Millisecond)

	// Add a check that takes too long
	r.AddCheck(func(ctx context.Context) *Check {
		time.Sleep(500 * time.Millisecond)
		return &Check{
			ID:     "slow-check",
			Status: StatusPass,
		}
	})

	report := r.Run(context.Background())

	found := false
	for _, c := range report.Checks {
		if c.Status == StatusTimeout {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one timeout check")
	}
}

func TestSetTimeout(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	r.SetTimeout(5 * time.Second)

	if r.timeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %s", r.timeout)
	}
}

func TestReportFields(t *testing.T) {
	r := NewRunner("v1.0.0", t.TempDir())
	report := r.Run(context.Background())

	if report.Version != "v1.0.0" {
		t.Errorf("expected version v1.0.0, got %s", report.Version)
	}
	if report.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if report.OS == "" {
		t.Error("expected non-empty OS")
	}
	if report.Arch == "" {
		t.Error("expected non-empty Arch")
	}
}

func TestStatusValues(t *testing.T) {
	statuses := []Status{StatusPass, StatusWarn, StatusFail, StatusSkip, StatusTimeout}
	expected := []string{"PASS", "WARN", "FAIL", "SKIP", "TIMEOUT"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}

func TestCategoryValues(t *testing.T) {
	categories := []Category{CatCore, CatRuntime, CatNetwork, CatStorage, CatSecurity, CatDependency, CatAgent, CatBuild}
	expected := []string{"core", "runtime", "network", "storage", "security", "dependency", "agent", "build"}

	for i, c := range categories {
		if string(c) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], c)
		}
	}
}

func TestComputeSummary(t *testing.T) {
	checks := []*Check{
		{Status: StatusPass, Category: CatCore},
		{Status: StatusPass, Category: CatCore},
		{Status: StatusWarn, Category: CatRuntime},
		{Status: StatusFail, Category: CatNetwork},
		{Status: StatusSkip, Category: CatStorage},
	}

	summary := computeSummary(checks, time.Second)

	if summary.Total != 5 {
		t.Errorf("expected total 5, got %d", summary.Total)
	}
	if summary.Pass != 2 {
		t.Errorf("expected 2 pass, got %d", summary.Pass)
	}
	if summary.Warn != 1 {
		t.Errorf("expected 1 warn, got %d", summary.Warn)
	}
	if summary.Fail != 1 {
		t.Errorf("expected 1 fail, got %d", summary.Fail)
	}
	if summary.Skip != 1 {
		t.Errorf("expected 1 skip, got %d", summary.Skip)
	}
}

func TestCheckSortedByCategory(t *testing.T) {
	r := NewRunner("test", t.TempDir())
	report := r.Run(context.Background())

	for i := 1; i < len(report.Checks); i++ {
		prev := report.Checks[i-1].Category
		curr := report.Checks[i].Category
		if prev > curr {
			t.Errorf("checks not sorted: %s > %s at index %d", prev, curr, i)
		}
	}
}
