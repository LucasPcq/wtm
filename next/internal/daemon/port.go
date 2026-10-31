package daemon

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Spawn starts command, in a process group of its own, and returns its
// process.
type Spawn func(command run.Command) (Process, error)

// A Process is a job's process, the leader of its own group.
type Process interface {
	// PID returns its process id, its group's too.
	PID() int
	// Wait waits for it to exit and its output to end, and returns its exit
	// code, -1 when a signal killed it; what is left of its group runs on.
	Wait() int
	// Stop stops its whole group, SIGTERM then SIGKILL past grace, and returns
	// once it exited.
	Stop(grace time.Duration)
	// Write types p into its terminal; it fails for a process run under none.
	Write(p []byte) (int, error)
	// Resize sizes its terminal, cols by rows; it fails for a process run
	// under none.
	Resize(cols, rows int) error
}

// OpenLog opens the log of a job for writing.
type OpenLog func(file run.LogFile) (Log, error)

// A Log is a job's log open for writing.
type Log interface {
	// Write appends lines; a log that cannot be written stops writing.
	Write(lines []run.Line)
	// Close closes it.
	Close()
}

// ReadLog returns the last n lines of a job's log, oldest first, none when it
// has none.
type ReadLog func(file run.LogFile, n int) []run.Line

// Orphans probes and stops the process groups a daemon that died left behind.
type Orphans interface {
	// Probe returns how each group of started stands (a group id, mapped to
	// when its record says its job started): alive, and identified when a
	// member of it started then.
	Probe(started map[int]time.Time) map[int]run.Group
	// Reap stops each group of pgids, SIGTERM then SIGKILL.
	Reap(pgids []int)
}

// Index is the daemon's record of the jobs up, for the next daemon.
type Index interface {
	// Load returns the records of the index, none when it is absent, cannot
	// be read, or a newer wtm wrote it.
	Load() []run.Record
	// Save replaces the index with records, unless a newer wtm wrote it.
	Save(records []run.Record) error
}

// Publish relays line on the event bus.
type Publish func(line domain.Line)

// Exists reports whether the directory at path is there.
type Exists func(path string) bool
