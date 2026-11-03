package controllers

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apispec"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
)

// WorkflowService is the daemon boundary for reusable workflows and their runs.
type WorkflowService interface {
	List(context.Context) ([]domain.Workflow, error)
	Save(context.Context, domain.Workflow) (domain.Workflow, error)
	Start(context.Context, string, map[string]string) (domain.WorkflowRun, error)
	Runs(context.Context, string) ([]domain.WorkflowRun, error)
	Act(context.Context, string, string, string) (domain.WorkflowRun, error)
}

// WorkflowsController exposes workflow definitions and execution actions.
type WorkflowsController struct{ Svc WorkflowService }

// Register mounts the workflow API routes.
func (c *WorkflowsController) Register(r chi.Router) {
	r.Get("/workflows", c.list)
	r.Post("/workflows", c.save)
	r.Put("/workflows/{workflowId}", c.save)
	r.Post("/workflows/{workflowId}/runs", c.start)
	r.Get("/workflows/{workflowId}/runs", c.runs)
	r.Post("/workflow-runs/{runId}/actions", c.act)
}

func (c *WorkflowsController) available(w http.ResponseWriter, r *http.Request) bool {
	if c.Svc != nil {
		return true
	}
	apispec.NotImplemented(w, r, r.Method, r.URL.Path)
	return false
}

func (c *WorkflowsController) list(w http.ResponseWriter, r *http.Request) {
	if !c.available(w, r) {
		return
	}
	items, err := c.Svc.List(r.Context())
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, WorkflowsResponse{Workflows: items})
}

func (c *WorkflowsController) save(w http.ResponseWriter, r *http.Request) {
	if !c.available(w, r) {
		return
	}
	var req SaveWorkflowRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		envelope.WriteAPIError(w, r, 400, "bad_request", "INVALID_JSON", "Invalid workflow body", nil)
		return
	}
	item, err := c.Svc.Save(r.Context(), domain.Workflow{ID: chi.URLParam(r, "workflowId"), Name: req.Name,
		ProjectID: req.ProjectID, Task: req.Task, InputDefaults: req.InputDefaults, Stages: req.Stages, RRule: req.RRule, Timezone: req.Timezone, Enabled: req.Enabled})
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	envelope.WriteJSON(w, status, WorkflowEnvelope{Workflow: item})
}

func (c *WorkflowsController) start(w http.ResponseWriter, r *http.Request) {
	if !c.available(w, r) {
		return
	}
	var req StartWorkflowRequest
	if err := decodeJSONStrict(r, &req); err != nil && !errors.Is(err, io.EOF) {
		envelope.WriteAPIError(w, r, 400, "bad_request", "INVALID_JSON", "Invalid workflow inputs", nil)
		return
	}
	run, err := c.Svc.Start(r.Context(), chi.URLParam(r, "workflowId"), req.Inputs)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusCreated, WorkflowRunEnvelope{Run: run})
}

func (c *WorkflowsController) runs(w http.ResponseWriter, r *http.Request) {
	if !c.available(w, r) {
		return
	}
	runs, err := c.Svc.Runs(r.Context(), chi.URLParam(r, "workflowId"))
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, WorkflowRunsResponse{Runs: runs})
}

func (c *WorkflowsController) act(w http.ResponseWriter, r *http.Request) {
	if !c.available(w, r) {
		return
	}
	var req WorkflowActionRequest
	if err := decodeJSONStrict(r, &req); err != nil {
		envelope.WriteAPIError(w, r, 400, "bad_request", "INVALID_JSON", "Invalid action body", nil)
		return
	}
	run, err := c.Svc.Act(r.Context(), chi.URLParam(r, "runId"), req.Action, req.Feedback)
	if err != nil {
		envelope.WriteError(w, r, err)
		return
	}
	envelope.WriteJSON(w, http.StatusOK, WorkflowRunEnvelope{Run: run})
}
