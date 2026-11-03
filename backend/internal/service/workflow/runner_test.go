package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite/sqlitetest"
)

// Use real persistence while replacing only the external agent processes.
type testRuntime struct {
	*sqlite.Store
	turns        map[string]domain.ConversationTurn
	messages     []ports.ChatUserMessage
	picks        []domain.ProviderSwitchRequest
	spawns       []ports.SpawnConfig
	sendErr      error
	failSave     bool
	failSchedule bool
}

func (f *testRuntime) SupportsChat(h domain.AgentHarness) bool {
	return h == domain.HarnessCodex || h == domain.HarnessClaudeCode
}

func (f *testRuntime) GetSession(_ context.Context, id domain.SessionID) (domain.SessionRecord, bool, error) {
	return domain.SessionRecord{ID: id, Mode: domain.SessionModeChat}, true, nil
}
func (f *testRuntime) TurnByID(_ context.Context, id string) (domain.ConversationTurn, error) {
	turn, ok := f.turns[id]
	if !ok {
		return turn, fmt.Errorf("unknown turn %s", id)
	}
	return turn, nil
}
func (f *testRuntime) Spawn(_ context.Context, cfg ports.SpawnConfig) (domain.Session, int, int, error) {
	f.spawns = append(f.spawns, cfg)
	return domain.Session{ID: "workflow-session"}, 0, 0, nil
}
func (f *testRuntime) SwitchProviderAndSend(_ context.Context, _ domain.SessionID, pick domain.ProviderSwitchRequest, msg ports.ChatUserMessage) (domain.ConversationTurn, bool, error) {
	if f.sendErr != nil {
		return domain.ConversationTurn{}, false, f.sendErr
	}
	if turn, ok := f.turns[msg.ClientMessageID]; ok {
		return turn, true, nil
	}
	f.messages = append(f.messages, msg)
	f.picks = append(f.picks, pick)
	turn := domain.ConversationTurn{ID: msg.ClientMessageID, State: domain.TurnStateRunning}
	f.turns[turn.ID] = turn
	return turn, true, nil
}
func (f *testRuntime) SaveWorkflowRun(ctx context.Context, run domain.WorkflowRun) error {
	if f.failSave && run.Stages[run.CurrentStage].TurnID != "" {
		f.failSave = false
		return errors.New("interrupted after dispatch")
	}
	return f.Store.SaveWorkflowRun(ctx, run)
}
func (f *testRuntime) SaveWorkflow(ctx context.Context, w domain.Workflow) error {
	if f.failSchedule {
		f.failSchedule = false
		return errors.New("interrupted after occurrence")
	}
	return f.Store.SaveWorkflow(ctx, w)
}

func fixture(t *testing.T) (*Service, *testRuntime, domain.Workflow) {
	t.Helper()
	dir := t.TempDir()
	f := &testRuntime{Store: sqlitetest.MustOpenAt(t, dir), turns: map[string]domain.ConversationTurn{}}
	s := New(f, f, f, dir)
	s.now = func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }
	w, err := s.Save(context.Background(), domain.Workflow{Name: "Research and build", Task: "Build an offline reader", Stages: []domain.WorkflowStage{
		{Name: "Research", Agent: domain.HarnessCodex, Instructions: "Save research.md", Approval: true},
		{Name: "Build", Agent: domain.HarnessClaudeCode, Model: "selected-model", Effort: "high", Instructions: "Implement the reader"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return s, f, w
}

func tick(t *testing.T, s *Service) {
	t.Helper()
	if err := s.Tick(context.Background(), s.now()); err != nil {
		t.Fatal(err)
	}
}
func loadRun(t *testing.T, s *Service, id string) domain.WorkflowRun {
	t.Helper()
	r, found, err := s.store.GetWorkflowRun(context.Background(), id)
	if err != nil || !found {
		t.Fatalf("run: found=%v err=%v", found, err)
	}
	return r
}
func startRun(t *testing.T, s *Service, w domain.Workflow) domain.WorkflowRun {
	t.Helper()
	r, err := s.Start(context.Background(), w.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	tick(t, s)
	return loadRun(t, s, r.ID)
}
func writeStage(t *testing.T, r domain.WorkflowRun) {
	t.Helper()
	dir := r.Stages[r.CurrentStage].ArtifactDir
	for name, value := range map[string]string{"handoff.md": "Offline cache selected. Build next.", "research.md": "Evidence and sources", "stage-complete.txt": completionKey(&r)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func finish(f *testRuntime, r domain.WorkflowRun, state domain.TurnState) {
	id := r.Stages[r.CurrentStage].TurnID
	f.turns[id] = domain.ConversationTurn{ID: id, State: state}
}

func TestHandoffApprovalSwitchAndRecovery(t *testing.T) {
	s, f, w := fixture(t)
	r := startRun(t, s, w)
	writeStage(t, r)
	tick(t, s) // Marker alone cannot advance a running turn.
	if got := loadRun(t, s, r.ID); got.Status != "running" || got.CurrentStage != 0 {
		t.Fatalf("advanced live turn: %+v", got)
	}
	finish(f, r, domain.TurnStateCompleted)
	tick(t, s)
	r = loadRun(t, s, r.ID)
	if r.Status != "awaiting_approval" || len(r.Stages[0].Files) != 2 {
		t.Fatalf("handoff: %+v", r)
	}
	// A fresh service observes the durable checkpoint without resending work.
	restarted := New(f, f, f, s.dataDir)
	tick(t, restarted)
	if len(f.messages) != 1 {
		t.Fatal("replayed completed stage")
	}
	if _, err := restarted.Act(context.Background(), r.ID, "approve", ""); err != nil {
		t.Fatal(err)
	}
	tick(t, restarted)
	r = loadRun(t, restarted, r.ID)
	if r.CurrentStage != 1 || len(f.spawns) != 1 || len(f.messages) != 2 {
		t.Fatalf("advance: %+v", r)
	}
	pick := f.picks[1]
	if pick.TargetHarness != domain.HarnessClaudeCode || pick.Model != "selected-model" || pick.Effort != "high" {
		t.Fatalf("picks: %+v", pick)
	}
	for _, want := range []string{w.Task, w.Stages[1].Instructions, r.Stages[0].Handoff, r.Stages[1].ArtifactDir} {
		if !strings.Contains(f.messages[1].Text, want) {
			t.Fatalf("missing inherited context %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(r.Stages[0].ArtifactDir, "research.md")); err != nil {
		t.Fatal(err)
	}
	tick(t, New(f, f, f, s.dataDir))
	if len(f.messages) != 2 {
		t.Fatal("replayed running turn")
	}
	writeStage(t, r)
	finish(f, r, domain.TurnStateCompleted)
	tick(t, restarted)
	tick(t, restarted)
	if got := loadRun(t, s, r.ID); got.Status != "completed" || len(f.messages) != 2 {
		t.Fatalf("completion: %+v", got)
	}
}
