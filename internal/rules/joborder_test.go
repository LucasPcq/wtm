package rules_test

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Les scripts package.json arrivent par ordre alphabétique, ce qui déclare
// `dev` avant `migrate`. Sans réordonnancement, `run up` démarre le serveur sur
// une base qu'aucune migration n'a touchée (LUC-208).
func TestTasksFirst(t *testing.T) {
	jobs := []domain.JobConfig{
		{Name: "dev", Kind: domain.JobKindService},
		{Name: "migrate", Kind: domain.JobKindTask},
		{Name: "seed", Kind: domain.JobKindTask},
		{Name: "web", Kind: domain.JobKindService},
	}

	got := strings.Join(rules.JobNames(rules.TasksFirst(jobs)), ",")
	if got != "migrate,seed,dev,web" {
		t.Errorf("got %s, want migrate,seed,dev,web", got)
	}
}

func TestTasksFirstIsStableWithinAGroup(t *testing.T) {
	jobs := []domain.JobConfig{
		{Name: "b", Kind: domain.JobKindTask},
		{Name: "a", Kind: domain.JobKindTask},
		{Name: "d", Kind: domain.JobKindService},
		{Name: "c", Kind: domain.JobKindService},
	}

	got := strings.Join(rules.JobNames(rules.TasksFirst(jobs)), ",")
	if got != "b,a,d,c" {
		t.Errorf("got %s; l'ordre déclaré est une intention, pas un tri", got)
	}
}
