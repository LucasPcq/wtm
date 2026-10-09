package agents

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func skillFile(t *testing.T, name string) string {
	t.Helper()
	content, ok := skillFiles()[name]
	if !ok {
		t.Fatalf("the skill ships no %s", name)
	}
	return content
}

func guideFile(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "guide", name))
	if err != nil {
		t.Fatalf("docs/guide/%s: %v", name, err)
	}
	return string(content)
}

// A type added to the bus without a word in the guide or the skill is a type
// no integration will know to read: both are checked, one type at a time.
func TestEveryEventTypeIsDocumented(t *testing.T) {
	skill := skillFile(t, skillReferencesDir+"/events.md")
	guide := guideFile(t, "events.md")
	for _, typ := range domain.EventTypes {
		quoted := "`" + string(typ) + "`"
		if !strings.Contains(skill, quoted) {
			t.Errorf("references/events.md does not document %s", quoted)
		}
		if !strings.Contains(guide, quoted) {
			t.Errorf("docs/guide/events.md does not document %s", quoted)
		}
	}
}

// A host that cannot find the probe keeps failing on an old wtm instead of
// asking for an upgrade.
func TestTheVersionProbeIsDocumented(t *testing.T) {
	probe := "wtm version --output json"
	for name, content := range map[string]string{
		"references/events.md": skillFile(t, skillReferencesDir+"/events.md"),
		"docs/guide/events.md": guideFile(t, "events.md"),
	} {
		if !strings.Contains(content, probe) {
			t.Errorf("%s does not document %s", name, probe)
		}
	}
}

// A consumer decides whether to retry from the exit code alone: a final code
// missing from one of these pages is one an integration will retry forever.
func TestEveryFinalEventsExitCodeIsDocumented(t *testing.T) {
	pages := map[string]string{
		"references/events.md":  skillFile(t, skillReferencesDir+"/events.md"),
		"SKILL.md":              skillFile(t, skillEntryFile),
		"docs/guide/events.md":  guideFile(t, "events.md"),
		"docs/guide/recipes.md": guideFile(t, "recipes.md"),
	}
	for _, code := range domain.EventsFinalExitCodes {
		quoted := "`" + strconv.Itoa(code) + "`"
		for name, content := range pages {
			if !strings.Contains(content, quoted) {
				t.Errorf("%s does not document exit code %s", name, quoted)
			}
		}
	}
}
