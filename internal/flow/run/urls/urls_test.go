package urls_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/urls"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

var jobs = []domain.JobConfig{
	{Name: "api", Kind: domain.JobKindService, Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
	{Name: "worker", Kind: domain.JobKindService},
}

func context(t *testing.T) flow.Context {
	t.Helper()
	repo, err := filepath.EvalSymlinks(gittest.InitRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	proxy := 8480
	enabled := true
	return flow.Context{
		ProjectDir: repo,
		StateDir:   filepath.Join(repo, ".git", "wtm"),
		Config:     domain.Config{Global: domain.GlobalConfig{Proxy: domain.ProxyConfig{Port: proxy, Enabled: &enabled}}},
	}
}

func onlyEntry(t *testing.T, reader urls.Reader, dir string) domain.JobURLEntry {
	t.Helper()
	entries, err := reader.In(t.Context(), dir)
	if err != nil {
		t.Fatalf("In: %v", err)
	}
	if len(entries) != 1 || entries[0].Job != "api" {
		t.Fatalf("entries = %+v, want api alone: worker publishes nothing", entries)
	}
	return entries[0]
}

// A published name is served where the daemon says its proxy answers, under
// the job's own host.
func TestAJobIsReachedByItsNameWhereTheProxyAnswers(t *testing.T) {
	daemon := processtest.Serve(t, nil)
	daemon.ProxyPublicPort = 8480
	ctx := context(t)

	reader := urls.Open(urls.Params{Context: ctx, Config: domain.RunConfig{Jobs: jobs}})

	if !reader.Serving() {
		t.Fatal("Serving() = false with the proxy up")
	}
	entry := onlyEntry(t, reader, ctx.ProjectDir)
	if !strings.Contains(entry.URL, "api.") || !strings.Contains(entry.URL, ":8480") {
		t.Errorf("url = %s, want api's name on the proxy's port", entry.URL)
	}
}

// --raw asks for the port the .env answers on, whatever the addressing says.
func TestRawIsTheJobsOwnPort(t *testing.T) {
	processtest.Serve(t, nil).ProxyPublicPort = 8480
	ctx := context(t)

	reader := urls.Open(urls.Params{Context: ctx, Config: domain.RunConfig{Jobs: jobs}, Raw: true})

	if reader.Serving() {
		t.Error("Serving() = true under --raw")
	}
	if entry := onlyEntry(t, reader, ctx.ProjectDir); !strings.HasSuffix(entry.URL, ":3000") {
		t.Errorf("url = %s, want the job's own port", entry.URL)
	}
}

func TestPortsAddressingPublishesNoName(t *testing.T) {
	globaldir.Isolate(t)
	ctx := context(t)

	reader := urls.Open(urls.Params{Context: ctx, Config: domain.RunConfig{Jobs: jobs, Addressing: domain.AddressingPorts}})

	if reader.Serving() {
		t.Error("Serving() = true under ports addressing")
	}
	if entry := onlyEntry(t, reader, ctx.ProjectDir); !strings.HasSuffix(entry.URL, ":3000") {
		t.Errorf("url = %s, want the job's own port", entry.URL)
	}
}

// A linked worktree's addresses are its own: its ordinal shifts every port off
// the main checkout's, so the two never answer on the same one.
func TestALinkedWorktreeIsReachedOnItsOwnPort(t *testing.T) {
	globaldir.Isolate(t)
	ctx := context(t)
	dir := filepath.Join(t.TempDir(), "feature")
	gittest.Git(t, ctx.ProjectDir, "worktree", "add", "-b", "feature", dir)
	reader := urls.Open(urls.Params{Context: ctx, Config: domain.RunConfig{Jobs: jobs, Addressing: domain.AddressingPorts}})

	main := onlyEntry(t, reader, ctx.ProjectDir)
	linked := onlyEntry(t, reader, dir)

	if linked.URL == main.URL || strings.HasSuffix(linked.URL, ":3000") {
		t.Errorf("linked = %s, main = %s, want the linked worktree shifted off 3000", linked.URL, main.URL)
	}
}
