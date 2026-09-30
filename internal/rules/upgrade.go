package rules

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

// NormalizeVersion strips the leading "v" that release tags carry and asset
// filenames do not.
func NormalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func stripBuildMetadata(v string) string {
	core, _, _ := strings.Cut(v, "+")
	return core
}

type parsedVersion struct {
	fields     [3]int
	prerelease string
}

func parseVersion(v string) (parsedVersion, bool) {
	// Build metadata is dropped before the pre-release split: "1.2.3+meta" would
	// otherwise fail to parse and silence the notifier.
	core, pre, _ := strings.Cut(stripBuildMetadata(NormalizeVersion(v)), "-")
	if core == "" {
		return parsedVersion{}, false
	}

	parts := strings.Split(core, ".")
	if len(parts) > 3 {
		return parsedVersion{}, false
	}

	out := parsedVersion{prerelease: pre}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return parsedVersion{}, false
		}
		out.fields[i] = n
	}

	return out, true
}

type NewerVersionParams struct {
	Current string
	Latest  string
}

// IsNewerVersion reports whether Latest supersedes Current. An unparseable
// version on either side reports false: the notifier stays silent rather than
// guessing.
func IsNewerVersion(params NewerVersionParams) bool {
	cur, ok := parseVersion(params.Current)
	if !ok {
		return false
	}

	next, ok := parseVersion(params.Latest)
	if !ok {
		return false
	}

	for i := range cur.fields {
		if next.fields[i] != cur.fields[i] {
			return next.fields[i] > cur.fields[i]
		}
	}

	if next.prerelease == cur.prerelease {
		return false
	}
	if cur.prerelease != "" && next.prerelease == "" {
		return true
	}
	if cur.prerelease == "" {
		return false
	}

	return next.prerelease > cur.prerelease
}

type ClassifyInstallParams struct {
	ResolvedPath string
	// GoBinDir is a thunk: resolving it costs a `go env` subprocess that only the
	// go-install branch needs, and the branches above it short-circuit first.
	GoBinDir func() string
	Version  string
}

// ClassifyInstall decides how the running binary was installed. Order matters:
// a `make install` build lands in GoBinDir and would otherwise be sent to fetch
// a published release over the user's own build.
func ClassifyInstall(params ClassifyInstallParams) domain.InstallMethod {
	if NormalizeVersion(params.Version) == domain.VersionDev {
		return domain.InstallSource
	}

	// Homebrew installs <prefix>/bin/wtm as a symlink into the Cellar, so the
	// resolved path is what carries the evidence.
	if isBrewCellarPath(params.ResolvedPath) {
		return domain.InstallHomebrew
	}

	if params.GoBinDir != nil {
		if dir := params.GoBinDir(); dir != "" && filepath.Dir(params.ResolvedPath) == filepath.Clean(dir) {
			return domain.InstallGoInstall
		}
	}

	return domain.InstallStandalone
}

const brewCellarSegment = "Cellar"

// isBrewCellarPath matches the full Homebrew layout —
// <prefix>/Cellar/<formula>/<version>/bin/<binary> — rather than the bare
// "Cellar" segment, which a user directory of that name would also carry.
func isBrewCellarPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 5 || parts[len(parts)-2] != "bin" {
		return false
	}

	for i, part := range parts[:len(parts)-3] {
		if part == brewCellarSegment && i < len(parts)-3 {
			return true
		}
	}

	return false
}

type ReleaseAssetNameParams struct {
	Version string
	GOOS    string
	GOARCH  string
}

// ReleaseAssetName builds the goreleaser archive name for a platform. Tags carry
// a leading "v"; asset filenames do not.
func ReleaseAssetName(params ReleaseAssetNameParams) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", domain.AppName, NormalizeVersion(params.Version), params.GOOS, params.GOARCH)
}

// UpgradeCommandFor names the exact command that updates this install, so every
// message can tell the user what to run rather than that an update exists.
func UpgradeCommandFor(method domain.InstallMethod) string {
	switch method {
	case domain.InstallHomebrew:
		return "brew upgrade " + domain.BrewFormula
	case domain.InstallGoInstall:
		return "go install " + domain.ModulePath + "@latest"
	case domain.InstallSource:
		return "git pull && make install"
	default:
		return domain.AppName + " " + domain.CmdUpgrade
	}
}

type ShouldCheckUpdateParams struct {
	Version     string
	Format      string
	Command     string
	StderrIsTTY bool
	CIEnv       bool
	OptOutEnv   bool
	ConfigCheck *bool
	CheckedAt   time.Time
	Now         time.Time
}

// updateCheckExcluded lists commands whose output is consumed by something other
// than a human: shell-init and resolve are eval'd by the shell, where a stray
// byte breaks the caller.
var updateCheckExcluded = map[string]bool{
	domain.CmdShellInit:  true,
	domain.CmdResolve:    true,
	domain.CmdUpgrade:    true,
	domain.CmdDaemon:     true,
	domain.CmdCompletion: true,
	domain.CmdSchema:     true,
	// Cobra's completion machinery: __complete runs on every shell Tab, where a
	// stray line garbles the prompt mid-redraw.
	domain.CmdShellComp:       true,
	domain.CmdShellCompNoDesc: true,
}

// UpdateNoticeAllowed reports whether this run may print an update notice at
// all. It is every suppression axis except the TTL, which gates the network
// call rather than the display: a notice served from cached state is free.
func UpdateNoticeAllowed(params ShouldCheckUpdateParams) bool {
	if NormalizeVersion(params.Version) == domain.VersionDev {
		return false
	}
	if !IsHumanFormat(params.Format) {
		return false
	}
	if !params.StderrIsTTY {
		return false
	}
	if params.CIEnv || params.OptOutEnv {
		return false
	}
	if params.ConfigCheck != nil && !*params.ConfigCheck {
		return false
	}
	return !updateCheckExcluded[params.Command]
}

// ShouldCheckUpdate reports whether this run may reach the network to refresh
// the cached latest version.
func ShouldCheckUpdate(params ShouldCheckUpdateParams) bool {
	if !UpdateNoticeAllowed(params) {
		return false
	}
	if params.CheckedAt.IsZero() {
		return true
	}

	// A CheckedAt in the future yields a negative elapsed and suppresses the
	// check — the intended behavior under clock skew.
	return params.Now.Sub(params.CheckedAt) >= domain.UpdateCheckTTL
}

// DashboardVersionSegments splits the header's version cluster into the part
// that is always shown and the call to action that only exists when an upgrade
// is available. They are returned apart because the surface styles them
// differently — the action has to carry, the version does not — and because the
// header drops the action first when the row runs out of room.
func DashboardVersionSegments(params NewerVersionParams) (version string, action string) {
	version = fmt.Sprintf(domain.DashboardVersionFmt, NormalizeVersion(params.Current))
	if !IsNewerVersion(params) {
		return version, ""
	}

	return version, fmt.Sprintf(domain.DashboardUpgradeFmt, NormalizeVersion(params.Latest))
}
