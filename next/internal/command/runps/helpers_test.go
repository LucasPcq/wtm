package runps_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/require"
)

// read runs run ps over s for req.
func read(t *testing.T, s seed, req runps.Request) runps.Outcome {
	t.Helper()
	outcome, err := usecase(s).Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	return outcome
}

// subjects are the subjects of read's items, in order.
func subjects(read runps.Outcome) []string {
	var names []string
	for _, item := range read.Items {
		names = append(names, item.Subject)
	}
	return names
}

// group is the detail of read's item whose subject is subject.
func group(t *testing.T, read runps.Outcome, subject string) runps.Detail {
	t.Helper()
	for _, item := range read.Items {
		if item.Subject == subject {
			require.NotNil(t, item.Detail, subject)
			return *item.Detail
		}
	}
	t.Fatalf("no group %s in %v", subject, subjects(read))
	return runps.Detail{}
}

// names are the names of jobs, in order, each with its state.
func names(jobs []runps.Job) []string {
	var found []string
	for _, job := range jobs {
		found = append(found, job.Name+":"+string(job.State))
	}
	return found
}

// codes are the codes of read's warnings, in order.
func codes(read runps.Outcome) []string {
	var found []string
	for _, warning := range read.Warnings {
		found = append(found, warning.Code)
	}
	return found
}
