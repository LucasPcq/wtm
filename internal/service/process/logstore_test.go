package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func readLog(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func TestJobLogPathEscapesTheJobName(t *testing.T) {
	got := JobLogPath(JobLogPathParams{LogDir: "/state/logs/feat", Job: "web/api dev"})
	want := filepath.Join("/state/logs/feat", "web%2Fapi%20dev.log")
	if got != want {
		t.Errorf("JobLogPath = %q, want %q", got, want)
	}
}

// A job with no file of its own persists nothing rather than sharing another's.
func TestJobLogPathIsEmptyForAnUnnameableJob(t *testing.T) {
	if got := JobLogPath(JobLogPathParams{LogDir: "/state/logs/feat", Job: ""}); got != "" {
		t.Errorf("JobLogPath = %q, want none", got)
	}

	if _, err := OpenLogSink(LogSinkParams{LogDir: t.TempDir(), Job: ""}); err == nil {
		t.Error("OpenLogSink opened a log for a job it cannot name")
	}
}

// Two jobs whose names differ keep two files, and starting one leaves the
// other's alone (LUC-198).
func TestLogSinkKeepsCollidingJobNamesApart(t *testing.T) {
	dir := t.TempDir()

	spaced, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web api"})
	if err != nil {
		t.Fatalf("open \"web api\": %v", err)
	}
	spaced.Write([]byte("spaced line\n"))
	spaced.Close()

	colon, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web:api"})
	if err != nil {
		t.Fatalf("open \"web:api\": %v", err)
	}
	colon.Write([]byte("colon line\n"))
	colon.Close()

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web api", Lines: 10})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "spaced line") {
		t.Errorf("\"web api\" reads back %v, want only its own line", lines)
	}
}

func TestLogSinkWritesSanitizedTimestampedLines(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "feat")
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	sink.Write([]byte("\x1b[32mready\x1b[0m\r\n"))
	sink.Write([]byte("building 10%\rbuilding 100%\nhalf a "))
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	lines := strings.Split(strings.TrimRight(readLog(t, filepath.Join(dir, "web.log")), "\n"), "\n")
	want := []string{"ready", "building 100%", "half a"}
	if len(lines) != len(want) {
		t.Fatalf("log has %d lines (%v), want %d", len(lines), lines, len(want))
	}
	for i, line := range lines {
		stamp, text, found := strings.Cut(line, domain.JobLogSeparator)
		if !found {
			t.Fatalf("line %q carries no timestamp", line)
		}
		if _, err := time.Parse(domain.JobLogTimestampLayout, stamp); err != nil {
			t.Errorf("line %q: %v", line, err)
		}
		if text != want[i] {
			t.Errorf("line %d = %q, want %q", i, text, want[i])
		}
	}
}

// The dashboard's tail and `wtm run logs` read this file while the job is still
// running, so batching the writes may cost the reader the flush interval and
// nothing more — certainly not the end of the job.
func TestLogSinkMakesALineReadableWhileTheJobRuns(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	t.Cleanup(func() { sink.Close() })

	sink.Write([]byte("listening on 3000\n"))

	path := filepath.Join(dir, "web.log")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(readLog(t, path), "listening on 3000") {
			return
		}
		time.Sleep(domain.JobLogFlushInterval / 4)
	}
	t.Fatal("the line never reached the file: a reader tailing it would wait for the job to end")
}

func TestLogSinkStartsAnEmptyLogOnEveryRun(t *testing.T) {
	dir := t.TempDir()

	first, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open first sink: %v", err)
	}
	first.Write([]byte("run one\n"))
	first.Close()

	second, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open second sink: %v", err)
	}
	second.Write([]byte("run two\n"))
	second.Close()

	content := readLog(t, filepath.Join(dir, "web.log"))
	if strings.Contains(content, "run one") {
		t.Errorf("a restarted job should not read back the previous run, got:\n%s", content)
	}
	if !strings.Contains(content, "run two") {
		t.Errorf("the current run is missing from its own log, got:\n%s", content)
	}
}

