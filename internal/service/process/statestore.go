package process

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// JobIndex is where a Manager records what is up. The daemon backs it with a
// file; a Manager given none keeps everything in memory, which is what every
// test and every embedded use wants.
type JobIndex interface {
	Save(records []domain.JobRecord) error
}

// StateStore is the durable index on disk. It is read once at daemon start-up
// and rewritten whenever a job goes up or comes down — a handful of entries, so
// the whole file is rewritten rather than amended.
type StateStore struct {
	path string
	// frozen is set when the file on disk belongs to a NEWER binary. Reading it
	// gives nothing, and writing over it would destroy that binary's record of
	// what it started, so the store goes read-only for the rest of its life. An
	// older format never freezes: see rules.ClassifyIndexVersion for why that
	// distinction is the difference between a cautious store and a dead one.
	frozen bool
}

// StatePath is where the index lives: beside the socket, under the global dir.
// Empty on a machine with no user-config directory at all, which every caller
// reads the way it reads an absent socket.
func StatePath() string {
	dir, err := fs.GlobalDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, domain.DaemonStateFileName)
}

func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

// Load reads the index. Anything unreadable — absent, truncated, half-written by
// a killed daemon — is an empty index rather than an error: a daemon that
// refuses to start because of its own bookkeeping is worse than one that forgot.
func (s *StateStore) Load() []domain.JobRecord {
	if s.path == "" {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}

	var state domain.DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil
	}
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{
		File:   state.Version,
		Binary: domain.DaemonStateVersion,
	})
	s.frozen = access.Freeze
	if !access.Read {
		return nil
	}
	return rules.CurrentJobRecords(state.Jobs)
}

// Frozen reports whether this store has gone read-only, which is a fact about
// the file rather than about this instance: any reader comparing the two versions
// reaches the same verdict. It is what lets a command warn that nothing it starts
// will be recorded, without a daemon to ask.
func (s *StateStore) Frozen() bool { return s.frozen }

// IndexFrozen answers that question for a caller holding nothing: it reads the
// index and reports whether a newer binary owns it. False for every ordinary
// case, an unreadable file included — a store that cannot be read is one the
// next write replaces.
func IndexFrozen() bool {
	store := NewStateStore(StatePath())
	store.Load()
	return store.Frozen()
}

func (s *StateStore) Save(records []domain.JobRecord) error {
	if s.path == "" || s.frozen {
		return nil
	}
	data, err := json.Marshal(domain.DaemonState{Version: domain.DaemonStateVersion, Jobs: records})
	if err != nil {
		return fmt.Errorf("encode job index: %w", err)
	}
	// Atomic: a daemon killed mid-write must leave the previous index intact
	// rather than a truncated file naming half the stacks it started.
	if err := fs.ReplaceFile(fs.ReplaceFileParams{Path: s.path, Data: data, Perm: 0o600}); err != nil {
		return fmt.Errorf("write job index: %w", err)
	}
	return nil
}

// HasIndexedJobs reports whether the index still holds a job for this worktree.
// It is what tells a stop path whether waking a daemon is worth it: the daemon
// exits on its own once no foreground job is left, so "nobody is listening" says
// nothing about whether a detached stack is still up.
func HasIndexedJobs(workDir string) bool {
	for _, record := range NewStateStore(StatePath()).Load() {
		if record.WorkDir == workDir {
			return true
		}
	}
	return false
}

// HasAnyIndexedJob is HasIndexedJobs across every worktree, for the callers that
// act on all of them (`run down --all`).
func HasAnyIndexedJob() bool {
	return len(NewStateStore(StatePath()).Load()) > 0
}
