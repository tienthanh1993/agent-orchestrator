# Artifact workflows implementation

## Run input forms

- [x] Detect master-prompt placeholders; collect required values in a run dialog with optional saved defaults.
- [x] Persist immutable inputs/resolved prompts; validate manual and scheduled execution; retain legacy runs.
- [x] Verify parsing, API, UI, scheduling and recovery; inspect the real isolated Electron app.
- [x] Package and install the Windows update, preserving existing AO data.

- [x] Persist reusable definitions and immutable run snapshots in SQLite; sequential stage state survives daemon restarts.
- [x] Reuse one Chat session/workspace/conversation and provider switching. Idempotent prompt dispatch; explicit completion plus settled turn required.
- [x] Stage directories under session artifacts, accumulated files available to later stages, persisted handoff notes.
- [x] Approval checkpoints, stop on failure, retry with feedback without repeating completed stages.
- [x] Manual runs and recurring schedules using the existing recurrence parser.
- [x] Generated daemon API and a visual sequential canvas with a small stage editor; run progress, files, handoff, approval and retry.
- [x] Tests for handoffs, switching, completion, recovery, failure, approval, scheduling; backend/frontend checks and isolated real Electron review. Repository-wide check limitations are recorded below.

Implementation scope: local feature explicitly authorized by the user; no publish, push, or upstream PR. Use Chat sessions so provider turn completion is authoritative. Files and handoff.md are the stage contract; no user-facing output schemas or mappings.

## Input form verification on 2026-11-02

- Passed: workflow service race tests, focused workflow controller tests, API spec and SQLite suites. Coverage includes parsing, literal substitutions, validation before launch, saved defaults, scheduled failures, immutable snapshots, approval/retry, restart recovery and legacy runs.
- Passed: frontend typecheck and focused Vitest tests (4 files / 9 tests), including Cancel without saving/launching, defaults, input errors and duplicate submission protection.
- Regenerated OpenAPI and TypeScript API types. New fields use the existing workflow JSON storage; no additional migration is required.
- Native review uses `D:/00.Projects/ao-input-lab` with data/profile/run file under `C:/Users/dopagame/.ao/dev/workflow-input-lab`. Fresh `npm ci --allow-git=all` succeeded; the command-specific option allows the Electron dependency pinned to Git under npm 12.
- Passed: backend `go build ./...` and `go vet ./...`, frontend Windows `npm run make`, and final diff whitespace check. Full Windows backend suite limitations remain listed below.
- Native input review: realistic five-field defaults, Tab/Shift+Tab navigation, multiline/special-character and long values, scrollable form with fixed actions, and a 963px-wide native window. Cancel left the isolated workflow with zero runs; reopening restored saved defaults. Renderer output contained development/preload-resource notices, with no uncaught renderer exception observed.
- Lab-only dependency workaround: the installed Windows SDK was unavailable for rebuilding better-sqlite3, so its matching prebuilt native binary and Forge metadata were copied from the main checkout after a real npm install. No node_modules symlink was used. The lab Electron/Forge/daemon stopped cleanly; ports 3003 and 5174 are free. The isolated checkout and scratch data are retained.
- Windows delivery: `frontend/out/make/Agent Orchestrator Setup 0.13.0.exe` installed successfully (exit 0). Installed `resources/app.asar` and `resources/daemon/ao.exe` SHA-256 hashes match the packaged build. The installed app at `C:/Users/dopagame/AppData/Local/Programs/AGENTO~1/agent-orchestrator` displays the new required company/market form from an unsaved example prompt. Production daemon is at `127.0.0.1:3001` and renderer is `app://renderer/index.html`.
- Data preservation: existing project/workflow IDs survived the update; a consistent database backup is at `C:/Users/dopagame/.ao/backups/workflow-inputs-before-install-20261102/ao.db`. No publishing or pushing was performed.
- Installed-app acceptance: the two-field dialog displayed inline required-value errors; cancellation discarded the unsaved example and the original saved workflow was restored. All saved workflow definitions remained identical to the post-install snapshot. Final `git diff --check` exited 0.

## Verification on 2026-11-02

- Passed: `go test -race ./internal/service/workflow`, covering inherited artifacts, provider switching, approval, failure/retry, stale completion markers, idempotent dispatch, schedule recovery and shutdown.
- Passed: `go test ./internal/httpd ./internal/httpd/apispec/... ./internal/storage/sqlite/...`, including API drift and storage checks; `go vet ./...`.
- Passed: frontend `npm run typecheck`, focused Vitest tests (3 files / 5 tests), and `npm run build`.
- Real Electron review used an isolated checkout at `D:/00.Projects/ao-workflow-lab` on the feature branch's base with the implementation copied into it. Data: `C:/Users/dopagame/.ao/dev/workflow-lab/data`; run file: `C:/Users/dopagame/.ao/dev/workflow-lab/running.json`; Electron profile: `C:/Users/dopagame/.ao/dev/workflow-lab/electron`. Renderer: `http://localhost:5174`; daemon: `http://127.0.0.1:3002`.
- Live workflow `Release checklist handoff` (`workflow-17c1d767-db35-48ac-80d5-8e1ac5c3ac40`, session `standalone-1`) finished with stage attempts `[1, 1, 2]`. Planning inherited research files; approval started Build; stopping Build produced a retryable failure; restarting Electron preserved progress and turn identities; feedback retried only Build. Earlier artifacts remained available, and `plan.md` opened in the existing artifact viewer. Final files included `handoff.md`, `release-checklist.md` and `test-results.md`. Renderer errors were empty after the saved-definition payload fix.
- Cleanup complete: isolated Electron/Forge, daemon and provider processes stopped; no listeners remain on ports 3002/9228; the temporary lab worktree was removed. Scratch run data is retained under the isolated AO directory. Final diff whitespace check passed.

## Repository-wide check limitations

- Full Windows `go test ./...` is not green. Failures include missing Unix shell/tmux dependencies, Windows path/rename behavior, project clone URL handling, Kimi reviewer, usage collection, session manager and daemon temporary-directory cleanup.
- The final controller suite also reproduced filesystem status, mobile pairing rename and project clone URL failures. API spec tests passed. These failures do not establish a full backend pass.
- Repository-wide golangci-lint reports thousands of findings, predominantly existing import-formatting issues. Workflow-specific findings were corrected; full repository lint is not claimed to pass. Local logs: `workflow-golangci-final.log`, `workflow-controller-lint.log`, `workflow-go-tests.log`, `workflow-final-boundaries.log`, `workflow-frontend-tests.log` and `workflow-build.log`.
- After the final DTO documentation/formatting correction, the controller lint rerun contains no findings for `dto.go` or `workflows.go`; other controller import-formatting findings remain.
