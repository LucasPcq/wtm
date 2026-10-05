package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

func ensureGH() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return domain.ErrGHNotInstalled
	}
	return nil
}

func ensureAuth(ctx context.Context) error {
	if err := ensureGH(); err != nil {
		return err
	}
	cmd := infra.Command(ctx, "gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		return domain.ErrGHNotAuthenticated
	}
	return nil
}

func runGH(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := infra.Command(ctx, "gh", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %v: %w\n%s", args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

type ghPR struct {
	Number            int          `json:"number"`
	Title             string       `json:"title"`
	Author            ghAuthor     `json:"author"`
	HeadRefName       string       `json:"headRefName"`
	BaseRefName       string       `json:"baseRefName"`
	URL               string       `json:"url"`
	IsCrossRepository bool         `json:"isCrossRepository"`
	IsDraft           bool         `json:"isDraft"`
	ReviewDecision    string       `json:"reviewDecision"`
	StatusCheckRollup []ghCheckRun `json:"statusCheckRollup"`
}

type ghAuthor struct {
	Login string `json:"login"`
}

// ghCheckRun is one entry of a PR's statusCheckRollup, which mixes two
// shapes: a modern Checks API CheckRun — a finished run carries Conclusion, a
// still-running one leaves it empty and carries Status instead — and a
// legacy Status API StatusContext (CircleCI, Travis, and other integrations
// that predate the Checks API), which never sets Conclusion at all and
// reports through State instead.
type ghCheckRun struct {
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
	State      string `json:"state"`
}

func convertGHPR(g ghPR) domain.PRInfo {
	return domain.PRInfo{
		Number:         g.Number,
		Title:          g.Title,
		Author:         g.Author.Login,
		Branch:         g.HeadRefName,
		BaseBranch:     g.BaseRefName,
		State:          domain.PRStateOpen,
		URL:            g.URL,
		IsFork:         g.IsCrossRepository,
		Draft:          g.IsDraft,
		Checks:         aggregateChecks(g.StatusCheckRollup),
		ReviewDecision: g.ReviewDecision,
	}
}

// checkOutcome is the three-bucket classification a rollup entry resolves
// to, whichever of the two shapes (§ghCheckRun) it arrived as.
type checkOutcome int

const (
	checkPending checkOutcome = iota
	checkPassed
	checkFailed
)

// aggregateChecks buckets a status-check rollup into passed/failed/pending.
func aggregateChecks(rollup []ghCheckRun) domain.PRChecks {
	var checks domain.PRChecks
	for _, run := range rollup {
		switch runOutcome(run) {
		case checkPassed:
			checks.Passed++
		case checkFailed:
			checks.Failed++
		default:
			checks.Pending++
		}
	}
	return checks
}

// runOutcome reads a rollup entry's real outcome: Conclusion, when the
// CheckRun has finished; State, for a legacy StatusContext entry, which
// never carries a Conclusion at all; neither set means still queued or
// in progress.
func runOutcome(run ghCheckRun) checkOutcome {
	if run.Conclusion != "" {
		return conclusionOutcome(run.Conclusion)
	}
	if run.State != "" {
		return stateOutcome(run.State)
	}
	return checkPending
}

// conclusionOutcome enumerates every known `conclusion` value explicitly,
// each routed by name rather than falling into a default. SKIPPED and
// NEUTRAL count as passed (domain.GHCheckConclusionSkipped documents the
// disclosed conflation). ACTION_REQUIRED means the workflow needs
// authorization to run — it has not run and broken, so it is pending, not
// failed. An unrecognised conclusion fails closed: it must read as neither a
// success nor still-running, never silently pass.
func conclusionOutcome(conclusion string) checkOutcome {
	switch conclusion {
	case domain.GHCheckConclusionSuccess, domain.GHCheckConclusionNeutral, domain.GHCheckConclusionSkipped:
		return checkPassed
	case domain.GHCheckConclusionActionRequired:
		return checkPending
	case domain.GHCheckConclusionFailure, domain.GHCheckConclusionCancelled, domain.GHCheckConclusionTimedOut:
		return checkFailed
	default:
		return checkFailed
	}
}

// stateOutcome maps the legacy Status API's `state` values. An unrecognised
// state fails closed, for the same reason an unrecognised conclusion does.
func stateOutcome(state string) checkOutcome {
	switch state {
	case domain.GHCheckStateSuccess:
		return checkPassed
	case domain.GHCheckStatePending:
		return checkPending
	case domain.GHCheckStateError, domain.GHCheckStateFailure:
		return checkFailed
	default:
		return checkFailed
	}
}

func parseJSON[T any](data []byte) (T, error) {
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return v, fmt.Errorf("parse gh JSON: %w", err)
	}
	return v, nil
}
