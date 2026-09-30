package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestComposeProjectNameIsOwnedByWtm(t *testing.T) {
	if !IsOwnedEnvKey(domain.EnvComposeProjectName) {
		t.Fatalf("%s must be owned by wtm", domain.EnvComposeProjectName)
	}
}

func TestAnOrdinaryKeyIsNotOwned(t *testing.T) {
	if IsOwnedEnvKey("DATABASE_URL") {
		t.Fatal("DATABASE_URL is the user's key, not wtm's")
	}
}

func TestOwnedKeyIsNeverAConflict(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Mode:     domain.EnvModeRefresh,
		Child:    []domain.EnvLine{pair(domain.EnvComposeProjectName, "repo-feat-x")},
		Main:     []domain.EnvLine{pair(domain.EnvComposeProjectName, "repo")},
		Template: []domain.EnvLine{pair(domain.EnvComposeProjectName, "repo")},
	})

	for _, entry := range diff.Entries {
		if entry.Key != domain.EnvComposeProjectName {
			continue
		}
		if entry.Status != domain.EnvKeyResolved {
			t.Fatalf("owned key classified %q, want %q", entry.Status, domain.EnvKeyResolved)
		}
	}
}

func TestOwnedKeyIsNeverAnOrphan(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Mode:  domain.EnvModeRefresh,
		Child: []domain.EnvLine{pair(domain.EnvComposeProjectName, "repo-feat-x")},
	})

	for _, entry := range diff.Entries {
		if entry.Key == domain.EnvComposeProjectName && entry.Status == domain.EnvKeyOrphan {
			t.Fatal("an owned key must not be reported as drift")
		}
	}
}

func TestOwnedEnvWritesTargetsTheComposeJobsOwnDirectory(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "stack", Cmd: "docker compose -f docker-compose.yml up -d", Cwd: "."},
		{Name: "web", Cmd: "pnpm run dev", Cwd: "apps/shop/web"},
	}}

	writes := OwnedEnvWrites(OwnedEnvWritesParams{
		Config: cfg,
		EnvFiles: []domain.EnvFile{
			{Target: ".env", Template: ".env.example"},
			{Target: "apps/shop/web/.env", Template: "apps/shop/web/.env.example"},
		},
		Values: map[string]string{domain.EnvComposeProjectName: "repo-feat-x"},
	})

	if len(writes) != 1 {
		t.Fatalf("got %d writes, want 1: %+v", len(writes), writes)
	}
	if writes[0].File != ".env" || writes[0].Key != domain.EnvComposeProjectName || writes[0].Value != "repo-feat-x" {
		t.Fatalf("unexpected write: %+v", writes[0])
	}
}

func TestOwnedEnvWritesSkipsADirectoryWithNoEnvTarget(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{
		{Name: "stack", Cmd: "docker compose up -d", Cwd: "infra"},
	}}

	writes := OwnedEnvWrites(OwnedEnvWritesParams{
		Config:   cfg,
		EnvFiles: []domain.EnvFile{{Target: ".env"}},
		Values:   map[string]string{domain.EnvComposeProjectName: "repo-feat-x"},
	})

	if len(writes) != 0 {
		t.Fatalf("wtm must not provision a file nobody declared: %+v", writes)
	}
}

func TestOwnedEnvWritesIgnoresAValueNobodyResolved(t *testing.T) {
	cfg := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "stack", Cmd: "docker compose up -d"}}}

	writes := OwnedEnvWrites(OwnedEnvWritesParams{
		Config:   cfg,
		EnvFiles: []domain.EnvFile{{Target: ".env"}},
		Values:   map[string]string{domain.EnvComposeProjectName: ""},
	})

	if len(writes) != 0 {
		t.Fatalf("an empty value writes nothing: %+v", writes)
	}
}

func TestUpsertEnvPairRewritesInPlace(t *testing.T) {
	lines := []domain.EnvLine{pair("A", "1"), pair(domain.EnvComposeProjectName, "repo"), pair("B", "2")}

	out, changed := UpsertEnvPair(UpsertEnvPairParams{Lines: lines, Key: domain.EnvComposeProjectName, Value: "repo-feat-x"})

	if !changed {
		t.Fatal("the value changed")
	}
	if len(out) != 3 || out[1].Value != "repo-feat-x" || out[1].Raw != "" {
		t.Fatalf("got %+v", out)
	}
	if lines[1].Value != "repo" {
		t.Fatal("the input must not be mutated")
	}
}

func TestUpsertEnvPairAppendsWhatTheFileLacks(t *testing.T) {
	out, changed := UpsertEnvPair(UpsertEnvPairParams{Lines: []domain.EnvLine{pair("A", "1")}, Key: "B", Value: "2"})

	if !changed || len(out) != 2 || out[1].Key != "B" {
		t.Fatalf("got %+v changed=%v", out, changed)
	}
}

func TestUpsertEnvPairReportsNoChangeOnTheSameValue(t *testing.T) {
	if _, changed := UpsertEnvPair(UpsertEnvPairParams{Lines: []domain.EnvLine{pair("A", "1")}, Key: "A", Value: "1"}); changed {
		t.Fatal("nothing changed")
	}
}

func TestOwnedKeyAbsentFromTheChildIsNotResolved(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Mode:     domain.EnvModeRefresh,
		Child:    []domain.EnvLine{pair("A", "1")},
		Main:     []domain.EnvLine{pair(domain.EnvComposeProjectName, "repo")},
		Template: []domain.EnvLine{pair(domain.EnvComposeProjectName, "repo")},
	})

	for _, entry := range diff.Entries {
		if entry.Key != domain.EnvComposeProjectName {
			continue
		}
		if entry.Status == domain.EnvKeyResolved && entry.CurrentValue == "" && entry.ResolvedValue == "" {
			t.Fatal("a key the child does not hold must not be reported as already settled")
		}
	}
}

func TestUpsertEnvPairKeepsTheFilesTrailingNewline(t *testing.T) {
	lines := ParseEnv("A=1\n")

	out, _ := UpsertEnvPair(UpsertEnvPairParams{Lines: lines, Key: "B", Value: "2"})

	if got := RenderEnv(out); got != "A=1\nB=2\n" {
		t.Fatalf("got %q, want %q", got, "A=1\nB=2\n")
	}
}

func TestUpsertEnvPairWritesIntoAnEmptyDocument(t *testing.T) {
	out, changed := UpsertEnvPair(UpsertEnvPairParams{Lines: ParseEnv(""), Key: "B", Value: "2"})

	if !changed {
		t.Fatal("the key was not there")
	}
	if got := RenderEnv(out); got != "B=2\n" {
		t.Fatalf("got %q", got)
	}
}

func TestUpsertEnvPairKeepsADeliberateBlankLineAtTheEnd(t *testing.T) {
	out, _ := UpsertEnvPair(UpsertEnvPairParams{Lines: ParseEnv("A=1\n\n"), Key: "B", Value: "2"})

	if got := RenderEnv(out); got != "A=1\nB=2\n\n" {
		t.Fatalf("got %q", got)
	}
}
