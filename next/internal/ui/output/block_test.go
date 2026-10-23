package output

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestAReadEndsOnEachSubjectNotReadItsWarningsThenTheReadsWarnings(t *testing.T) {
	read := domain.Read[struct{}]{
		Items: []domain.Unit[struct{}]{
			{Subject: "a", Err: &domain.Error{Message: "couldn't read /a"}},
			{Subject: "b", Warnings: []domain.Warning{{Message: "couldn't read the record of b"}}},
		},
		Warnings: []domain.Warning{{Message: "the base branch main does not exist"}},
	}
	var out bytes.Buffer

	ReadEnd(&out, read)

	assert.Equal(t, "  ✗ couldn't read /a\n  ! couldn't read the record of b\n  ! the base branch main does not exist\n", out.String())
}

func TestAReadInterruptedEndsOnItsInterruptionEvenWithEverySubjectRead(t *testing.T) {
	var out bytes.Buffer

	ReadEnd(&out, domain.Read[struct{}]{Items: []domain.Unit[struct{}]{{Subject: "a", Status: domain.StatusDone}}, Interrupted: true})

	assert.Equal(t, "  ✗ interrupted\n", out.String())
}
