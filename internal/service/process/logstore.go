package process

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type JobLogPathParams struct {
	LogDir string
	Job    string
}

// JobLogPath is empty for a job that cannot be given a file of its own, which
// every reader and writer below treats as "this job persists nothing" rather
// than falling back on a path shared with another job.
func JobLogPath(params JobLogPathParams) string {
	name := rules.JobLogFileName(params.Job)
	if params.LogDir == "" || name == "" {
		return ""
	}
	return filepath.Join(params.LogDir, name)
}

type LogSinkParams struct {
	LogDir string
	Job    string
	// MaxBytes overrides the rotation threshold; zero means domain.JobLogMaxBytes.
	MaxBytes int64
}

// LogSink persists a job's raw output as sanitized, timestamped lines, rotating
// the file once it outgrows its threshold. It is an io.Writer so it can sit
// beside the output hub on the job's drain path — a subscriber would have its
// chunks dropped under load, and a log that loses lines is not a log.
type LogSink struct {
	path     string
	maxBytes int64

	mu   sync.Mutex
	file *os.File
	// A reader tailing the log waits on this timer, never on the job's next chunk.
	buf     *bufio.Writer
	flush   *time.Timer
	size    int64
	pending string
}

// OpenLogSink creates the log directory if needed and opens the job's log on an
// empty file, its backups dropped: one file is one run. The boundary is
// structural rather than a rule the reader has to know, which is what stops a
// tail from crossing back into a run that ended days ago (LUC-198). Only this
// job's files go — a directory holds every job of the worktree.
func OpenLogSink(params LogSinkParams) (*LogSink, error) {
	if params.LogDir == "" {
		return nil, fmt.Errorf("log dir is required")
	}
	if err := os.MkdirAll(params.LogDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	path := JobLogPath(JobLogPathParams{LogDir: params.LogDir, Job: params.Job})
	if path == "" {
		return nil, fmt.Errorf("job %q cannot be named as a log file", params.Job)
	}
	removeBackups(path)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open job log: %w", err)
	}

	return &LogSink{
		path:     path,
		maxBytes: resolveMaxBytes(params.MaxBytes),
		file:     file,
		buf:      bufio.NewWriterSize(file, domain.JobLogBufferBytes),
	}, nil
}

func resolveMaxBytes(configured int64) int64 {
	if configured <= 0 {
		return domain.JobLogMaxBytes
	}
	return configured
}

// Write never fails and never blocks the caller on the file: the job's output
// matters more than its archive, so a broken sink stops writing and lets the
// PTY keep draining.
func (s *LogSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return len(p), nil
	}
	result := rules.SanitizeLogChunk(rules.SanitizeChunkParams{
		Chunk:   string(p),
		Pending: s.pending,
		At:      time.Now(),
	})
	s.pending = result.Pending
	s.append(result.Records)
	return len(p), nil
}

// Close flushes the line the job left unterminated and releases the file.
func (s *LogSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return nil
	}
	if text := rules.SanitizeLogLine(s.pending); text != "" {
		s.append([]domain.LogRecord{{At: time.Now(), Text: text}})
	}
	s.pending = ""
	s.stopFlush()

	// append may have given up on the file on its way — a rotation that failed.
	if s.file == nil {
		return nil
	}
	flushed, file := s.buf.Flush(), s.file
	s.file, s.buf = nil, nil
	return errors.Join(flushed, file.Close())
}

func (s *LogSink) append(records []domain.LogRecord) {
	if len(records) == 0 {
		return
	}

	var buf strings.Builder
	for _, record := range records {
		buf.WriteString(rules.FormatLogRecord(record))
		buf.WriteByte('\n')
	}

	if s.size > 0 && s.size+int64(buf.Len()) > s.maxBytes {
		if err := s.rotate(); err != nil {
			s.disable()
			return
		}
	}

	written, err := s.buf.WriteString(buf.String())
	s.size += int64(written)
	if err != nil {
		s.disable()
		return
	}
	s.armFlush()

	// Rotating only before the write let a batch bigger than the threshold land
	// in the active file and stay there, growing it without bound. A record is
	// never split — a log line belongs to one file — so an oversized one is
	// written whole and retired immediately.
	if s.size >= s.maxBytes {
		if err := s.rotate(); err != nil {
			s.disable()
		}
	}
}

func (s *LogSink) rotate() error {
	if err := s.buf.Flush(); err != nil {
		return err
	}
	if err := s.file.Close(); err != nil {
		return err
	}
	s.file = nil

	oldest := domain.JobLogMaxFiles - 1
	os.Remove(backupPath(s.path, oldest))
	for rank := oldest - 1; rank >= 1; rank-- {
		os.Rename(backupPath(s.path, rank), backupPath(s.path, rank+1))
	}
	if err := os.Rename(s.path, backupPath(s.path, 1)); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	s.file = file
	s.buf.Reset(file)
	s.size = 0
	return nil
}

