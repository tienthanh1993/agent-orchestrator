package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestFailureRetryRetainsEarlierStagesAndRejectsStaleMarker(t *testing.T) {
	s, f, w := fixture(t)
	w.Stages[0].Approval = false
	if _, err := s.Save(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	r := startRun(t, s, w)
	writeStage(t, r)
	finish(f, r, domain.TurnStateCompleted)
	tick(t, s)
	tick(t, s)
	r = loadRun(t, s, r.ID)
	writeStage(t, r)
	finish(f, r, domain.TurnStateFailed)
	tick(t, s)
	if got := loadRun(t, s, r.ID); got.Status != "failed" || got.Stages[0].Status != "completed" {
		t.Fatalf("failure: %+v", got)
	}
	feedback := "Fix the offline lookup"
	if _, err := s.Act(context.Background(), r.ID, "retry", feedback); err != nil {
		t.Fatal(err)
	}
	tick(t, s)
	r = loadRun(t, s, r.ID)
	if r.Stages[1].Attempt != 2 || r.Stages[0].Status != "completed" || len(f.spawns) != 1 {
		t.Fatalf("retry: %+v", r)
	}
	if !strings.Contains(f.messages[2].Text, feedback) {
		t.Fatal("retry feedback missing")
	}
	if _, err := os.Stat(filepath.Join(r.Stages[0].ArtifactDir, "research.md")); err != nil {
		t.Fatal(err)
	}
	finish(f, r, domain.TurnStateCompleted)
	tick(t, s) // Old completion marker belongs to the failed attempt.
	if got := loadRun(t, s, r.ID); got.Status != "failed" {
		t.Fatal("stale marker advanced the retry")
	}
	if _, err := s.Act(context.Background(), r.ID, "retry", "Finish and save"); err != nil {
		t.Fatal(err)
	}
	tick(t, s)
	r = loadRun(t, s, r.ID)
	writeStage(t, r)
	finish(f, r, domain.TurnStateCompleted)
	tick(t, s)
	if got := loadRun(t, s, r.ID); got.Status != "completed" {
		t.Fatalf("retry completion: %+v", got)
	}
}

func TestCompletedTurnRequiresSavedHandoff(t *testing.T) {
	for _, broken := range []string{"no marker", "empty handoff", "large handoff", "rolled back"} {
		t.Run(broken, func(t *testing.T) {
			s, f, w := fixture(t)
			r := startRun(t, s, w)
			writeStage(t, r)
			dir := r.Stages[0].ArtifactDir
			var err error
			switch broken {
			case "no marker":
				err = os.Remove(filepath.Join(dir, "stage-complete.txt"))
			case "empty handoff":
				err = os.WriteFile(filepath.Join(dir, "handoff.md"), nil, 0600)
			case "large handoff":
				err = os.WriteFile(filepath.Join(dir, "handoff.md"), []byte(strings.Repeat("x", 32001)), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			finish(f, r, domain.TurnStateCompleted)
			if broken == "rolled back" {
				turn := f.turns[r.Stages[0].TurnID]
				now := s.now()
				turn.RolledBackAt = &now
				f.turns[turn.ID] = turn
			}
			tick(t, s)
			if got := loadRun(t, s, r.ID); got.Status != "failed" || got.CurrentStage != 0 {
				t.Fatalf("invalid completion: %+v", got)
			}
		})
	}
}

func TestRestartAfterDispatchUsesSameMessageIdentity(t *testing.T) {
	s, f, w := fixture(t)
	r, err := s.Start(context.Background(), w.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.failSave = true
	if err := s.Tick(context.Background(), s.now()); err == nil {
		t.Fatal("expected interrupted persistence")
	}
	r = loadRun(t, s, r.ID)
	if r.Stages[0].TurnID != "" || len(f.messages) != 1 {
		t.Fatalf("crash window: %+v", r)
	}
	tick(t, New(f, f, f, s.dataDir))
	r = loadRun(t, s, r.ID)
	if len(f.messages) != 1 || len(f.spawns) != 1 || r.Stages[0].TurnID == "" {
		t.Fatal("duplicated agent work after restart")
	}
}

func TestScheduleOccurrenceSurvivesCursorFailure(t *testing.T) {
	s, f, w := fixture(t)
	w.Enabled, w.RRule, w.Timezone = true, "FREQ=DAILY;BYHOUR=9;BYMINUTE=0", "UTC"
	w, err := s.Save(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}
	due := *w.NextRunAt
	s.now = func() time.Time { return due }
	f.failSchedule = true
	if err := s.Tick(context.Background(), due); err == nil {
		t.Fatal("expected cursor failure")
	}
	tick(t, s)
	runs, err := s.Runs(context.Background(), w.ID)
	if err != nil || len(runs) != 1 || len(f.spawns) != 1 {
		t.Fatalf("duplicated scheduled occurrence: %d runs, %d sessions, %v", len(runs), len(f.spawns), err)
	}
	w, _, err = f.GetWorkflow(context.Background(), w.ID)
	if err != nil || w.NextRunAt == nil || !w.NextRunAt.After(due) {
		t.Fatalf("cursor: %+v %v", w, err)
	}
	// Editing a reusable definition does not change a run's instructions.
	w.Task = "Another task"
	if _, err := s.Save(context.Background(), w); err != nil {
		t.Fatal(err)
	}
	if loadRun(t, s, runs[0].ID).Definition.Task == w.Task {
		t.Fatal("run snapshot changed")
	}
}

func TestShutdownDoesNotFailStage(t *testing.T) {
	s, f, w := fixture(t)
	r := startRun(t, s, w)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.sendErr = errors.New("shutdown")
	if err := s.fail(ctx, &r, f.sendErr); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got := loadRun(t, s, r.ID); got.Status != "running" {
		t.Fatal("shutdown persisted as failure")
	}
}
