package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestSpecialPicksItsStepsByPathNeverByKind(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})
	last := len(extract.Questions{}.Form().Fields) - 1
	extra := domain.FieldSpec{Path: "provision_extra"}
	fields := map[string]domain.Field{
		"multiSelect": domain.MultiSelectField{FieldSpec: extra},
		"text":        domain.TextField{FieldSpec: extra},
	}
	for name, field := range fields {
		t.Run(name, func(t *testing.T) {
			assert.Nil(t, special(d.s, last, field))
		})
	}
}
