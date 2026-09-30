package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// The shape the whole feature is about: an API and a web app in one worktree,
// the API's own PORT bound by number, the web app's two settings addressed by
// name, and a database that has no name and never will.
func planFixture(addressing domain.Addressing, publicPort int, values map[string]string) domain.EnvPortPlan {
	api := domain.JobConfig{
		Name: "api-dev", Ports: map[string]int{"PORT": 4001},
		URL: &domain.JobURLConfig{Port: "PORT"},
	}
	web := domain.JobConfig{
		Name: "web-dev", Ports: map[string]int{"PORT": 5173},
		URL: &domain.JobURLConfig{Port: "PORT"},
	}
	db := domain.JobConfig{Name: "db", Ports: map[string]int{"POSTGRES_PORT": 5432}}

	links := []domain.EnvPortLink{
		{File: "apps/api/.env", Key: "PORT", Job: "api-dev", Port: "PORT"},
		{File: "apps/api/.env", Key: "CORS_ORIGIN", Job: "web-dev", Port: "PORT"},
		{File: "apps/api/.env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
		{File: "apps/web/.env", Key: "VITE_API_URL", Job: "api-dev", Port: "PORT"},
	}

	lines := map[string][]domain.EnvLine{}
	for _, link := range links {
		lines[link.File] = append(lines[link.File], domain.EnvLine{
			Kind: domain.EnvLinePair, Key: link.Key, Value: values[link.Key],
		})
	}

	return rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links: links,
		Bases: map[domain.PortRef]int{
			{Job: "api-dev", Name: "PORT"}:     4001,
			{Job: "web-dev", Name: "PORT"}:     5173,
			{Job: "db", Name: "POSTGRES_PORT"}: 5432,
		},
		Offset: 10,
		Lines:  lines,
		Origins: rules.OriginContext{
			Addressing: addressing,
			Jobs:       map[string]domain.JobConfig{"api-dev": api, "web-dev": web, "db": db},
			Worktree:   "feat-x",
			Project:    "monorepo",
			PublicPort: publicPort,
		},
	})
}

func entryFor(t *testing.T, plan domain.EnvPortPlan, key string) domain.EnvPortEntry {
	t.Helper()
	for _, e := range plan.Entries {
		if e.Key == key {
			return e
		}
	}
	t.Fatalf("no entry for %q", key)
	return domain.EnvPortEntry{}
}

var portedValues = map[string]string{
	"PORT":         "4001",
	"CORS_ORIGIN":  "http://localhost:5173",
	"DATABASE_URL": "postgres://wtm:wtm@localhost:5432/app",
	"VITE_API_URL": "http://localhost:4001",
}

func TestPlanEnvPortsNamesRewritesOnlyURLsOfPublishedJobs(t *testing.T) {
	plan := planFixture(domain.AddressingNames, 10080, portedValues)

	cases := []struct {
		key        string
		addressing domain.Addressing
		value      string
	}{
		{"PORT", domain.AddressingPorts, "4011"},
		{"CORS_ORIGIN", domain.AddressingNames, "http://web-dev.feat-x.monorepo.localhost:10080"},
		{"DATABASE_URL", domain.AddressingPorts, "postgres://wtm:wtm@localhost:5442/app"},
		{"VITE_API_URL", domain.AddressingNames, "http://api-dev.feat-x.monorepo.localhost:10080"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			e := entryFor(t, plan, tc.key)
			if e.Status != domain.EnvPortStatusRewrite {
				t.Fatalf("status %q", e.Status)
			}
			if e.Addressing != tc.addressing {
				t.Fatalf("addressing: got %q, want %q", e.Addressing, tc.addressing)
			}
			if e.NewValue != tc.value {
				t.Fatalf("value:\n got %q\nwant %q", e.NewValue, tc.value)
			}
		})
	}
}

// A published job's own PORT key is the case the issue's first criterion misses:
// the job publishes a name, yet the value must stay a bare number.
func TestPlanEnvPortsNamesLeavesBindingKeysAsNumbers(t *testing.T) {
	plan := planFixture(domain.AddressingNames, 10080, portedValues)
	if e := entryFor(t, plan, "PORT"); e.Addressing != domain.AddressingPorts {
		t.Fatalf("a binding key must keep its number, got %q", e.Addressing)
	}
}

