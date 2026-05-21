package ambition

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCreateGoal(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))

	deadline := time.Now().Add(30 * 24 * time.Hour)
	g, err := e.CreateGoal("Ship v1.0", "Launch first version", "eng-div", PriorityCritical, &deadline, "100 paying users")
	if err != nil {
		t.Fatal(err)
	}
	if g.Status != GoalDraft {
		t.Errorf("expected draft, got %s", g.Status)
	}
	if g.SuccessMetric != "100 paying users" {
		t.Error("success metric mismatch")
	}

	got, err := e.GetGoal(g.ID)
	if err != nil || got.Title != "Ship v1.0" {
		t.Error("goal retrieval failed")
	}
}

func TestDecompose(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))

	parent, _ := e.CreateGoal("Ship v1.0", "", "org", PriorityHigh, nil, "")

	subs := []struct {
		Title, Description, Owner string
		Priority                  Priority
		TargetDate                *time.Time
	}{
		{"Backend API", "Build REST API", "eng-div", PriorityHigh, nil},
		{"Frontend UI", "Build web UI", "eng-div", PriorityNormal, nil},
		{"Launch Marketing", "Pre-launch campaign", "mkt-div", PriorityNormal, nil},
	}

	subGoals, err := e.Decompose(parent.ID, subs)
	if err != nil {
		t.Fatal(err)
	}
	if len(subGoals) != 3 {
		t.Fatalf("expected 3 sub-goals, got %d", len(subGoals))
	}

	parent, _ = e.GetGoal(parent.ID)
	if len(parent.SubGoals) != 3 {
		t.Error("parent should reference sub-goals")
	}
	if parent.Status != GoalPlanned {
		t.Errorf("parent should be planned after decomposition, got %s", parent.Status)
	}
}

func TestMilestones(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))
	g, _ := e.CreateGoal("Project X", "", "org", PriorityNormal, nil, "")

	due := time.Now().Add(7 * 24 * time.Hour)
	ms, err := e.AddMilestone(g.ID, "Alpha Release", "Internal alpha", &due)
	if err != nil {
		t.Fatal(err)
	}
	if ms.Status != "pending" {
		t.Errorf("expected pending milestone, got %s", ms.Status)
	}

	err = e.CompleteMilestone(ms.ID)
	if err != nil {
		t.Fatal(err)
	}
	ms, _ = e.milestones[ms.ID]
	if ms.Status != "completed" {
		t.Errorf("expected completed, got %s", ms.Status)
	}
}

func TestTasksAndProgress(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))
	g, _ := e.CreateGoal("Goal X", "", "org", PriorityNormal, nil, "")

	// Add tasks
	t1, _ := e.AddTask(g.ID, "", "Task 1", "Do thing 1", "a1", "eng", PriorityNormal, nil, "1h")
	t2, _ := e.AddTask(g.ID, "", "Task 2", "Do thing 2", "a2", "eng", PriorityNormal, nil, "2h")
	t3, _ := e.AddTask(g.ID, "", "Task 3", "Do thing 3", "a1", "eng", PriorityNormal, []string{t1.ID}, "1h")

	// Activate goal
	e.ActivateGoal(g.ID)

	// Start and complete tasks
	e.StartTask(t1.ID)
	e.CompleteTask(t1.ID)

	g, _ = e.GetGoal(g.ID)
	if g.Progress < 33 || g.Progress > 34 {
		t.Errorf("expected ~33%% progress, got %f", g.Progress)
	}

	// t3 depends on t1 (now complete), should be startable
	err := e.StartTask(t3.ID)
	if err != nil {
		t.Errorf("t3 should be startable after t1 completes: %v", err)
	}

	e.CompleteTask(t2.ID)
	e.CompleteTask(t3.ID)

	g, _ = e.GetGoal(g.ID)
	if g.Status != GoalCompleted {
		t.Errorf("goal should be completed when all tasks done, got %s", g.Status)
	}
	if g.Progress != 100 {
		t.Errorf("expected 100%% progress, got %f", g.Progress)
	}
}

