package checkout

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		maxLen int
		want   string
	}{
		{"shorter than max", "abc", 10, "abc"},
		{"equal to max", "abcde", 5, "abcde"},
		{"longer than max", "abcdef", 5, "abcd…"},
		{"empty", "", 5, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := truncate(c.in, c.maxLen); got != c.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.maxLen, got, c.want)
			}
		})
	}
}

func TestBuildPRItems_LinkedAndForkDisabled(t *testing.T) {
	prs := []domain.PRInfo{
		{Number: 1, Title: "open one", Author: "alice", Branch: "feat/a"},
		{Number: 2, Title: "already checked out", Author: "bob", Branch: "feat/b"},
		{Number: 3, Title: "from a fork", Author: "carol", Branch: "feat/c", IsFork: true},
	}
	existing := []string{"feat/b"}

	items := buildPRItems(prs, existing)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	if items[0].Disabled || len(items[0].Badges) != 0 {
		t.Errorf("PR #1 should be selectable with no badge, got %+v", items[0])
	}
	if !items[1].Disabled || items[1].Badges[0].Text != domain.BadgeTextLinked {
		t.Errorf("PR #2 should be disabled with 'linked' badge, got %+v", items[1])
	}
	if !items[2].Disabled || items[2].Badges[0].Text != domain.BadgeTextFork {
		t.Errorf("PR #3 should be disabled with 'fork' badge, got %+v", items[2])
	}
}

func TestBuildBranchItems_BaseFirst(t *testing.T) {
	// Candidates arrive pre-sorted from rules.MergeBranchCandidates; buildBranchItems
	// only pins the base first and preserves the remaining order.
	candidates := []domain.BranchCandidate{
		{Name: "alpha"}, {Name: "main"}, {Name: "zeta"},
	}
	items := buildBranchItems(candidates, "main")

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Value != "main" {
		t.Errorf("base branch should be first, got %q", items[0].Value)
	}
	if items[1].Value != "alpha" || items[2].Value != "zeta" {
		t.Errorf("rest should keep sorted order, got %q then %q", items[1].Value, items[2].Value)
	}
}

func TestResolveSourcePrecedence(t *testing.T) {
	pr := &domain.PRInfo{BaseBranch: "base"}

	cases := []struct {
		name         string
		fromOverride string
		preselected  *domain.PRInfo
		want         string
	}{
		{"flag override wins", "flagbr", pr, "flagbr"},
		{"falls back to PR base", "", pr, "base"},
		{"empty when nothing", "", nil, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// No parent step in prev, so the flag/PR-base fallbacks are exercised.
			if got := resolveSource(nil, c.fromOverride, c.preselected); got != c.want {
				t.Fatalf("resolveSource() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveEnvPrecedence(t *testing.T) {
	if got := resolveEnv(nil, "override"); got != "override" {
		t.Errorf("resolveEnv with override = %q, want %q", got, "override")
	}
	if got := resolveEnv(nil, ""); got != "" {
		t.Errorf("resolveEnv without override = %q, want empty", got)
	}
}

func TestBuildBranchItems_RemoteAfterSeparator(t *testing.T) {
	candidates := []domain.BranchCandidate{
		{Name: "main"},
		{Name: "origin/release", IsRemote: true},
	}
	items := buildBranchItems(candidates, "main")

	if len(items) != 3 {
		t.Fatalf("expected base + separator + remote, got %d: %+v", len(items), items)
	}
	if !items[1].Separator {
		t.Errorf("expected a separator before remotes, got %+v", items[1])
	}
	if items[2].Value != "origin/release" || len(items[2].Badges) == 0 || items[2].Badges[0].Text != domain.BadgeTextRemote {
		t.Errorf("expected remote item with remote badge, got %+v", items[2])
	}
}

// A PR checked out with every flag given runs no wizard at all, and must still
// land on the isolation the flag, else the project, asked for.
func TestPreselectedWithEverythingFixedKeepsTheIsolation(t *testing.T) {
	pr := domain.PRInfo{Number: 7, Branch: "feat/x", BaseBranch: "main"}
	res, err := RunWizard(WizardParams{
		Preselected:      &pr,
		IsolationApplies: true,
		IsolationDefault: domain.IsolationIsolated,
		// The flag answered it: nothing is left to ask.
		IsolationOverride: domain.IsolationVerbatim,
	})
	if err != nil {
		t.Fatalf("RunWizard: %v", err)
	}
	if res.Isolation != domain.IsolationVerbatim {
		t.Errorf("isolation = %q, want the flag's", res.Isolation)
	}
}

func TestCheckoutRecapNamesTheIsolation(t *testing.T) {
	pr := domain.PRInfo{Number: 7, Title: "t", Branch: "feat/x", BaseBranch: "main"}
	params := WizardParams{Preselected: &pr, IsolationApplies: true, IsolationOverride: domain.IsolationVerbatim}
	label := func([]components.Step) string { return prDisplay(pr) }
	recap := buildCheckoutRecap(nil, params, label, label)
	if !strings.Contains(recap, domain.IsolationSummaryVerbatim) {
		t.Errorf("recap = %q, want the isolation named", recap)
	}
}
