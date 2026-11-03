// Package workflow runs sequential agents with files as the handoff contract.
package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	"github.com/aoagents/agent-orchestrator/backend/internal/service/automation"
)

// Store persists workflow definitions, run progress and the shared session facts.
type Store interface {
	SaveWorkflow(context.Context, domain.Workflow) error
	GetWorkflow(context.Context, string) (domain.Workflow, bool, error)
	ListWorkflows(context.Context) ([]domain.Workflow, error)
	InsertWorkflowRun(context.Context, domain.WorkflowRun) error
	SaveWorkflowRun(context.Context, domain.WorkflowRun) error
	GetWorkflowRun(context.Context, string) (domain.WorkflowRun, bool, error)
	ListWorkflowRuns(context.Context, string) ([]domain.WorkflowRun, error)
	ListActiveWorkflowRuns(context.Context) ([]domain.WorkflowRun, error)
	GetProject(context.Context, string) (domain.ProjectRecord, bool, error)
	GetSession(context.Context, domain.SessionID) (domain.SessionRecord, bool, error)
	TurnByID(context.Context, string) (domain.ConversationTurn, error)
}

// Spawner creates the shared Chat session through the session service.
type Spawner interface {
	Spawn(context.Context, ports.SpawnConfig) (domain.Session, int, int, error)
}

// Chat switches agents and dispatches idempotent stage messages.
type Chat interface {
	SupportsChat(domain.AgentHarness) bool
	SwitchProviderAndSend(context.Context, domain.SessionID, domain.ProviderSwitchRequest, ports.ChatUserMessage) (domain.ConversationTurn, bool, error)
}

// Service coordinates sequential workflows using durable run progress.
type Service struct {
	store   Store
	spawner Spawner
	chat    Chat
	dataDir string
	mu      sync.Mutex
	now     func() time.Time
}

// New constructs a workflow service rooted in the daemon's artifact directory.
func New(store Store, spawner Spawner, chat Chat, dataDir string) *Service {
	return &Service{store: store, spawner: spawner, chat: chat, dataDir: dataDir, now: time.Now}
}

// List returns the reusable workflow definitions.
func (s *Service) List(ctx context.Context) ([]domain.Workflow, error) {
	return s.store.ListWorkflows(ctx)
}

// Save validates and persists a definition without changing existing runs.
func (s *Service) Save(ctx context.Context, w domain.Workflow) (domain.Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Name, w.Task = strings.TrimSpace(w.Name), strings.TrimSpace(w.Task)
	if w.Name == "" || len(w.Name) > 120 || w.Task == "" || len(w.Task) > 32000 || len(w.Stages) == 0 || len(w.Stages) > 30 {
		return w, apierr.Invalid("INVALID_WORKFLOW", "Provide a name, task and between 1 and 30 stages", nil)
	}
	for i := range w.Stages {
		stage := &w.Stages[i]
		stage.Name, stage.Instructions = strings.TrimSpace(stage.Name), strings.TrimSpace(stage.Instructions)
		if stage.Name == "" || len(stage.Name) > 120 || stage.Instructions == "" || len(stage.Instructions) > 32000 || !stage.Agent.IsKnown() || !s.chat.SupportsChat(stage.Agent) {
			return w, apierr.Invalid("INVALID_WORKFLOW_STAGE", fmt.Sprintf("Stage %d needs a name, a Chat-capable agent and instructions", i+1), nil)
		}
	}
	fields, err := placeholders(w.Task)
	if err != nil {
		return w, err
	}
	defaults := make(map[string]string)
	for _, field := range fields {
		if value, ok := w.InputDefaults[field.name]; ok {
			if len(value) > 32000 {
				return w, apierr.Invalid("INVALID_WORKFLOW_INPUTS", "Input defaults cannot exceed 32000 bytes", nil)
			}
			defaults[field.name] = value
		}
	}
	w.InputDefaults = defaults
	if w.Enabled && strings.TrimSpace(w.RRule) != "" {
		if _, _, err := resolveInputs(w.Task, defaults, nil); err != nil {
			return w, err
		}
	}
	if w.ProjectID != "" {
		project, found, err := s.store.GetProject(ctx, string(w.ProjectID))
		if err != nil {
			return w, err
		}
		if !found || !project.ArchivedAt.IsZero() {
			return w, apierr.NotFound("PROJECT_NOT_FOUND", "Unknown project")
		}
	}
	var old domain.Workflow
	if w.ID == "" {
		w.ID = "workflow-" + uuid.NewString()
	} else {
		var found bool
		var err error
		old, found, err = s.store.GetWorkflow(ctx, w.ID)
		if err != nil {
			return w, err
		}
		if !found {
			return w, apierr.NotFound("WORKFLOW_NOT_FOUND", "Unknown workflow")
		}
	}
	w.NextRunAt = nil
	if strings.TrimSpace(w.RRule) != "" {
		schedule, err := automation.CanonicalizeSchedule(automation.ScheduleInput{RRule: w.RRule, Timezone: w.Timezone}, s.now())
		if err != nil {
			return w, apierr.Invalid("INVALID_WORKFLOW_SCHEDULE", err.Error(), nil)
		}
		w.RRule, w.Timezone, w.NextRunAt = schedule.RRuleText, schedule.Timezone, &schedule.NextRunAt
		if old.RRule == w.RRule && old.Timezone == w.Timezone && old.Enabled == w.Enabled {
			w.NextRunAt = old.NextRunAt
		}
	} else {
		w.Enabled = false
	}
	return w, s.store.SaveWorkflow(ctx, w)
}