func TestPlanEnvPortsNamesIsIdempotent(t *testing.T) {
	first := planFixture(domain.AddressingNames, 10080, portedValues)

	settled := map[string]string{}
	for _, e := range first.Entries {
		settled[e.Key] = e.NewValue
	}
	second := planFixture(domain.AddressingNames, 10080, settled)

	for _, e := range second.Entries {
		if e.Status != domain.EnvPortStatusUnchanged {
			t.Fatalf("%s: a second pass must settle, got %q (%q)", e.Key, e.Status, e.NewValue)
		}
	}
}

// Installing the redirection after the fact changes the public port, and the
// next pass has to drop the one already written.
func TestPlanEnvPortsNamesRecallsThePublicPort(t *testing.T) {
	ported := planFixture(domain.AddressingNames, 10080, portedValues)
	settled := map[string]string{}
	for _, e := range ported.Entries {
		settled[e.Key] = e.NewValue
	}

	plan := planFixture(domain.AddressingNames, domain.ProxyPrivilegedPort, settled)
	e := entryFor(t, plan, "VITE_API_URL")
	if e.NewValue != "http://api-dev.feat-x.monorepo.localhost" {
		t.Fatalf("got %q", e.NewValue)
	}
}

// A .env copied from another worktree carries that worktree's segment.
func TestPlanEnvPortsNamesCorrectsTheWorktreeSegment(t *testing.T) {
	plan := planFixture(domain.AddressingNames, 10080, map[string]string{
		"PORT":         "4001",
		"CORS_ORIGIN":  "http://web-dev.feat-y.monorepo.localhost:10080",
		"DATABASE_URL": "postgres://wtm:wtm@localhost:5432/app",
		"VITE_API_URL": "http://api-dev.feat-y.monorepo.localhost:10080",
	})
	e := entryFor(t, plan, "CORS_ORIGIN")
	if e.NewValue != "http://web-dev.feat-x.monorepo.localhost:10080" {
		t.Fatalf("got %q", e.NewValue)
	}
}

// Switching back to ports must undo wtm's own writing, not report it as noise.
func TestPlanEnvPortsSwitchBackToPorts(t *testing.T) {
	named := planFixture(domain.AddressingNames, 10080, portedValues)
	settled := map[string]string{}
	for _, e := range named.Entries {
		settled[e.Key] = e.NewValue
	}

	plan := planFixture(domain.AddressingPorts, 10080, settled)
	for _, tc := range []struct{ key, want string }{
		{"CORS_ORIGIN", "http://localhost:5183"},
		{"VITE_API_URL", "http://localhost:4011"},
	} {
		e := entryFor(t, plan, tc.key)
		if e.NewValue != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.key, e.NewValue, tc.want)
		}
	}
}

// A machine with the proxy off cannot honour what the project asked for.
func TestPlanEnvPortsNamesFallsBackWithNoProxy(t *testing.T) {
	plan := planFixture(domain.AddressingNames, 0, portedValues)
	for _, e := range plan.Entries {
		if e.Addressing != domain.AddressingPorts {
			t.Fatalf("%s: nothing serves names, expected a port", e.Key)
		}
	}
	if e := entryFor(t, plan, "VITE_API_URL"); e.NewValue != "http://localhost:4011" {
		t.Fatalf("got %q", e.NewValue)
	}
}

func TestPlanEnvPortsNamesReportsRefusals(t *testing.T) {
	plan := planFixture(domain.AddressingNames, 10080, map[string]string{
		"PORT":         "4001",
		"CORS_ORIGIN":  "https://localhost:5173",
		"DATABASE_URL": "postgres://wtm:wtm@localhost:5432/app",
		"VITE_API_URL": "https://api.staging.example.com",
	})

	if e := entryFor(t, plan, "CORS_ORIGIN"); e.Status != domain.EnvPortStatusSecureScheme {
		t.Fatalf("https must be refused, got %q", e.Status)
	}
	if e := entryFor(t, plan, "VITE_API_URL"); e.Status != domain.EnvPortStatusForeignHost {
		t.Fatalf("a foreign host must be reported, got %q", e.Status)
	}
	if len(rules.EnvPortAnomalies(plan)) != 2 {
		t.Fatalf("both refusals must reach the report, got %d", len(rules.EnvPortAnomalies(plan)))
	}
}

