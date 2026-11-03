package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
)

type workflowInputService struct {
	WorkflowService
	inputs map[string]string
	calls  int
}

func (s *workflowInputService) Start(_ context.Context, id string, inputs map[string]string) (domain.WorkflowRun, error) {
	s.calls++
	s.inputs = inputs
	if id == "requires-input" {
		return domain.WorkflowRun{}, apierr.Invalid("INVALID_WORKFLOW_INPUTS", "Fill in every workflow input", map[string]any{"fields": map[string]string{"company": "Enter a value"}})
	}
	return domain.WorkflowRun{ID: "run-1", Inputs: inputs}, nil
}

func TestWorkflowStartInputBody(t *testing.T) {
	for _, tc := range []struct {
		name, body, id string
		status, calls  int
	}{
		{"bodyless", "", "plain", 201, 1},
		{"empty object", "{}", "plain", 201, 1},
		{"values", `{"inputs":{"company":"Acme"}}`, "plain", 201, 1},
		{"wrong type", `{"inputs":{"company":42}}`, "plain", 400, 0},
		{"unknown field", `{"extra":true}`, "plain", 400, 0},
		{"malformed", `{"inputs":`, "plain", 400, 0},
		{"missing required", "", "requires-input", 400, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &workflowInputService{}
			router := chi.NewRouter()
			(&WorkflowsController{Svc: svc}).Register(router)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/workflows/"+tc.id+"/runs", strings.NewReader(tc.body)))
			if rec.Code != tc.status || svc.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", rec.Code, svc.calls, rec.Body.String())
			}
			if tc.name == "values" && svc.inputs["company"] != "Acme" {
				t.Fatal("input was not forwarded")
			}
			if tc.id == "requires-input" {
				var problem envelope.APIError
				if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != "INVALID_WORKFLOW_INPUTS" || problem.Details["fields"] == nil {
					t.Fatalf("lost validation details: %+v", problem)
				}
			}
		})
	}
}
