package run

import "io"

// A Tail asks the daemon for the lines of the job Name of the worktree at
// WorkDir: its last Lines (at most [MaxTail], all it keeps when zero), then,
// when Follow, each new one until the job ends.
type Tail struct {
	Name    string
	WorkDir string
	Lines   int
	Follow  bool
}

// A Window is the job Name of the worktree at WorkDir and the size, Cols by
// Rows, of the terminal a client draws it in: an attach sizes the job's
// terminal with it, a resize sizes it again.
type Window struct {
	Name    string
	WorkDir string
	Cols    int
	Rows    int
}

// A Session is a client attached to a job's terminal: what the job printed
// last as it came, raw (History), then what it prints (Stream, closed when the
// job ends or the session falls behind), and where what the client types goes
// (Input, nil for a job that runs under no terminal). Release ends it.
type Session struct {
	History []byte
	Stream  <-chan []byte
	Input   io.Writer
	Release func()
}

// A LogFile is the log of the job Job kept in the directory Dir; a run opened
// to Append continues the one already there (a detached stack launched again
// is the same run), any other empties it first: one file is one run.
type LogFile struct {
	Dir    string
	Job    string
	Append bool
}
