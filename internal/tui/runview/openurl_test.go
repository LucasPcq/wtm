package runview

import (
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func TestOpenKeyDoesNothingWithoutAURL(t *testing.T) {
	opened := ""
	model := New(Params{Open: func(url string) error {
		opened = url
		return nil
	}})
	model.selected = "web"

	if cmd := model.openSelectedURL(); cmd != nil {
		t.Fatalf("a job that publishes no URL must produce no command")
	}
	if opened != "" {
		t.Errorf("nothing must have been opened, got %q", opened)
	}
}

func TestOpenKeyOpensTheSelectedJobURL(t *testing.T) {
	opened := ""
	model := New(Params{Open: func(url string) error {
		opened = url
		return nil
	}})
	model.selected = "web"
	model.sequence.remember(runlogs.Event{Job: "web", URL: "http://localhost:3010"})

	cmd := model.openSelectedURL()
	if cmd == nil {
		t.Fatal("a published job must produce a command")
	}
	cmd()

	if opened != "http://localhost:3010" {
		t.Errorf("opened %q, want http://localhost:3010", opened)
	}
}

// A machine with no xdg-open must say so: a key that silently does nothing is
// indistinguishable from a frozen terminal.
func TestOpenKeyReportsAnOpenerThatFailed(t *testing.T) {
	model := New(Params{Open: func(string) error { return errors.New("no xdg-open") }})
	model.selected = "web"
	model.sequence.remember(runlogs.Event{Job: "web", URL: "http://localhost:3010"})

	msg := model.openSelectedURL()()
	failed, ok := msg.(openFailedMsg)
	if !ok {
		t.Fatalf("msg = %T, want openFailedMsg", msg)
	}

	next, _ := model.Update(failed)
	shown, ok := next.(Model)
	if !ok {
		t.Fatal("Update must return the model")
	}
	if !strings.Contains(shown.notice, "no xdg-open") {
		t.Errorf("notice = %q, want the failure named", shown.notice)
	}
}
