package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

var planFiles = []domain.ProxyPlannedFile{
	{
		Path:    "/Users/dev/Library/LaunchAgents/dev.wtm.proxy.plist",
		Content: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<plist version=\"1.0\"><dict></dict></plist>\n",
		Change:  "new file — launchd binds :80 and hands it to wtm",
	},
}

func TestProxyPlanReportShowsChangesNotContents(t *testing.T) {
	var buf bytes.Buffer

	ProxyPlanReport(&buf, ProxyPlanReportParams{Files: planFiles, Script: "launchctl load /Users/dev/Library/LaunchAgents/dev.wtm.proxy.plist", Reversible: true})

	out := buf.String()
	if strings.Contains(out, "<?xml") {
		t.Errorf("le récap par défaut ne déverse pas les contenus:\n%s", out)
	}
	for _, want := range []string{"dev.wtm.proxy.plist", "launchd binds :80", "launchctl load", "uninstall", "--dry-run"} {
		if !strings.Contains(out, want) {
			t.Errorf("le récap doit contenir %q:\n%s", want, out)
		}
	}
}

func TestProxyPlanReportFullShowsContents(t *testing.T) {
	var buf bytes.Buffer

	ProxyPlanReport(&buf, ProxyPlanReportParams{Files: planFiles, Full: true, Reversible: true})

	out := buf.String()
	if !strings.Contains(out, "<?xml") {
		t.Errorf("--dry-run montre les contenus:\n%s", out)
	}
	if strings.Contains(out, "--dry-run") {
		t.Error("inutile de renvoyer vers --dry-run quand on y est déjà")
	}
}
