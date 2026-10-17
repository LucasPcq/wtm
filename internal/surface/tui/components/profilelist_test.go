package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func threeProfiles() []domain.ProfileConfig {
	return []domain.ProfileConfig{
		{Name: "api", Jobs: []string{"docker-compose", "api-dev"}},
		{Name: "web", Jobs: []string{"docker-compose", "web-dev"}},
		{Name: "all", Jobs: []string{"docker-compose", "api-dev", "web-dev"}, Default: true},
	}
}

func newProfileList() ProfileListModel {
	return NewProfileList(NewProfileListParams{Title: "Profils", Profiles: threeProfiles()})
}

func profKey(m ProfileListModel, r rune) ProfileListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return m
}

func profDown(m ProfileListModel) ProfileListModel {
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	return m
}

func hasJob(jobs []string, name string) bool {
	for _, job := range jobs {
		if job == name {
			return true
		}
	}
	return false
}

func TestProfileListStartsFromTheProposal(t *testing.T) {
	if got := newProfileList().Profiles(); len(got) != 3 {
		t.Fatalf("expected the 3 proposed profiles, got %d", len(got))
	}
}

func TestProfileListRemoves(t *testing.T) {
	m := profKey(newProfileList(), 'd')

	got := m.Profiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles after removal, got %d", len(got))
	}
	if got[0].Name != "web" {
		t.Errorf("removed the wrong row: %s remains first", got[0].Name)
	}
}

func TestProfileListMergesTwoRows(t *testing.T) {
	// La fusion porte le cas monorepo réel : six profils proposés, deux
	// fusions, et on obtient app1 et app2.
	m := profKey(newProfileList(), 'f') // marque "api"
	m = profDown(m)                     // curseur sur "web"
	m = profKey(m, 'f')                 // fusionne "web" dans "api"

	got := m.Profiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles after the merge, got %d", len(got))
	}

	merged := got[0]
	if merged.Name != "api" {
		t.Errorf("la fusion garde le nom de la ligne marquée, got %s", merged.Name)
	}
	for _, want := range []string{"docker-compose", "api-dev", "web-dev"} {
		if !hasJob(merged.Jobs, want) {
			t.Errorf("merged jobs are missing %s: %v", want, merged.Jobs)
		}
	}
}

func TestProfileListMergeKeepsOneCopyOfASharedJob(t *testing.T) {
	m := profKey(newProfileList(), 'f')
	m = profDown(m)
	m = profKey(m, 'f')

	count := 0
	for _, job := range m.Profiles()[0].Jobs {
		if job == "docker-compose" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("docker-compose apparaît %d fois, want 1", count)
	}
}

func TestProfileListMergeMarkCanBeCancelled(t *testing.T) {
	m := profKey(newProfileList(), 'f')
	m = profKey(m, 'f') // même ligne : annule le marquage

	if len(m.Profiles()) != 3 {
		t.Fatalf("annuler un marquage ne fusionne rien, got %d", len(m.Profiles()))
	}
}

func TestProfileListMovesTheDefaultWhenItsRowGoesAway(t *testing.T) {
	// Le picker pré-sélectionne le default : le perdre le laisserait sans point
	// de départ.
	m := newProfileList()
	m = profDown(profDown(m)) // curseur sur "all", qui porte Default
	m = profKey(m, 'd')

	got := m.Profiles()
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got))
	}
	if !got[0].Default {
		t.Error("le drapeau default doit retomber sur la première ligne restante")
	}
}

func TestProfileListConfirmsOnTheDoneRow(t *testing.T) {
	m := newProfileList()
	if m.Done() {
		t.Fatal("a fresh list is not done")
	}
	m = profDown(profDown(profDown(m)))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done() {
		t.Error("la ligne Done valide l'étape")
	}
}

func TestProfileListNeverWritesThroughToItsCaller(t *testing.T) {
	// ProposeProfiles hands back the config's own slice when a split already
	// exists. Renaming through it edited the loaded run.toml, so a rename the
	// user then escaped out of survived.
	existing := []domain.ProfileConfig{
		{Name: "api", Jobs: []string{"docker-compose"}},
		{Name: "web", Jobs: []string{"web-dev"}},
	}
	m := NewProfileList(NewProfileListParams{Title: "t", Profiles: existing})

	m = profKey(m, 'r')
	for _, r := range "renamed" {
		m = profKey(m, r)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.Profiles()[0].Name; got != "renamed" {
		t.Fatalf("the rename did not take in the model (%q) — the test would pass for the wrong reason", got)
	}
	if existing[0].Name != "api" {
		t.Errorf("the caller's slice was renamed to %q", existing[0].Name)
	}
	if existing[0].Default || existing[1].Default {
		t.Error("the caller's slice gained a default")
	}
}
