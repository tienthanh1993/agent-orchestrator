package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	"github.com/aoagents/agent-orchestrator/backend/internal/service/automation"
	"github.com/aoagents/agent-orchestrator/backend/internal/sessionartifacts"
)

// Tick is serialized with definition edits and approval/retry. Durable prompt
// keys make replay safe if the daemon stops between sending and saving TurnID.
func (s *Service) Tick(ctx context.Context, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	definitions, err := s.store.ListWorkflows(ctx)
	if err != nil {
		return err
	}
	var problems []error
	for _, w := range definitions {
		if !w.Enabled || w.NextRunAt == nil || w.NextRunAt.After(at) {
			continue
		}
		r := newRun(w, at, w.NextRunAt)
		values, task, inputErr := resolveInputs(w.Task, w.InputDefaults, nil)
		r.Inputs, r.ResolvedTask = values, &task
		if inputErr != nil {
			r.Status, r.Stages[0].Status = "failed", "failed"
			r.Stages[0].Error = inputErr.Error() + ". Correct the saved input defaults and start a new run."
		}
		// The unique occurrence is saved before advancing the cursor. Replaying
		// this pair after a crash cannot create a second logical occurrence.
		if err := s.store.InsertWorkflowRun(ctx, r); err != nil {
			problems = append(problems, err)
			continue
		}
		next, err := automation.NextOccurrence(w.RRule, w.Timezone, at)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		w.NextRunAt = &next
		if err := s.store.SaveWorkflow(ctx, w); err != nil {
			problems = append(problems, err)
		}
	}
	runs, err := s.store.ListActiveWorkflowRuns(ctx)
	if err != nil {
		return errors.Join(append(problems, err)...)
	}
	for _, r := range runs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.advance(ctx, &r); err != nil {
			problems = append(problems, fmt.Errorf("workflow run %s: %w", r.ID, err))
		}
	}
	return errors.Join(problems...)
}

func (s *Service) fail(ctx context.Context, r *domain.WorkflowRun, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	} // shutdown is not a stage failure
	r.Status = "failed"
	stage := &r.Stages[r.CurrentStage]
	stage.Status, stage.Error = "failed", err.Error()
	s.refreshFiles(ctx, r)
	return s.store.SaveWorkflowRun(ctx, *r)
}

func (s *Service) advance(ctx context.Context, r *domain.WorkflowRun) error {
	if r.ResolvedTask != nil && *r.ResolvedTask == "" {
		return s.fail(ctx, r, fmt.Errorf("workflow inputs are unresolved; correct the saved defaults and start a new run"))
	}
	stage := &r.Stages[r.CurrentStage]
	definition := r.Definition.Stages[r.CurrentStage]
	if r.SessionID == "" {
		// A promptless Chat session gives us the final artifact path before any
		// agent work. Session creation itself has a durable idempotency key.
		session, _, _, err := s.spawner.Spawn(ctx, ports.SpawnConfig{
			ProjectID: r.Definition.ProjectID, Kind: domain.KindWorker, Harness: definition.Agent,
			RequestedMode: domain.SessionModeChat, DisplayName: r.Definition.Name,
			ClientRequestID: r.ID, ClientRequestHash: r.ID,
			AgentConfig:    ports.AgentConfig{Model: definition.Model, Effort: definition.Effort},
			EffortOverride: true,
		})
		if err != nil {
			return s.fail(ctx, r, fmt.Errorf("create workflow session: %w", err))
		}
		r.SessionID, r.Status = session.ID, "running"
		if err := s.store.SaveWorkflowRun(ctx, *r); err != nil {
			return err
		}
	}
	session, found, err := s.store.GetSession(ctx, r.SessionID)
	if err != nil {
		return err
	}
	if !found || session.IsTerminated {
		return s.fail(ctx, r, fmt.Errorf("workflow session is unavailable; restore it before retrying"))
	}
	if session.Mode != domain.SessionModeChat {
		return s.fail(ctx, r, fmt.Errorf("workflow requires its Chat session; switch back to Chat before retrying"))
	}
	rootDir := sessionartifacts.Dir(s.dataDir, r.SessionID)
	stage.ArtifactDir = filepath.Join(rootDir, stagePath(r.CurrentStage))
	if stage.TurnID == "" {
		if err := os.MkdirAll(rootDir, 0o700); err != nil {
			return s.fail(ctx, r, err)
		}
		root, err := os.OpenRoot(rootDir)
		if err != nil {
			return s.fail(ctx, r, err)
		}
		err = root.MkdirAll(stagePath(r.CurrentStage), 0o700)
		_ = root.Close()
		if err != nil {
			return s.fail(ctx, r, err)
		}
		stage.Status = "dispatching"
		if err := s.store.SaveWorkflowRun(ctx, *r); err != nil {
			return err
		}
		key := completionKey(r)
		turn, _, err := s.chat.SwitchProviderAndSend(ctx, r.SessionID, domain.ProviderSwitchRequest{
			TargetHarness: definition.Agent, Model: definition.Model, Effort: definition.Effort, IdempotencyKey: key,
		}, ports.ChatUserMessage{Text: stagePrompt(r, rootDir), ClientMessageID: key, Origin: domain.MessageOriginAutomation})
		if err != nil {
			return s.fail(ctx, r, fmt.Errorf("start stage: %w", err))
		}
		stage.TurnID, stage.Status = turn.ID, "running"
		return s.store.SaveWorkflowRun(ctx, *r)
	}
	turn, err := s.store.TurnByID(ctx, stage.TurnID)
	if err != nil {
		return err
	}
	switch turn.State {
	case domain.TurnStateQueued, domain.TurnStateRunning:
		return nil
	case domain.TurnStateCompleted:
		if turn.RolledBackAt != nil {
			return s.fail(ctx, r, fmt.Errorf("stage turn was rolled back; retry to continue"))
		}
		// A durable completed turn and an attempt-specific marker are both
		// required. Idle, unknown runtime probes and stale files prove nothing.
		handoff, err := readCompletion(rootDir, r)
		if err != nil {
			return s.fail(ctx, r, err)
		}
		stage.Handoff, stage.Status = handoff, "completed"
		s.refreshFiles(ctx, r)
		if r.CurrentStage == len(r.Stages)-1 {
			r.Status = "completed"
		} else if definition.Approval {
			stage.Status, r.Status = "awaiting_approval", "awaiting_approval"
		} else {
			r.CurrentStage++
		}
		return s.store.SaveWorkflowRun(ctx, *r)
	default:
		return s.fail(ctx, r, fmt.Errorf("stage turn %s: %s", turn.State, turn.ErrorMessage))
	}
}
