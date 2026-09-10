package runview

import (
	"sync"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
)

// paneSource is where a pane's bytes come from. A pane only ever has one: the
// daemon replays a job's history when a stream is attached, so a pane fed by
// anything else is rebuilt rather than appended to, or the replay would show
// twice.
type paneSource int

const (
	sourceLive paneSource = iota
	sourceHistory
	// sourceSequence is a job whose bytes come from the run starting it, which
	// reports them itself rather than through a subscription.
	sourceSequence
)

type jobPane struct {
	pane   *Pane
	source paneSource
	stream runlogs.Stream
	// pendingCR carries a chunk that ended on a CR across to the next one, so a
	// CRLF split between two writes is not turned into two line breaks.
	pendingCR bool
}

// paneStore holds the panes of the jobs on screen. The goroutine reading a
// stream writes into a pane as the bytes arrive while the render loop reads it,
// so the map is guarded here rather than left to Bubbletea's copies of the
// model.
type paneStore struct {
	mu    sync.Mutex
	size  PaneSize
	panes map[jobKey]*jobPane
	// followed is the only job whose writes are worth a frame: a starting profile
	// feeds every pane it opens, and one of them is on screen.
	followed jobKey
}

func newPaneStore(size PaneSize) *paneStore {
	return &paneStore{size: size, panes: map[jobKey]*jobPane{}}
}

type openPaneParams struct {
	Key    jobKey
	Source paneSource
}

// open returns the job's pane, building a fresh one when the job has none or
// when its bytes are about to come from somewhere else than they did.
func (s *paneStore) open(params openPaneParams) *Pane {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openLocked(params)
}

func (s *paneStore) entryLocked(params openPaneParams) *jobPane {
	s.openLocked(params)
	return s.panes[params.Key]
}

func (s *paneStore) openLocked(params openPaneParams) *Pane {
	if entry, held := s.panes[params.Key]; held && entry.source == params.Source {
		return entry.pane
	}
	// A pane being replaced may still be subscribed to its job; dropping it
	// without closing that would leave the job feeding a pane nobody reads.
	s.closeStreamLocked(params.Key)

	pane := NewPane(PaneParams{Size: s.size})
	s.panes[params.Key] = &jobPane{pane: pane, source: params.Source}
	return pane
}

type writeChunkParams struct {
	Key    jobKey
	Source paneSource
	Chunk  []byte
	// NormalizeEOL terminates the chunk's bare LFs the way a PTY would. A job
	// running on a pipe has no line discipline to do it, so the emulator moved
	// down a row without returning to column zero and every line landed one step
	// further right than the last (LUC-208).
	NormalizeEOL bool
}

func (s *paneStore) write(params writeChunkParams) {
	s.mu.Lock()
	entry := s.entryLocked(openPaneParams{Key: params.Key, Source: params.Source})
	chunk := params.Chunk
	if params.NormalizeEOL {
		normalized := rules.NormalizeEOL(rules.NormalizeEOLParams{Chunk: chunk, PendingCR: entry.pendingCR})
		chunk, entry.pendingCR = normalized.Chunk, normalized.PendingCR
	}
	pane := entry.pane
	s.mu.Unlock()

	pane.Write(chunk)
}

type writeLinesParams struct {
	Key   jobKey
	Lines []string
}

func (s *paneStore) writeLines(params writeLinesParams) {
	pane := s.open(openPaneParams{Key: params.Key, Source: sourceHistory})
	for _, line := range params.Lines {
		pane.Write([]byte(line + "\r\n"))
	}
}

// attach binds a stream to a fresh pane and returns it for the goroutine that
// will feed it. Any stream the job already had is closed first: two
// subscriptions on one job would double its output and fight over its PTY size.
func (s *paneStore) attach(params attachPaneParams) *Pane {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeStreamLocked(params.Key)

	pane := NewPane(PaneParams{Size: s.size})
	s.panes[params.Key] = &jobPane{pane: pane, source: sourceLive, stream: params.Stream}
	return pane
}

type attachPaneParams struct {
	Key    jobKey
	Stream runlogs.Stream
}

// release closes a job's stream and drops the pane it fed. The pane is what
// makes it worth doing: a styled cell per column it was ever written, which at
// the scrollback the panes hold runs to megabytes each. Nothing is lost with
// it — the daemon replays a live job's recent output on the next attach, and a
// stopped one's log file is read back in one call. The one pane worth keeping
// is the one a run is writing into right now, which the caller keeps for
// itself: nothing replays that yet.
func (s *paneStore) release(job jobKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, held := s.panes[job]; !held {
		return
	}
	s.closeStreamLocked(job)
	delete(s.panes, job)
}

// endStream drops a subscription whose job stopped writing, keeping the pane:
// what the job printed before it went is the last thing worth reading, and the
// log file has nothing more to add to it.
func (s *paneStore) endStream(job jobKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeStreamLocked(job)
}

func (s *paneStore) closeStreamLocked(job jobKey) {
	entry, held := s.panes[job]
	if !held || entry.stream == nil {
		return
	}
	entry.stream.Close()
	entry.stream = nil
}

func (s *paneStore) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for job := range s.panes {
		s.closeStreamLocked(job)
	}
	s.panes = map[jobKey]*jobPane{}
}

func (s *paneStore) entry(job jobKey) (jobPane, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held, ok := s.panes[job]
	if !ok {
		return jobPane{}, false
	}
	return *held, true
}

// stream is the subscription a surface writes to, present only while the job is
// attached.
func (s *paneStore) stream(job jobKey) runlogs.Stream {
	entry, ok := s.entry(job)
	if !ok {
		return nil
	}
	return entry.stream
}

// follow is set from the one place the cursor moves, so the redraw clock keeps
// watching the drawn pane without being restarted when the selection changes.
func (s *paneStore) follow(key jobKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.followed = key
}

func (s *paneStore) takeDirty() bool {
	s.mu.Lock()
	entry, held := s.panes[s.followed]
	s.mu.Unlock()
	if !held {
		return false
	}
	return entry.pane.TakeDirty()
}

// hasStream reports whether anything is still feeding a pane, which is the
// only reason to keep redrawing on a clock.
func (s *paneStore) hasStream() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, entry := range s.panes {
		if entry.stream != nil {
			return true
		}
	}
	return false
}

// resize sizes every pane the store holds and reports the streams whose job now
// draws at a different size. Resizing a PTY is a round-trip to the daemon, so it
// is left to the caller to make outside this lock.
func (s *paneStore) resize(size PaneSize) []runlogs.Stream {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size = size

	var changed []runlogs.Stream
	for _, entry := range s.panes {
		if entry.pane.Resize(size) && entry.stream != nil {
			changed = append(changed, entry.stream)
		}
	}
	return changed
}
