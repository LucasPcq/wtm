package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func twoPorts() []domain.PortEntry {
	return []domain.PortEntry{
		{Job: "docker-compose", Name: "POSTGRES_PORT", Base: 5432},
		{Job: "web-dev", Name: "WEB_PORT", Base: 5173},
	}
}

func newPortList() PortListModel {
	return NewPortList(NewPortListParams{Title: "Ports", Entries: twoPorts()})
}

func portKey(m PortListModel, key string) PortListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return m
}

func portEnter(m PortListModel) PortListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return m
}

func portDown(m PortListModel) PortListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	return m
}

func TestPortListStartsFromTheDetection(t *testing.T) {
	m := newPortList()
	got := m.Entries()
	if len(got) != 2 {
		t.Fatalf("expected the 2 detected ports, got %d", len(got))
	}
	if got[0].Base != 5432 {
		t.Errorf("base = %d, want the detected 5432", got[0].Base)
	}
}

func TestPortListEditsABase(t *testing.T) {
	// Enter sur une ligne l'édite — la même convention que HookListModel.
	m := portEnter(newPortList())
	m = portKey(m, "5555")
	m = portEnter(m)

	if got := m.Entries()[0].Base; got != 5555 {
		t.Errorf("base = %d, want 5555", got)
	}
}

func TestPortListRefusesANonPort(t *testing.T) {
	// Une saisie invalide ne doit pas écraser une valeur détectée qui, elle,
	// marche : mieux vaut refuser que déclarer un port impossible.
	m := portEnter(newPortList())
	m = portKey(m, "99999")
	m = portEnter(m)

	if got := m.Entries()[0].Base; got != 5432 {
		t.Errorf("base = %d, want the detected 5432 kept", got)
	}
	if m.Done() {
		t.Error("une saisie refusée ne valide pas l'étape")
	}
}

func TestPortListEditsTheRowUnderTheCursor(t *testing.T) {
	m := portDown(newPortList())
	m = portEnter(m)
	m = portKey(m, "5599")
	m = portEnter(m)

	got := m.Entries()
	if got[0].Base != 5432 {
		t.Errorf("la première ligne ne devait pas bouger, got %d", got[0].Base)
	}
	if got[1].Base != 5599 {
		t.Errorf("base = %d, want 5599", got[1].Base)
	}
}

func TestPortListKeepsTheDetectedValueOnAnEmptyEntry(t *testing.T) {
	m := portEnter(newPortList())
	m = portEnter(m)

	if got := m.Entries()[0].Base; got != 5432 {
		t.Errorf("base = %d, want the detected 5432 kept", got)
	}
}

func TestPortListConfirmsOnTheDoneRow(t *testing.T) {
	m := newPortList()
	if m.Done() {
		t.Fatal("a fresh list is not done")
	}
	// Une ligne par port, puis la ligne « Done ».
	m = portDown(portDown(m))
	m = portEnter(m)
	if !m.Done() {
		t.Error("la ligne Done valide l'étape")
	}
}

func undeclaredList() PortListModel {
	return NewPortList(NewPortListParams{
		Title: "t",
		Entries: []domain.PortEntry{
			{Job: "api-dev", Name: "PORT", Base: 3001},
			{Job: "web-dev", Name: "PORT"},
		},
	})
}

func TestPortListMarksAServiceWithNoPort(t *testing.T) {
	view := undeclaredList().View()

	if !strings.Contains(view, domain.PortListUndeclared) {
		t.Errorf("an undeclared service must read as one, not as port 0:\n%s", view)
	}
	if strings.Contains(view, "= 0") {
		t.Errorf("a zero base must never render as a port:\n%s", view)
	}
}

func TestPortListDeclaresAPortForAServiceThatHadNone(t *testing.T) {
	m := undeclaredList()
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyEnter))
	for _, r := range "5173" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(key(tea.KeyEnter))

	if got := m.Entries()[1].Base; got != 5173 {
		t.Errorf("base = %d, want the port the user declared", got)
	}
}

func TestPortListEmptyAnswerLeavesAServiceUndeclared(t *testing.T) {
	m := undeclaredList()
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyEnter))
	m, _ = m.Update(key(tea.KeyEnter))

	if got := m.Entries()[1].Base; got != 0 {
		t.Errorf("base = %d, want it left undeclared", got)
	}
}

func TestPortListAnswersThatAServiceBindsNothing(t *testing.T) {
	model := NewPortList(NewPortListParams{Entries: []domain.PortEntry{{Job: "dev:crm", Name: "PORT", CanBindNone: true}}})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	if !updated.Entries()[0].BindsNone {
		t.Fatal("n answers the row rather than leaving it open")
	}
	if model.Entries()[0].BindsNone {
		t.Fatal("the wizard rebuilds a step from its entries; the answer must not reach back into them")
	}
}

func TestPortListTakesTheAnswerBackAndOffersThePortAgain(t *testing.T) {
	model := NewPortList(NewPortListParams{Entries: []domain.PortEntry{{Job: "dev:crm", Name: "PORT", BindsNone: true, CanBindNone: true}}})

	answered, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	back, _ := answered.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if back.Entries()[0].BindsNone {
		t.Fatal("enter on such a row takes the question back")
	}
	if back.Entries()[0].Name == "" {
		t.Fatal("the row must keep something to declare")
	}
}

func TestPortListTakesTheAnswerBackWithTheSameKey(t *testing.T) {
	model := NewPortList(NewPortListParams{Entries: []domain.PortEntry{{Job: "dev:crm", Name: "PORT", CanBindNone: true}}})

	answered, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	back, _ := answered.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	if back.Entries()[0].BindsNone {
		t.Fatal("the key that answers the row un-answers it")
	}
}

func TestPortListRefusesTheAnswerWhereItWouldContradictADeclaredPort(t *testing.T) {
	model := NewPortList(NewPortListParams{Entries: []domain.PortEntry{{Job: "web", Name: "PORT", Base: 3000}}})

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})

	if updated.Entries()[0].BindsNone {
		t.Fatal("run.toml refuses binds_no_port next to a declared port, so the step must not take that answer")
	}
}
