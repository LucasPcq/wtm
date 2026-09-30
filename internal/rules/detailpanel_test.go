package rules

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

func sectionKeys(sections []domain.DetailSection) []string {
	keys := make([]string, 0, len(sections))
	for _, section := range sections {
		keys = append(keys, section.Key)
	}
	return keys
}

func TestDetailSectionsOmitsWhatHasNothingToSay(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Detail:       domain.WorktreeDetail{Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}}},
		DetailLoaded: true,
	})

	for _, key := range sectionKeys(sections) {
		if key == domain.DetailSectionReview {
			t.Error("REVIEW ne doit pas apparaître sans PR")
		}
		if key == domain.DetailSectionChanges {
			t.Error("CHANGES ne doit pas apparaître sur un worktree propre")
		}
	}
}

func TestReviewShowsUnavailableReasonWhenPRDataFailedToLoad(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:        domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PRUnavailable: "GitHub CLI not found",
		DetailLoaded:  true,
	})

	var review *domain.DetailSection
	for i := range sections {
		if sections[i].Key == domain.DetailSectionReview {
			review = &sections[i]
		}
	}
	if review == nil {
		t.Fatal("un gh cassé doit produire une section REVIEW, pas son absence silencieuse")
	}
	if len(review.Lines) != 1 || !strings.Contains(review.Lines[0], "GitHub CLI not found") {
		t.Errorf("REVIEW = %v, want the unavailable reason", review.Lines)
	}
}

func TestReviewStaysAbsentWithNoPRAndNoFailure(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		DetailLoaded: true,
	})

	for _, key := range sectionKeys(sections) {
		if key == domain.DetailSectionReview {
			t.Error("pas de PR et gh disponible : REVIEW doit rester absente, exactement comme aujourd'hui")
		}
	}
}

func TestReviewSectionShowsChecksAndDecision(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PR: &domain.PRInfo{
			Number: 67, Title: "feat: x", State: "OPEN",
			Checks:         domain.PRChecks{Passed: 12, Failed: 1},
			ReviewDecision: "CHANGES_REQUESTED",
		},
	})

	var review domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionReview {
			review = section
		}
	}
	if review.Key == "" {
		t.Fatal("REVIEW absente alors qu'une PR existe")
	}

	body := strings.Join(review.Lines, "\n")
	for _, want := range []string{"#67", "feat: x", "12", "changes requested"} {
		if !strings.Contains(body, want) {
			t.Errorf("REVIEW = %q, doit contenir %q", body, want)
		}
	}
}

func TestReviewSectionWithoutChecks(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		PR:     &domain.PRInfo{Number: 68, Title: "feat: y", State: "OPEN"},
	})
	for _, section := range sections {
		if section.Key != domain.DetailSectionReview {
			continue
		}
		if strings.Contains(strings.Join(section.Lines, "\n"), "checks") {
			t.Error("pas de ligne checks quand aucun check n'a tourné")
		}
	}
}

func TestDetailSectionsKeepsFixedOrder(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		PR:     &domain.PRInfo{Number: 67, Title: "feat: x", State: "OPEN"},
		Detail: domain.WorktreeDetail{
			Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
			Changes: domain.WorkingChanges{Modified: 2, Files: []domain.PorcelainEntry{{Status: " M", Path: "a.go"}}},
		},
		DetailLoaded: true,
	})

	want := []string{
		domain.DetailSectionReview,
		domain.DetailSectionChanges,
		domain.DetailSectionActivity,
		domain.DetailSectionLinks,
	}
	got := sectionKeys(sections)
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFitSectionsDropsFromTheBottom(t *testing.T) {
	sections := []domain.DetailSection{
		{Key: domain.DetailSectionReview, Lines: []string{"a", "b"}},
		{Key: domain.DetailSectionChanges, Lines: []string{"c", "d"}},
		{Key: domain.DetailSectionActivity, Lines: []string{"e", "f"}},
		{Key: domain.DetailSectionLinks, Lines: []string{"g", "h"}},
	}

	// DetailSectionChrome = 3 (title + blank above + blank below), so
	// two 2-line sections cost 2*(3+2) = 10, not 8 as before that correction.
	got := sectionKeys(FitSections(FitSectionsParams{Sections: sections, Height: 10}))
	want := []string{domain.DetailSectionReview, domain.DetailSectionChanges}
	if len(got) != len(want) {
		t.Fatalf("sections retenues = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVitalChipsStateFirstAndOnlyStateColored(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	chips := VitalChips(VitalChipsParams{
		Status: domain.WorktreeStatus{
			Branch: "feat/x", IsDirty: true, CommitsAhead: 3,
			OriginAhead: 2, OriginBehind: 1, OriginState: domain.DivergenceDiverged,
		},
		LastCommitAt: now.Add(-3 * time.Hour),
		Now:          now,
	})

	if len(chips) == 0 {
		t.Fatal("aucun chip")
	}
	if !chips[0].State {
		t.Error("le premier chip doit être l'état : c'est la lecture la plus rapide")
	}
	for i, chip := range chips[1:] {
		if chip.State {
			t.Errorf("chip[%d] est marqué State — l'état est le seul chip coloré", i+1)
		}
	}
}

