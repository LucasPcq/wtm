package status_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/require"
)

// read runs req over w and requires it read.
func read(t *testing.T, w world, req status.Request) status.Outcome {
	t.Helper()
	got, err := w.usecase.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	return got
}

// detailOf is the detail of the item of got whose subject is subject.
func detailOf(t *testing.T, got status.Outcome, subject string) status.Detail {
	t.Helper()
	for _, item := range got.Items {
		if item.Subject == subject {
			require.NotNil(t, item.Detail, subject)
			return *item.Detail
		}
	}
	t.Fatalf("no item %s", subject)
	return status.Detail{}
}

// only is the detail of the one item of got.
func only(t *testing.T, got status.Outcome) status.Detail {
	t.Helper()
	require.Len(t, got.Items, 1)
	require.NotNil(t, got.Items[0].Detail, got.Items[0].Err)
	return *got.Items[0].Detail
}

// codes are the codes of warnings, in order.
func codes(warnings []domain.Warning) []string {
	var found []string
	for _, warning := range warnings {
		found = append(found, warning.Code)
	}
	return found
}

// problemCodes are the codes of problems, in order.
func problemCodes(problems []status.Problem) []string {
	var found []string
	for _, problem := range problems {
		found = append(found, problem.Code)
	}
	return found
}

// fixOf is the fix of a missing .env file that runs command with answers.
func fixOf(command string, answers ...domain.Answered) *domain.FollowUp {
	return &domain.FollowUp{Command: command, Answers: answers, Code: status.CodeEnvMissing}
}

// fieldCodes are the codes of problems, in order.
func fieldCodes(problems []domain.FieldError) []string {
	var found []string
	for _, problem := range problems {
		found = append(found, problem.Code)
	}
	return found
}
