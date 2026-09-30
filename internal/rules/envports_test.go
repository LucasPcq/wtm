package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func planOne(t *testing.T, content string, base int, offset int) domain.EnvPortEntry {
	t.Helper()
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "DB_PORT"}},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "DB_PORT"}: base},
		Offset: offset,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv(content)},
	})
	if len(plan.Entries) != 1 {
		t.Fatalf("PlanEnvPorts() returned %d entries, want 1", len(plan.Entries))
	}
	return plan.Entries[0]
}

func TestPlanEnvPorts(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		base       int
		offset     int
		wantStatus domain.EnvPortStatus
		wantValue  string
	}{
		{"bare port", "URL=5432\n", 5432, 10, domain.EnvPortStatusRewrite, "5442"},
		{"port inside a database url", "URL=postgres://u:pw@localhost:5432/app\n", 5432, 10, domain.EnvPortStatusRewrite, "postgres://u:pw@localhost:5442/app"},
		{"port inside an http url", "URL=http://localhost:3000/api\n", 3000, 10, domain.EnvPortStatusRewrite, "http://localhost:3010/api"},
		{"quoted value", "URL=\"http://localhost:3000\"\n", 3000, 10, domain.EnvPortStatusRewrite, "http://localhost:3010"},
		{"offset zero leaves the value alone", "URL=postgres://localhost:5432/app\n", 5432, 0, domain.EnvPortStatusUnchanged, ""},
		{"already carries the resolved port", "URL=postgres://localhost:5442/app\n", 5432, 10, domain.EnvPortStatusUnchanged, ""},
		{"base absent from the value", "URL=postgres://localhost:6000/app\n", 5432, 10, domain.EnvPortStatusNotFound, ""},
		{"base twice in the value", "URL=http://localhost:3000/v3000\n", 3000, 10, domain.EnvPortStatusAmbiguous, ""},
		{"key absent from the file", "OTHER=1\n", 5432, 10, domain.EnvPortStatusMissingKey, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := planOne(t, c.content, c.base, c.offset)
			if got.Status != c.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, c.wantStatus)
			}
			if got.NewValue != c.wantValue {
				t.Errorf("new value = %q, want %q", got.NewValue, c.wantValue)
			}
		})
	}
}

// A base must never match inside a longer number: rewriting 5432 inside 54321
// silently corrupts the value, which is the failure this whole feature exists to
// avoid rather than cause.
func TestPlanEnvPortsRespectsDigitBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantStatus domain.EnvPortStatus
		wantValue  string
	}{
		{"base is a prefix of a longer number", "URL=http://localhost:54321/x\n", domain.EnvPortStatusNotFound, ""},
		{"base is a suffix of a longer number", "URL=http://localhost:15432/x\n", domain.EnvPortStatusNotFound, ""},
		{"longer number alongside a real match", "URL=http://a:54321/b:5432\n", domain.EnvPortStatusRewrite, "http://a:54321/b:5442"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := planOne(t, c.content, 5432, 10)
			if got.Status != c.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, c.wantStatus)
			}
			if got.NewValue != c.wantValue {
				t.Errorf("new value = %q, want %q", got.NewValue, c.wantValue)
			}
		})
	}
}

func TestPlanEnvPortsSkipsUndeclaredPort(t *testing.T) {
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "URL", Job: "svc", Port: "GONE"}},
		Bases:  map[domain.PortRef]int{{Job: "svc", Name: "DB_PORT"}: 5432},
		Offset: 10,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv("URL=5432\n")},
	})
	if len(plan.Entries) != 0 {
		t.Fatalf("PlanEnvPorts() = %+v, want no entries", plan.Entries)
	}
}

