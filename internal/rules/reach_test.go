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

func TestReachBlockPutURLsFirstAndNameARunnersApps(t *testing.T) {
	lines := soleWorktreeReach([]domain.ReachEntry{
		{Job: "postgres", Ports: []domain.NamedPort{{Name: "POSTGRES_PORT", Port: 5432}}, Namespace: "app_main"},
		{Job: "dev:shop", URLs: []domain.JobURLEntry{
			{Job: "shop-web", URL: "http://shop-web.main.app.localhost"},
			{Job: "shop-api", URL: "http://shop-api.main.app.localhost"},
		}},
		{Job: "migrate"},
	})

	want := []string{
		"shop-web  http://shop-web.main.app.localhost",
		"shop-api  http://shop-api.main.app.localhost",
		"postgres  :5432 · app_main",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestReachBlockListManyPortsOnePerLine(t *testing.T) {
	lines := soleWorktreeReach([]domain.ReachEntry{{Job: "compose", Ports: rules.NamedPorts(map[string]int{
		"REDIS_PORT": 6379, "MINIO_PORT": 9000, "MINIO_CONSOLE_PORT": 9001, "ADMINER_PORT": 8080,
	})}})

	want := []string{
		"compose  adminer        :8080",
		"         minio-console  :9001",
		"         minio          :9000",
		"         redis          :6379",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("lines =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestReachBlockSayNothingWhenNothingIsReachable(t *testing.T) {
	if lines := soleWorktreeReach([]domain.ReachEntry{{Job: "migrate"}}); lines != nil {
		t.Errorf("lines = %q, want none", lines)
	}
}

// A shared service is told apart and comes first: stopping the worktree leaves
// it running, and that is the one thing a reader must see.
func TestReachBlockSetsTheSharedServicesApart(t *testing.T) {
	sections := rules.ReachBlock(rules.ReachBlockParams{Worktrees: []rules.ReachWorktree{{
		Name: "feat",
		Entries: []domain.ReachEntry{
			{Job: "docker-compose", Ports: []domain.NamedPort{{Name: "REDIS_PORT", Port: 6389}, {Name: "MINIO_PORT", Port: 9010}}},
			{Job: "postgres", Ports: []domain.NamedPort{{Name: "POSTGRES_PORT", Port: 5432}}, Namespace: "app_feat", SharedIn: "main"},
		},
	}}})

	if len(sections) != 2 {
		t.Fatalf("sections = %+v, want main's shared service then the worktree's jobs", sections)
	}
	if !sections[0].Shared || sections[0].Title != "Shared, running in main" || sections[0].Lines[0] != "postgres        :5432 · app_feat" {
		t.Errorf("first section = %+v, want postgres under main, aligned with the second", sections[0])
	}
	if sections[1].Title != domain.ReachTitle || !strings.HasPrefix(sections[1].Lines[0], "docker-compose") {
		t.Errorf("second section = %+v, want the worktree's own jobs", sections[1])
	}
}

// From main, a service main joined runs here: it is shared with the others,
// not "running in main".
func TestReachBlockSaysWhoSharesAServiceRunningHere(t *testing.T) {
	sections := rules.ReachBlock(rules.ReachBlockParams{Worktrees: []rules.ReachWorktree{{
		Name:    "main",
		Entries: []domain.ReachEntry{{Job: "postgres", Ports: []domain.NamedPort{{Port: 5432}}, SharedIn: "main"}},
	}}})

	if len(sections) != 1 || sections[0].Title != domain.ReachSharedHereTitle {
		t.Errorf("sections = %+v, want the service shared with other worktrees", sections)
	}
}

// Above several worktrees a shared service is listed once, each holder's
// namespace under it, and every worktree's own jobs under its own name.
func TestReachBlockListsASharedServiceOnceAboveSeveralWorktrees(t *testing.T) {
	postgres := func(namespace string) domain.ReachEntry {
		return domain.ReachEntry{Job: "postgres", Ports: []domain.NamedPort{{Port: 5432}}, Namespace: namespace, SharedIn: "main"}
	}
	web := func(worktree string) domain.ReachEntry {
		return domain.ReachEntry{Job: "web", URLs: []domain.JobURLEntry{{Job: "web", URL: "http://web." + worktree}}}
	}
	sections := rules.ReachBlock(rules.ReachBlockParams{Worktrees: []rules.ReachWorktree{
		{Name: "main", Entries: []domain.ReachEntry{web("main"), postgres("app_main")}},
		{Name: "feat", Entries: []domain.ReachEntry{web("feat"), postgres("app_feat")}},
	}})

	if len(sections) != 3 {
		t.Fatalf("sections = %+v, want the shared service once, then each worktree", sections)
	}
	want := []string{
		"postgres  :5432",
		"          main  app_main",
		"          feat  app_feat",
	}
	if strings.Join(sections[0].Lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("shared lines =\n%s\nwant\n%s", strings.Join(sections[0].Lines, "\n"), strings.Join(want, "\n"))
	}
	if sections[1].Title != "main" || sections[2].Title != "feat" || sections[2].Worktree != "feat" {
		t.Errorf("sections = %+v, want main then feat, titled by name", sections)
	}
}

func soleWorktreeReach(entries []domain.ReachEntry) []string {
	var lines []string
	for _, section := range rules.ReachBlock(rules.ReachBlockParams{Worktrees: []rules.ReachWorktree{{Name: "main", Entries: entries}}}) {
		lines = append(lines, section.Lines...)
	}
	return lines
}
