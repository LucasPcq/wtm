package process

import (
	"os"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

type recordingIndex struct {
	saved [][]domain.JobRecord
}

func (r *recordingIndex) Save(records []domain.JobRecord) error {
	r.saved = append(r.saved, records)
	return nil
}

func TestAdoptRegistersDetachedJobsAsDetached(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWith(ManagerParams{Index: &recordingIndex{}, Stacks: &unknownStacks{}})

	manager.Adopt([]domain.JobRecord{detachedRecord(t, dir)})

	jobs := manager.List()
	if len(jobs) != 1 {
		t.Fatalf("adopted %d jobs, want 1", len(jobs))
	}
	if jobs[0].Status != domain.JobStatusDetached {
		t.Fatalf("status = %q, want %q", jobs[0].Status, domain.JobStatusDetached)
	}
	if jobs[0].PTY != nil || jobs[0].Cmd != nil {
		t.Fatal("an adopted job carries no process: the daemon that owned it is gone")
	}
}

func TestAdoptedDetachedJobLeavesTheDaemonFreeToExit(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWith(ManagerParams{Stacks: &unknownStacks{}})

	manager.Adopt([]domain.JobRecord{detachedRecord(t, dir)})

	if manager.IsRunning() {
		t.Fatal("a detached stack must not hold the daemon alive: the index is what brings it back")
	}
}

func TestAdoptDropsEntriesOfDeletedWorktrees(t *testing.T) {
	dir := t.TempDir()
	record := detachedRecord(t, dir)
	os.RemoveAll(dir)

	manager := NewManagerWith(ManagerParams{Stacks: &unknownStacks{}})
	manager.Adopt([]domain.JobRecord{record})

	if len(manager.List()) != 0 {
		t.Fatal("a stop command cannot run in a worktree that no longer exists")
	}
}

func TestAdoptRewritesTheIndexWithWhatItKept(t *testing.T) {
	dir := t.TempDir()
	gone := detachedRecord(t, "/definitely/not/here")
	index := &recordingIndex{}

	manager := NewManagerWith(ManagerParams{Index: index, Stacks: &unknownStacks{}})
	manager.Adopt([]domain.JobRecord{detachedRecord(t, dir), gone})

	if len(index.saved) != 1 {
		t.Fatalf("saved %d times, want 1", len(index.saved))
	}
	if len(index.saved[0]) != 1 {
		t.Fatalf("indexed %d records, want only the surviving one", len(index.saved[0]))
	}
}

func TestStopOnAnAdoptedJobRunsItsStopCommandAndClearsTheIndex(t *testing.T) {
	dir := t.TempDir()
	marker := dir + "/stopped"
	record := detachedRecord(t, dir)
	record.Config.Stop = "touch " + marker
	index := &recordingIndex{}

	manager := NewManagerWith(ManagerParams{Index: index, Stacks: &unknownStacks{}})
	manager.Adopt([]domain.JobRecord{record})

	if err := manager.Stop("db", dir); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("the stop command of an adopted job must run: that is the whole point of the index")
	}
	last := index.saved[len(index.saved)-1]
	if len(last) != 0 {
		t.Fatalf("index still holds %d records after the stop", len(last))
	}
}