func TestEnvPortNotices(t *testing.T) {
	cases := []struct {
		name  string
		plan  domain.EnvPortPlan
		title string
	}{
		{
			name:  "an address written with a port says how to drop it",
			plan:  planFixture(domain.AddressingNames, 10080, portedValues),
			title: domain.EnvOriginPortedTitle,
		},
		{
			name:  "the redirection installed leaves nothing to say",
			plan:  planFixture(domain.AddressingNames, domain.ProxyPrivilegedPort, portedValues),
			title: "",
		},
		{
			name:  "a machine with no proxy says why it wrote ports",
			plan:  planFixture(domain.AddressingNames, 0, portedValues),
			title: domain.EnvOriginProxyOffTitle,
		},
		{
			name:  "a project on ports is told nothing about the proxy",
			plan:  planFixture(domain.AddressingPorts, 10080, portedValues),
			title: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notices := rules.EnvPortNotices(tc.plan)
			if tc.title == "" {
				if len(notices) != 0 {
					t.Fatalf("expected silence, got %+v", notices)
				}
				return
			}
			if len(notices) != 1 || notices[0].Title != tc.title {
				t.Fatalf("got %+v, want one notice titled %q", notices, tc.title)
			}
		})
	}
}

// A project whose every link is a bare port has nothing to hear about the
// proxy's port, even on a machine serving names.
func TestEnvPortNoticesStaySilentWithoutAnyAddress(t *testing.T) {
	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links: []domain.EnvPortLink{{File: ".env", Key: "PORT", Job: "db", Port: "POSTGRES_PORT"}},
		Bases: map[domain.PortRef]int{{Job: "db", Name: "POSTGRES_PORT"}: 5432},
		Lines: map[string][]domain.EnvLine{
			".env": {{Kind: domain.EnvLinePair, Key: "PORT", Value: "5432"}},
		},
		Origins: rules.OriginContext{
			Addressing: domain.AddressingNames,
			Jobs:       map[string]domain.JobConfig{"db": {Name: "db"}},
			Worktree:   "feat-x", Project: "monorepo", PublicPort: 10080,
		},
	})
	if notices := rules.EnvPortNotices(plan); len(notices) != 0 {
		t.Fatalf("got %+v", notices)
	}
}

// A .env provisioned from a parent worktree carries that worktree's ports, not
// the declared base — so nothing anchored the shift and the file kept the
// parent's numbers. F3 made the gap visible rather than causing it: the address
// keys healed to this worktree while the port keys stayed on the parent's.
func TestPlanEnvPortsRewindsAParentWorktreesPorts(t *testing.T) {
	plan := rules.PlanEnvPorts(rules.PlanEnvPortsParams{
		Links: []domain.EnvPortLink{
			{File: ".env", Key: "PORT", Job: "api-dev", Port: "PORT"},
			{File: ".env", Key: "DATABASE_URL", Job: "db", Port: "POSTGRES_PORT"},
		},
		Bases: map[domain.PortRef]int{
			{Job: "api-dev", Name: "PORT"}:     4001,
			{Job: "db", Name: "POSTGRES_PORT"}: 5432,
		},
		Offset: 40,
		Block:  10,
		Lines: map[string][]domain.EnvLine{
			".env": {
				{Kind: domain.EnvLinePair, Key: "PORT", Value: "4021"},
				{Kind: domain.EnvLinePair, Key: "DATABASE_URL", Value: "postgres://u:pw@localhost:5452/app"},
			},
		},
		Origins: rules.OriginContext{
			Addressing: domain.AddressingNames,
			Jobs: map[string]domain.JobConfig{
				"api-dev": {Name: "api-dev", URL: &domain.JobURLConfig{Port: "PORT"}},
				"db":      {Name: "db"},
			},
			Worktree: "f3-child", Project: "monorepo", PublicPort: 10080,
		},
	})

	for _, tc := range []struct{ key, want string }{
		{"PORT", "4041"},
		{"DATABASE_URL", "postgres://u:pw@localhost:5472/app"},
	} {
		e := entryFor(t, plan, tc.key)
		if e.Status != domain.EnvPortStatusRewrite || e.NewValue != tc.want {
			t.Fatalf("%s: got %q (%q), want %q", tc.key, e.NewValue, e.Status, tc.want)
		}
	}
}

// A creation says only what went wrong: the proxy's port is a property of the
// machine, told by `wtm env` and `run addressing`, not by every new worktree.
func TestEnvPortNoticesOnCreateKeepOnlyTheProxyOff(t *testing.T) {
	if got := rules.EnvPortNoticesOnCreate(planFixture(domain.AddressingNames, 10080, portedValues)); len(got) != 0 {
		t.Errorf("a ported address on create printed %+v, want nothing", got)
	}
	got := rules.EnvPortNoticesOnCreate(planFixture(domain.AddressingNames, 0, portedValues))
	if len(got) != 1 || got[0].Title != domain.EnvOriginProxyOffTitle {
		t.Errorf("proxy off on create = %+v, want its notice", got)
	}
}