// The backups matter as much as the active file: TailJobLog reaches into them
// whenever the current run is shorter than the tail it was asked for, which is
// how a job that just restarted read back a days-old run (LUC-198).
func TestLogSinkDropsTheBackupsOfThePreviousRun(t *testing.T) {
	dir := t.TempDir()

	first, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 200})
	if err != nil {
		t.Fatalf("open first sink: %v", err)
	}
	for i := 0; i < 60; i++ {
		first.Write([]byte("run one\n"))
	}
	first.Close()

	if _, err := os.Stat(filepath.Join(dir, "web.log.1")); err != nil {
		t.Fatalf("the first run should have rotated at least once: %v", err)
	}

	second, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open second sink: %v", err)
	}
	second.Write([]byte("run two\n"))
	second.Close()

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 1000})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	for _, line := range lines {
		if strings.Contains(line, "run one") {
			t.Fatalf("tail reached into the previous run's backups: %v", lines)
		}
	}
}

// Only this job's files go: two jobs share one directory, and a restart of one
// must not blank the other.
func TestLogSinkLeavesOtherJobsAlone(t *testing.T) {
	dir := t.TempDir()

	other, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "api"})
	if err != nil {
		t.Fatalf("open api sink: %v", err)
	}
	other.Write([]byte("api line\n"))
	other.Close()

	web, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open web sink: %v", err)
	}
	web.Write([]byte("web line\n"))
	web.Close()

	if content := readLog(t, filepath.Join(dir, "api.log")); !strings.Contains(content, "api line") {
		t.Errorf("api's log was cleared by web starting, got:\n%s", content)
	}
}

func TestLogSinkRotatesAtTheThresholdAndKeepsMaxFiles(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 200})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	// Each line is ~30 bytes once stamped, so this crosses the threshold several
	// times and must never leave more than domain.JobLogMaxFiles files behind.
	for i := 0; i < 60; i++ {
		sink.Write([]byte("line\n"))
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != domain.JobLogMaxFiles {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("log dir holds %v, want %d files", names, domain.JobLogMaxFiles)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("stat %s: %v", entry.Name(), err)
		}
		if info.Size() > 200+64 {
			t.Errorf("%s is %d bytes, want it bounded by the rotation threshold", entry.Name(), info.Size())
		}
	}
}

func TestTailJobLogReturnsTheLastLines(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	for i := 0; i < 10; i++ {
		sink.Write([]byte("line " + string(rune('0'+i)) + "\n"))
	}
	sink.Close()

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 3})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("tail returned %d lines, want 3", len(lines))
	}
	if !strings.HasSuffix(lines[2], "line 9") {
		t.Errorf("last line = %q, want the most recent one", lines[2])
	}
}

func logTexts(t *testing.T, lines []string) []string {
	t.Helper()
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		stamp, text, found := strings.Cut(line, domain.JobLogSeparator)
		if !found {
			t.Fatalf("line %q carries no timestamp", line)
		}
		if _, err := time.Parse(domain.JobLogTimestampLayout, stamp); err != nil {
			t.Fatalf("line %q: %v", line, err)
		}
		out = append(out, text)
	}
	return out
}

// writeNumberedLines fills a job log with `count` lines, rotating every four.
func writeNumberedLines(t *testing.T, dir string, count int) {
	t.Helper()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 120})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	for i := 0; i < count; i++ {
		sink.Write([]byte(fmt.Sprintf("line %d\n", i)))
	}
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
}

func TestTailJobLogReachesIntoTheRotatedBackups(t *testing.T) {
	dir := t.TempDir()
	writeNumberedLines(t, dir, 8)

	backup := readLog(t, filepath.Join(dir, "web.log.1"))
	if !strings.Contains(backup, "line 2") {
		t.Fatalf("the fixture no longer rotates line 2 out of the active file:\n%s", backup)
	}
	if strings.Contains(readLog(t, filepath.Join(dir, "web.log")), "line 2") {
		t.Fatal("the fixture leaves line 2 in the active file, so the tail proves nothing")
	}

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 6})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}

	want := []string{"line 2", "line 3", "line 4", "line 5", "line 6", "line 7"}
	if got := logTexts(t, lines); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tail = %v, want %v — the two oldest come from the rotated backup", got, want)
	}
}

func TestTailJobLogSpansEveryRotatedFileInOrder(t *testing.T) {
	dir := t.TempDir()
	writeNumberedLines(t, dir, 12)

	for _, name := range []string{"web.log", "web.log.1", "web.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("the fixture did not produce %s: %v", name, err)
		}
	}

	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 12})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}

	want := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		want = append(want, fmt.Sprintf("line %d", i))
	}
	if got := logTexts(t, lines); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("tail = %v, want the three files read oldest first: %v", got, want)
	}
}

