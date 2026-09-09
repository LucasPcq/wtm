package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TestWriteRunConfigJSON_LowercaseKeys ensures the wire format uses lowercase
// field names matching run.schema.json ("job", "profile", "name", "kind"…).
// This is a breaking change from the pre-json-tag behaviour where Go's default
// encoder used exported field names ("Jobs", "Name", etc.).
func TestWriteRunConfigJSON_LowercaseKeys(t *testing.T) {
	cfg := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev", Cwd: "."},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "full", Jobs: []string{"dev"}, Default: true},
		},
	}

	var buf bytes.Buffer
	if err := WriteRunConfigJSON(&buf, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := buf.String()

	// Verify lowercase JSON keys.
	for _, key := range []string{`"job"`, `"profile"`, `"name"`, `"kind"`, `"cmd"`, `"cwd"`, `"jobs"`, `"default"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("expected key %s in JSON output\noutput: %s", key, raw)
		}
	}

	// Verify PascalCase keys are absent.
	for _, key := range []string{`"Jobs"`, `"Profiles"`, `"Name"`, `"Kind"`, `"Cmd"`, `"Cwd"`} {
		if strings.Contains(raw, key) {
			t.Errorf("unexpected PascalCase key %s in JSON output\noutput: %s", key, raw)
		}
	}
}

// TestWriteRunConfigJSON_Roundtrip verifies that export → unmarshal is lossless.
func TestWriteRunConfigJSON_Roundtrip(t *testing.T) {
	orig := domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "dev", Kind: domain.JobKindService, Cmd: "pnpm dev", Cwd: "."},
			{Name: "build", Kind: domain.JobKindTask, Cmd: "pnpm build"},
		},
		Profiles: []domain.ProfileConfig{
			{Name: "ci", Jobs: []string{"build"}, Default: false},
		},
	}

	var buf bytes.Buffer
	if err := WriteRunConfigJSON(&buf, orig); err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got domain.RunConfig
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(got.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(got.Jobs))
	}
	if got.Jobs[0].Name != "dev" || got.Jobs[0].Kind != domain.JobKindService {
		t.Errorf("job[0] mismatch: %+v", got.Jobs[0])
	}
	if len(got.Profiles) != 1 || got.Profiles[0].Name != "ci" {
		t.Errorf("profile mismatch: %+v", got.Profiles)
	}
}

func TestWriteImportResultText(t *testing.T) {
	var buf bytes.Buffer
	WriteImportResultText(&buf, ImportResult{
		Jobs:     []string{"web", "api"},
		Profiles: []string{"front"},
		EnvPorts: 2,
	})

	out := buf.String()
	for _, want := range []string{"web", "api", "front", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output: %q", want, out)
		}
	}
}

func TestWriteImportResultJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteImportResultJSON(&buf, ImportResult{
		Jobs:     []string{"web", "api"},
		Profiles: []string{"front"},
		EnvPorts: 2,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got ImportResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Jobs) != 2 || got.Jobs[0] != "web" || got.Profiles[0] != "front" || got.EnvPorts != 2 {
		t.Errorf("round-trip lost content: %+v", got)
	}
	for _, key := range []string{`"jobs"`, `"profiles"`, `"env_ports"`} {
		if !strings.Contains(buf.String(), key) {
			t.Errorf("payload %s misses %s", buf.String(), key)
		}
	}
}

// Empty slices must reach a caller as [], never null: an agent iterating the
// field would break on null.
func TestWriteImportResultJSONNeverEmitsNull(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteImportResultJSON(&buf, ImportResult{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "null") {
		t.Errorf("payload %s carries null", buf.String())
	}
}

func TestWriteImportResultTextEmptyConfig(t *testing.T) {
	var buf bytes.Buffer
	WriteImportResultText(&buf, ImportResult{})
	if buf.Len() == 0 {
		t.Error("an empty payload still replaced the file: it must say so")
	}
}

func TestFormatRunningJobs_ShowsUptime(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	out := FormatRunningJobs(FormatRunningJobsParams{
		Jobs: []domain.JobInfo{{
			Name:      "dev",
			Kind:      domain.JobKindService,
			Status:    domain.JobStatusRunning,
			PID:       4242,
			WorkDir:   "/work/feat",
			StartedAt: now.Add(-5 * time.Minute),
		}},
		Now: now,
	})

	if !strings.Contains(out, "UPTIME") {
		t.Errorf("expected an UPTIME column, got:\n%s", out)
	}
	if !strings.Contains(out, "5m") {
		t.Errorf("expected the uptime 5m, got:\n%s", out)
	}
	if row := strings.Fields(strings.Split(out, "\n")[1]); len(row) != 6 {
		t.Errorf("expected 6 filled cells, got %q", row)
	}
}

// TestFormatRunningJobs_NoUptimeWithoutARunningStart pins the two cases where
// the column stays blank rather than counting: a job the daemon never spawned,
// and one whose run is over.
func TestFormatRunningJobs_NoUptimeWithoutARunningStart(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		status    domain.JobStatus
		startedAt time.Time
	}{
		{"jamais démarré", domain.JobStatusRunning, time.Time{}},
		{"arrêté", domain.JobStatusStopped, now.Add(-5 * time.Minute)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			job := domain.JobInfo{
				Name:      "dev",
				Kind:      domain.JobKindService,
				Status:    c.status,
				PID:       7,
				WorkDir:   "/work/feat",
				StartedAt: c.startedAt,
			}
			out := FormatRunningJobs(FormatRunningJobsParams{Jobs: []domain.JobInfo{job}, Now: now})
			row := strings.Fields(strings.Split(out, "\n")[1])
			if len(row) != 5 {
				t.Errorf("expected 5 filled cells (no uptime), got %q", row)
			}
		})
	}
}

func TestWriteRunningJobsJSON_ExposesStartAndExit(t *testing.T) {
	startedAt := time.Date(2026, 8, 20, 11, 0, 0, 0, time.UTC)
	code := 3

	var buf bytes.Buffer
	err := WriteRunningJobsJSON(&buf, []domain.JobInfo{
		{Name: "dev", Status: domain.JobStatusCrashed, StartedAt: startedAt, ExitCode: &code},
		{Name: "ghost", Status: domain.JobStatusStopped},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []domain.JobInfo
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got[0].StartedAt.Equal(startedAt) {
		t.Errorf("started_at = %v, want %v", got[0].StartedAt, startedAt)
	}
	if got[0].ExitCode == nil || *got[0].ExitCode != code {
		t.Errorf("exit_code = %v, want %d", got[0].ExitCode, code)
	}

	// A job the daemon never ran carries neither key rather than an epoch date
	// and a code that would read as a clean exit. Read as keys, not as text: the
	// payload is indented, so a literal search for `"exit_code":0` matches
	// nothing whatever the tags say — and a nil code never renders as 0 anyway.
	var payloads []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payloads); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"started_at", "exit_code"} {
		if value, present := payloads[1][key]; present {
			t.Errorf("%s = %v, want no key at all on a job the daemon never ran", key, value)
		}
	}
}

func TestWriteRunningJobsJSON_OmitsURLForAJobThatPublishesNone(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteRunningJobsJSON(&buf, []domain.JobInfo{
		{Name: "web", Status: domain.JobStatusRunning, URL: "http://localhost:3010"},
		{Name: "db", Status: domain.JobStatusRunning},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payloads []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payloads); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payloads[0]["url"] != "http://localhost:3010" {
		t.Errorf("url = %v, want the published address", payloads[0]["url"])
	}
	if value, present := payloads[1]["url"]; present {
		t.Errorf("url = %v, want no key at all on a job that publishes none", value)
	}
}

// A style given a string that ends in a line break sees two lines and pads the
// empty second one to the width of the first — a run of spaces landing in front
// of the first job.
func TestRunningJobsTableAlignsItsFirstRowWithTheRest(t *testing.T) {
	table := FormatRunningJobs(FormatRunningJobsParams{
		Jobs: []domain.JobInfo{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, PID: 4211, WorkDir: "/w/main"},
			{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning, PID: 4212, WorkDir: "/w/feat-a"},
		},
		Now: time.Now(),
	})

	lines := strings.Split(strings.TrimRight(ansi.Strip(table), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("table has %d lines, want a header and one row per job:\n%s", len(lines), table)
	}
	for index, line := range lines[1:] {
		if indentOf(line) != indentOf(lines[0]) {
			t.Errorf("row %d starts at column %d, the header at %d:\n%s",
				index, indentOf(line), indentOf(lines[0]), table)
		}
	}
}

func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// The declared ports come last and unaligned: a compose stack declaring seven
// of them used to push every command off the screen.
func TestFormatRunConfigShowsDeclaredPortsAfterTheCommand(t *testing.T) {
	out := ansi.Strip(FormatRunConfig(domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm run dev",
				Ports: map[string]int{"VITE_PORT": 5173}, URL: &domain.JobURLConfig{Port: "VITE_PORT"}},
			{Name: "worker", Kind: domain.JobKindService, Cmd: "pnpm run worker"},
		},
	}))

	if !strings.Contains(out, "VITE_PORT=5173") {
		t.Errorf("output does not name the declared port:\n%s", out)
	}
	if !strings.Contains(out, domain.RunListURLMark) {
		t.Errorf("output does not mark the published job:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "worker") && strings.HasSuffix(line, " ") {
			t.Errorf("a job with no port left trailing padding: %q", line)
		}
	}
}

// A shared job listed like every other read as one service per worktree, and
// its declared ports read as shifting — which is exactly what they do not do.
func TestFormatRunConfigMarksASharedJob(t *testing.T) {
	got := FormatRunConfig(domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Kind: domain.JobKindService, Cmd: "docker compose up db", Scope: domain.JobScopeShared},
		{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev"},
	}})

	lines := strings.Split(got, "\n")
	var dbLine, webLine string
	for _, line := range lines {
		if strings.Contains(line, "db ") || strings.Contains(line, "db\t") {
			dbLine = line
		}
		if strings.Contains(line, "web") {
			webLine = line
		}
	}
	if !strings.Contains(dbLine, domain.SharedJobTag) {
		t.Errorf("the shared job carries no marker:\n%s", got)
	}
	if strings.Contains(webLine, domain.SharedJobTag) {
		t.Errorf("a per-worktree job was marked shared:\n%s", got)
	}
}