// armFlush is set from the first write of a batch and never pushed back by the
// ones after it: the interval bounds how stale a reader's tail can be, and a
// timer that kept moving would never bound anything.
func (s *LogSink) armFlush() {
	if s.flush != nil {
		return
	}
	s.flush = time.AfterFunc(domain.JobLogFlushInterval, s.flushPending)
}

func (s *LogSink) flushPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flush = nil
	if s.file == nil {
		return
	}
	if err := s.buf.Flush(); err != nil {
		s.disable()
	}
}

func (s *LogSink) stopFlush() {
	if s.flush == nil {
		return
	}
	s.flush.Stop()
	s.flush = nil
}

func (s *LogSink) disable() {
	s.stopFlush()
	if s.file != nil {
		s.file.Close()
		s.file = nil
	}
	s.buf = nil
}

func removeBackups(path string) {
	for rank := 1; rank < domain.JobLogMaxFiles; rank++ {
		os.Remove(backupPath(path, rank))
	}
}

func backupPath(path string, rank int) string {
	return path + "." + strconv.Itoa(rank)
}

type TailParams struct {
	LogDir string
	Job    string
	Lines  int
}

// TailJobLog returns the last lines persisted for a job, reaching into the
// rotated backups when the active file is too short to fill the request. A job
// that never wrote a log reads as no lines, not as an error.
func TailJobLog(params TailParams) ([]string, error) {
	if params.LogDir == "" || params.Lines <= 0 {
		return nil, nil
	}

	path := JobLogPath(JobLogPathParams{LogDir: params.LogDir, Job: params.Job})
	if path == "" {
		return nil, nil
	}
	var lines []string
	for rank := 0; rank < domain.JobLogMaxFiles && len(lines) < params.Lines; rank++ {
		file := path
		if rank > 0 {
			file = backupPath(path, rank)
		}
		content, err := os.ReadFile(file)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read job log: %w", err)
		}
		lines = append(splitLogLines(content), lines...)
	}

	if len(lines) > params.Lines {
		return lines[len(lines)-params.Lines:], nil
	}
	return lines, nil
}

// PurgeWorktreeLogs deletes a worktree's whole log directory. Idempotent: an
// already-absent directory is a success.
func PurgeWorktreeLogs(logDir string) error {
	if logDir == "" {
		return nil
	}
	return os.RemoveAll(logDir)
}

func splitLogLines(content []byte) []string {
	trimmed := strings.TrimRight(string(content), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// LoggedJobs names the jobs that have left output in a worktree's log
// directory. It is the trace half of what a surface shows: the daemon's index
// says what lives, this says what ran, and a job in neither was never started
// here and has nothing to show for itself.
//
// An unreadable directory answers nothing rather than failing: a surface that
// cannot list the logs still has the index to draw, and refusing to draw at all
// would be the worse answer.
func LoggedJobs(logDir string) map[string]bool {
	if logDir == "" {
		return nil
	}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil
	}

	logged := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if job := rules.JobFromLogFileName(entry.Name()); job != "" {
			logged[job] = true
		}
	}
	return logged
}

type PruneLogsParams struct {
	LogDir string
	// Keep names the jobs whose logs survive: the ones this run is starting, and
	// the ones already up in this worktree. Anything else in the directory is a
	// previous run's — another profile's jobs, or a job run.toml no longer even
	// declares — and is what made "this job has a log here" mean "this job ran
	// here at some point in the last fortnight".
	Keep map[string]bool
}

// PruneJobLogs makes the log directory hold one run, the way OpenLogSink makes
// each file hold one (LUC-198): starting a run drops the logs of the jobs it is
// not starting. Without it the directory only ever grew, and every surface
// reading it showed the worktree's archaeology rather than its state.
//
// A job that is up is kept whatever the run is starting: its sink is writing to
// that file right now, and deleting it under a live job loses the output of
// something nobody asked to stop.
//
// Best effort, like the rest of this store: a log that cannot be removed is not
// a reason to refuse a run.
func PruneJobLogs(params PruneLogsParams) {
	if params.LogDir == "" {
		return
	}
	entries, err := os.ReadDir(params.LogDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		job := rules.JobFromLogFileName(entry.Name())
		// A file this store did not write is left alone: it is not ours to judge.
		if job == "" || params.Keep[job] {
			continue
		}
		os.Remove(filepath.Join(params.LogDir, entry.Name()))
	}
}
