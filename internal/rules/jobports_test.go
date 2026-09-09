package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestEffectivePortOffsetBlock(t *testing.T) {
	tests := []struct {
		name string
		cfg  domain.RunConfig
		want int
	}{
		{"unset falls back to the default", domain.RunConfig{}, domain.PortOffsetBlock},
		{"zero is not a block of nothing", domain.RunConfig{PortOffsetBlock: 0}, domain.PortOffsetBlock},
		{"declared wins", domain.RunConfig{PortOffsetBlock: 100}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectivePortOffsetBlock(tt.cfg); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestJobPorts(t *testing.T) {
	ports := map[string]int{"PORT": 3000, "DB_PORT": 5432}

	main := JobPorts(JobPortsParams{Ports: ports, PortOffset: 0})
	if main["PORT"] != 3000 || main["DB_PORT"] != 5432 {
		t.Errorf("main checkout must keep the declared bases, got %v", main)
	}

	second := JobPorts(JobPortsParams{Ports: ports, PortOffset: 10})
	if second["PORT"] != 3010 || second["DB_PORT"] != 5442 {
		t.Errorf("got %v, want PORT=3010 DB_PORT=5442", second)
	}
}

func TestIsEnvVarName(t *testing.T) {
	valid := []string{"PORT", "_PORT", "DB_PORT2", "port"}
	for _, name := range valid {
		if !IsEnvVarName(name) {
			t.Errorf("%q should be a valid name", name)
		}
	}
	invalid := []string{"", "2PORT", "DB-PORT", "DB PORT", "DB.PORT"}
	for _, name := range invalid {
		if IsEnvVarName(name) {
			t.Errorf("%q should be rejected", name)
		}
	}
}

func jobWithPorts(name string, ports map[string]int) domain.JobConfig {
	return domain.JobConfig{Name: name, Kind: domain.JobKindService, Cmd: "run", Ports: ports}
}

func TestPortCollisions(t *testing.T) {
	tests := []struct {
		name string
		cfg  domain.RunConfig
		want int
	}{
		{
			// The question this ticket set out to answer: an offset is uniform, so
			// neighbouring bases keep their spacing instead of overlapping.
			name: "three neighbouring databases are fine",
			cfg:  domain.RunConfig{Jobs: []domain.JobConfig{jobWithPorts("db", map[string]int{"A": 5434, "B": 5435, "C": 5436})}},
			want: 0,
		},
		{
			name: "a gap of exactly one block collides on the next worktree",
			cfg:  domain.RunConfig{Jobs: []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 3010})}},
			want: 1,
		},
		{
			name: "far apart on the same residue is arithmetic, not a conflict",
			cfg:  domain.RunConfig{Jobs: []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ALT": 8080})}},
			want: 0,
		},
		{
			name: "the same base twice collides inside one worktree",
			cfg: domain.RunConfig{Jobs: []domain.JobConfig{
				jobWithPorts("web", map[string]int{"PORT": 3000}),
				jobWithPorts("api", map[string]int{"PORT": 3000}),
			}},
			want: 1,
		},
		{
			name: "a wider block clears a gap the default rejects",
			cfg: domain.RunConfig{
				PortOffsetBlock: 100,
				Jobs:            []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 3010})},
			},
			want: 0,
		},
		{
			name: "a wider block creates gaps the default cleared",
			cfg: domain.RunConfig{
				PortOffsetBlock: 100,
				Jobs:            []domain.JobConfig{jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 3100})},
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PortCollisions(tt.cfg); len(got) != tt.want {
				t.Errorf("got %d collisions %v, want %d", len(got), got, tt.want)
			}
		})
	}
}

func TestPortCollisionsHorizon(t *testing.T) {
	block := domain.PortOffsetBlock
	inside := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"A": 3000, "B": 3000 + block*domain.PortCollisionHorizon}),
	}}
	if got := PortCollisions(inside); len(got) != 1 {
		t.Errorf("a pair meeting at the horizon must be reported, got %v", got)
	}

	outside := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"A": 3000, "B": 3000 + block*(domain.PortCollisionHorizon+1)}),
	}}
	if got := PortCollisions(outside); len(got) != 0 {
		t.Errorf("a pair meeting past the horizon must be left alone, got %v", got)
	}
}

func TestPortCollisionNamesBothSides(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"PORT": 3000}),
		jobWithPorts("api", map[string]int{"ADMIN": 3010}),
	}}
	collisions := PortCollisions(cfg)
	if len(collisions) != 1 {
		t.Fatalf("got %d collisions, want 1", len(collisions))
	}
	c := collisions[0]
	if c.A.Job != "web" || c.A.Name != "PORT" || c.B.Job != "api" || c.B.Name != "ADMIN" {
		t.Errorf("both sides must be named, got %+v", c)
	}
	if c.Worktrees != 1 {
		t.Errorf("got %d worktrees apart, want 1", c.Worktrees)
	}
}

