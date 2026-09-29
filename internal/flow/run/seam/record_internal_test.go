package seam

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

var db = domain.JobConfig{
	Name: "db", Kind: domain.JobKindService, Scope: domain.JobScopeShared,
	Namespace: &domain.JobNamespaceConfig{Name: "app_{worktree}", Create: "true", Remove: "true"},
}

// recordingSeam is a worktree whose shared db carves a namespace, its meta.json
// in place so the namespace has somewhere to be written down.
func recordingSeam(t *testing.T) (Seam, string) {
	t.Helper()
	stateDir := t.TempDir()
	metaDir := rules.WorktreeMetaDir(stateDir, "feat")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, domain.MetaFileName), []byte(`{"source_branch":"main","ordinal":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return Seam{
		service:  &runlogstest.Service{},
		workDir:  "/work/feat",
		worktree: "feat",
		stateDir: stateDir,
		env:      map[string]string{},
		shared:   &domain.SharedJobContext{WorkDir: "/work/main"},
	}, metaDir
}

type watchingSink struct {
	onEvent func(runlogs.Event)
	events  []runlogs.Event
}

func (s *watchingSink) Emit(event runlogs.Event) {
	s.events = append(s.events, event)
	if s.onEvent != nil {
		s.onEvent(event)
	}
}

// Recorded when db reports started, before the next job is even asked for: a
// run interrupted there has created a database a clean must still know about.
func TestANamespaceIsRecordedTheMomentItsServiceStarts(t *testing.T) {
	seam, _ := recordingSeam(t)
	var heldWhenWebStarted []string
	sink := &watchingSink{onEvent: func(event runlogs.Event) {
		if event.Phase == runlogs.PhaseStarting && event.Job == web.Name {
			heldWhenWebStarted = worktree.NamespacesOf(worktree.ParentBranchParams{StateDir: seam.stateDir, Branch: "feat"})
		}
	}}

	if _, err := seam.Starter(StartParams{Jobs: []domain.JobConfig{db, web}})(context.Background(), sink); err != nil {
		t.Fatalf("start: %v", err)
	}
	if strings.Join(heldWhenWebStarted, ",") != "db" {
		t.Errorf("held when web started = %v, want db already recorded", heldWhenWebStarted)
	}
}

// A namespace that could not be written down is one no clean will drop: the
// run says so rather than leaking it in silence.
func TestANamespaceThatCannotBeRecordedIsSaid(t *testing.T) {
	seam, metaDir := recordingSeam(t)
	if err := os.Chmod(metaDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(metaDir, 0o755) })
	sink := &watchingSink{}

	if _, err := seam.Starter(StartParams{Jobs: []domain.JobConfig{db}})(context.Background(), sink); err != nil {
		t.Fatalf("start: %v", err)
	}
	for _, event := range sink.events {
		if event.Phase == runlogs.PhaseWarning && event.Job == "db" && strings.Contains(event.Notice, "wtm clean") {
			return
		}
	}
	t.Errorf("events = %+v, want a warning naming db", sink.events)
}

func TestAVerbatimWorktreeRecordsNothing(t *testing.T) {
	seam, _ := recordingSeam(t)
	seam.env[domain.EnvIsolation] = string(domain.IsolationVerbatim)

	if _, err := seam.Starter(StartParams{Jobs: []domain.JobConfig{db}})(context.Background(), &watchingSink{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if held := worktree.NamespacesOf(worktree.ParentBranchParams{StateDir: seam.stateDir, Branch: "feat"}); len(held) != 0 {
		t.Errorf("held = %v, want nothing: a verbatim worktree carves nothing", held)
	}
}