func TestVitalChipsNeverMentionsCreated(t *testing.T) {
	now := time.Now()
	chips := VitalChips(VitalChipsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", CreatedAt: now.Add(-48 * time.Hour)},
		LastCommitAt: now.Add(-time.Hour),
		Now:          now,
	})
	for _, chip := range chips {
		if chip.Text == "" {
			t.Error("un chip vide ne doit pas être émis")
		}
		if len(chip.Text) >= 7 && chip.Text[:7] == "created" {
			t.Error("created appartient à LINKS, pas à la bande vitale")
		}
	}
}

func TestChangesSectionSummaryIsOnTitleRowNotALine(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		Height: 24,
		Detail: domain.WorktreeDetail{
			Changes: domain.WorkingChanges{
				Modified:  2,
				Untracked: 1,
				Files: []domain.PorcelainEntry{
					{Status: " M", Path: "a.go"},
					{Status: " M", Path: "b.go"},
					{Status: "??", Path: "c.go"},
				},
			},
		},
		DetailLoaded: true,
	})

	var changes domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionChanges {
			changes = section
		}
	}
	if changes.Key == "" {
		t.Fatal("CHANGES absente alors que le worktree est sale")
	}
	if !strings.Contains(changes.TitleRight, "2 modified") || !strings.Contains(changes.TitleRight, "1 untracked") {
		t.Errorf("TitleRight = %q, doit porter le résumé des comptes", changes.TitleRight)
	}
	if len(changes.Lines) == 0 || !strings.Contains(changes.Lines[0], "a.go") {
		t.Errorf("Lines[0] = %q, doit être le premier fichier — le résumé n'est plus une ligne", changes.Lines[0])
	}
	for _, line := range changes.Lines {
		if strings.Contains(line, "modified") {
			t.Errorf("Lines = %v, le résumé ne doit plus apparaître comme une ligne du corps", changes.Lines)
		}
	}
}

func TestSectionsHeightCountsTheChromeAroundEachSection(t *testing.T) {
	// A leading blank, the REVIEW title, a blank under it, then 2 body lines —
	// 5 rows total, not 4. Pins DetailSectionChrome = 3.
	review := domain.DetailSection{
		Key: domain.DetailSectionReview,
		Lines: []string{
			"#67  feat(ui): improve dashboard design  OPEN",
			"checks ✓ 12  ✗ 1  ·  review  changes requested",
		},
	}
	if got := sectionsHeight([]domain.DetailSection{review}); got != 5 {
		t.Errorf("sectionsHeight(REVIEW, 2 lignes de corps) = %d, want 5 (3 de chrome + 2 lignes)", got)
	}
}

func TestListBudgetsLeaveRoomForLinksAtHeight30With18Files(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	files := make([]domain.PorcelainEntry, 18)
	for i := range files {
		files[i] = domain.PorcelainEntry{Status: " M", Path: "file.go"}
	}
	commits := make([]domain.CommitSummary, 5)
	for i := range commits {
		commits[i] = domain.CommitSummary{SHA: "abc1234", Subject: "feat: x", At: now.Add(-time.Hour)}
	}

	// Fully populated REVIEW + LINKS (every LINKS field, all 5 lines) plus 18
	// changed files and 5 commits: with the old DetailFixedRows=10 guess, this
	// stack computes to 34 rows at Height=30 and FitSections drops LINKS,
	// taking Path off screen on a panel that had room for everything.
	params := DetailSectionsParams{
		Status: domain.WorktreeStatus{
			Branch: "feat/x", Path: "/wt/x", IsDirty: true, CreatedAt: now.Add(-48 * time.Hour),
		},
		Parent: "main",
		Height: 30,
		Now:    now,
		PR:     &domain.PRInfo{Number: 67, Title: "feat: x", State: "OPEN"},
		Detail: domain.WorktreeDetail{
			Commits:  commits,
			Changes:  domain.WorkingChanges{Modified: 18, Files: files},
			Children: []string{"chore/deps-bump"},
			EnvDrift: domain.EnvDriftSummary{Configured: true, Missing: 2},
		},
		DetailLoaded: true,
	}

	fit := FitSections(FitSectionsParams{Sections: DetailSections(params), Height: params.Height})

	for _, key := range sectionKeys(fit) {
		if key == domain.DetailSectionLinks {
			return
		}
	}
	t.Errorf("LINKS absente à Height=30 — le panneau avait de la place pour Path (sections retenues: %v)",
		sectionKeys(fit))
}

