package rules_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestShortPortName(t *testing.T) {
	cases := map[string]string{
		"REDIS_PORT":         "redis",
		"MINIO_CONSOLE_PORT": "minio-console",
		"PORT":               "port",
		"HTTP":               "http",
	}
	for in, want := range cases {
		if got := rules.ShortPortName(in); got != want {
			t.Errorf("ShortPortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReachSummaryCarriesOneFragment(t *testing.T) {
	url := domain.JobURLEntry{Job: "web", URL: "http://web.main.app.localhost:11080"}
	cases := []struct {
		name  string
		entry domain.ReachEntry
		want  string
	}{
		{"one url", domain.ReachEntry{URLs: []domain.JobURLEntry{url}}, url.URL},
		{"several urls", domain.ReachEntry{URLs: []domain.JobURLEntry{url, url}}, "2 urls"},
		{"one port", domain.ReachEntry{Ports: []domain.NamedPort{{Name: "POSTGRES_PORT", Port: 5432}}}, ":5432"},
		{"several ports", domain.ReachEntry{Ports: []domain.NamedPort{{Port: 1}, {Port: 2}, {Port: 3}}}, "3 ports"},
		{"nothing", domain.ReachEntry{Job: "migrate"}, ""},
	}
	for _, c := range cases {
		if got := rules.ReachSummary(c.entry); got != c.want {
			t.Errorf("%s: ReachSummary = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestReachLinesPutURLsFirstAndNameARunnersApps(t *testing.T) {
	lines := rules.ReachLines(rules.ReachLinesParams{Entries: []domain.ReachEntry{
		{Job: "postgres", Ports: []domain.NamedPort{{Name: "POSTGRES_PORT", Port: 5432}}, Namespace: "app_main"},
		{Job: "dev:shop", URLs: []domain.JobURLEntry{
			{Job: "shop-web", URL: "http://shop-web.main.app.localhost"},
			{Job: "shop-api", URL: "http://shop-api.main.app.localhost"},
		}},
		{Job: "migrate"},
	}})

	want := []string{
		"shop-web  http://shop-web.main.app.localhost",
		"shop-api  http://shop-api.main.app.localhost",
		"postgres  :5432 · app_main",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestReachLinesWrapManyPortsInColumns(t *testing.T) {
	lines := rules.ReachLines(rules.ReachLinesParams{
		Width: 50,
		Entries: []domain.ReachEntry{{Job: "compose", Ports: rules.NamedPorts(map[string]int{
			"REDIS_PORT": 6379, "MINIO_PORT": 9000, "MINIO_CONSOLE_PORT": 9001, "ADMINER_PORT": 8080,
		})}},
	})

	if len(lines) != 2 {
		t.Fatalf("lines = %q, want the four ports wrapped over two lines at width 50", lines)
	}
	if !strings.HasPrefix(lines[0], "compose  adminer :8080") || !strings.HasPrefix(lines[1], "         ") {
		t.Errorf("lines = %q, want the label once and the next line aligned under the first cell", lines)
	}
	for _, line := range lines {
		if len([]rune(line)) > 50 {
			t.Errorf("line %q is wider than 50", line)
		}
	}
}

func TestReachLinesSayNothingWhenNothingIsReachable(t *testing.T) {
	if lines := rules.ReachLines(rules.ReachLinesParams{Entries: []domain.ReachEntry{{Job: "migrate"}}}); lines != nil {
		t.Errorf("lines = %q, want none", lines)
	}
}

func TestReachLinesBalanceTheWrappedPorts(t *testing.T) {
	ports := map[string]int{}
	for i, name := range []string{"A_PORT", "B_PORT", "C_PORT", "D_PORT", "E_PORT", "F_PORT"} {
		ports[name] = 9000 + i
	}
	lines := rules.ReachLines(rules.ReachLinesParams{
		Width:   60,
		Entries: []domain.ReachEntry{{Job: "compose", Ports: rules.NamedPorts(ports)}},
	})

	if len(lines) != 2 || strings.Count(lines[0], ":") != 3 || strings.Count(lines[1], ":") != 3 {
		t.Errorf("lines = %q, want six ports as three and three", lines)
	}
}

// A shared service this worktree only holds is told apart: stopping the
// worktree leaves it running, and that is the one thing a reader must see.
func TestReachSectionsSetTheSharedServicesApart(t *testing.T) {
	sections := rules.ReachSections(rules.ReachLinesParams{Entries: []domain.ReachEntry{
		{Job: "docker-compose", Ports: []domain.NamedPort{{Name: "REDIS_PORT", Port: 6389}, {Name: "MINIO_PORT", Port: 9010}}},
		{Job: "postgres", Ports: []domain.NamedPort{{Name: "POSTGRES_PORT", Port: 5432}}, Namespace: "app_feat", SharedIn: "main"},
	}})

	if len(sections) != 2 {
		t.Fatalf("sections = %+v, want this worktree's jobs then main's", sections)
	}
	if sections[0].Title != domain.ReachTitle || !strings.HasPrefix(sections[0].Lines[0], "docker-compose") {
		t.Errorf("first section = %+v, want the worktree's own jobs", sections[0])
	}
	if sections[1].Title != "Shared, running in main" || sections[1].Lines[0] != "postgres        :5432 · app_feat" {
		t.Errorf("second section = %+v, want postgres under main, aligned with the first", sections[1])
	}
}
