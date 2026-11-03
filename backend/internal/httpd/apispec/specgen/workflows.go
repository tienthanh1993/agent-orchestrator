package specgen

import (
	"net/http"

	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/controllers"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/envelope"
)

func workflowOperations() []operation {
	id := []any{controllers.WorkflowIDParam{}}
	errs := []respUnit{{400, envelope.APIError{}}, {404, envelope.APIError{}}, {409, envelope.APIError{}}, {500, envelope.APIError{}}, {501, envelope.APIError{}}}
	return []operation{
		{method: http.MethodGet, path: "/api/v1/workflows", id: "listWorkflows", tag: "workflows", summary: "List reusable workflows", resps: append([]respUnit{{200, controllers.WorkflowsResponse{}}}, errs...)},
		{method: http.MethodPost, path: "/api/v1/workflows", id: "createWorkflow", tag: "workflows", summary: "Create an artifact workflow", reqBody: controllers.SaveWorkflowRequest{}, resps: append([]respUnit{{201, controllers.WorkflowEnvelope{}}}, errs...)},
		{method: http.MethodPut, path: "/api/v1/workflows/{workflowId}", id: "updateWorkflow", tag: "workflows", summary: "Update a reusable workflow", pathParams: id, reqBody: controllers.SaveWorkflowRequest{}, resps: append([]respUnit{{200, controllers.WorkflowEnvelope{}}}, errs...)},
		{method: http.MethodPost, path: "/api/v1/workflows/{workflowId}/runs", id: "startWorkflow", tag: "workflows", summary: "Run a workflow", pathParams: id, reqBody: controllers.StartWorkflowRequest{}, optionalReqBody: true, resps: append([]respUnit{{201, controllers.WorkflowRunEnvelope{}}}, errs...)},
		{method: http.MethodGet, path: "/api/v1/workflows/{workflowId}/runs", id: "workflowRuns", tag: "workflows", summary: "List workflow runs and artifacts", pathParams: id, resps: append([]respUnit{{200, controllers.WorkflowRunsResponse{}}}, errs...)},
		{method: http.MethodPost, path: "/api/v1/workflow-runs/{runId}/actions", id: "workflowRunAction", tag: "workflows", summary: "Approve or retry a stage", pathParams: []any{controllers.WorkflowRunIDParam{}}, reqBody: controllers.WorkflowActionRequest{}, resps: append([]respUnit{{200, controllers.WorkflowRunEnvelope{}}}, errs...)},
	}
}
