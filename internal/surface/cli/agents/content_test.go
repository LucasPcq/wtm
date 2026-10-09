package agents

import (
	"strings"
	"testing"
)

func TestTheSkillShipsItsEntryPointAndReferences(t *testing.T) {
	files := skillFiles()
	entry, ok := files[skillEntryFile]
	if !ok {
		t.Fatalf("no %s in %v", skillEntryFile, files)
	}
	if !strings.HasPrefix(entry, "---\nname: using-wtm\n") {
		t.Errorf("missing or wrong frontmatter:\n%s", entry[:120])
	}
	if !strings.Contains(entry, "--output json") {
		t.Error("SKILL.md should mention --output json")
	}
	references := 0
	for name := range files {
		if strings.HasPrefix(name, skillReferencesDir+"/") {
			references++
			if !strings.Contains(entry, name) {
				t.Errorf("SKILL.md never points at %s", name)
			}
		}
	}
	if references == 0 {
		t.Error("the skill ships no reference file")
	}
}
