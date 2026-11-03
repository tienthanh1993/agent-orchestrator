package workflow

import (
	"context"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

func TestDefinitionValidation(t *testing.T) {
	for _, invalid := range []string{"name", "task", "stage", "agent", "project", "schedule"} {
		t.Run(invalid, func(t *testing.T) {
			s, _, w := fixture(t)
			switch invalid {
			case "name":
				w.Name = " "
			case "task":
				w.Task = " "
			case "stage":
				w.Stages = nil
			case "agent":
				w.Stages[0].Agent = domain.HarnessAider
			case "project":
				w.ProjectID = "missing"
			case "schedule":
				w.Enabled, w.RRule = true, "invalid"
			}
			if _, err := s.Save(context.Background(), w); err == nil {
				t.Fatalf("accepted invalid %s", invalid)
			}
		})
	}
}

func TestActionsCannotSkipRunningStages(t *testing.T) {
	s, _, w := fixture(t)
	r := startRun(t, s, w)
	for _, action := range []string{"approve", "retry", "skip"} {
		if _, err := s.Act(context.Background(), r.ID, action, ""); err == nil {
			t.Fatalf("accepted %s on running stage", action)
		}
	}
	if got := loadRun(t, s, r.ID); got.CurrentStage != 0 || got.Stages[0].Attempt != 1 {
		t.Fatalf("run changed: %+v", got)
	}
}
