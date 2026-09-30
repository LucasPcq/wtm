package rules_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"patch bump", "0.26.1", "0.26.2", true},
		{"minor bump", "0.26.1", "0.27.0", true},
		{"major bump", "0.26.1", "1.0.0", true},
		{"equal", "0.26.1", "0.26.1", false},
		{"older", "0.27.0", "0.26.1", false},
		{"leading v on latest", "0.26.1", "v0.27.0", true},
		{"leading v on both", "v0.26.1", "v0.26.1", false},
		{"numeric not lexical", "0.9.0", "0.10.0", true},
		{"prerelease below its release", "1.0.0-rc1", "1.0.0", true},
		{"release above its prerelease", "1.0.0", "1.0.0-rc1", false},
		{"prerelease ordering", "1.0.0-rc1", "1.0.0-rc2", true},
		{"dev never notifies", "dev", "0.27.0", false},
		{"garbage current", "banana", "0.27.0", false},
		{"garbage latest", "0.26.1", "banana", false},
		{"empty latest", "0.26.1", "", false},
		{"short version", "1.0", "1.0.1", true},
		{"build metadata is ignored", "1.2.3+abc", "1.2.4", true},
		{"build metadata does not make it newer", "1.2.3", "1.2.3+abc", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.IsNewerVersion(rules.NewerVersionParams{Current: tc.current, Latest: tc.latest}); got != tc.want {
				t.Fatalf("IsNewerVersion(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"v0.26.1", "0.26.1"},
		{"0.26.1", "0.26.1"},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := rules.NormalizeVersion(tc.in); got != tc.want {
				t.Fatalf("NormalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClassifyInstall(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ClassifyInstallParams
		want   domain.InstallMethod
	}{
		{
			name: "homebrew arm mac",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/opt/homebrew/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     goBin("/Users/x/go/bin"),
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "homebrew intel mac",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/usr/local/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     goBin("/Users/x/go/bin"),
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "linuxbrew",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/home/linuxbrew/.linuxbrew/Cellar/wtm/0.26.1/bin/wtm",
				GoBinDir:     goBin("/home/x/go/bin"),
				Version:      "0.26.1",
			},
			want: domain.InstallHomebrew,
		},
		{
			name: "go install",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/Users/x/go/bin/wtm",
				GoBinDir:     goBin("/Users/x/go/bin"),
				Version:      "0.26.1",
			},
			want: domain.InstallGoInstall,
		},
		{
			name: "standalone in usr local bin",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/usr/local/bin/wtm",
				GoBinDir:     goBin("/Users/x/go/bin"),
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
		{
			name: "user directory literally named Cellar is not brew",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/Users/x/Cellar/wtm",
				GoBinDir:     goBin("/Users/x/go/bin"),
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
		{
			name: "dev build in go bin is source, not go-install",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/Users/x/go/bin/wtm",
				GoBinDir:     goBin("/Users/x/go/bin"),
				Version:      "dev",
			},
			want: domain.InstallSource,
		},
		{
			name: "empty go bin dir does not swallow everything",
			params: rules.ClassifyInstallParams{
				ResolvedPath: "/usr/local/bin/wtm",
				GoBinDir:     goBin(""),
				Version:      "0.26.1",
			},
			want: domain.InstallStandalone,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ClassifyInstall(tc.params); got != tc.want {
				t.Fatalf("ClassifyInstall(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestReleaseAssetName(t *testing.T) {
	cases := []struct {
		name   string
		params rules.ReleaseAssetNameParams
		want   string
	}{
		{"darwin arm64", rules.ReleaseAssetNameParams{Version: "0.26.1", GOOS: "darwin", GOARCH: "arm64"}, "wtm_0.26.1_darwin_arm64.tar.gz"},
		{"linux amd64", rules.ReleaseAssetNameParams{Version: "0.26.1", GOOS: "linux", GOARCH: "amd64"}, "wtm_0.26.1_linux_amd64.tar.gz"},
		{"tag with leading v is normalized", rules.ReleaseAssetNameParams{Version: "v0.26.1", GOOS: "darwin", GOARCH: "amd64"}, "wtm_0.26.1_darwin_amd64.tar.gz"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rules.ReleaseAssetName(tc.params); got != tc.want {
				t.Fatalf("ReleaseAssetName(%+v) = %q, want %q", tc.params, got, tc.want)
			}
		})
	}
}

func TestUpgradeCommandFor(t *testing.T) {
	cases := []struct {
		method domain.InstallMethod
		want   string
	}{
		{domain.InstallHomebrew, "brew upgrade LucasPcq/tap/wtm"},
		{domain.InstallGoInstall, "go install github.com/LucasPcq/wtm@latest"},
		{domain.InstallStandalone, "wtm upgrade"},
		{domain.InstallSource, "git pull && make install"},
	}

	for _, tc := range cases {
		t.Run(string(tc.method), func(t *testing.T) {
			if got := rules.UpgradeCommandFor(tc.method); got != tc.want {
				t.Fatalf("UpgradeCommandFor(%q) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}

func baseCheckParams() rules.ShouldCheckUpdateParams {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	return rules.ShouldCheckUpdateParams{
		Version:     "0.26.1",
		Format:      domain.OutputText,
		Command:     domain.CmdList,
		StderrIsTTY: true,
		CheckedAt:   now.Add(-48 * time.Hour),
		Now:         now,
	}
}

func TestShouldCheckUpdate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*rules.ShouldCheckUpdateParams)
		want   bool
	}{
		{"nominal interactive run", func(p *rules.ShouldCheckUpdateParams) {}, true},
		{"dev build never checks", func(p *rules.ShouldCheckUpdateParams) { p.Version = "dev" }, false},
		{"json output", func(p *rules.ShouldCheckUpdateParams) { p.Format = domain.OutputJSON }, false},
		{"no tty", func(p *rules.ShouldCheckUpdateParams) { p.StderrIsTTY = false }, false},
		{"ci", func(p *rules.ShouldCheckUpdateParams) { p.CIEnv = true }, false},
		{"env opt out", func(p *rules.ShouldCheckUpdateParams) { p.OptOutEnv = true }, false},
		{"config opt out", func(p *rules.ShouldCheckUpdateParams) { p.ConfigCheck = boolPtr(false) }, false},
		{"config opt in is not an override of ci", func(p *rules.ShouldCheckUpdateParams) {
			p.ConfigCheck = boolPtr(true)
			p.CIEnv = true
		}, false},
		{"shell-init excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdShellInit }, false},
		{"resolve excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdResolve }, false},
		{"upgrade excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdUpgrade }, false},
		{"daemon excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdDaemon }, false},
		{"completion excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdCompletion }, false},
		{"schema excluded", func(p *rules.ShouldCheckUpdateParams) { p.Command = domain.CmdSchema }, false},
		{"inside ttl", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = p.Now.Add(-1 * time.Hour) }, false},
		{"exactly at ttl", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = p.Now.Add(-domain.UpdateCheckTTL) }, true},
		{"never checked", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = time.Time{} }, true},
		{"clock skew: checked in the future", func(p *rules.ShouldCheckUpdateParams) { p.CheckedAt = p.Now.Add(2 * time.Hour) }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := baseCheckParams()
			tc.mutate(&params)
			if got := rules.ShouldCheckUpdate(params); got != tc.want {
				t.Fatalf("ShouldCheckUpdate(%+v) = %v, want %v", params, got, tc.want)
			}
		})
	}
}

func goBin(dir string) func() string { return func() string { return dir } }

func TestDashboardVersionSegments(t *testing.T) {
	cases := []struct {
		name        string
		params      rules.NewerVersionParams
		wantVersion string
		wantAction  string
	}{
		{
			name:        "upgrade available",
			params:      rules.NewerVersionParams{Current: "0.26.0", Latest: "0.26.1"},
			wantVersion: "v0.26.0",
			wantAction:  "→ 0.26.1 · run wtm upgrade",
		},
		{
			name:        "up to date shows the version alone",
			params:      rules.NewerVersionParams{Current: "0.26.1", Latest: "0.26.1"},
			wantVersion: "v0.26.1",
			wantAction:  "",
		},
		{
			name:        "nothing known yet",
			params:      rules.NewerVersionParams{Current: "0.26.1"},
			wantVersion: "v0.26.1",
			wantAction:  "",
		},
		{
			name:        "source build never calls to action",
			params:      rules.NewerVersionParams{Current: domain.Version, Latest: "0.26.1"},
			wantVersion: "vdev",
			wantAction:  "",
		},
		{
			name:        "tag form is normalized on both sides",
			params:      rules.NewerVersionParams{Current: "v0.26.0", Latest: "v0.26.1"},
			wantVersion: "v0.26.0",
			wantAction:  "→ 0.26.1 · run wtm upgrade",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version, action := rules.DashboardVersionSegments(tc.params)
			if version != tc.wantVersion {
				t.Fatalf("version = %q, want %q", version, tc.wantVersion)
			}
			if action != tc.wantAction {
				t.Fatalf("action = %q, want %q", action, tc.wantAction)
			}
		})
	}
}