func TestChangesSectionSaysWhyOnFailure(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		Height:       24,
		DetailLoaded: true,
		Detail: domain.WorktreeDetail{
			Failures: map[domain.DetailFamily]error{domain.DetailFamilyChanges: errors.New("git status failed")},
		},
	})

	var changes domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionChanges {
			changes = section
		}
	}
	if changes.Key == "" {
		t.Fatal("git status en échec doit produire une section CHANGES qui dit pourquoi, pas son absence")
	}
	if len(changes.Lines) != 1 || !strings.Contains(changes.Lines[0], "git status failed") {
		t.Errorf("CHANGES = %v, want the failure reason", changes.Lines)
	}
}

func TestActivitySectionSaysWhyOnFailure(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Height:       24,
		DetailLoaded: true,
		Detail: domain.WorktreeDetail{
			Failures: map[domain.DetailFamily]error{domain.DetailFamilyCommits: errors.New("git log failed")},
		},
	})

	var activity domain.DetailSection
	for _, section := range sections {
		if section.Key == domain.DetailSectionActivity {
			activity = section
		}
	}
	if activity.Key == "" {
		t.Fatal("git log en échec doit produire une section ACTIVITY qui dit pourquoi, pas son absence")
	}
	if len(activity.Lines) != 1 || !strings.Contains(activity.Lines[0], "git log failed") {
		t.Errorf("ACTIVITY = %v, want the failure reason", activity.Lines)
	}
}

func TestFirstLoadPlaceholdersOrderedLikeRealSections(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: true},
		Height: 40,
		// DetailLoaded left false: state 3, nothing cached yet.
	})

	want := []string{domain.DetailSectionChanges, domain.DetailSectionActivity, domain.DetailSectionLinks}
	got := sectionKeys(sections)
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section[%d] = %q, want %q — rules/ owns the order, not the renderer", i, got[i], want[i])
		}
	}
	for _, key := range []string{domain.DetailSectionChanges, domain.DetailSectionActivity} {
		for _, section := range sections {
			if section.Key != key {
				continue
			}
			if len(section.Lines) != 1 || section.Lines[0] != domain.DashboardLoadingField {
				t.Errorf("%s placeholder = %v, want a single %q line", key, section.Lines, domain.DashboardLoadingField)
			}
		}
	}
}

func TestFailureLineCarriesTheGlyphNotConfiguredDoesNot(t *testing.T) {
	failure := envLine(domain.WorktreeDetail{
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: errors.New("git error")},
	})
	if !strings.Contains(failure, "⚠") {
		t.Errorf("envLine failure = %q, want the warning glyph", failure)
	}

	absent := envLine(domain.WorktreeDetail{EnvDrift: domain.EnvDriftSummary{Configured: false}})
	if strings.Contains(absent, "⚠") {
		t.Errorf("envLine legitimate absence = %q, must stay glyph-free — that contrast is the point", absent)
	}
}

func TestEnvLineGuardsNilFailureError(t *testing.T) {
	line := envLine(domain.WorktreeDetail{
		Failures: map[domain.DetailFamily]error{domain.DetailFamilyEnv: nil},
	})
	if strings.Contains(line, "<nil>") {
		t.Errorf("envLine = %q, une erreur nil ne doit jamais être formatée", line)
	}
}

// TestListBudgetsAreFixedCapsNotStateDriven pins §the reworked rule: each list
// gets a fixed maximum row count regardless of dirty/clean state or how much
// height is actually free — the old split gave the leftover to CHANGES when
// dirty and to ACTIVITY when clean, which read as randomness because the
// reasoning was invisible. A dirty and a clean worktree, given the exact same
// abundant height, must produce the exact same cap for both lists.
func TestListBudgetsAreFixedCapsNotStateDriven(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	files := make([]domain.PorcelainEntry, 20)
	for i := range files {
		files[i] = domain.PorcelainEntry{Status: " M", Path: "file.go"}
	}
	commits := make([]domain.CommitSummary, 20)
	for i := range commits {
		commits[i] = domain.CommitSummary{SHA: "abc1234", Subject: "feat: x", At: now.Add(-time.Hour)}
	}
	detail := domain.WorktreeDetail{
		Commits: commits,
		Changes: domain.WorkingChanges{Modified: 20, Files: files},
	}

	for _, dirty := range []bool{true, false} {
		sections := DetailSections(DetailSectionsParams{
			Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x", IsDirty: dirty},
			Height:       100,
			Now:          now,
			Detail:       detail,
			DetailLoaded: true,
		})

		var changes, activity domain.DetailSection
		for _, section := range sections {
			switch section.Key {
			case domain.DetailSectionChanges:
				changes = section
			case domain.DetailSectionActivity:
				activity = section
			}
		}
		if len(changes.Lines) != domain.DashboardDetailChanges {
			t.Errorf("dirty=%v: CHANGES = %d lines, want the fixed cap %d", dirty, len(changes.Lines), domain.DashboardDetailChanges)
		}
		if len(activity.Lines) != domain.DashboardDetailCommits {
			t.Errorf("dirty=%v: ACTIVITY = %d lines, want the fixed cap %d", dirty, len(activity.Lines), domain.DashboardDetailCommits)
		}
	}
}

