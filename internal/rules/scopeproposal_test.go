package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func scopeScanWith(services ...domain.ComposeService) map[string]domain.ComposeScan {
	return map[string]domain.ComposeScan{"docker-compose.yml": {File: "docker-compose.yml", Services: services}}
}

func choicesFor(t *testing.T, existing domain.RunConfig, services ...domain.ComposeService) []ServiceScopeChoice {
	t.Helper()
	return ServiceScopeChoices(ServiceScopeChoicesParams{
		Scans:    scopeScanWith(services...),
		Files:    []string{"docker-compose.yml"},
		Existing: existing,
	})
}

// A service built here serves this worktree's own source, so the question is
// not the reader's to answer. Shown all the same, with its reason.
func TestServiceScopeChoicesFixesABuiltService(t *testing.T) {
	got := choicesFor(t, domain.RunConfig{}, domain.ComposeService{Name: "api", HasBuild: true})
	if len(got) != 1 {
		t.Fatalf("choices = %v", got)
	}
	if !got[0].Fixed || got[0].Reason != domain.ScopeReasonBuild {
		t.Errorf("choice = %+v, want fixed with the build reason", got[0])
	}
	if got[0].Scope == domain.JobScopeShared {
		t.Error("a built service was proposed as shared")
	}
}

// A registry image is a genuinely open question, and nothing is pre-answered.
func TestServiceScopeChoicesLeavesAnImageOpen(t *testing.T) {
	got := choicesFor(t, domain.RunConfig{}, domain.ComposeService{Name: "db", Image: "postgres:16"})
	if got[0].Fixed {
		t.Error("an image service was pre-answered")
	}
	if got[0].Scope != domain.JobScopePerWorktree {
		t.Errorf("scope = %q, want the safe default", got[0].Scope)
	}
	// Deliberately nothing: a recipe for postgres would guess the port variable,
	// the user and the host, and a wrong command that is accepted reads as a wtm
	// bug rather than as a line to write.
	if got[0].Namespace != nil {
		t.Errorf("namespace = %+v, want none pre-filled", got[0].Namespace)
	}
}

// The config outranks detection wherever it speaks: a re-init shows what was
// decided last time, not what a fresh look would propose.
func TestServiceScopeChoicesReadTheExistingConfigFirst(t *testing.T) {
	existing := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "db", Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "mine_{worktree}", Create: "my-script"},
	}}}

	got := choicesFor(t, existing, domain.ComposeService{Name: "db", Image: "postgres:16"})
	if got[0].Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want the config's own answer", got[0].Scope)
	}
	if got[0].Namespace == nil || got[0].Namespace.Create != "my-script" {
		t.Errorf("namespace = %+v, want the one already written, not the recipe", got[0].Namespace)
	}
}

// A service the config declares per-worktree keeps that answer rather than
// being proposed for sharing all over again on every re-init.
func TestServiceScopeChoicesKeepAPerWorktreeAnswer(t *testing.T) {
	existing := domain.RunConfig{Jobs: []domain.JobConfig{{Name: "db"}}}
	got := choicesFor(t, existing, domain.ComposeService{Name: "db", Image: "postgres:16"})
	if got[0].Scope != domain.JobScopePerWorktree || got[0].Namespace != nil {
		t.Errorf("choice = %+v, want the config's per-worktree answer with no recipe", got[0])
	}
}

// The list is complete, so a re-init cannot silently drop a service the reader
// had already answered for.
func TestServiceScopeChoicesListsEveryService(t *testing.T) {
	got := choicesFor(t, domain.RunConfig{},
		domain.ComposeService{Name: "db", Image: "postgres:16"},
		domain.ComposeService{Name: "api", HasBuild: true},
		domain.ComposeService{Name: "keycloak", Image: "quay.io/keycloak/keycloak:24"},
	)
	if len(got) != 3 {
		t.Fatalf("choices = %d, want every service listed", len(got))
	}
	if got[2].Namespace != nil {
		t.Errorf("keycloak namespace = %+v, want none", got[2].Namespace)
	}
}

func TestSharedFromChoicesKeepsOnlyWhatWasShared(t *testing.T) {
	got := SharedFromChoices([]ServiceScopeChoice{
		{File: "a.yml", Service: "db", Scope: domain.JobScopeShared},
		{File: "a.yml", Service: "api"},
		{File: "a.yml", Service: "built", Scope: domain.JobScopeShared, Fixed: true},
	})
	if len(got) != 1 || got[0].Service != "db" {
		t.Errorf("shared = %v, want db alone: a fixed choice is never shared", got)
	}
}

// Three rows per shared service, and only the name carries a proposal: wtm has
// nothing honest to say about the two commands.
func TestNamespaceFieldsProposeOnlyTheName(t *testing.T) {
	got := NamespaceFields(NamespaceFieldsParams{
		Shared: []domain.SharedComposeService{{File: "c.yml", Service: "db-crm"}},
		Ports:  map[string][]string{"db-crm": {"CRM_DB_PORT"}},
	})

	if len(got) != 3 {
		t.Fatalf("fields = %d, want three", len(got))
	}
	if got[0].Field != domain.NamespaceFieldName || got[0].Value != domain.NamespaceNameDefault {
		t.Errorf("name row = %+v", got[0])
	}
	if got[1].Value != "" || got[2].Value != "" {
		t.Errorf("a command was pre-filled: %+v %+v", got[1], got[2])
	}
	// The variables are the job's own, under the names it declares them by;
	// TestNamespaceVarsAreGrouped covers how they are laid out.
	last := got[1].Vars[len(got[1].Vars)-1]
	if len(last.Vars) == 0 || last.Vars[0] != "$CRM_DB_PORT" {
		t.Errorf("vars = %+v, want the job's own port variable", got[1].Vars)
	}
}

