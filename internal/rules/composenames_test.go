package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestComposeIsolatedName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		project string
		want    string
	}{
		{
			name:    "le préfixe du projet est repris comme défaut, le reste comme suffixe",
			value:   "monorepo-exemple-wtm-postgres",
			project: "monorepo-exemple-wtm",
			want:    `"${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-postgres"`,
		},
		{
			name:    "un nom sans rapport avec le projet est fronté tel quel",
			value:   "pg",
			project: "monorepo-exemple-wtm",
			want:    `"${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-pg"`,
		},
		{
			name:    "un nom qui est exactement le projet n'a pas de suffixe à garder",
			value:   "monorepo-exemple-wtm",
			project: "monorepo-exemple-wtm",
			want:    `"${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}"`,
		},
		{
			name:    "le projet est slugifié comme COMPOSE_PROJECT_NAME l'est",
			value:   "My.Repo-postgres",
			project: "My.Repo",
			want:    `"${COMPOSE_PROJECT_NAME:-my-repo}-postgres"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rules.ComposeIsolatedName(rules.ComposeIsolatedNameParams{
				Name:    tt.value,
				Project: tt.project,
			})
			if got != tt.want {
				t.Errorf("ComposeIsolatedName() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestComposeIsolatedNameReproducesTheOriginalStandalone(t *testing.T) {
	// The default is the whole reason the rewrite is safe to commit: a checkout
	// with no wtm driving it must still get the name the file used to pin.
	got := rules.ComposeIsolatedName(rules.ComposeIsolatedNameParams{
		Name:    "monorepo-exemple-wtm-postgres",
		Project: "monorepo-exemple-wtm",
	})
	want := `"${COMPOSE_PROJECT_NAME:-monorepo-exemple-wtm}-postgres"`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestPlanComposeNames(t *testing.T) {
	scans := map[string]domain.ComposeScan{
		"docker-compose.yml": {
			File: "docker-compose.yml",
			Names: []domain.ComposeAbsoluteName{
				{File: "docker-compose.yml", Kind: domain.ComposeNameContainer, Owner: "postgres", Status: domain.ComposeNameAbsolute, Name: "app-postgres", Token: "app-postgres", Replacement: `"x"`},
				{File: "docker-compose.yml", Kind: domain.ComposeNameVolume, Owner: "data", Status: domain.ComposeNameTemplated, Name: "${X}-data"},
				{File: "docker-compose.yml", Kind: domain.ComposeNameNetwork, Owner: "back", Status: domain.ComposeNameUnsupported, Reason: domain.ComposeNameReasonAnchor},
			},
		},
		"other.yml": {File: "other.yml", Names: []domain.ComposeAbsoluteName{
			{File: "other.yml", Kind: domain.ComposeNameContainer, Owner: "web", Status: domain.ComposeNameAbsolute, Name: "web", Token: "web", Replacement: `"y"`},
		}},
	}

	t.Run("autorisé, seuls les noms absolus des fichiers choisis sont patchés", func(t *testing.T) {
		plan := rules.PlanComposeNames(rules.PlanComposeNamesParams{
			Scans: scans,
			Files: []string{"docker-compose.yml"},
			Patch: true,
		})

		if len(plan.Patches["docker-compose.yml"]) != 1 {
			t.Fatalf("expected 1 patch, got %d", len(plan.Patches["docker-compose.yml"]))
		}
		if plan.Patches["docker-compose.yml"][0].Owner != "postgres" {
			t.Errorf("patched the wrong owner: %s", plan.Patches["docker-compose.yml"][0].Owner)
		}
		if _, patched := plan.Patches["other.yml"]; patched {
			t.Error("other.yml was not selected and must not be patched")
		}
		if len(plan.Withheld) != 1 || plan.Withheld[0].Owner != "back" {
			t.Errorf("expected the unsupported name withheld, got %+v", plan.Withheld)
		}
	})

	t.Run("non autorisé, un nom absolu est retenu plutôt que réécrit", func(t *testing.T) {
		plan := rules.PlanComposeNames(rules.PlanComposeNamesParams{
			Scans: scans,
			Files: []string{"docker-compose.yml"},
			Patch: false,
		})

		if len(plan.Patches) != 0 {
			t.Fatalf("nothing may be patched without authorization, got %+v", plan.Patches)
		}
		if len(plan.Withheld) != 2 {
			t.Fatalf("expected both the absolute and the unsupported name withheld, got %d", len(plan.Withheld))
		}
	})
}

func TestComposeNamePatchLines(t *testing.T) {
	lines := rules.ComposeNamePatchLines(map[string][]domain.ComposeAbsoluteName{
		"docker-compose.yml": {{
			File: "docker-compose.yml", Kind: domain.ComposeNameContainer, Owner: "postgres",
			Token: "app-postgres", Replacement: `"${COMPOSE_PROJECT_NAME:-app}-postgres"`,
		}},
	})

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	for _, want := range []string{"docker-compose.yml", "postgres", "app-postgres", "COMPOSE_PROJECT_NAME"} {
		if !contains(lines[0], want) {
			t.Errorf("line %q is missing %q", lines[0], want)
		}
	}
}

func TestComposeNamesRenameAVolume(t *testing.T) {
	if !rules.ComposeNamesRenameAVolume(map[string][]domain.ComposeAbsoluteName{
		"docker-compose.yml": {{Kind: domain.ComposeNameVolume}},
	}) {
		t.Error("a renamed volume must be flagged: its data does not follow")
	}
	if rules.ComposeNamesRenameAVolume(map[string][]domain.ComposeAbsoluteName{
		"docker-compose.yml": {{Kind: domain.ComposeNameContainer}},
	}) {
		t.Error("a renamed container carries no data and needs no warning")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