// TestLinksFieldsAreIndentedLikeTheOtherLists pins that LINKS lines carry the
// same DetailListIndent prefix CHANGES and ACTIVITY already use, so the
// section does not look misaligned against its neighbours.
func TestLinksFieldsAreIndentedLikeTheOtherLists(t *testing.T) {
	section := linksSection(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		Parent: "main",
	})
	if len(section.Lines) == 0 {
		t.Fatal("LINKS a besoin d'au moins une ligne pour ce test")
	}
	for _, line := range section.Lines {
		if !strings.HasPrefix(line, domain.DetailListIndent) {
			t.Errorf("ligne LINKS = %q, want le préfixe %q comme CHANGES/ACTIVITY", line, domain.DetailListIndent)
		}
	}
}

// TestReviewLinesAreIndentedLikeTheOtherLists pins that REVIEW's PR header
// line and its checks/review-decision line carry the same DetailListIndent
// prefix as CHANGES/ACTIVITY/LINKS, so no section looks misaligned against
// its neighbours.
func TestReviewLinesAreIndentedLikeTheOtherLists(t *testing.T) {
	section := reviewSection(reviewSectionParams{
		PR: &domain.PRInfo{
			Number: 67, Title: "feat: x", State: "OPEN",
			Checks:         domain.PRChecks{Passed: 12, Failed: 1},
			ReviewDecision: domain.GHReviewDecisionApproved,
		},
	})
	if len(section.Lines) != 2 {
		t.Fatalf("REVIEW lines = %v, want 2 (header + checks)", section.Lines)
	}
	for _, line := range section.Lines {
		if !strings.HasPrefix(line, domain.DetailListIndent) {
			t.Errorf("ligne REVIEW = %q, want le préfixe %q comme CHANGES/ACTIVITY/LINKS", line, domain.DetailListIndent)
		}
	}
}

// TestActivityTitleRightShowsBranchDiff pins that ACTIVITY mirrors CHANGES:
// the committed diff volume against the base branch, plus the file count
// `git diff --shortstat` reports alongside it, show on the title row.
func TestActivityTitleRightShowsBranchDiff(t *testing.T) {
	section := activitySection(activitySectionParams{
		Commits: []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
		Budget:  domain.DashboardDetailCommits,
		Loaded:  true,
		Diff:    domain.DiffStat{FilesChanged: 7, Insertions: 214, Deletions: 38},
	})
	if !strings.Contains(section.TitleRight, "214") || !strings.Contains(section.TitleRight, "38") {
		t.Errorf("ACTIVITY.TitleRight = %q, want the committed diff volume", section.TitleRight)
	}
	if !strings.Contains(section.TitleRight, "7 files changed") {
		t.Errorf("ACTIVITY.TitleRight = %q, want the file count alongside it, like CHANGES", section.TitleRight)
	}
}

// TestActivityTitleRightSaysWhyOnDiffFailure pins that a diff-stat read
// failure never fabricates a zero: it says why, like every other family.
func TestActivityTitleRightSaysWhyOnDiffFailure(t *testing.T) {
	section := activitySection(activitySectionParams{
		Commits:     []domain.CommitSummary{{SHA: "abc1234", Subject: "feat: x"}},
		Budget:      domain.DashboardDetailCommits,
		Loaded:      true,
		DiffFailure: errors.New("git diff failed"),
	})
	if !strings.Contains(section.TitleRight, "git diff failed") {
		t.Errorf("ACTIVITY.TitleRight = %q, want the failure reason, not a fabricated zero", section.TitleRight)
	}
}

func runSectionOf(t *testing.T, params DetailSectionsParams) domain.DetailSection {
	t.Helper()
	for _, section := range DetailSections(params) {
		if section.Key == domain.DetailSectionRun {
			return section
		}
	}
	t.Fatal("no RUN section")
	return domain.DetailSection{}
}

func cellOf(row domain.DetailRow, kind domain.DetailCellKind) string {
	for _, cell := range row.Cells {
		if cell.Kind == kind {
			return cell.Text
		}
	}
	return ""
}