func TestTaskDependencies(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))
	g, _ := e.CreateGoal("Goal", "", "org", PriorityNormal, nil, "")

	t1, _ := e.AddTask(g.ID, "", "First", "", "a1", "", PriorityNormal, nil, "")
	t2, _ := e.AddTask(g.ID, "", "Second", "", "a1", "", PriorityNormal, []string{t1.ID}, "")

	// t2 depends on t1 — should not be startable
	err := e.StartTask(t2.ID)
	if err == nil {
		t.Error("should not be able to start task with incomplete dependency")
	}

	e.CompleteTask(t1.ID)
	err = e.StartTask(t2.ID)
	if err != nil {
		t.Errorf("should be startable after dependency completes: %v", err)
	}
}

func TestBlockTask(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))
	g, _ := e.CreateGoal("Goal", "", "org", PriorityNormal, nil, "")

	task, _ := e.AddTask(g.ID, "", "Blocked task", "", "a1", "", PriorityNormal, nil, "")
	e.BlockTask(task.ID, "waiting on external API")

	task, _ = e.tasks[task.ID]
	if task.Status != TaskBlocked {
		t.Error("task should be blocked")
	}
	if task.BlockReason != "waiting on external API" {
		t.Error("block reason mismatch")
	}
}

func TestPursuitReport(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))
	deadline := time.Now().Add(14 * 24 * time.Hour)
	g, _ := e.CreateGoal("Big Goal", "The big one", "org", PriorityHigh, &deadline, "ship it")

	e.ActivateGoal(g.ID)
	e.AddTask(g.ID, "", "Task 1", "", "a1", "", PriorityNormal, nil, "")
	e.AddTask(g.ID, "", "Task 2", "", "a2", "", PriorityNormal, nil, "")

	tasks := e.ListTasks(g.ID, "")
	e.CompleteTask(tasks[0].ID)
	e.BlockTask(tasks[1].ID, "waiting for design")

	report, err := e.PursuitReport(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalTasks != 2 {
		t.Errorf("expected 2 tasks, got %d", report.TotalTasks)
	}
	if report.DoneTasks != 1 {
		t.Errorf("expected 1 done, got %d", report.DoneTasks)
	}
	if len(report.Blockers) != 1 {
		t.Errorf("expected 1 blocker, got %d", len(report.Blockers))
	}
}

func TestCancelGoal(t *testing.T) {
	e := NewEngine(filepath.Join(t.TempDir(), "amb.json"))
	parent, _ := e.CreateGoal("Parent", "", "org", PriorityNormal, nil, "")

	subs, _ := e.Decompose(parent.ID, []struct {
		Title, Description, Owner string
		Priority                  Priority
		TargetDate                *time.Time
	}{
		{"Sub1", "", "eng", PriorityNormal, nil},
		{"Sub2", "", "eng", PriorityNormal, nil},
	})

	e.ActivateGoal(subs[0].ID)
	e.CancelGoal(parent.ID)

	parent, _ = e.GetGoal(parent.ID)
	if parent.Status != GoalCancelled {
		t.Error("parent should be cancelled")
	}

	// Sub-goals should also be cancelled
	for _, sgID := range parent.SubGoals {
		sg, _ := e.GetGoal(sgID)
		if sg.Status != GoalCancelled {
			t.Errorf("sub-goal %s should be cancelled, got %s", sgID, sg.Status)
		}
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "amb.json")

	e1 := NewEngine(path)
	g, _ := e1.CreateGoal("Persist Test", "", "org", PriorityNormal, nil, "")
	e1.AddTask(g.ID, "", "Task", "", "a1", "", PriorityNormal, nil, "")

	e2 := NewEngine(path)
	if len(e2.goals) != 1 {
		t.Errorf("expected 1 loaded goal, got %d", len(e2.goals))
	}
	if len(e2.tasks) != 1 {
		t.Errorf("expected 1 loaded task, got %d", len(e2.tasks))
	}
}