func TestApplyEnvPortsRewritesAndPreservesTheRest(t *testing.T) {
	content := "# header\nOTHER=keep\nURL=postgres://u:pw@localhost:5432/app\n"
	lines := ParseEnv(content)
	entry := planOne(t, content, 5432, 10)

	got := RenderEnv(ApplyEnvPorts(lines, []domain.EnvPortEntry{entry}))
	want := "# header\nOTHER=keep\nURL=postgres://u:pw@localhost:5442/app\n"
	if got != want {
		t.Errorf("ApplyEnvPorts() rendered\n%q\nwant\n%q", got, want)
	}
}

func TestApplyEnvPortsIgnoresNonRewrites(t *testing.T) {
	content := "URL=postgres://localhost:6000/app\n"
	lines := ParseEnv(content)
	entry := planOne(t, content, 5432, 10)

	if got := RenderEnv(ApplyEnvPorts(lines, []domain.EnvPortEntry{entry})); got != content {
		t.Errorf("ApplyEnvPorts() = %q, want the input untouched %q", got, content)
	}
}

// Applying twice must be a no-op: `wtm env` runs whenever the user asks, and the
// second run sees the resolved port rather than the base.
func TestApplyEnvPortsIsIdempotent(t *testing.T) {
	content := "URL=postgres://localhost:5432/app\n"

	first := RenderEnv(ApplyEnvPorts(ParseEnv(content), []domain.EnvPortEntry{planOne(t, content, 5432, 10)}))
	second := RenderEnv(ApplyEnvPorts(ParseEnv(first), []domain.EnvPortEntry{planOne(t, first, 5432, 10)}))

	if second != first {
		t.Errorf("second apply changed the file: %q then %q", first, second)
	}
}

func TestReduceEnvPortValue(t *testing.T) {
	cases := []struct {
		name  string
		value string
		base  int
		want  string
	}{
		{"a shifted port rewinds to its base", "postgres://localhost:5442/app", 5432, "postgres://localhost:5432/app"},
		{"the base is already the base", "postgres://localhost:5432/app", 5432, "postgres://localhost:5432/app"},
		{"another worktree's offset rewinds too", "postgres://localhost:5472/app", 5432, "postgres://localhost:5432/app"},
		{"a port off the block is left alone", "postgres://localhost:6000/app", 5432, "postgres://localhost:6000/app"},
		{"a port below the base is left alone", "postgres://localhost:5422/app", 5432, "postgres://localhost:5422/app"},
		{"two candidates leave the value alone", "http://a:3000/b:3010", 3000, "http://a:3000/b:3010"},
		{"an unrelated number is ignored", "http://localhost:3010/api/v1", 3000, "http://localhost:3000/api/v1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ReduceEnvPortValue(ReduceEnvPortParams{Value: c.value, Base: c.base, Block: 10})
			if got != c.want {
				t.Errorf("ReduceEnvPortValue(%q) = %q, want %q", c.value, got, c.want)
			}
		})
	}
}

// The reconciliation compares a child against a source that may be another
// worktree, bound on an offset the reader never learns. Both must reduce to the
// same text or refresh mode reports a conflict between two spellings of one value.
func TestReduceEnvPortValueMakesTwoWorktreesCompareEqual(t *testing.T) {
	reduce := func(value string) string {
		return ReduceEnvPortValue(ReduceEnvPortParams{Value: value, Base: 5432, Block: 10})
	}

	child := reduce("postgres://u:pw@localhost:5442/app")
	parent := reduce("postgres://u:pw@localhost:5472/app")
	main := reduce("postgres://u:pw@localhost:5432/app")

	if child != main || parent != main {
		t.Errorf("reduced child %q, parent %q, main %q — want all three equal", child, parent, main)
	}
}

// A genuine difference must survive the reduction, or the feature would hide the
// conflicts the reconciliation exists to surface.
func TestReduceEnvPortValueKeepsRealConflictsVisible(t *testing.T) {
	reduce := func(value string) string {
		return ReduceEnvPortValue(ReduceEnvPortParams{Value: value, Base: 5432, Block: 10})
	}

	if reduce("postgres://u:new@localhost:5442/app") == reduce("postgres://u:old@localhost:5432/app") {
		t.Error("two values differing by their password reduced to the same text")
	}
}