func TestRunSectionLeadsThePanelAndNamesEveryDeclaredJob(t *testing.T) {
	now := time.Now()
	sections := DetailSections(DetailSectionsParams{
		Status:       domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"},
		DetailLoaded: true,
		Height:       60,
		Now:          now,
		RunConfig:    domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}, {Name: "worker"}}},
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x", StartedAt: now.Add(-4 * time.Minute)},
			{Name: "worker", Status: domain.JobStatusStopped, WorkDir: "/wt/x"},
		},
		Addresses: map[string]domain.JobAddress{"web": {Ports: []int{3000}, URL: "http://web.wtm"}},
	})

	if sections[0].Key != domain.DetailSectionRun {
		t.Fatalf("first section = %q, want RUN in the lead", sections[0].Key)
	}
	if want := fmt.Sprintf(domain.DetailRunUpCountFmt, 1); sections[0].TitleRight != want {
		t.Errorf("TitleRight = %q, want %q", sections[0].TitleRight, want)
	}
	if len(sections[0].Rows) != 2 {
		t.Fatalf("rows = %d, want the job that is up and the one this session stopped", len(sections[0].Rows))
	}
	if got := cellOf(sections[0].Rows[1], domain.DetailCellName); got != "worker" {
		t.Errorf("second row name = %q, want the stopped job listed", got)
	}
}

// The url already carries the port: printing both is the same fact twice, and
// it is what made the section read as columns of noise.
func TestRunSectionShowsTheURLNeverAlsoItsPort(t *testing.T) {
	now := time.Now()
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: now,
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs: []domain.JobInfo{{
			Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x",
			StartedAt: now.Add(-4 * time.Minute),
		}},
		Addresses: map[string]domain.JobAddress{"web": {Ports: []int{3000}, URL: "http://web.wtm"}},
	})

	if section.Lines != nil {
		t.Errorf("Lines = %v, want a rowed section", section.Lines)
	}
	row := section.Rows[0]
	if got := cellOf(row, domain.DetailCellAddress); got != "http://web.wtm" {
		t.Errorf("address = %q, want the url alone", got)
	}
	if row.URL != "http://web.wtm" {
		t.Errorf("URL = %q, want it carried for the click", row.URL)
	}
	if !row.Up {
		t.Error("Up = false, want the running job marked up")
	}
	if cellOf(row, domain.DetailCellMeta) == "" {
		t.Error("meta is empty, want the uptime")
	}
}

func TestRunSectionFallsBackToPortsWhenAJobPublishesNoURL(t *testing.T) {
	now := time.Now()
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: now,
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "pg"}}},
		Jobs: []domain.JobInfo{{
			Name: "pg", Status: domain.JobStatusRunning, WorkDir: "/wt/x", StartedAt: now.Add(-time.Minute),
		}},
		Addresses: map[string]domain.JobAddress{"pg": {Ports: []int{5432}}},
	})

	row := section.Rows[0]
	if got, want := cellOf(row, domain.DetailCellAddress), fmt.Sprintf(domain.DetailJobPortFmt, 5432); got != want {
		t.Errorf("address = %q, want %q", got, want)
	}
	if row.URL != "" {
		t.Errorf("URL = %q, want none", row.URL)
	}
}

func TestRunSectionSaysNothingAboutADownJobBeyondItsGlyph(t *testing.T) {
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "worker"}}},
		Jobs: []domain.JobInfo{{
			Name: "worker", Status: domain.JobStatusStopped, WorkDir: "/wt/x",
		}},
		Addresses: map[string]domain.JobAddress{"worker": {Ports: []int{9000}, URL: "http://worker.wtm"}},
	})

	row := section.Rows[0]
	if row.Up {
		t.Error("Up = true, want down")
	}
	if got := cellOf(row, domain.DetailCellGlyph); got != domain.DetailJobDownGlyph {
		t.Errorf("glyph = %q, want %q", got, domain.DetailJobDownGlyph)
	}
	for _, kind := range []domain.DetailCellKind{domain.DetailCellAddress, domain.DetailCellMeta} {
		if got := cellOf(row, kind); got != "" {
			t.Errorf("%s = %q, want nothing: a stopped job answers nowhere", kind, got)
		}
	}
	if row.URL != "" {
		t.Errorf("URL = %q, want none: it is where it would answer, not where it does", row.URL)
	}
}

// A job of the same name in another worktree is not this one: the daemon
// indexes every repository it knows.
func TestRunSectionIgnoresAnotherWorktreesJobs(t *testing.T) {
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs:      []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/other"}},
	})

	if section.TitleRight != domain.DetailRunNothing {
		t.Fatalf("TitleRight = %q, want nothing counted here", section.TitleRight)
	}
}

func TestRunSectionSaysNothingIsUpRatherThanVanishing(t *testing.T) {
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
	})

	if section.TitleRight != domain.DetailRunNothing {
		t.Errorf("TitleRight = %q, want the section to say what it is worth", section.TitleRight)
	}
	if len(section.Rows) != 1 {
		t.Errorf("rows = %v, want the declared job listed as down", section.Rows)
	}
}

