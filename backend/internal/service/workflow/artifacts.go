package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/sessionartifacts"
)

func stagePath(index int) string { return filepath.Join("workflow", fmt.Sprintf("%02d", index+1)) }

func completionKey(r *domain.WorkflowRun) string {
	return fmt.Sprintf("%s-stage-%d-attempt-%d", r.ID, r.CurrentStage+1, r.Stages[r.CurrentStage].Attempt)
}

func stagePrompt(r *domain.WorkflowRun, artifacts string) string {
	stage := r.Definition.Stages[r.CurrentStage]
	state := r.Stages[r.CurrentStage]
	task := r.Definition.Task // Runs created before input forms keep their original task.
	if r.ResolvedTask != nil {
		task = *r.ResolvedTask
	}
	previous := "This is the first stage."
	if r.CurrentStage > 0 {
		previous = r.Stages[r.CurrentStage-1].Handoff
	}
	return fmt.Sprintf(`Workflow: %s
Stage %d of %d: %s

Original task:
%s

Your instructions:
%s

Previous stage handoff:
%s

Retry feedback (if any):
%s

You share the same workspace and AO conversation with all stages. Read the
earlier stage directories under "%s" and use their accumulated artifacts.
Keep code changes in this workspace. Save this stage's deliverables together
under "%s". Preserve earlier stages' files.

When finished, write handoff.md in your stage directory. Briefly describe what
you accomplished, relevant decisions, deliverable paths and remaining work.
Only after all work and handoff.md are saved, explicitly mark this attempt
complete by writing stage-complete.txt in that directory with this exact text:
%s
Then finish your turn. Do not perform the next stage yourself. If blocked or
unsuccessful, explain the problem and end without writing the completion file.
`, r.Definition.Name, r.CurrentStage+1, len(r.Stages), stage.Name, task,
		stage.Instructions, previous, state.Feedback, artifacts, state.ArtifactDir, completionKey(r))
}

func readCompletion(dir string, r *domain.WorkflowRun) (string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	marker, err := readSmall(root, filepath.Join(stagePath(r.CurrentStage), "stage-complete.txt"), 512)
	if err != nil || strings.TrimSpace(marker) != completionKey(r) {
		return "", fmt.Errorf("turn finished without marking this stage complete; retry with feedback")
	}
	handoff, err := readSmall(root, filepath.Join(stagePath(r.CurrentStage), "handoff.md"), 32000)
	if err != nil {
		return "", fmt.Errorf("read handoff.md: %w", err)
	}
	if strings.TrimSpace(handoff) == "" {
		return "", fmt.Errorf("handoff.md is empty; retry to save a handoff")
	}
	return handoff, nil
}

func readSmall(root *os.Root, name string, limit int64) (string, error) {
	f, err := root.Open(name)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return "", fmt.Errorf("%s must be a regular file no larger than %d bytes", name, limit)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return "", err
	}
	if int64(len(b)) > limit {
		return "", fmt.Errorf("%s is too large", name)
	}
	return string(b), nil
}

func (s *Service) refreshFiles(ctx context.Context, r *domain.WorkflowRun) {
	if r.SessionID == "" {
		return
	}
	files, err := sessionartifacts.List(ctx, sessionartifacts.Dir(s.dataDir, r.SessionID))
	if err != nil {
		return
	}
	for i := range r.Stages {
		r.Stages[i].Files = []domain.SessionArtifactFile{}
		prefix := filepath.ToSlash(stagePath(i)) + "/"
		for _, file := range files {
			if strings.HasPrefix(file.Path, prefix) && file.Name != "stage-complete.txt" {
				r.Stages[i].Files = append(r.Stages[i].Files, file)
			}
		}
	}
}
