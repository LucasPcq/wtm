package selfupdate_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

// hermeticEnv isolates a notify test from the machine running it: StartCheck
// reads the environment to decide, and CI exports CI/GITHUB_ACTIONS, which would
// suppress every check and make these tests pass locally but fail on CI.
func hermeticEnv(t *testing.T) {
	t.Helper()

	// GOBIN short-circuits goBinDir, which Notice reaches through DetectInstall:
	// without it every notice shells out to `go env`, and the go command writes a
	// telemetry directory under the fake config home asynchronously — after the
	// subprocess exits, racing t.TempDir's cleanup.
	t.Setenv("GOBIN", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv(domain.EnvCI, "")
	t.Setenv(domain.EnvGitHubActions, "")
	t.Setenv(domain.EnvNoUpdateCheck, "")
}

func TestStartCheckSuppressedReturnsNilAndNoticeIsNilSafe(t *testing.T) {
	hermeticEnv(t)
	t.Setenv(domain.EnvNoUpdateCheck, "1")

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
	})
	if check != nil {
		t.Fatalf("StartCheck = %v, want nil when the opt-out env var is set", check)
	}

	if _, _, _, ok := check.Notice(time.Millisecond); ok {
		t.Fatal("Notice on a nil Check must report ok=false, not panic")
	}
}

func TestNoticeServesTheCachedVersionWithoutNetwork(t *testing.T) {
	hermeticEnv(t)

	// A recent check with a newer version cached: no refresh is due, so the
	// notice must come straight from state with no network call.
	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now(),
		LatestVersion: "9.9.9",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
	})
	if check == nil {
		t.Fatal("StartCheck = nil; a TTY run with a newer cached version must arm a notice")
	}

	current, latest, _, ok := check.Notice(time.Millisecond)
	if !ok {
		t.Fatal("Notice reported nothing; a cached newer version must produce a notice")
	}
	if current != "0.1.0" || latest != "9.9.9" {
		t.Fatalf("Notice = (%q, %q), want (0.1.0, 9.9.9)", current, latest)
	}
}

func TestNoticeSilentWhenCachedVersionIsNotNewer(t *testing.T) {
	hermeticEnv(t)

	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now(),
		LatestVersion: "0.1.0",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
	})
	if check == nil {
		t.Fatal("StartCheck = nil; the display gate must allow a TTY run")
	}

	if _, _, _, ok := check.Notice(time.Millisecond); ok {
		t.Fatal("Notice fired for an identical version")
	}
}

func TestNoticePrefersTheRefreshedVersionOverTheCache(t *testing.T) {
	hermeticEnv(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer srv.Close()

	// Stale enough that a refresh is due, with an older version cached: the
	// notice must report what the refresh found, not what was on disk.
	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now().Add(-domain.UpdateCheckTTL - time.Hour),
		LatestVersion: "0.5.0",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
		BaseURL:     srv.URL,
	})
	if check == nil {
		t.Fatal("StartCheck = nil; a stale cache on a TTY run must trigger a refresh")
	}

	_, latest, _, ok := check.Notice(5 * time.Second)
	if !ok {
		t.Fatal("Notice reported nothing")
	}
	if latest != "9.9.9" {
		t.Fatalf("latest = %q, want 9.9.9 from the refresh (cache held 0.5.0)", latest)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("handler hits = %d, want exactly 1", got)
	}

	// The refresh must also land in the state file, so the next run is free.
	if state := selfupdate.LoadState(); state.LatestVersion != "9.9.9" {
		t.Fatalf("persisted LatestVersion = %q, want 9.9.9", state.LatestVersion)
	}
}

func TestNoticeFallsBackToTheCacheWhenTheRefreshIsTooSlow(t *testing.T) {
	hermeticEnv(t)

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer srv.Close()

	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now().Add(-domain.UpdateCheckTTL - time.Hour),
		LatestVersion: "0.5.0",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
		BaseURL:     srv.URL,
	})
	if check == nil {
		t.Fatal("StartCheck = nil")
	}

	_, latest, _, ok := check.Notice(50 * time.Millisecond)
	if !ok {
		t.Fatal("Notice went silent; a slow refresh must still serve the cached version")
	}
	if latest != "0.5.0" {
		t.Fatalf("latest = %q, want the cached 0.5.0 when the refresh misses the window", latest)
	}

	// Drained before returning, with the temp HOME still installed: the refresh
	// goroutine calls SaveState when it lands, and t.Setenv restores the real
	// environment the moment this function returns — a goroutine still in flight
	// would write the test's fixture into the user's own state file.
	close(release)
	check.Notice(5 * time.Second)
}

func TestStartCheckDoesNotReachTheNetworkInsideTheTTL(t *testing.T) {
	hermeticEnv(t)

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	if err := selfupdate.SaveState(domain.UpdateState{
		CheckedAt:     time.Now(),
		LatestVersion: "9.9.9",
	}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	check := selfupdate.StartCheck(selfupdate.StartCheckParams{
		Version:     "0.1.0",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
		BaseURL:     srv.URL,
	})
	if check == nil {
		t.Fatal("StartCheck = nil; a fresh cache must still serve a notice")
	}

	if _, latest, _, ok := check.Notice(time.Second); !ok || latest != "9.9.9" {
		t.Fatalf("Notice = (%q, %v), want (9.9.9, true) from cache", latest, ok)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("handler hits = %d, want 0 — the TTL must suppress the network call", got)
	}
}
