package rules

import (
	"reflect"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// pair is a test helper building a parsed KEY=VALUE line (Raw omitted; DiffEnv only
// reads Kind/Key/Value/Export).
func pair(key, value string) domain.EnvLine {
	return domain.EnvLine{Kind: domain.EnvLinePair, Key: key, Value: value}
}

func TestDiffEnvResolutionCascade(t *testing.T) {
	cases := []struct {
		name       string
		parent     []domain.EnvLine
		main       []domain.EnvLine
		wantValue  string
		wantSource string
	}{
		{"parent wins over main", []domain.EnvLine{pair("K", "fromParent")}, []domain.EnvLine{pair("K", "fromMain")}, "fromParent", domain.EnvSourceParent},
		{"main used when parent absent", nil, []domain.EnvLine{pair("K", "fromMain")}, "fromMain", domain.EnvSourceMain},
		{"empty parent value falls through to main", []domain.EnvLine{pair("K", "")}, []domain.EnvLine{pair("K", "fromMain")}, "fromMain", domain.EnvSourceMain},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diff := DiffEnv(EnvDiffParams{
				Template: []domain.EnvLine{pair("K", "")},
				Parent:   c.parent,
				Main:     c.main,
				Child:    nil,
				Mode:     domain.EnvModeAdd,
			})
			if len(diff.Entries) != 1 {
				t.Fatalf("Entries = %d, want 1", len(diff.Entries))
			}
			e := diff.Entries[0]
			if e.Status != domain.EnvKeyResolved {
				t.Fatalf("Status = %q, want resolved", e.Status)
			}
			if e.ResolvedValue != c.wantValue || e.Source != c.wantSource {
				t.Errorf("resolved = (%q, %q), want (%q, %q)", e.ResolvedValue, e.Source, c.wantValue, c.wantSource)
			}
		})
	}
}

func TestDiffEnvMissingUnresolved(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair("API_KEY", "your-key-here")},
		Child:    nil,
		Mode:     domain.EnvModeAdd,
	})
	if len(diff.Entries) != 1 {
		t.Fatalf("Entries = %d, want 1", len(diff.Entries))
	}
	e := diff.Entries[0]
	if e.Status != domain.EnvKeyMissing {
		t.Errorf("Status = %q, want missing_unresolved", e.Status)
	}
	if e.Placeholder != "your-key-here" {
		t.Errorf("Placeholder = %q, want template value", e.Placeholder)
	}
	if e.ResolvedValue != "" || e.Source != "" {
		t.Errorf("template must not resolve a value: got (%q, %q)", e.ResolvedValue, e.Source)
	}
}

func TestDiffEnvAddModeNeverConflicts(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair("K", "")},
		Main:     []domain.EnvLine{pair("K", "fromMain")},
		Child:    []domain.EnvLine{pair("K", "localValue")},
		Mode:     domain.EnvModeAdd,
	})
	e := diff.Entries[0]
	if e.Status != domain.EnvKeyResolved {
		t.Errorf("add mode Status = %q, want resolved (existing value untouched)", e.Status)
	}
	if e.CurrentValue != "localValue" {
		t.Errorf("CurrentValue = %q, want localValue", e.CurrentValue)
	}
	if EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("add mode must never classify a conflict")
	}
}

func TestDiffEnvRefreshMode(t *testing.T) {
	cases := []struct {
		name     string
		child    string
		main     string
		wantStat domain.EnvKeyStatus
	}{
		{"differing value is a conflict", "old", "new", domain.EnvKeyConflict},
		{"equal value is resolved", "same", "same", domain.EnvKeyResolved},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diff := DiffEnv(EnvDiffParams{
				Template: []domain.EnvLine{pair("K", "")},
				Main:     []domain.EnvLine{pair("K", c.main)},
				Child:    []domain.EnvLine{pair("K", c.child)},
				Mode:     domain.EnvModeRefresh,
			})
			e := diff.Entries[0]
			if e.Status != c.wantStat {
				t.Errorf("Status = %q, want %q", e.Status, c.wantStat)
			}
			if c.wantStat == domain.EnvKeyConflict {
				if e.CurrentValue != c.child || e.ResolvedValue != c.main {
					t.Errorf("conflict values = (%q, %q), want (%q, %q)", e.CurrentValue, e.ResolvedValue, c.child, c.main)
				}
			}
		})
	}
}

