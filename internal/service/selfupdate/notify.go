package selfupdate

import (
	"context"
	"os"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type StartCheckParams struct {
	Version     string
	Format      string
	Command     string
	StderrIsTTY bool
	// BaseURL overrides the releases endpoint, for tests.
	BaseURL string
	// ConfigCheck is a thunk: reading the global config costs a stat and a strict
	// TOML decode, and must not be paid by commands the cheap axes already
	// exclude — `resolve` sits in the shell's cd path.
	ConfigCheck func() *bool
}

type Check struct {
	done    chan struct{}
	version string
	// cached is fixed at construction; fresh is written by the refresh goroutine
	// and may only be read once done is closed.
	cached string
	fresh  string
}

// StartCheck arms the passive update notice. The notice is served from cached
// state, so it costs nothing and appears even when the refresh below is slow or
// fails; the network call only refreshes that cache, at most once per TTL.
// Returns nil when policy forbids a notice on this run.
func StartCheck(params StartCheckParams) *Check {
	gate := rules.ShouldCheckUpdateParams{
		Version:     params.Version,
		Format:      params.Format,
		Command:     params.Command,
		StderrIsTTY: params.StderrIsTTY,
		CIEnv:       os.Getenv(domain.EnvCI) != "" || os.Getenv(domain.EnvGitHubActions) != "",
		OptOutEnv:   os.Getenv(domain.EnvNoUpdateCheck) != "",
		Now:         time.Now(),
	}

	// Evaluated in two passes so an excluded command pays neither the state file
	// nor the config decode: a nil ConfigCheck reads as "unset", so this first
	// pass is every axis except the config one.
	if !rules.UpdateNoticeAllowed(gate) {
		return nil
	}

	if params.ConfigCheck != nil {
		gate.ConfigCheck = params.ConfigCheck()
		if !rules.UpdateNoticeAllowed(gate) {
			return nil
		}
	}

	state := LoadState()
	gate.CheckedAt = state.CheckedAt

	check := &Check{done: make(chan struct{}), version: params.Version}
	if rules.IsNewerVersion(rules.NewerVersionParams{Current: params.Version, Latest: state.LatestVersion}) {
		check.cached = state.LatestVersion
	}

	if !rules.ShouldCheckUpdate(gate) {
		close(check.done)
		return check
	}

	// The attempt is recorded before it runs: the goroutine dies with the process
	// if the request outlives the drain window, and a timestamp written only on
	// success would let every command re-issue a network call. If it cannot be
	// persisted at all (no writable config dir), skip the fetch entirely rather
	// than hit the API on every single invocation.
	if err := SaveState(domain.UpdateState{CheckedAt: time.Now(), LatestVersion: state.LatestVersion}); err != nil {
		close(check.done)
		return check
	}

	go func() {
		defer close(check.done)

		release, err := FetchRelease(FetchReleaseParams{
			BaseURL:   params.BaseURL,
			UserAgent: domain.AppName + "/" + rules.NormalizeVersion(params.Version),
			Timeout:   domain.UpdateCheckTimeout,
		})
		if err != nil {
			return
		}

		_ = SaveState(domain.UpdateState{CheckedAt: time.Now(), LatestVersion: release.Version})

		if rules.IsNewerVersion(rules.NewerVersionParams{Current: params.Version, Latest: release.Version}) {
			check.fresh = release.Version
		}
	}()

	return check
}

// Notice waits at most timeout for a refresh to land, then reports the newest
// version known — fresh or cached. The install method is resolved here rather
// than in the goroutine: it shells out to `go env`, which would blow the drain
// window and cost the notice.
func (c *Check) Notice(ctx context.Context, timeout time.Duration) (current string, latest string, method domain.InstallMethod, ok bool) {
	if c == nil {
		return "", "", "", false
	}

	latest = c.cached

	select {
	case <-c.done:
		if c.fresh != "" {
			latest = c.fresh
		}
	case <-time.After(timeout):
	}

	if latest == "" {
		return "", "", "", false
	}

	return rules.NormalizeVersion(c.version), latest, DetectInstall(ctx, c.version).Method, true
}
