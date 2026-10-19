package provision

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"

	"github.com/stretchr/testify/assert"
)

func TestAMarkedEnvThatFollowsTheConfigReadsAsTheConfigDefault(t *testing.T) {
	remembered := domain.Marks{Remembered: []string{domain.PathEnvFrom}, Mark: domain.MarkRemember}
	cases := map[string]struct {
		params RecapParams
		want   domain.RecapValue
	}{
		"marked_follows": {RecapParams{EnvFrom: project.EnvExample, FollowsConfig: true, Marks: remembered}, domain.RecapValue{Value: "example", Note: NoteConfigDefault, Text: "config default (example)"}},
		"marked":         {RecapParams{EnvFrom: project.EnvExample, Marks: remembered}, domain.RecapValue{Value: "example", Text: "example"}},
		"follows":        {RecapParams{EnvFrom: project.EnvExample, FollowsConfig: true}, domain.RecapValue{Value: "example", Text: "example"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, []domain.RecapValue{c.want}, RecapRows(c.params)[0].Values)
		})
	}
}

func TestAMarkedFastForwardOfNoSourceKeepsItsRow(t *testing.T) {
	saved := domain.Marks{Remembered: []string{domain.PathFastForward}, Mark: domain.MarkSaved}

	assert.Equal(t, []domain.RecapRow{{Key: domain.PathFastForward, Label: "Fast-forward", Path: domain.PathFastForward, Mark: domain.MarkSaved, Values: []domain.RecapValue{{Note: NoteNone, Text: "keep as-is"}}}}, RecapRows(RecapParams{Marks: saved}))
	assert.Empty(t, RecapRows(RecapParams{}))
}