func TestPortDeclarationsAreStable(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		jobWithPorts("web", map[string]int{"PORT": 3000, "ADMIN": 9000, "DEBUG": 9229}),
		jobWithPorts("api", map[string]int{"API_PORT": 8080}),
	}}
	want := []PortDeclaration{
		{Job: "web", Name: "ADMIN", Base: 9000},
		{Job: "web", Name: "DEBUG", Base: 9229},
		{Job: "web", Name: "PORT", Base: 3000},
		{Job: "api", Name: "API_PORT", Base: 8080},
	}
	for range 20 {
		got := PortDeclarations(cfg)
		if len(got) != len(want) {
			t.Fatalf("got %d declarations, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("declaration %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
	}
}

func TestLabelWithPorts(t *testing.T) {
	tests := []struct {
		name  string
		ports map[string]int
		want  string
	}{
		{"no declaration leaves the line alone", nil, "web started"},
		{"one port", map[string]int{"PORT": 3010}, "web started · PORT=3010"},
		{"several are sorted", map[string]int{"PORT": 3010, "ADMIN": 9010}, "web started · ADMIN=9010 PORT=3010"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LabelWithPorts(LabelWithPortsParams{Label: "web started", Ports: tt.ports})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePorts(t *testing.T) {
	ports, err := ParsePorts([]string{"PORT=3000", " DB_PORT = 5432 "})
	if err != nil {
		t.Fatal(err)
	}
	if ports["PORT"] != 3000 || ports["DB_PORT"] != 5432 {
		t.Errorf("got %v", ports)
	}
}

func TestParsePortsNothing(t *testing.T) {
	ports, err := ParsePorts(nil)
	if err != nil || ports != nil {
		t.Errorf("got %v, %v", ports, err)
	}
}

func TestParsePortsRejections(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		want    string
	}{
		{"no equals sign", []string{"3000"}, "NAME=PORT"},
		{"bad variable name", []string{"DB-PORT=5432"}, "not a valid environment variable name"},
		{"not a number", []string{"PORT=abc"}, "not a number"},
		{"out of range", []string{"PORT=70000"}, "outside"},
		{"declared twice", []string{"PORT=3000", "PORT=4000"}, "twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePorts(tt.entries)
			if err == nil {
				t.Fatalf("expected %v to be refused", tt.entries)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

// The flag and the wizard write the same form they read, so a job edited twice
// keeps its ports.
func TestPortEntriesRoundTrip(t *testing.T) {
	ports := map[string]int{"PORT": 3000, "DB_PORT": 5432}
	back, err := ParsePorts(PortEntries(ports))
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != len(ports) || back["PORT"] != 3000 || back["DB_PORT"] != 5432 {
		t.Errorf("got %v, want %v", back, ports)
	}
}

// Le daemon démarre avant les jobs et tient déjà son port : un job dont la
// base l'atteint échouera à binder, sans que rien ne l'ait annoncé.
func TestProxyPortCollisionsSignaleUneBaseQuiAtteintLeProxy(t *testing.T) {
	cfg := domain.RunConfig{
		PortOffsetBlock: 10,
		Jobs: []domain.JobConfig{
			{Name: "web", Ports: map[string]int{domain.PortNameDefault: 3990}},
		},
	}

	got := ProxyPortCollisions(ProxyPortCollisionsParams{Config: cfg, ProxyPort: 4000})

	if len(got) != 1 {
		t.Fatalf("collisions = %+v, want une seule", got)
	}
	if got[0].Declaration.Job != "web" || got[0].Worktrees != 1 {
		t.Errorf("collision = %+v, want web au premier worktree suivant", got[0])
	}
}

// Une base au-dessus du port du proxy ne l'atteint jamais : un offset est
// toujours positif, il éloigne.
func TestProxyPortCollisionsIgnoreUneBaseAuDessus(t *testing.T) {
	cfg := domain.RunConfig{
		PortOffsetBlock: 10,
		Jobs: []domain.JobConfig{
			{Name: "web", Ports: map[string]int{domain.PortNameDefault: 4010}},
		},
	}

	if got := ProxyPortCollisions(ProxyPortCollisionsParams{Config: cfg, ProxyPort: 4000}); len(got) != 0 {
		t.Errorf("collisions = %+v, want aucune", got)
	}
}

// Une base qui n'atteint le proxy qu'au-delà de l'horizon est de
// l'arithmétique, pas un conflit — même règle que PortCollisions.
func TestProxyPortCollisionsIgnoreAuDelaDeLhorizon(t *testing.T) {
	cfg := domain.RunConfig{
		PortOffsetBlock: 1,
		Jobs: []domain.JobConfig{
			{Name: "web", Ports: map[string]int{domain.PortNameDefault: 1000}},
		},
	}

	if got := ProxyPortCollisions(ProxyPortCollisionsParams{Config: cfg, ProxyPort: 4000}); len(got) != 0 {
		t.Errorf("collisions = %+v, want aucune", got)
	}
}

// Sans proxy configuré, il n'y a personne à heurter.
func TestProxyPortCollisionsSansProxy(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "web", Ports: map[string]int{domain.PortNameDefault: 4000}},
	}}

	if got := ProxyPortCollisions(ProxyPortCollisionsParams{Config: cfg}); len(got) != 0 {
		t.Errorf("collisions = %+v, want aucune", got)
	}
}

// Le message doit porter la base et le port du proxy aux bonnes places : une
// inversion se lirait bien et dirait le contraire.
func TestProxyPortCollisionLinesNommeLesDeuxPorts(t *testing.T) {
	lines := ProxyPortCollisionLines([]ProxyPortCollision{{
		Declaration: PortDeclaration{Job: "web", Name: domain.PortNameDefault, Base: 3990},
		Worktrees:   1,
	}}, 4000)

	if len(lines) != 1 {
		t.Fatalf("lignes = %v, want une seule", lines)
	}
	if !strings.Contains(lines[0], "base 3990") || !strings.Contains(lines[0], "port 4000") {
		t.Errorf("ligne = %q, want la base 3990 et le port 4000 nommés", lines[0])
	}
}

// A shared job runs in the main checkout, so its declared port is its real
// port: 5432 stays 5432 whichever worktree asked. That stability is what lets a
// tenant's env write its URL literally.
func TestJobPortsIgnoresOffsetWhenShared(t *testing.T) {
	got := JobPorts(JobPortsParams{
		Ports:      map[string]int{"CRM_DB_PORT": 5432},
		PortOffset: 300,
		Scope:      domain.JobScopeShared,
	})
	if got["CRM_DB_PORT"] != 5432 {
		t.Errorf("port = %d, want 5432", got["CRM_DB_PORT"])
	}
}

func TestJobPortsAppliesOffsetWhenPerWorktree(t *testing.T) {
	got := JobPorts(JobPortsParams{
		Ports:      map[string]int{"CRM_DB_PORT": 5432},
		PortOffset: 300,
	})
	if got["CRM_DB_PORT"] != 5732 {
		t.Errorf("port = %d, want 5732", got["CRM_DB_PORT"])
	}
}

// A shared job binds its declared port in every worktree, so a hook and a .env
// must read that port and not this worktree's shift — otherwise the whole
// worktree addresses a service that answers elsewhere.
func TestLifecyclePortsDoNotShiftASharedJob(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Scope: domain.JobScopeShared, Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "web", Ports: map[string]int{"WEB_PORT": 3000}},
	}}

	got := LifecyclePorts(LifecyclePortsParams{Config: cfg, PortOffset: 30})
	if got["POSTGRES_PORT"] != 5432 {
		t.Errorf("POSTGRES_PORT = %d, want 5432 unshifted", got["POSTGRES_PORT"])
	}
	if got["WEB_PORT"] != 3030 {
		t.Errorf("WEB_PORT = %d, want 3030", got["WEB_PORT"])
	}
}

// Two shared declarations never move, so they meet only where they are already
// equal — a gap of one block between them is not a collision.
func TestPortCollisionsBetweenTwoSharedDeclarations(t *testing.T) {
	apart := domain.RunConfig{PortOffsetBlock: 10, Jobs: []domain.JobConfig{
		{Name: "a", Scope: domain.JobScopeShared, Ports: map[string]int{"A": 5432}},
		{Name: "b", Scope: domain.JobScopeShared, Ports: map[string]int{"B": 5442}},
	}}
	if got := PortCollisions(apart); len(got) != 0 {
		t.Errorf("collisions = %v, want none: neither declaration ever moves", got)
	}

	same := domain.RunConfig{PortOffsetBlock: 10, Jobs: []domain.JobConfig{
		{Name: "a", Scope: domain.JobScopeShared, Ports: map[string]int{"A": 5432}},
		{Name: "b", Scope: domain.JobScopeShared, Ports: map[string]int{"B": 5432}},
	}}
	if got := PortCollisions(same); len(got) != 1 {
		t.Errorf("collisions = %v, want one: both sit on 5432", got)
	}
}

// A per-worktree job still walks onto a fixed one every block.
func TestPortCollisionsSharedAgainstPerWorktree(t *testing.T) {
	cfg := domain.RunConfig{PortOffsetBlock: 10, Jobs: []domain.JobConfig{
		{Name: "db", Scope: domain.JobScopeShared, Ports: map[string]int{"A": 5432}},
		{Name: "web", Ports: map[string]int{"B": 5422}},
	}}
	if got := PortCollisions(cfg); len(got) != 1 {
		t.Errorf("collisions = %v, want one: the moving side reaches 5432", got)
	}
}
