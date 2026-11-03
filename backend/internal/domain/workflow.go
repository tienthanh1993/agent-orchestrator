package domain

import "time"

// WorkflowStage is deliberately only an agent, instructions and a checkpoint.
type WorkflowStage struct {
	Name         string       `json:"name"`
	Agent        AgentHarness `json:"agent"`
	Model        string       `json:"model,omitempty"`
	Effort       string       `json:"effort,omitempty"`
	Instructions string       `json:"instructions"`
	Approval     bool         `json:"approval"`
}

// Workflow is a reusable ordered definition. Runs keep their own snapshot.
type Workflow struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ProjectID     ProjectID         `json:"projectId"`
	Task          string            `json:"task"`
	InputDefaults map[string]string `json:"inputDefaults,omitempty"`
	Stages        []WorkflowStage   `json:"stages"`
	RRule         string            `json:"rrule,omitempty"`
	Timezone      string            `json:"timezone,omitempty"`
	Enabled       bool              `json:"enabled"`
	NextRunAt     *time.Time        `json:"nextRunAt,omitempty"`
}

// WorkflowStageRun stores execution facts, not session display status.
type WorkflowStageRun struct {
	Status      string                `json:"status"`
	Attempt     int                   `json:"attempt"`
	TurnID      string                `json:"turnId,omitempty"`
	ArtifactDir string                `json:"artifactDir,omitempty"`
	Handoff     string                `json:"handoff,omitempty"`
	Feedback    string                `json:"feedback,omitempty"`
	Error       string                `json:"error,omitempty"`
	Files       []SessionArtifactFile `json:"files"`
}

// WorkflowRun owns one shared session and immutable definition snapshot.
type WorkflowRun struct {
	ID           string             `json:"id"`
	WorkflowID   string             `json:"workflowId"`
	Definition   Workflow           `json:"definition"`
	Inputs       map[string]string  `json:"inputs,omitempty"`
	ResolvedTask *string            `json:"resolvedTask,omitempty"`
	SessionID    SessionID          `json:"sessionId,omitempty"`
	Status       string             `json:"status"`
	CurrentStage int                `json:"currentStage"`
	Stages       []WorkflowStageRun `json:"stages"`
	CreatedAt    time.Time          `json:"createdAt"`
	ScheduledFor *time.Time         `json:"scheduledFor,omitempty"`
}
