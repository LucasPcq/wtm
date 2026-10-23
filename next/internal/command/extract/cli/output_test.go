package cli

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
)

func TestAnOutcomeWithoutItsTargetDrawsNothing(t *testing.T) {
	var out bytes.Buffer

	drawOutcome(&out, extract.Outcome{Result: &extract.Result{}}, output.View{})

	assert.Empty(t, out.String())
}

func TestATargetLeftIncompleteStandsAndSaysWhy(t *testing.T) {
	outcome := unfinished(extract.Item{
		Subject:  "feat/bad",
		Status:   domain.StatusFailed,
		Reason:   domain.ReasonIncomplete,
		Err:      &domain.Error{Kind: domain.KindInternal, Code: "unit.phase_failed", Message: "on_create failed: hook failed: exit status 3"},
		Warnings: []domain.Warning{untouched("nothing was moved: the files are still in feat/login")},
		Detail:   &extract.Detail{Path: "/repo.trees/feat-bad"},
	})

	formtest.Golden(t, "failed", drawn(outcome))
}

func TestTheWarningsOfAnUnfinishedTargetFollowItsLine(t *testing.T) {
	ports := []env.Entry{
		{File: ".env", Key: "DB", Port: "pg", Status: env.StatusMissingKey},
		{File: "api/.env", Key: "API", Port: "http", Base: 3000, Status: env.StatusNotFound},
	}
	outcome := unfinished(extract.Item{
		Subject: "feat/x", Status: domain.StatusFailed, Reason: domain.ReasonIncomplete,
		Err:      &domain.Error{Kind: domain.KindInternal, Code: "unit.phase_failed", Message: "on_create failed: hook failed: exit status 3"},
		Detail:   &extract.Detail{Path: "/repo.trees/feat-x", EnvPorts: env.Redacted{Entries: ports}},
		Warnings: []domain.Warning{{Code: "provision.env_no_template", Message: "no template found for .env, skipping"}, {Code: "provision.env_ports_left_alone", Message: "Env ports left alone", Ports: ports}, untouched("nothing was moved: the files are still in feat/login")},
	})
	outcome.Warnings = []domain.Warning{{Code: "extract.ff_dirty", Message: "couldn't fast-forward main: its worktree has uncommitted changes — the target starts from your local branch"}}

	formtest.Golden(t, "unfinished_warnings", drawn(outcome))
}

func TestATargetWhoseMoveFailedStillStands(t *testing.T) {
	outcome := unfinished(extract.Item{
		Subject: "feat/clash",
		Status:  domain.StatusFailed,
		Err:     &domain.Error{Kind: domain.KindConflict, Code: extract.CodeConflict, Message: "cannot apply the selected changes"},
		Detail:  &extract.Detail{Path: "/repo.trees/feat-clash"},
	})

	text := drawn(outcome)

	assert.Contains(t, text, "feat/clash  ../repo.trees/feat-clash\n")
	assert.Contains(t, text, "→ wtm go feat/clash")
}

func TestATargetThatNeverStoodHasNoRowNorGo(t *testing.T) {
	outcome := unfinished(extract.Item{Subject: "feat/slow", Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted, Warnings: []domain.Warning{untouched("nothing was moved: the files are still in feat/login")}})

	formtest.Golden(t, "interrupted", drawn(outcome))
}
