package dashboard

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

func TestConfirmOptionsNameBothOutcomesWhenLabelled(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{YesLabel: "Push to origin", NoLabel: "Keep local"})

	if len(options) != 3 {
		t.Fatalf("a labelled confirm must offer both outcomes, got %d options", len(options))
	}
	if options[0].Label != "Keep local" || options[2].Label != "Push to origin" {
		t.Fatalf("the harmless outcome must lead, got %+v", options)
	}
}

func TestConfirmOptionsStayPlainWithoutLabels(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{})

	if len(options) != 1 || options[0].Value != confirmYes {
		t.Fatalf("an unlabelled confirm must keep its single option, got %+v", options)
	}
}

func TestConfirmOptionsLeadsWithYesWhenItIsTheDefault(t *testing.T) {
	options := confirmOptions(flow.ConfirmParams{YesLabel: "Push to origin", NoLabel: "Keep local", DefaultYes: true})

	if options[0].Label != "Push to origin" || options[2].Label != "Keep local" {
		t.Fatalf("the default outcome must lead, got %+v", options)
	}
}

func openPushConfirm(t *testing.T) (Model, chan promptReply) {
	t.Helper()
	session := confirmSession(flow.ConfirmParams{
		Title:    "Push 2 rebased branch(es) to origin?",
		Warning:  domain.SyncPushWarning,
		YesLabel: domain.SyncPushOption,
		NoLabel:  domain.SyncKeepLocalOption,
	})

	model := newTestModel(t, testWidth, testHeight, "feat")
	reply := make(chan promptReply, 1)
	model = prompt(t, model, promptMsg{
		title:   "Push 2 rebased branch(es) to origin?",
		shape:   modalForm,
		session: session,
		reply:   reply,
	})
	if len(model.modal.rows) == 0 {
		t.Fatal("the confirmation must be built before it is shown")
	}
	return model, reply
}

// The modal is where the push question is actually answered, so it is the
// rendering that has to keep both outcomes. Asserting on confirmOptions alone is
// what let a form drawing a single button ship.
func TestBothPushOutcomesAreRenderedAsTheirOwnButton(t *testing.T) {
	model, _ := openPushConfirm(t)

	buttons := confirmButtonRows(model)
	if len(buttons) != 2 {
		t.Fatalf("%d confirm button(s), want one per outcome the question names", len(buttons))
	}
	if buttons[0].label != domain.SyncKeepLocalOption || buttons[1].label != domain.SyncPushOption {
		t.Fatalf("the harmless outcome must lead and the other must be reachable, got %+v", buttons)
	}

	body := strings.Join(model.modal.body(m0zones{}), "\n")
	if !strings.Contains(body, domain.SyncPushOption) {
		t.Errorf("the push outcome never reaches the screen:\n%s", body)
	}
}

func TestPressingAnOutcomeAnswersWithThatOutcome(t *testing.T) {
	model, reply := openPushConfirm(t)

	if got := model.modal.answers.Value(keyConfirm); got != confirmNo {
		t.Fatalf("the form opens answered %q, want the harmless outcome it leads with", got)
	}

	pressRow(t, model, rowIndex(t, model, func(row formRow) bool {
		return row.kind == formButton && row.label == domain.SyncPushOption
	}))

	answered := waitReply(t, reply)
	if answered.err != nil {
		t.Fatalf("confirmed with %v", answered.err)
	}
	if got := answered.answers.Value(keyConfirm); got != confirmYes {
		t.Fatalf("pressing %q answered %q, want the outcome that was pressed",
			domain.SyncPushOption, got)
	}
}

func TestPressingTheLeadingOutcomeKeepsTheBranchesLocal(t *testing.T) {
	model, reply := openPushConfirm(t)

	pressRow(t, model, rowIndex(t, model, func(row formRow) bool {
		return row.kind == formButton && row.label == domain.SyncKeepLocalOption
	}))

	answered := waitReply(t, reply)
	if got := answered.answers.Value(keyConfirm); got != confirmNo {
		t.Fatalf("keeping local answered %q, want the push refused", got)
	}
}