func TestElideEnvValueHidesCredentials(t *testing.T) {
	got := ElideEnvValue(ElideEnvValueParams{Value: "postgres://user:supersecret@localhost:5442/app"})
	if got != "…@localhost:5442/app" {
		t.Fatalf("ElideEnvValue() = %q, want the credentials elided", got)
	}
}

func TestElideEnvValueKeepsShortValues(t *testing.T) {
	if got := ElideEnvValue(ElideEnvValueParams{Value: "http://localhost:3010"}); got != "http://localhost:3010" {
		t.Errorf("ElideEnvValue() = %q, want it unchanged", got)
	}
}

func TestEnvPortBases(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "db", Ports: map[string]int{"POSTGRES_PORT": 5432}},
		{Name: "api", Ports: map[string]int{"API_PORT": 3000}},
	}}

	got := EnvPortBases(cfg)
	want := map[domain.PortRef]int{{Job: "db", Name: "POSTGRES_PORT"}: 5432, {Job: "api", Name: "API_PORT"}: 3000}
	if len(got) != len(want) {
		t.Fatalf("EnvPortBases() = %v, want %v", got, want)
	}
	for name, base := range want {
		if got[name] != base {
			t.Errorf("EnvPortBases()[%s] = %d, want %d", name, got[name], base)
		}
	}
}

// Un .env déjà décalé ne contient plus la base déclarée : sans seconde passe,
// aucun lien n'est proposé et `wtm create` ne réalignera jamais ce fichier.
func TestEnvPortCandidatesOffreUneCleQuiNancreAucuneBase(t *testing.T) {
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{
			"apps/web/.env": {{Kind: domain.EnvLinePair, Key: domain.PortNameDefault, Value: "3010"}},
		},
		Bases:     map[domain.PortRef]int{{Job: "web", Name: domain.PortNameDefault}: 3000},
		JobsByDir: map[string]string{"apps/web": "web"},
	})

	if len(got) != 1 {
		t.Fatalf("candidats = %+v, want un seul", got)
	}
	if got[0].Job != "web" || got[0].Key != domain.PortNameDefault || !got[0].ByDir {
		t.Errorf("candidat = %+v, want web/PORT rattaché par répertoire", got[0])
	}
}

// L'ancrage strict reste prioritaire : une valeur qui contient la base est
// rattachée par elle, pas par le répertoire, et ne se dédouble pas.
func TestEnvPortCandidatesPrefereLancrageStrict(t *testing.T) {
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{
			"apps/web/.env": {{Kind: domain.EnvLinePair, Key: domain.PortNameDefault, Value: "3000"}},
		},
		Bases:     map[domain.PortRef]int{{Job: "web", Name: domain.PortNameDefault}: 3000},
		JobsByDir: map[string]string{"apps/web": "web"},
	})

	if len(got) != 1 || got[0].ByDir {
		t.Errorf("candidats = %+v, want un seul, ancré sur la valeur", got)
	}
}

// Sans job connu pour le répertoire, rien à rattacher : wtm ne devine pas.
func TestEnvPortCandidatesNoffreRienSansJobPourLeRepertoire(t *testing.T) {
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{
			"apps/web/.env": {{Kind: domain.EnvLinePair, Key: domain.PortNameDefault, Value: "3010"}},
		},
		Bases: map[domain.PortRef]int{{Job: "web", Name: domain.PortNameDefault}: 3000},
	})

	if len(got) != 0 {
		t.Errorf("candidats = %+v, want aucun", got)
	}
}