func TestRunSectionIsAbsentWhenTheProjectHasNoRun(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x"}, DetailLoaded: true, Height: 60, Now: time.Now(),
	})

	for _, section := range sections {
		if section.Key == domain.DetailSectionRun {
			t.Fatal("no run configured is a legitimate absence, not a state to announce")
		}
	}
}

func TestRunSectionFoldsWhatItCannotShow(t *testing.T) {
	jobs := make([]domain.JobConfig, 0, 20)
	for index := range 20 {
		jobs = append(jobs, domain.JobConfig{Name: fmt.Sprintf("job%d", index)})
	}

	// A panel too short for twenty jobs: the section folds what does not fit and
	// says how much.
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true, Height: 12, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: jobs},
		Jobs:      indexedStopped(jobs),
	})

	if got := len(section.Rows); got >= len(jobs) {
		t.Fatalf("RUN has %d rows for %d jobs on a 12-row panel, want them folded", got, len(jobs))
	}
	last := cellOf(section.Rows[len(section.Rows)-1], domain.DetailCellNote)
	if last == "" {
		t.Errorf("last row = %q, want it to name what was folded", last)
	}
}

// The other lists keep a fixed cap because what they could show is unbounded.
// The jobs are a closed list the panel exists to show, so RUN spends the rows
// nothing else wants rather than folding an address away under blank ones.
func TestRunSectionTakesTheRowsNothingElseWants(t *testing.T) {
	jobs := make([]domain.JobConfig, 0, domain.DashboardDetailJobs+3)
	for index := range domain.DashboardDetailJobs + 3 {
		jobs = append(jobs, domain.JobConfig{Name: fmt.Sprintf("job%d", index)})
	}

	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true, Height: 80, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: jobs},
		Jobs:      indexedStopped(jobs),
	})

	if got := len(section.Rows); got != len(jobs) {
		t.Errorf("RUN has %d rows for %d jobs on an 80-row panel, want every one of them", got, len(jobs))
	}
	for _, row := range section.Rows {
		if cellOf(row, domain.DetailCellNote) != "" {
			t.Errorf("a « more » row survived on a panel with room to spare: %+v", row)
		}
	}
}

// RUN is the most volatile thing the panel holds, so it leads it — and gives up
// its place last when the height runs short.
func TestRunSectionIsTheLastToFallWhenTheHeightRunsShort(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 8, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
	})
	kept := FitSections(FitSectionsParams{Sections: sections, Height: 6})

	if len(kept) != 1 || kept[0].Key != domain.DetailSectionRun {
		t.Fatalf("kept = %+v, want RUN alone standing", kept)
	}
}

// A job dropped from run.toml while it is still up has no line here, so the
// header must not count it: a count naming something the body does not show
// reads as a section that lost a row.
func TestRunSectionCountsOnlyWhatItCouldList(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs: []domain.JobInfo{
			{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x"},
			{Name: "gone", Status: domain.JobStatusRunning, WorkDir: "/wt/x"},
		},
	})

	if want := fmt.Sprintf(domain.DetailRunUpCountFmt, 1); sections[0].TitleRight != want {
		t.Fatalf("TitleRight = %q, want %q — only the declared job has a row", sections[0].TitleRight, want)
	}
}

// A folded job is still up, and the count says so: the « … N more » line is
// what explains the difference between the header and the rows.
func TestRunSectionCountsWhatItFolded(t *testing.T) {
	jobs := make([]domain.JobConfig, 0, domain.DashboardDetailJobs+2)
	infos := make([]domain.JobInfo, 0, domain.DashboardDetailJobs+2)
	for index := range domain.DashboardDetailJobs + 2 {
		name := fmt.Sprintf("job%d", index)
		jobs = append(jobs, domain.JobConfig{Name: name})
		infos = append(infos, domain.JobInfo{Name: name, Status: domain.JobStatusRunning, WorkDir: "/wt/x"})
	}

	sections := DetailSections(DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 80, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: jobs}, Jobs: infos,
	})

	if want := fmt.Sprintf(domain.DetailRunUpCountFmt, len(jobs)); sections[0].TitleRight != want {
		t.Fatalf("TitleRight = %q, want %q", sections[0].TitleRight, want)
	}
}

