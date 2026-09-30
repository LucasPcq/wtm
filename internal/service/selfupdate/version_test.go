package selfupdate_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/selfupdate"
)

func TestResolveVersionKeepsTheLinkedVersion(t *testing.T) {
	if got := selfupdate.ResolveVersion("0.26.1"); got != "0.26.1" {
		t.Fatalf("ResolveVersion(0.26.1) = %q, want 0.26.1 (the ldflag wins)", got)
	}
}

// The test binary is built from this source tree, so the build info reports
// "(devel)" and the fallback must decline it rather than hand back a fake
// version. The go-install case — a real module version in the build info — is
// covered end to end in the PR checklist, not here: it cannot be produced from
// inside `go test`.
func TestResolveVersionDeclinesADevelBuild(t *testing.T) {
	if got := selfupdate.ResolveVersion(domain.VersionDev); got != domain.VersionDev {
		t.Fatalf("ResolveVersion(dev) = %q, want %q for a source build", got, domain.Version)
	}
}

func TestCachedUpgrade(t *testing.T) {
	cases := []struct {
		name    string
		cached  string
		current string
		want    string
	}{
		{"newer release cached", "0.27.0", "0.26.1", "0.27.0"},
		{"tag form is normalized", "v0.27.0", "0.26.1", "0.27.0"},
		{"same version", "0.26.1", "0.26.1", ""},
		{"older cached", "0.25.0", "0.26.1", ""},
		{"nothing cached", "", "0.26.1", ""},
		{"source build is never behind", "0.27.0", domain.Version, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("HOME", t.TempDir())

			if tc.cached != "" {
				if err := selfupdate.SaveState(domain.UpdateState{LatestVersion: tc.cached}); err != nil {
					t.Fatalf("SaveState: %v", err)
				}
			}

			if got := selfupdate.CachedUpgrade(tc.current); got != tc.want {
				t.Fatalf("CachedUpgrade(%q) with %q cached = %q, want %q", tc.current, tc.cached, got, tc.want)
			}
		})
	}
}