// A namespace already in run.toml is opened for amendment, not asked for again.
func TestNamespaceFieldsOpenOnWhatIsAlreadyWritten(t *testing.T) {
	existing := domain.RunConfig{Jobs: []domain.JobConfig{{
		Name: "db-crm", Scope: domain.JobScopeShared,
		Namespace: &domain.JobNamespaceConfig{Name: "mine_{worktree}", Create: "./scripts/create.sh"},
	}}}

	got := NamespaceFields(NamespaceFieldsParams{
		Shared:   []domain.SharedComposeService{{File: "c.yml", Service: "db-crm"}},
		Existing: existing,
	})
	if got[0].Value != "mine_{worktree}" || got[1].Value != "./scripts/create.sh" {
		t.Errorf("fields = %+v, want the config's own values", got[:2])
	}
}

// An empty create is an answer: the service is shared outright, data included.
func TestNamespacesFromFieldsDropsAServiceWithNoCreate(t *testing.T) {
	got := NamespacesFromFields([]domain.NamespaceField{
		{Job: "db", Field: domain.NamespaceFieldName, Value: "app_{worktree}"},
		{Job: "db", Field: domain.NamespaceFieldCreate},
		{Job: "db", Field: domain.NamespaceFieldRemove},
		{Job: "kc", Field: domain.NamespaceFieldName, Value: "{worktree}"},
		{Job: "kc", Field: domain.NamespaceFieldCreate, Value: "./scripts/kc.sh"},
		{Job: "kc", Field: domain.NamespaceFieldRemove, Value: "./scripts/kc-rm.sh"},
	})

	if got["db"] != nil {
		t.Errorf("db = %+v, want none: no create means shared outright", got["db"])
	}
	if got["kc"] == nil || got["kc"].Create != "./scripts/kc.sh" {
		t.Errorf("kc = %+v", got["kc"])
	}
}

// The name is data wtm substitutes into before anything runs, so it takes the
// {…} placeholders. Offering it $WTM_WORKTREE advertised something that cannot
// work there: no shell ever sees a name, so nothing would expand it.
func TestNamespaceVarsOnTheNameRowAreThePlaceholders(t *testing.T) {
	got := NamespaceFields(NamespaceFieldsParams{
		Shared: []domain.SharedComposeService{{Service: "db"}},
		Ports:  map[string][]string{"db": {"CRM_DB_PORT"}},
	})[0].Vars

	if len(got) != 1 || got[0].Label != domain.NamespaceVarSubstituted {
		t.Fatalf("groups = %+v, want the substituted placeholders alone", got)
	}
	if got[0].Vars[0] != domain.NamespaceTokenWorktree {
		t.Errorf("vars = %v, want %s", got[0].Vars, domain.NamespaceTokenWorktree)
	}
	for _, name := range got[0].Vars {
		if strings.HasPrefix(name, "$") {
			t.Errorf("the name row offers %s, which no shell expands there", name)
		}
	}
}

// The two commands are /bin/sh lines, so they take environment variables,
// grouped by the half that is the same everywhere and the half that is this
// job's.
func TestNamespaceVarsOnACommandRowAreEnvironmentVariables(t *testing.T) {
	got := NamespaceFields(NamespaceFieldsParams{
		Shared: []domain.SharedComposeService{{Service: "db"}},
		Ports:  map[string][]string{"db": {"CRM_DB_PORT"}},
	})[1].Vars

	if len(got) != 2 || got[0].Label != domain.NamespaceVarWorktree || got[1].Label != domain.NamespaceVarPorts {
		t.Fatalf("groups = %+v", got)
	}
	if got[1].Vars[0] != "$CRM_DB_PORT" {
		t.Errorf("ports = %v, want the job's own variable", got[1].Vars)
	}
}

// A job that declares no port gets no ports row rather than an empty one.
func TestNamespaceVarsOmitAnEmptyPortsGroup(t *testing.T) {
	got := NamespaceFields(NamespaceFieldsParams{
		Shared: []domain.SharedComposeService{{Service: "keycloak"}},
	})[1].Vars
	if len(got) != 1 {
		t.Errorf("groups = %+v, want the worktree one alone", got)
	}
}

func TestWrapVarsBreaksOnlyBetweenVariables(t *testing.T) {
	vars := []string{"$AAAA", "$BBBB", "$CCCC", "$DDDD"}
	got := WrapVars(vars, 14)

	if len(got) < 2 {
		t.Fatalf("lines = %v, want a wrap", got)
	}
	seen := 0
	for _, line := range got {
		width := 0
		for i, name := range line {
			if i > 0 {
				width += len(domain.NamespaceVarSep)
			}
			width += len(name)
			seen++
		}
		if width > 14 && len(line) > 1 {
			t.Errorf("line %v is %d wide, past 14", line, width)
		}
	}
	if seen != len(vars) {
		t.Errorf("wrapped %d variables, want %d — none may be dropped", seen, len(vars))
	}
}

// A variable longer than the room gets its own line rather than being cut.
func TestWrapVarsNeverCutsAVariable(t *testing.T) {
	got := WrapVars([]string{"$A_VERY_LONG_PORT_VARIABLE_NAME", "$B"}, 8)
	if got[0][0] != "$A_VERY_LONG_PORT_VARIABLE_NAME" {
		t.Errorf("lines = %v, want the long name whole on its own line", got)
	}
}