func TestTailJobLogOnAMissingFile(t *testing.T) {
	lines, err := TailJobLog(TailParams{LogDir: t.TempDir(), Job: "never-ran", Lines: 10})
	if err != nil {
		t.Fatalf("tail on a missing log should not fail: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("lines = %v, want none", lines)
	}
}

func TestPurgeWorktreeLogsIsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "feat")
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}
	sink.Write([]byte("something\n"))
	sink.Close()

	if err := PurgeWorktreeLogs(dir); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("log dir still there after the purge (%v)", err)
	}
	if err := PurgeWorktreeLogs(dir); err != nil {
		t.Errorf("purging an absent log dir should succeed, got %v", err)
	}
	if err := PurgeWorktreeLogs(""); err != nil {
		t.Errorf("purging an unresolved log dir should succeed, got %v", err)
	}
}

func TestLogSinkBoundsTheTailOfALineThatNeverEnds(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web"})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	for i := 0; i < 20000; i++ {
		sink.Write([]byte("\r\x1b[K[" + strings.Repeat("=", i%20) + "] building packages"))
	}

	sink.mu.Lock()
	held := len(sink.pending)
	sink.mu.Unlock()
	if held > 64 {
		t.Errorf("the sink holds %d bytes of unterminated line, want only the last frame", held)
	}

	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
	lines, err := TailJobLog(TailParams{LogDir: dir, Job: "web", Lines: 10})
	if err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("log holds %d lines, want the redraw's last frame only", len(lines))
	}
	if !strings.HasSuffix(lines[0], "[===================] building packages") {
		t.Errorf("line = %q, want the last frame of the redraw", lines[0])
	}
}

func TestLogSinkRetiresARecordBiggerThanTheThreshold(t *testing.T) {
	dir := t.TempDir()
	sink, err := OpenLogSink(LogSinkParams{LogDir: dir, Job: "web", MaxBytes: 100})
	if err != nil {
		t.Fatalf("open sink: %v", err)
	}

	sink.Write([]byte(strings.Repeat("x", 50000) + "\n"))

	active := filepath.Join(dir, "web.log")
	info, err := os.Stat(active)
	if err != nil {
		t.Fatalf("stat active log: %v", err)
	}
	if info.Size() > 100 {
		t.Errorf("active log is %d bytes right after an oversized record, want it bounded by the 100-byte threshold", info.Size())
	}
	if _, err := os.Stat(active + ".1"); err != nil {
		t.Errorf("the oversized record was not retired to a backup: %v", err)
	}

	sink.Write([]byte("after\n"))
	if err := sink.Close(); err != nil {
		t.Fatalf("close sink: %v", err)
	}
	if !strings.Contains(readLog(t, active), "after") {
		t.Error("the line written after the oversized one is missing from the active log")
	}
}

// A log directory holds one run, the way a log file holds one (LUC-198).
// Without this it only ever grew: a worktree used for a fortnight held the logs
// of every profile ever started in it, plus jobs run.toml no longer declares,
// and every surface reading it showed that instead of what was running.
func TestPruneJobLogsLeavesTheRunAndDropsWhatCameBefore(t *testing.T) {
	dir := t.TempDir()
	for _, job := range []string{"docker-compose", "dev:crm", "dev:shop", "crm-admin-dev", "build"} {
		path := JobLogPath(JobLogPathParams{LogDir: dir, Job: job})
		if err := os.WriteFile(path, []byte("line\n"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", job, err)
		}
	}
	// A rotated backup belongs to its job and goes with it.
	if err := os.WriteFile(backupPath(JobLogPath(JobLogPathParams{LogDir: dir, Job: "build"}), 1), []byte("old\n"), 0o644); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	// Something this store never wrote is not ours to judge.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("seed stranger: %v", err)
	}

	PruneJobLogs(PruneLogsParams{
		LogDir: dir,
		// The run starts dev:crm; docker-compose is already up beside it.
		Keep: map[string]bool{"dev:crm": true, "docker-compose": true},
	})

	left := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		left[entry.Name()] = true
	}

	for _, name := range []string{"dev:crm.log", "docker-compose.log", "notes.txt"} {
		if !left[name] {
			t.Errorf("%s was removed, want it kept", name)
		}
	}
	for _, name := range []string{"dev:shop.log", "crm-admin-dev.log", "build.log", "build.log.1"} {
		if left[name] {
			t.Errorf("%s survived, want a previous run's log dropped", name)
		}
	}
}
