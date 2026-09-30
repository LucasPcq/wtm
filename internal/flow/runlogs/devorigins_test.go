package runlogs_test

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func runNext(t *testing.T, params runlogs.RunParams) *runlogstest.Sink {
	t.Helper()
	sink := &runlogstest.Sink{}
	params.Sink = sink
	params.Service = &runlogstest.Service{Ports: map[string]map[string]int{"web": {"PORT": 3010}}, ProxyPort: params.ProxyPort}
	params.Jobs = []domain.JobConfig{{
		Name:  "web",
		Kind:  domain.JobKindService,
		Cmd:   "pnpm run dev --port ${PORT}",
		Ports: map[string]int{"PORT": 3000},
		URL:   &domain.JobURLConfig{Port: "PORT"},
	}}
	params.Env = map[string]string{domain.EnvWorktree: "feat"}
	if _, err := runlogs.Run(context.Background(), params); err != nil {
		t.Fatalf("Run: %v", err)
	}
	return sink
}

func fixesFor(sink *runlogstest.Sink) []domain.DevOriginFix {
	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseStarted {
			return e.DevOrigins
		}
	}
	return nil
}

func TestRunReportsAMissingAllowedDevOrigins(t *testing.T) {
	sink := runNext(t, runlogs.RunParams{
		Project:   "myapp",
		ProxyPort: 4000,
		NextConfig: func(domain.JobConfig) (string, string) {
			return "apps/web/next.config.ts", "export default { reactStrictMode: true }\n"
		},
	})

	fixes := fixesFor(sink)
	if len(fixes) != 1 {
		t.Fatalf("fixes = %v, want the one line the project is missing", fixes)
	}
	if !strings.Contains(fixes[0].Line, domain.DevOriginsKey) || !strings.Contains(fixes[0].Line, "apps/web/next.config.ts") {
		t.Errorf("line = %q, want the option and the file to edit named", fixes[0].Line)
	}
}

func TestRunSaysNothingWhenTheProxyIsOff(t *testing.T) {
	// Under its own port there is no other host to allow, so the warning would
	// be noise about a problem the run does not have.
	sink := runNext(t, runlogs.RunParams{
		Project: "myapp",
		NextConfig: func(domain.JobConfig) (string, string) {
			return "apps/web/next.config.ts", "export default {}\n"
		},
	})

	if fixes := fixesFor(sink); len(fixes) != 0 {
		t.Errorf("fixes = %v, want none", fixes)
	}
}

func TestRunSaysNothingWhenAllowedDevOriginsIsAlreadyThere(t *testing.T) {
	sink := runNext(t, runlogs.RunParams{
		Project:   "myapp",
		ProxyPort: 4000,
		NextConfig: func(domain.JobConfig) (string, string) {
			return "apps/web/next.config.ts", `export default { allowedDevOrigins: ["*.localhost:4000"] }` + "\n"
		},
	})

	if fixes := fixesFor(sink); len(fixes) != 0 {
		t.Errorf("fixes = %v, want none", fixes)
	}
}

// The daemon is the authority on whether the proxy is up. A run that asked for
// one and did not get it must not announce a name nothing serves — and must say
// why, once, because a forked daemon's stderr goes nowhere.
func TestRunFallsBackAndExplainsWhenTheProxyCouldNotBind(t *testing.T) {
	sink := &runlogstest.Sink{}
	service := &runlogstest.Service{Ports: map[string]map[string]int{"web": {"PORT": 3010}}}

	if _, err := runlogs.Run(context.Background(), runlogs.RunParams{
		Service: service,
		Sink:    sink,
		Jobs: []domain.JobConfig{
			{Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 3000}, URL: &domain.JobURLConfig{Port: "PORT"}},
			{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev", Ports: map[string]int{"PORT": 4000}, URL: &domain.JobURLConfig{Port: "PORT"}},
		},
		Env:       map[string]string{domain.EnvWorktree: "feat"},
		Project:   "myapp",
		ProxyPort: 4000,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var notices int
	for _, e := range sink.Events {
		if e.Phase == runlogs.PhaseNotice {
			notices++
			if !strings.Contains(e.Notice, "4000") {
				t.Errorf("notice = %q, want the port it could not take named", e.Notice)
			}
		}
		if e.Phase == runlogs.PhaseStarted && strings.Contains(e.URL, ".localhost:") {
			t.Errorf("URL = %q, want the direct form when nothing serves the name", e.URL)
		}
	}
	// Two jobs would repeat one fact about the run.
	if notices != 1 {
		t.Errorf("notices = %d, want exactly one for the whole run", notices)
	}
}
