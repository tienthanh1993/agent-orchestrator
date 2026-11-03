-- +goose Up
-- +goose StatementBegin
CREATE TABLE workflows (id TEXT PRIMARY KEY, body TEXT NOT NULL CHECK (json_valid(body)));
CREATE TABLE workflow_runs (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL REFERENCES workflows(id),
    occurrence TEXT NOT NULL,
    status TEXT NOT NULL,
    body TEXT NOT NULL CHECK (json_valid(body)),
    UNIQUE(workflow_id, occurrence)
);
CREATE INDEX workflow_runs_active ON workflow_runs(status);
-- +goose StatementEnd

-- +goose Down
DROP TABLE workflow_runs;
DROP TABLE workflows;