func TestDiffEnvOrphanAndMainOnly(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair("EXPECTED", "")},
		Main:     []domain.EnvLine{pair("EXPECTED", "v"), pair("MAIN_ONLY", "m")},
		Child:    []domain.EnvLine{pair("EXPECTED", "v"), pair("LOCAL_ONLY", "x")},
		Mode:     domain.EnvModeAdd,
	})

	byKey := map[string]domain.EnvKeyDiff{}
	for _, e := range diff.Entries {
		byKey[e.Key] = e
	}

	if got := byKey["LOCAL_ONLY"].Status; got != domain.EnvKeyOrphan {
		t.Errorf("LOCAL_ONLY Status = %q, want orphan", got)
	}
	if got := byKey["MAIN_ONLY"].Status; got != domain.EnvKeyResolved {
		t.Errorf("MAIN_ONLY Status = %q, want resolved (main-only is expected, not orphan)", got)
	}
	if byKey["MAIN_ONLY"].ResolvedValue != "m" {
		t.Errorf("MAIN_ONLY ResolvedValue = %q, want m", byKey["MAIN_ONLY"].ResolvedValue)
	}
}

func TestDiffEnvEntryOrder(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair("T_ONLY", "ph")},
		Main:     []domain.EnvLine{pair("M_ONLY", "mv")},
		Child:    []domain.EnvLine{pair("C1", "a"), pair("C2", "b")},
		Mode:     domain.EnvModeAdd,
	})
	var order []string
	for _, e := range diff.Entries {
		order = append(order, e.Key)
	}
	want := []string{"C1", "C2", "T_ONLY", "M_ONLY"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("entry order = %v, want %v", order, want)
	}
}

func TestApplyEnvDiffConflictKeepOverwriteEdit(t *testing.T) {
	child := []domain.EnvLine{{Kind: domain.EnvLinePair, Key: "K", Value: "old", Raw: "K=old"}}
	diff := DiffEnv(EnvDiffParams{
		Main:  []domain.EnvLine{pair("K", "new")},
		Child: child,
		Mode:  domain.EnvModeRefresh,
	})

	cases := []struct {
		name      string
		decisions map[string]domain.EnvConflictDecision
		filled    map[string]string
		want      string
	}{
		{"keep by default", nil, nil, "K=old"},
		{"explicit keep", map[string]domain.EnvConflictDecision{"K": domain.EnvDecisionKeep}, nil, "K=old"},
		{"overwrite", map[string]domain.EnvConflictDecision{"K": domain.EnvDecisionOverwrite}, nil, "K=new"},
		{"edit via filled value wins", map[string]domain.EnvConflictDecision{"K": domain.EnvDecisionKeep}, map[string]string{"K": "edited"}, "K=edited"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := ApplyEnvDiff(ApplyEnvDiffParams{
				Child:        child,
				Diff:         diff,
				Decisions:    c.decisions,
				FilledValues: c.filled,
			})
			if got := RenderEnv(out); got != c.want {
				t.Errorf("RenderEnv = %q, want %q", got, c.want)
			}
		})
	}
}

func TestApplyEnvDiffAddsResolvedAndFilledMissing(t *testing.T) {
	child := []domain.EnvLine{{Kind: domain.EnvLinePair, Key: "EXISTING", Value: "1", Raw: "EXISTING=1"}}
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair("SECRET", "placeholder")},
		Main:     []domain.EnvLine{pair("FROM_MAIN", "m")},
		Child:    child,
		Mode:     domain.EnvModeAdd,
	})

	// Without a supplied value the missing key is not added; the resolved key is.
	out := ApplyEnvDiff(ApplyEnvDiffParams{Child: child, Diff: diff})
	if got, want := RenderEnv(out), "EXISTING=1\nFROM_MAIN=m"; got != want {
		t.Errorf("without fill: RenderEnv = %q, want %q", got, want)
	}

	// Supplying the missing value appends it too, in entry order (template before main).
	out = ApplyEnvDiff(ApplyEnvDiffParams{Child: child, Diff: diff, FilledValues: map[string]string{"SECRET": "s3cr3t"}})
	if got, want := RenderEnv(out), "EXISTING=1\nSECRET=s3cr3t\nFROM_MAIN=m"; got != want {
		t.Errorf("with fill: RenderEnv = %q, want %q", got, want)
	}
}