// A panel too short to list every job used to fold whichever came last in
// run.toml — including running ones, whose address is the reason the section is
// read at all. What is folded must be what has nothing to say.
func TestRunSectionFoldsStoppedJobsBeforeRunningOnes(t *testing.T) {
	now := time.Now()
	jobs := make([]domain.JobConfig, 0, 30)
	for i := range 30 {
		jobs = append(jobs, domain.JobConfig{Name: fmt.Sprintf("job%02d", i)})
	}

	// Short enough that even the grown section cannot hold them all.
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 12, Now: now,
		RunConfig: domain.RunConfig{Jobs: jobs},
		// The last declared job is the one that runs, so a budget served in
		// declared order would fold exactly it.
		Jobs: []domain.JobInfo{{
			Name: "job29", Status: domain.JobStatusRunning, WorkDir: "/wt/x",
			StartedAt: now.Add(-time.Minute),
		}},
		Addresses: map[string]domain.JobAddress{"job29": {Ports: []int{3000}, URL: "http://job29.wtm"}},
	})

	if len(section.Rows) >= len(jobs) {
		t.Fatalf("the panel fitted all %d jobs at height 24, so nothing was folded: the fixture no longer exercises the rule", len(jobs))
	}
	if got := cellOf(section.Rows[0], domain.DetailCellName); got != "job29" {
		t.Errorf("first row = %q, want the running job kept when the rest is folded", got)
	}
}

// The panel hands out the ports of a worktree whose .env was never settled on
// the names it publishes. Without a word under the rows, that worktree simply
// looks like the one without named URLs.
func TestRunSectionCarriesWhatHasToBeSaidAboutItsAddresses(t *testing.T) {
	note := "main answers on its ports — `wtm env main` switches it"
	sections := DetailSections(DetailSectionsParams{
		Status:      domain.WorktreeStatus{Branch: "main", Path: "/wt/main"},
		RunConfig:   domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}}},
		AddressNote: note,
	})

	for _, section := range sections {
		if section.Key != domain.DetailSectionRun {
			continue
		}
		rows := section.Rows
		last, air := rows[len(rows)-1], rows[len(rows)-2]
		if last.Cells[0].Kind != domain.DetailCellWarn || last.Cells[0].Text != note {
			t.Fatalf("last RUN row = %+v, want the note under the jobs", last)
		}
		// One row draws one line, so the air above it is a row of its own —
		// counting it any other way overflows the panel by a line.
		if air.Cells[0].Kind != domain.DetailCellGap {
			t.Errorf("row above the note = %+v, want the blank line setting it apart", air)
		}
		return
	}
	t.Fatal("no RUN section at all")
}

func TestRunSectionSaysNothingWhenTheAddressesAreSettled(t *testing.T) {
	sections := DetailSections(DetailSectionsParams{
		Status:    domain.WorktreeStatus{Branch: "main", Path: "/wt/main"},
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web", Kind: domain.JobKindService}}},
	})

	for _, section := range sections {
		if section.Key != domain.DetailSectionRun {
			continue
		}
		for _, row := range section.Rows {
			if row.Cells[0].Kind == domain.DetailCellWarn || row.Cells[0].Kind == domain.DetailCellGap {
				t.Errorf("a settled worktree got a note anyway: %+v", row)
			}
		}
	}
}

// A `turbo run dev` is one process holding several apps. Listing the apps
// beside it repeats what the runner already answers for, and it is the reason
// the section became unreadable on a monorepo.
func runnerPanel(t *testing.T, up []string) domain.DetailSection {
	t.Helper()
	now := time.Now()

	running := make(map[string]bool, len(up))
	for _, name := range up {
		running[name] = true
	}
	infos := make([]domain.JobInfo, 0, 3)
	for _, name := range []string{"dev", "web", "api"} {
		info := domain.JobInfo{Name: name, Status: domain.JobStatusStopped, WorkDir: "/wt/x"}
		if running[name] {
			info.Status, info.StartedAt = domain.JobStatusRunning, now.Add(-4*time.Minute)
		}
		infos = append(infos, info)
	}

	return runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: now,
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "dev", Runs: []string{"web", "api"}},
			{Name: "web"},
			{Name: "api"},
		}},
		Jobs: infos,
		Addresses: map[string]domain.JobAddress{
			"dev": {Held: []domain.JobURLEntry{
				{Job: "web", URL: "http://web.wtm"},
				{Job: "api", URL: "http://api.wtm"},
			}},
			"web": {URL: "http://web.wtm"},
			"api": {URL: "http://api.wtm"},
		},
	})
}

func TestRunSectionFoldsTheJobsARunningRunnerHolds(t *testing.T) {
	section := runnerPanel(t, []string{"dev"})

	if len(section.Rows) != 1 {
		t.Fatalf("rows = %d, want the runner alone", len(section.Rows))
	}
	if got := cellOf(section.Rows[0], domain.DetailCellName); got != "dev" {
		t.Errorf("row name = %q, want the runner", got)
	}
	// The addresses the reader came for are the apps'. The runner's line counts
	// them and folds them away; unfolding it is what gives each one its own row.
	if got := cellOf(section.Rows[0], domain.DetailCellAddress); got != "2 addresses" {
		t.Errorf("address = %q, want the count of what the runner holds", got)
	}
	if !section.Rows[0].Fold || !section.Rows[0].Folded {
		t.Errorf("runner row = %+v, want it foldable and folded by default", section.Rows[0])
	}
}

