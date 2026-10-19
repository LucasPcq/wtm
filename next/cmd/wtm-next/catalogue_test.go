package main

import (
	"net/http"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheCatalogueListsEachCommandWithItsSchemaOutsideAnyRepository(t *testing.T) {
	var names []string
	for _, command := range catalogue() {
		names = append(names, command.Name)
		assert.NotEmpty(t, command.Form.Fields, command.Name)
	}

	assert.Equal(t, []string{"create", "extract", "list"}, names)
}

func TestAnInvalidCallIsRefusedWithTheStatusOfItsKind(t *testing.T) {
	main := configured(t)
	t.Setenv("HOME", t.TempDir())
	create := command(t, "create")

	for name, c := range map[string]struct {
		Dir     string
		Answers []domain.Answered
		Status  int
		Code    string
	}{
		"unknown_field":     {Dir: main, Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}, {Path: "brnach", Values: []string{"x"}}}, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"two_values_of_one": {Dir: main, Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}, {Path: "from", Values: []string{"main", "dev"}}}, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"no_branch":         {Dir: main, Status: http.StatusBadRequest, Code: domain.CodeInvalidRequest},
		"no_repository":     {Dir: t.TempDir(), Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/x"}}}, Status: http.StatusPreconditionFailed, Code: domain.CodeNotARepository},
	} {
		t.Run(name, func(t *testing.T) {
			got := create.Execute(t.Context(), api.Call{Dir: c.Dir, Answers: c.Answers})

			assert.Equal(t, c.Status, got.Status)
			require.NotNil(t, got.Envelope.Err)
			assert.Equal(t, c.Code, got.Envelope.Err.Code)
			assert.Equal(t, "create", got.Envelope.Command)
		})
	}
}