func newRun(w domain.Workflow, at time.Time, scheduled *time.Time) domain.WorkflowRun {
	r := domain.WorkflowRun{ID: "workflow-run-" + uuid.NewString(), WorkflowID: w.ID, Definition: w,
		ResolvedTask: new(string),
		Status:       "pending", CreatedAt: at.UTC(), ScheduledFor: scheduled, Stages: make([]domain.WorkflowStageRun, len(w.Stages))}
	for i := range r.Stages {
		r.Stages[i] = domain.WorkflowStageRun{Status: "pending", Attempt: 1, Files: []domain.SessionArtifactFile{}}
	}
	return r
}

// Start queues a manual run with an immutable snapshot of its definition.
func (s *Service) Start(ctx context.Context, id string, inputs map[string]string) (domain.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w, found, err := s.store.GetWorkflow(ctx, id)
	if err != nil {
		return domain.WorkflowRun{}, err
	}
	if !found {
		return domain.WorkflowRun{}, apierr.NotFound("WORKFLOW_NOT_FOUND", "Unknown workflow")
	}
	r := newRun(w, s.now(), nil)
	values, task, err := resolveInputs(w.Task, w.InputDefaults, inputs)
	if err != nil {
		return domain.WorkflowRun{}, err
	}
	r.Inputs, r.ResolvedTask = values, &task
	return r, s.store.InsertWorkflowRun(ctx, r)
}

// Runs returns run history with the current files in each stage directory.
func (s *Service) Runs(ctx context.Context, id string) ([]domain.WorkflowRun, error) {
	runs, err := s.store.ListWorkflowRuns(ctx, id)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		s.refreshFiles(ctx, &runs[i])
	}
	return runs, nil
}

// Act changes only the paused current stage; earlier completed work is retained.
func (s *Service) Act(ctx context.Context, id, action, feedback string) (domain.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, found, err := s.store.GetWorkflowRun(ctx, id)
	if err != nil {
		return r, err
	}
	if !found {
		return r, apierr.NotFound("WORKFLOW_RUN_NOT_FOUND", "Unknown workflow run")
	}
	if len(feedback) > 16000 {
		return r, apierr.Invalid("INVALID_WORKFLOW_FEEDBACK", "Feedback exceeds 16000 bytes", nil)
	}
	stage := &r.Stages[r.CurrentStage]
	switch {
	case action == "approve" && r.Status == "awaiting_approval":
		stage.Status = "completed"
		r.CurrentStage++
		r.Status = "running"
	case action == "retry" && r.Status == "failed":
		if r.ResolvedTask != nil && *r.ResolvedTask == "" {
			return r, apierr.Conflict("WORKFLOW_INPUTS_UNRESOLVED", "Correct the saved input defaults and start a new run", nil)
		}
		stage.Attempt++
		stage.Status, stage.TurnID, stage.Error, stage.Feedback = "pending", "", "", feedback
		r.Status = "running"
	default:
		return r, apierr.Conflict("WORKFLOW_ACTION_UNAVAILABLE", "This run cannot perform that action", nil)
	}
	return r, s.store.SaveWorkflowRun(ctx, r)
}
