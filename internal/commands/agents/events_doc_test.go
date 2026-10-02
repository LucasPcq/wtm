package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A type added to the bus without a word in the guide or the skill is a type
// no integration will know to read: both are checked, one type at a time.
func TestEveryEventTypeIsDocumented(t *testing.T) {
	skill, ok := skillFiles()[skillReferencesDir+"/events.md"]
	if !ok {
		t.Fatal("the skill ships no references/events.md")
	}
	guide, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "guide", "events.md"))
	if err != nil {
		t.Fatalf("docs/guide/events.md: %v", err)
	}
	for _, typ := range domain.EventTypes {
		quoted := "`" + string(typ) + "`"
		if !strings.Contains(skill, quoted) {
			t.Errorf("references/events.md does not document %s", quoted)
		}
		if !strings.Contains(string(guide), quoted) {
			t.Errorf("docs/guide/events.md does not document %s", quoted)
		}
	}
}
