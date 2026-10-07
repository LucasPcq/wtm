package wt

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func statusDoc(t *testing.T, args ...string) domain.StatusDocument {
	t.Helper()
	stdout, _, err := runWtCmd(t, append([]string{domain.CmdStatus, "--" + domain.FlagOutput, domain.OutputJSON}, args...)...)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var doc domain.StatusDocument
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	return doc
}

// No --yes in JSON: the command asks nothing, so nothing has to be bypassed.
func TestStatusReadsTheWorktreeItIsRunFromOrTheOneItNames(t *testing.T) {
	dir := createRepo(t)
	branch := "feat/status"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}

	restore := chdir(t, resolveWorktreePath(t, dir, branch))
	inside := statusDoc(t)
	restore()
	named := statusDoc(t, branch)
	restore = chdir(t, dir)
	main := statusDoc(t)
	restore()

	if inside.Branch != branch || named.Branch != branch {
		t.Errorf("inside = %q, named = %q, want %q both", inside.Branch, named.Branch, branch)
	}
	if main.Branch != "main" || !main.Main {
		t.Errorf("main = %+v, want the main checkout", main)
	}
	if inside.Problems == nil || inside.Jobs == nil || inside.Env.Missing == nil {
		t.Errorf("doc = %+v, want empty lists rather than null", inside)
	}
}

func TestStatusAllWritesOneDocumentPerWorktree(t *testing.T) {
	createRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/all", "--from", "main", "--"+domain.FlagYes); err != nil {
		t.Fatalf("create: %v", err)
	}

	stdout, _, err := runWtCmd(t, domain.CmdStatus, "--"+domain.FlagAll, "--"+domain.FlagOutput, domain.OutputJSON)
	if err != nil {
		t.Fatalf("status --all: %v", err)
	}
	var docs []domain.StatusDocument
	if err := json.Unmarshal([]byte(stdout), &docs); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	if len(docs) != 2 {
		t.Errorf("docs = %+v, want main and feat/all", docs)
	}
}

func TestStatusAllRefusesAWorktreeAsAUsageError(t *testing.T) {
	createRepo(t)

	_, _, err := runWtCmd(t, domain.CmdStatus, "main", "--"+domain.FlagAll)

	if !errors.Is(err, domain.ErrUsage) {
		t.Errorf("err = %v, want a usage error", err)
	}
}

func TestStatusTextConcludesOnWhatIsLeftToFix(t *testing.T) {
	restore := chdir(t, createRepo(t))
	defer restore()

	stdout, _, err := runWtCmd(t, domain.CmdStatus)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(stdout, "main — nothing to fix") {
		t.Errorf("stdout = %q, want the headline", stdout)
	}
}