// Une valeur qui n'est pas un port nu — une URL, un placeholder — n'est jamais
// devinée : la seconde passe n'a aucun ancrage pour situer le port dedans.
func TestEnvPortCandidatesNeDevinePasDansUneValeurComposite(t *testing.T) {
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{
			"apps/web/.env": {{Kind: domain.EnvLinePair, Key: "API_PORT", Value: "http://localhost:9999"}},
		},
		Bases:     map[domain.PortRef]int{{Job: "web", Name: "API_PORT"}: 3000},
		JobsByDir: map[string]string{"apps/web": "web"},
	})

	if len(got) != 0 {
		t.Errorf("candidats = %+v, want aucun", got)
	}
}

// La clé doit nommer le port d'un job donné : un PORT rattaché au répertoire
// d'un autre job serait un lien qui déplace le mauvais numéro.
func TestEnvPortCandidatesNeRattachePasAUnAutreJob(t *testing.T) {
	got := EnvPortCandidates(EnvPortCandidatesParams{
		Lines: map[string][]domain.EnvLine{
			"apps/web/.env": {{Kind: domain.EnvLinePair, Key: domain.PortNameDefault, Value: "3010"}},
		},
		Bases:     map[domain.PortRef]int{{Job: "api", Name: domain.PortNameDefault}: 3000},
		JobsByDir: map[string]string{"apps/web": "web"},
	})

	if len(got) != 0 {
		t.Errorf("candidats = %+v, want aucun", got)
	}
}

// A CORS_ORIGIN listing two front-ends follows both: each origin moves onto the
// port its own job binds, and one link picking a service would have left the
// other pinned to the main checkout in every worktree.
func TestPlanEnvPortsMovesEveryPortAKeyFollows(t *testing.T) {
	const value = "http://localhost:5173,http://localhost:5174"
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links: []domain.EnvPortLink{
			{File: ".env", Key: "CORS_ORIGIN", Job: "web", Port: "VITE_PORT"},
			{File: ".env", Key: "CORS_ORIGIN", Job: "admin", Port: "VITE_PORT"},
		},
		Bases: map[domain.PortRef]int{
			{Job: "web", Name: "VITE_PORT"}:   5173,
			{Job: "admin", Name: "VITE_PORT"}: 5174,
		},
		Offset: 10,
		Block:  10,
		Lines:  map[string][]domain.EnvLine{".env": ParseEnv("CORS_ORIGIN=" + value + "\n")},
	})

	if len(plan.Entries) != 1 {
		t.Fatalf("entries = %+v, want one per key however many ports it follows", plan.Entries)
	}
	entry := plan.Entries[0]
	if entry.Status != domain.EnvPortStatusRewrite {
		t.Fatalf("status = %q, want a rewrite", entry.Status)
	}
	if entry.NewValue != "http://localhost:5183,http://localhost:5184" {
		t.Errorf("value = %q, want both origins moved", entry.NewValue)
	}
	if len(entry.Moves) != 2 {
		t.Errorf("moves = %+v, want one per port followed", entry.Moves)
	}
}

// A port no job declares is not this worktree's: shifting it would break an
// address that points somewhere else entirely.
func TestPlanEnvPortsLeavesAnUndeclaredPortAlone(t *testing.T) {
	plan := PlanEnvPorts(PlanEnvPortsParams{
		Links:  []domain.EnvPortLink{{File: ".env", Key: "CORS_ORIGIN", Job: "web", Port: "VITE_PORT"}},
		Bases:  map[domain.PortRef]int{{Job: "web", Name: "VITE_PORT"}: 5173},
		Offset: 10,
		Block:  10,
		Lines: map[string][]domain.EnvLine{
			".env": ParseEnv("CORS_ORIGIN=http://localhost:5173,https://app.example.com:8443\n"),
		},
	})

	if got := plan.Entries[0].NewValue; got != "http://localhost:5173,https://app.example.com:8443" && got != "http://localhost:5183,https://app.example.com:8443" {
		t.Fatalf("value = %q, unexpected shape", got)
	}
	if !strings.Contains(plan.Entries[0].NewValue, "app.example.com:8443") {
		t.Errorf("value = %q, want the external origin untouched", plan.Entries[0].NewValue)
	}
}
