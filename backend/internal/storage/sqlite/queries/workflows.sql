-- name: SaveWorkflow :exec
INSERT INTO workflows (id, body) VALUES (?, ?)
ON CONFLICT(id) DO UPDATE SET body = excluded.body;

-- name: GetWorkflow :one
SELECT body FROM workflows WHERE id = ?;

-- name: ListWorkflows :many
SELECT body FROM workflows ORDER BY id;

-- name: DeleteWorkflow :exec
DELETE FROM workflows WHERE id = ?;

-- name: InsertWorkflowRun :exec
INSERT INTO workflow_runs (id, workflow_id, occurrence, status, body) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(workflow_id, occurrence) DO NOTHING;

-- name: SaveWorkflowRun :exec
UPDATE workflow_runs SET status = ?, body = ? WHERE id = ?;

-- name: GetWorkflowRun :one
SELECT body FROM workflow_runs WHERE id = ?;

-- name: ListWorkflowRuns :many
SELECT body FROM workflow_runs WHERE workflow_id = ? ORDER BY rowid DESC LIMIT 100;

-- name: ListActiveWorkflowRuns :many
SELECT body FROM workflow_runs WHERE status IN ('pending', 'running') ORDER BY rowid;