func TestApplyEnvDiffOrphanPrune(t *testing.T) {
	child := []domain.EnvLine{
		{Kind: domain.EnvLinePair, Key: "KEEP", Value: "1", Raw: "KEEP=1"},
		{Kind: domain.EnvLinePair, Key: "ORPHAN", Value: "2", Raw: "ORPHAN=2"},
	}
	diff := DiffEnv(EnvDiffParams{
		Main:  []domain.EnvLine{pair("KEEP", "1")},
		Child: child,
		Mode:  domain.EnvModeAdd,
	})

	noPrune := ApplyEnvDiff(ApplyEnvDiffParams{Child: child, Diff: diff, Prune: false})
	if got, want := RenderEnv(noPrune), "KEEP=1\nORPHAN=2"; got != want {
		t.Errorf("no-prune: RenderEnv = %q, want %q", got, want)
	}

	prune := ApplyEnvDiff(ApplyEnvDiffParams{Child: child, Diff: diff, Prune: true})
	if got, want := RenderEnv(prune), "KEEP=1"; got != want {
		t.Errorf("prune: RenderEnv = %q, want %q", got, want)
	}
}

func TestApplyEnvDiffRoundTripPreservesUnrelatedLines(t *testing.T) {
	child := []domain.EnvLine{
		{Kind: domain.EnvLineComment, Raw: "# header"},
		{Kind: domain.EnvLinePair, Key: "A", Value: "1", Raw: "A=1   # aligned"},
		{Kind: domain.EnvLineBlank, Raw: ""},
		{Kind: domain.EnvLinePair, Key: "B", Value: "old", Raw: "B=old"},
	}
	diff := DiffEnv(EnvDiffParams{
		Main:  []domain.EnvLine{pair("A", "1"), pair("B", "new")},
		Child: child,
		Mode:  domain.EnvModeRefresh,
	})

	out := ApplyEnvDiff(ApplyEnvDiffParams{
		Child:     child,
		Diff:      diff,
		Decisions: map[string]domain.EnvConflictDecision{"B": domain.EnvDecisionOverwrite},
	})
	// A resolved (== source) stays byte-for-byte; comment/blank preserved; B re-rendered.
	want := "# header\nA=1   # aligned\n\nB=new"
	if got := RenderEnv(out); got != want {
		t.Errorf("RenderEnv = %q, want %q", got, want)
	}
}

// A child holding its own worktree's port must not be reported as conflicting
// with a source holding another worktree's — the two are the same setting, and
// --on-conflict overwrite would otherwise undo the isolation on every run.
func TestDiffEnvIgnoresThePortOffsetBetweenWorktrees(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Main:       ParseEnv("DATABASE_URL=postgres://u:pw@localhost:5432/app\n"),
		Child:      ParseEnv("DATABASE_URL=postgres://u:pw@localhost:5442/app\n"),
		Mode:       domain.EnvModeRefresh,
		PortValues: map[string]EnvValueRef{"DATABASE_URL": {Base: 5432}},
		PortBlock:  10,
	})

	if EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("DiffEnv() reported a conflict for a value differing only by the offset: %+v", diff.Entries)
	}
}

func TestDiffEnvStillReportsARealConflictOnALinkedKey(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Main:       ParseEnv("DATABASE_URL=postgres://u:old@localhost:5432/app\n"),
		Child:      ParseEnv("DATABASE_URL=postgres://u:new@localhost:5442/app\n"),
		Mode:       domain.EnvModeRefresh,
		PortValues: map[string]EnvValueRef{"DATABASE_URL": {Base: 5432}},
		PortBlock:  10,
	})

	if !EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("DiffEnv() hid a password conflict behind the port reduction: %+v", diff.Entries)
	}
}

