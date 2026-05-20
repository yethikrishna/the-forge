package schedule_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/forge/sword/internal/schedule"
)

func TestAdd(t *testing.T) {
	s := schedule.NewScheduler("", func(_ context.Context, _ *schedule.Schedule) (string, error) {
		return "ok", nil
	})

	sched, err := s.Add("test", "@hourly", "agent-1", "check-email")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if sched.ID == "" {
		t.Error("should have ID")
	}
	if sched.Name != "test" {
		t.Errorf("expected 'test', got %s", sched.Name)
	}
	if !sched.Enabled {
		t.Error("should be enabled by default")
	}
	if sched.NextRun.IsZero() {
		t.Error("should have next run time")
	}
}

func TestGet(t *testing.T) {
	s := schedule.NewScheduler("", func(_ context.Context, _ *schedule.Schedule) (string, error) {
		return "ok", nil
	})

	sched, _ := s.Add("test", "@hourly", "agent-1", "check-email")
	retrieved, err := s.Get(sched.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if retrieved.Name != "test" {
		t.Errorf("expected 'test', got %s", retrieved.Name)
	}
}

func TestGetNotFound(t *testing.T) {
	s := schedule.NewScheduler("", nil)
	_, err := s.Get("nonexistent")
	if err == nil {
		t.Error("should error for nonexistent")
	}
}

func TestList(t *testing.T) {
	s := schedule.NewScheduler("", nil)

	s.Add("first", "@daily", "agent-1", "task1")
	s.Add("second", "@hourly", "agent-2", "task2")

	list := s.List()
	if len(list) != 2 {
		t.Errorf("expected 2, got %d", len(list))
	}
}

func TestDelete(t *testing.T) {
	s := schedule.NewScheduler("", nil)

	sched, _ := s.Add("test", "@hourly", "agent-1", "task")
	err := s.Delete(sched.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = s.Get(sched.ID)
	if err == nil {
		t.Error("should be deleted")
	}
}

func TestEnableDisable(t *testing.T) {
	s := schedule.NewScheduler("", nil)

	sched, _ := s.Add("test", "@hourly", "agent-1", "task")

	s.Disable(sched.ID)
	retrieved, _ := s.Get(sched.ID)
	if retrieved.Enabled {
		t.Error("should be disabled")
	}

	s.Enable(sched.ID)
	retrieved, _ = s.Get(sched.ID)
	if !retrieved.Enabled {
		t.Error("should be enabled")
	}
}

func TestRunNow(t *testing.T) {
	var runCount atomic.Int32

	s := schedule.NewScheduler("", func(_ context.Context, sched *schedule.Schedule) (string, error) {
		runCount.Add(1)
		return "result for " + sched.Task, nil
	})

	sched, _ := s.Add("test", "@daily", "agent-1", "check-mail")

	log, err := s.RunNow(context.Background(), sched.ID)
	if err != nil {
		t.Fatalf("run now: %v", err)
	}

	if log.Status != "success" {
		t.Errorf("expected success, got %s", log.Status)
	}
	if runCount.Load() != 1 {
		t.Errorf("expected 1 run, got %d", runCount.Load())
	}

	// Check run count updated
	retrieved, _ := s.Get(sched.ID)
	if retrieved.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", retrieved.RunCount)
	}
}

func TestLogs(t *testing.T) {
	s := schedule.NewScheduler("", func(_ context.Context, _ *schedule.Schedule) (string, error) {
		return "ok", nil
	})

	sched, _ := s.Add("test", "@daily", "agent-1", "task")
	s.RunNow(context.Background(), sched.ID)
	s.RunNow(context.Background(), sched.ID)

	logs := s.Logs(10)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestParseCronHourly(t *testing.T) {
	next, err := schedule.ParseCron("@hourly")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if next.IsZero() {
		t.Error("should have next time")
	}
}

func TestParseCronDaily(t *testing.T) {
	next, err := schedule.ParseCron("@daily")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if next.IsZero() {
		t.Error("should have next time")
	}
}

func TestParseCronEvery(t *testing.T) {
	next, err := schedule.ParseCron("@every 5m")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if next.IsZero() {
		t.Error("should have next time")
	}
}

func TestParseCronInvalid(t *testing.T) {
	_, err := schedule.ParseCron("invalid")
	if err == nil {
		t.Error("should error for invalid cron")
	}
}

func TestParseCron5Field(t *testing.T) {
	next, err := schedule.ParseCron("0 9 * * *")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if next.IsZero() {
		t.Error("should have next time")
	}
}

func TestWithArgs(t *testing.T) {
	s := schedule.NewScheduler("", nil)

	sched, _ := s.Add("test", "@hourly", "agent-1", "task",
		schedule.WithArgs(map[string]string{"key": "value"}),
	)

	if sched.Args["key"] != "value" {
		t.Error("should have args")
	}
}

func TestWithTags(t *testing.T) {
	s := schedule.NewScheduler("", nil)

	sched, _ := s.Add("test", "@hourly", "agent-1", "task",
		schedule.WithTags("production", "critical"),
	)

	if len(sched.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(sched.Tags))
	}
}

func TestFormatSchedule(t *testing.T) {
	sched := &schedule.Schedule{
		ID:      "test-1",
		Name:    "Daily Check",
		Cron:    "@daily",
		Agent:   "agent-1",
		Task:    "check-email",
		Enabled: true,
	}

	formatted := schedule.FormatSchedule(sched)
	if formatted == "" {
		t.Error("should not be empty")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/schedule.json"

	s := schedule.NewScheduler(path, nil)
	s.Add("persist-test", "@daily", "agent-1", "task")

	// Reload
	s2 := schedule.NewScheduler(path, nil)
	list := s2.List()
	if len(list) != 1 {
		t.Errorf("expected 1 after reload, got %d", len(list))
	}
}
