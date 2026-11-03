package workflow

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
)

func TestInputResolution(t *testing.T) {
	for _, tc := range []struct {
		name, task       string
		defaults, inputs map[string]string
		want             string
		invalid          bool
	}{
		{name: "plain", task: "Build a reader", want: "Build a reader"},
		{name: "duplicates and whitespace", task: "{{ company }} / {{company}} / {{Company}}", inputs: map[string]string{"company": "Acme", "Company": "Other"}, want: "Acme / Acme / Other"},
		{name: "literal replacement", task: "Research {{company}} in {{market}}", defaults: map[string]string{"company": "Default", "market": "US"}, inputs: map[string]string{"company": "A $1 & <tag>\n{{market}}"}, want: "Research A $1 & <tag>\n{{market}} in US"},
		{name: "missing", task: "{{company}}", invalid: true},
		{name: "blank override", task: "{{company}}", defaults: map[string]string{"company": "Acme"}, inputs: map[string]string{"company": " \n "}, invalid: true},
		{name: "unknown", task: "Plain", inputs: map[string]string{"unused": "value"}, invalid: true},
		{name: "empty", task: "{{ }}", invalid: true},
		{name: "unclosed", task: "{{company", invalid: true},
		{name: "nested", task: "{{company {{nested}}", invalid: true},
		{name: "multiline name", task: "{{company\nname}}", invalid: true},
		{name: "long result", task: "{{a}}{{a}}", inputs: map[string]string{"a": strings.Repeat("x", 16001)}, invalid: true},
		{name: "long trailing text", task: "{{a}} tail", inputs: map[string]string{"a": strings.Repeat("x", 32000)}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, got, err := resolveInputs(tc.task, tc.defaults, tc.inputs)
			if (err != nil) != tc.invalid || (!tc.invalid && got != tc.want) {
				t.Fatalf("result=%q err=%v", got, err)
			}
		})
	}
}

func TestManualInputsValidateBeforeQueuingAndRemainImmutable(t *testing.T) {
	s, f, w := fixture(t)
	ctx := context.Background()
	w.Task = "Research {{ company }} in {{market}}"
	w.InputDefaults = map[string]string{"market": "US", "removed": "unused"}
	w, err := s.Save(ctx, w)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := w.InputDefaults["removed"]; exists {
		t.Fatal("obsolete default retained")
	}
	_, err = s.Start(ctx, w.ID, nil)
	var inputErr *apierr.Error
	if !errors.As(err, &inputErr) || inputErr.Details["fields"].(map[string]string)["company"] == "" {
		t.Fatalf("missing field error: %v", err)
	}
	runs, _ := s.Runs(ctx, w.ID)
	if len(runs) != 0 || len(f.spawns) != 0 {
		t.Fatal("invalid input started work")
	}
	r, err := s.Start(ctx, w.ID, map[string]string{"company": "Acme {{literal}}", "market": "EU"})
	if err != nil {
		t.Fatal(err)
	}
	w.Task, w.InputDefaults = "Changed template", map[string]string{}
	if _, err := s.Save(ctx, w); err != nil {
		t.Fatal(err)
	}
	// A new service reads the run from SQLite, not from the edited definition.
	s = New(f, f, f, s.dataDir)
	tick(t, s)
	r = loadRun(t, s, r.ID)
	want := "Research Acme {{literal}} in EU"
	if r.ResolvedTask == nil || *r.ResolvedTask != want || r.Definition.Task != "Research {{ company }} in {{market}}" {
		t.Fatalf("snapshot changed: %+v", r)
	}
	if r.Definition.InputDefaults["market"] != "US" {
		t.Fatal("manual override changed default")
	}
	writeStage(t, r)
	finish(f, r, domain.TurnStateCompleted)
	tick(t, s)
	if _, err := s.Act(ctx, r.ID, "approve", ""); err != nil {
		t.Fatal(err)
	}
	tick(t, s)
	r = loadRun(t, s, r.ID)
	finish(f, r, domain.TurnStateFailed)
	tick(t, s)
	if _, err := s.Act(ctx, r.ID, "retry", "Try again"); err != nil {
		t.Fatal(err)
	}
	tick(t, s)
	if len(f.messages) != 3 {
		t.Fatalf("messages=%d", len(f.messages))
	}
	for _, message := range f.messages {
		if !strings.Contains(message.Text, want) {
			t.Fatalf("lost resolved prompt: %s", message.Text)
		}
	}
	if !reflect.DeepEqual(loadRun(t, s, r.ID).Inputs, map[string]string{"company": "Acme {{literal}}", "market": "EU"}) {
		t.Fatal("lost run inputs")
	}
}

func TestScheduledInputsAndMissingDefaults(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "defaults", true: "missing defaults"}[invalid], func(t *testing.T) {
			s, f, w := fixture(t)
			ctx := context.Background()
			w.Task = "Research {{company}}"
			w.Enabled, w.RRule, w.Timezone = true, "FREQ=DAILY;BYHOUR=9;BYMINUTE=0", "UTC"
			if _, err := s.Save(ctx, w); err == nil {
				t.Fatal("enabled schedule without defaults")
			}
			w.InputDefaults = map[string]string{"company": "Acme"}
			w, err := s.Save(ctx, w)
			if err != nil {
				t.Fatal(err)
			}
			due := *w.NextRunAt
			if invalid {
				// Simulate a persisted definition from an older or external writer.
				w.InputDefaults = nil
				if err := f.Store.SaveWorkflow(ctx, w); err != nil {
					t.Fatal(err)
				}
			}
			s.now = func() time.Time { return due }
			f.failSchedule = true
			if err := s.Tick(ctx, due); err == nil {
				t.Fatal("expected cursor failure")
			}
			tick(t, s)
			runs, err := s.Runs(ctx, w.ID)
			if err != nil || len(runs) != 1 {
				t.Fatalf("runs=%d err=%v", len(runs), err)
			}
			r := runs[0]
			if invalid {
				if r.Status != "failed" || r.SessionID != "" || len(f.spawns) != 0 || r.Stages[0].Error == "" {
					t.Fatalf("invalid run started: %+v", r)
				}
				if _, err := s.Act(ctx, r.ID, "retry", ""); err == nil {
					t.Fatal("retried unresolved prompt")
				}
			} else if r.ResolvedTask == nil || *r.ResolvedTask != "Research Acme" || r.Inputs["company"] != "Acme" || len(f.spawns) != 1 {
				t.Fatalf("scheduled defaults lost: %+v", r)
			}
			saved, _, _ := s.store.GetWorkflow(ctx, w.ID)
			if !saved.NextRunAt.After(due) {
				t.Fatal("schedule cursor did not advance")
			}
		})
	}
}

func TestLegacyRunDoesNotReinterpretPlaceholders(t *testing.T) {
	s, _, w := fixture(t)
	w.Task = "Keep {{literal}} in existing task"
	r := newRun(w, s.now(), nil)
	r.ResolvedTask = nil
	if !strings.Contains(stagePrompt(&r, "artifacts"), w.Task) {
		t.Fatal("legacy prompt changed")
	}
}