func TestDiffEnvComparesUnlinkedKeysVerbatim(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Main:       ParseEnv("PORT=5432\n"),
		Child:      ParseEnv("PORT=5442\n"),
		Mode:       domain.EnvModeRefresh,
		PortValues: map[string]EnvValueRef{"OTHER": {Base: 5432}},
		PortBlock:  10,
	})

	if !EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("DiffEnv() reduced a key no link follows: %+v", diff.Entries)
	}
}

// The drift the addressing switch would otherwise create: main keeps a port
// because it is the one checkout that exists without wtm, while a worktree holds
// the name. Both spell one setting, and every `wtm env` would report every URL
// key as a conflict if the reduction did not canonicalize them together.
func TestDiffEnvIgnoresTheAddressingBetweenMainAndAWorktree(t *testing.T) {
	refs := map[string]EnvValueRef{
		"VITE_API_URL": {Base: 4001, JobLabel: "api-dev", Project: "monorepo"},
	}

	diff := DiffEnv(EnvDiffParams{
		Main:       ParseEnv("VITE_API_URL=http://localhost:4001\n"),
		Child:      ParseEnv("VITE_API_URL=http://api-dev.feat-x.monorepo.localhost:10080\n"),
		Mode:       domain.EnvModeRefresh,
		PortValues: refs,
		PortBlock:  10,
	})

	if EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("DiffEnv() reported a conflict between a port and its own address: %+v", diff.Entries)
	}
}

// Two worktrees differ by their route's worktree segment alone, which is the
// same setting seen twice — not a difference to reconcile.
func TestDiffEnvIgnoresTheWorktreeSegmentBetweenTwoWorktrees(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Main:  ParseEnv("VITE_API_URL=http://api-dev.feat-y.monorepo.localhost:10080\n"),
		Child: ParseEnv("VITE_API_URL=http://api-dev.feat-x.monorepo.localhost\n"),
		Mode:  domain.EnvModeRefresh,
		PortValues: map[string]EnvValueRef{
			"VITE_API_URL": {Base: 4001, JobLabel: "api-dev", Project: "monorepo"},
		},
		PortBlock: 10,
	})

	if EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("DiffEnv() reported a conflict between two worktrees' routes: %+v", diff.Entries)
	}
}

// The reduction must not become a blanket amnesty: a route pointing at another
// job is a real difference, and hiding it would let one app call another.
func TestDiffEnvStillReportsAConflictBetweenTwoJobsRoutes(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Main:  ParseEnv("VITE_API_URL=http://api-dev.feat-x.monorepo.localhost:10080\n"),
		Child: ParseEnv("VITE_API_URL=http://web-dev.feat-x.monorepo.localhost:10080\n"),
		Mode:  domain.EnvModeRefresh,
		PortValues: map[string]EnvValueRef{
			"VITE_API_URL": {Base: 4001, JobLabel: "api-dev", Project: "monorepo"},
		},
		PortBlock: 10,
	})

	if !EnvDiffHasStatus(EnvDiffFilter{Diff: diff, Status: domain.EnvKeyConflict}) {
		t.Errorf("DiffEnv() hid a route pointing at another job: %+v", diff.Entries)
	}
}

// A key the owned pass writes into this file is the pass's to add: offering
// the template's placeholder for it announces a value that is never written.
func TestDiffEnvLeavesAnAbsentOwnedKeyToTheOwnedPass(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair(domain.EnvComposeProjectName, "placeholder"), pair("PORT", "3000")},
		Child:    []domain.EnvLine{pair("PORT", "3000")},
		Mode:     domain.EnvModeAdd,
		Owned:    map[string]bool{domain.EnvComposeProjectName: true},
	})
	for _, entry := range diff.Entries {
		if entry.Key == domain.EnvComposeProjectName {
			t.Fatalf("entry = %+v, want the key left to the owned pass", entry)
		}
	}
}

func TestDiffEnvStillReportsAnAbsentIdentityKeyNoPassWrites(t *testing.T) {
	diff := DiffEnv(EnvDiffParams{
		Template: []domain.EnvLine{pair(domain.EnvComposeProjectName, "placeholder")},
		Mode:     domain.EnvModeAdd,
	})
	if len(diff.Entries) != 1 || diff.Entries[0].Status != domain.EnvKeyMissing {
		t.Fatalf("entries = %+v, want the key missing like any other", diff.Entries)
	}
}
