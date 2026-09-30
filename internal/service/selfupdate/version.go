package selfupdate

import (
	"runtime/debug"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// devBuildInfoVersion is what the toolchain records for a binary built from a
// local source tree rather than fetched by version.
const devBuildInfoVersion = "(devel)"

// ResolveVersion recovers the real version of the running binary. Releases carry
// it through goreleaser's ldflag, but `go install <module>@<version>` never runs
// goreleaser — those binaries report "dev" while the module version sits in the
// build info. Without this fallback every go-install user is classified as a
// source build: refused by `wtm upgrade` and never notified.
func ResolveVersion(linked string) string {
	if rules.NormalizeVersion(linked) != domain.VersionDev {
		return linked
	}

	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == devBuildInfoVersion {
		return domain.VersionDev
	}

	return info.Main.Version
}

// CachedUpgrade reports the newer version already known from the last passive
// check, or "" when there is nothing to announce. It reads only the state file:
// a surface calling this must not pay a network round-trip to render.
func CachedUpgrade(version string) string {
	latest := LoadState().LatestVersion
	if !rules.IsNewerVersion(rules.NewerVersionParams{Current: version, Latest: latest}) {
		return ""
	}

	return rules.NormalizeVersion(latest)
}