func TestRunSectionKeepsTheChildrenOfARunnerThatIsDown(t *testing.T) {
	section := runnerPanel(t, nil)

	if len(section.Rows) != 3 {
		t.Fatalf("rows = %d, want every declared job: nothing holds them", len(section.Rows))
	}
}

func TestRunSectionKeepsAChildStartedOnItsOwn(t *testing.T) {
	section := runnerPanel(t, []string{"dev", "web"})

	// Both up is a conflict the run flow warns about, but the panel's job is to
	// show what is running — and `web` is a process of its own here.
	if len(section.Rows) != 2 {
		t.Fatalf("rows = %d, want the runner and the child it does not hold", len(section.Rows))
	}
}

// indexedStopped puts every job in the daemon's index, stopped, for the tests
// whose subject is the row budget rather than which jobs are worth a row.
func indexedStopped(jobs []domain.JobConfig) []domain.JobInfo {
	infos := make([]domain.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		infos = append(infos, domain.JobInfo{Name: job.Name, Status: domain.JobStatusStopped, WorkDir: "/wt/x"})
	}
	return infos
}

// What the project can run beyond this worktree is a catalogue, closed under one
// line rather than laid flat: twelve rows reading "down" were what buried the
// three that were up.
func TestRunSectionClosesWithWhatItDidNotShow(t *testing.T) {
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "web"}, {Name: "api"}, {Name: "migrate"}, {Name: "seed"},
		}},
		Jobs: []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x"}},
	})

	if len(section.Rows) != 2 {
		t.Fatalf("rows = %+v, want the running job and the catalogue line", section.Rows)
	}
	want := fmt.Sprintf(domain.DetailDeclaredMoreFmt, 3)
	if got := cellOf(section.Rows[1], domain.DetailCellNote); got != want {
		t.Errorf("last row = %q, want %q", got, want)
	}
}

// Nothing to say beyond the rows: a worktree running everything it declares
// gets no trailing line at all.
func TestRunSectionSaysNothingWhenEveryDeclaredJobIsShown(t *testing.T) {
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: time.Now(),
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}},
		Jobs:      []domain.JobInfo{{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/wt/x"}},
	})

	for _, row := range section.Rows {
		if got := cellOf(row, domain.DetailCellNote); got != "" {
			t.Errorf("a catalogue line appeared with nothing to count: %q", got)
		}
	}
}

// Unfolding a runner gives each app it answers for a row of its own, carrying
// its own url — which the single joined-and-truncated cell they shared never
// did: six urls behind a "," ran past the panel, and the four that were cut
// were unreachable from here at all.
func TestRunSectionGivesAnUnfoldedRunnersChildrenARowEach(t *testing.T) {
	now := time.Now()
	section := runSectionOf(t, DetailSectionsParams{
		Status: domain.WorktreeStatus{Branch: "feat/x", Path: "/wt/x"}, DetailLoaded: true,
		Height: 60, Now: now,
		RunConfig: domain.RunConfig{Jobs: []domain.JobConfig{
			{Name: "dev", Runs: []string{"web", "api"}}, {Name: "web"}, {Name: "api"},
		}},
		Jobs: []domain.JobInfo{{
			Name: "dev", Status: domain.JobStatusRunning, WorkDir: "/wt/x", StartedAt: now.Add(-time.Minute),
		}},
		Addresses: map[string]domain.JobAddress{"dev": {Held: []domain.JobURLEntry{
			{Job: "web", URL: "http://web.wtm"},
			{Job: "api", URL: "http://api.wtm"},
		}}},
		Expanded: map[string]bool{"dev": true},
	})

	if len(section.Rows) != 3 {
		t.Fatalf("rows = %+v, want the runner and one row per address it answers for", section.Rows)
	}
	if !section.Rows[0].Fold || section.Rows[0].Folded {
		t.Errorf("runner row = %+v, want it marked open", section.Rows[0])
	}
	for index, want := range []struct{ name, url string }{
		{"web", "http://web.wtm"},
		{"api", "http://api.wtm"},
	} {
		row := section.Rows[index+1]
		if got := cellOf(row, domain.DetailCellName); got != want.name {
			t.Errorf("child %d name = %q, want %q", index, got, want.name)
		}
		// Its own url, so its own row is a thing a reader can click.
		if row.URL != want.url {
			t.Errorf("child %d URL = %q, want %q", index, row.URL, want.url)
		}
		if row.Depth != 1 {
			t.Errorf("child %d depth = %d, want it hung under its runner", index, row.Depth)
		}
		if row.Key != HeldRowKey("dev", want.name) {
			t.Errorf("child %d key = %q, want it qualified by its runner", index, row.Key)
		}
	}
}
