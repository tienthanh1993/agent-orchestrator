package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite/gen"
)

// SaveWorkflow creates or replaces a reusable definition.
func (s *Store) SaveWorkflow(ctx context.Context, w domain.Workflow) error {
	b, err := json.Marshal(w)
	if err != nil {
		return err
	}
	return s.qw.SaveWorkflow(ctx, gen.SaveWorkflowParams{ID: w.ID, Body: string(b)})
}

// GetWorkflow reads one definition, reporting absence separately from failure.
func (s *Store) GetWorkflow(ctx context.Context, id string) (domain.Workflow, bool, error) {
	b, err := s.qr.GetWorkflow(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Workflow{}, false, nil
	}
	if err != nil {
		return domain.Workflow{}, false, err
	}
	var w domain.Workflow
	err = json.Unmarshal([]byte(b), &w)
	return w, true, err
}

// ListWorkflows returns all saved workflow definitions.
func (s *Store) ListWorkflows(ctx context.Context) ([]domain.Workflow, error) {
	rows, err := s.qr.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domain.Workflow, 0, len(rows))
	for _, b := range rows {
		var w domain.Workflow
		if err := json.Unmarshal([]byte(b), &w); err != nil {
			return nil, err
		}
		items = append(items, w)
	}
	return items, nil
}

// InsertWorkflowRun queues a run once per workflow and scheduled occurrence.
func (s *Store) InsertWorkflowRun(ctx context.Context, r domain.WorkflowRun) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	occurrence := r.ID
	if r.ScheduledFor != nil {
		occurrence = r.ScheduledFor.UTC().Format("2006-01-02T15:04:05.999999999Z")
	}
	return s.qw.InsertWorkflowRun(ctx, gen.InsertWorkflowRunParams{
		ID: r.ID, WorkflowID: r.WorkflowID, Occurrence: occurrence, Status: r.Status, Body: string(b),
	})
}

// SaveWorkflowRun persists progress and the status used by the runner query.
func (s *Store) SaveWorkflowRun(ctx context.Context, r domain.WorkflowRun) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.qw.SaveWorkflowRun(ctx, gen.SaveWorkflowRunParams{ID: r.ID, Status: r.Status, Body: string(b)})
}

// GetWorkflowRun reads a run snapshot and its durable progress.
func (s *Store) GetWorkflowRun(ctx context.Context, id string) (domain.WorkflowRun, bool, error) {
	b, err := s.qr.GetWorkflowRun(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.WorkflowRun{}, false, nil
	}
	if err != nil {
		return domain.WorkflowRun{}, false, err
	}
	var r domain.WorkflowRun
	err = json.Unmarshal([]byte(b), &r)
	return r, true, err
}

// ListWorkflowRuns returns recent runs for a definition, newest first.
func (s *Store) ListWorkflowRuns(ctx context.Context, id string) ([]domain.WorkflowRun, error) {
	rows, err := s.qr.ListWorkflowRuns(ctx, id)
	if err != nil {
		return nil, err
	}
	return decodeWorkflowRuns(rows)
}

// ListActiveWorkflowRuns returns pending and running work for the next tick.
func (s *Store) ListActiveWorkflowRuns(ctx context.Context) ([]domain.WorkflowRun, error) {
	rows, err := s.qr.ListActiveWorkflowRuns(ctx)
	if err != nil {
		return nil, err
	}
	return decodeWorkflowRuns(rows)
}

func decodeWorkflowRuns(rows []string) ([]domain.WorkflowRun, error) {
	items := make([]domain.WorkflowRun, 0, len(rows))
	for _, b := range rows {
		var r domain.WorkflowRun
		if err := json.Unmarshal([]byte(b), &r); err != nil {
			return nil, err
		}
		items = append(items, r)
	}
	return items, nil
}
