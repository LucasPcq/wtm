package render

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestAddressingJSONNamesEachWorktreeByBranchAndPath(t *testing.T) {
	var buf bytes.Buffer
	err := WriteAddressingResultJSON(&buf, AddressingResult{
		Addressing: domain.AddressingNames,
		Settled:    []domain.WorktreeRef{{Branch: "feature", Path: "/wt/feature"}},
		MainLeft:   &domain.WorktreeRef{Branch: "main", Path: "/wt/main"},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	var document struct {
		Settled  []map[string]string `json:"settled"`
		Pending  []map[string]string `json:"pending"`
		MainLeft map[string]string   `json:"main_left"`
	}
	if err := json.Unmarshal(buf.Bytes(), &document); err != nil {
		t.Fatalf("decode %s: %v", buf.String(), err)
	}
	if len(document.Settled) != 1 || document.Settled[0]["branch"] != "feature" || document.Settled[0]["path"] != "/wt/feature" {
		t.Errorf("settled = %v", document.Settled)
	}
	if document.Pending == nil {
		t.Error("pending is absent, want an empty array")
	}
	if document.MainLeft["branch"] != "main" || document.MainLeft["path"] != "/wt/main" {
		t.Errorf("main_left = %v", document.MainLeft)
	}
}
