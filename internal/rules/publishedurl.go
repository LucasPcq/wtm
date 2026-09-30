package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// PickPublishedURL resolves which published job the caller meant. A name that
// publishes nothing is refused by name, an ambiguity is refused naming the flag,
// and a single published job is the answer rather than a question — the same
// three outcomes whether the caller could be asked or not.
func PickPublishedURL(entries []domain.JobURLEntry, jobName string) (domain.JobURLEntry, error) {
	if len(entries) == 0 {
		return domain.JobURLEntry{}, domain.ErrJobNonePublished
	}
	if jobName != "" {
		return findPublished(entries, jobName)
	}
	if len(entries) > 1 {
		return domain.JobURLEntry{}, fmt.Errorf("%w — %s", domain.ErrJobAmbiguous, PublishedJobNames(entries))
	}
	return entries[0], nil
}

func findPublished(entries []domain.JobURLEntry, name string) (domain.JobURLEntry, error) {
	for _, entry := range entries {
		if entry.Job == name {
			return entry, nil
		}
	}
	return domain.JobURLEntry{}, fmt.Errorf(domain.RunJobURLNoneFmt, name, PublishedJobNames(entries))
}

// PublishedJobNames names the jobs an error or a picker lists.
func PublishedJobNames(entries []domain.JobURLEntry) string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Job)
	}
	return strings.Join(names, domain.RunURLListSep)
}
