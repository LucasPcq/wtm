package rules

import "github.com/LucasPcq/wtm/internal/domain"

// IndexVersionParams compares the format of the index on disk against the one
// this binary writes. Two ints of the same type, so they are named rather than
// ordered: swapping them silently inverts the verdict.
type IndexVersionParams struct {
	File   int
	Binary int
}

// IndexAccess is what a daemon may do with the index it just read. The two
// falses are a third answer, and the one that matters: an abandoned format holds
// nothing this binary can read, yet there is nothing in it to protect either.
type IndexAccess struct {
	// Read says the entries are in the shape this binary reads.
	Read bool
	// Freeze makes the store read-only for the rest of its life, which is the
	// only safe answer to an index a NEWER binary owns: overwriting it would
	// destroy that binary's record of what it started.
	Freeze bool
}

// ClassifyIndexVersion tells the two directions apart, which is the whole point.
// Freezing on any difference was a silent way to disable the index outright: a
// stale file from an older format made every Save a no-op, so nothing was
// recorded, detached stacks were never picked back up, and the orphan
// reconciliation had nothing to reconcile with. An older format is one this
// binary has moved past — there is nothing in it worth keeping, so it is simply
// replaced by the next write.
func ClassifyIndexVersion(params IndexVersionParams) IndexAccess {
	if params.File == params.Binary {
		return IndexAccess{Read: true}
	}
	if params.File > params.Binary {
		return IndexAccess{Freeze: true}
	}
	return IndexAccess{}
}

// CurrentJobStatus reads a status a daemon built before the attached → joined
// rename may still report.
func CurrentJobStatus(status domain.JobStatus) domain.JobStatus {
	if status == domain.JobStatusLegacyAttached {
		return domain.JobStatusJoined
	}
	return status
}

func CurrentJobInfos(jobs []domain.JobInfo) []domain.JobInfo {
	if jobs == nil {
		return nil
	}
	current := make([]domain.JobInfo, len(jobs))
	for i, job := range jobs {
		job.Status = CurrentJobStatus(job.Status)
		current[i] = job
	}
	return current
}

func CurrentJobRecords(records []domain.JobRecord) []domain.JobRecord {
	if records == nil {
		return nil
	}
	current := make([]domain.JobRecord, len(records))
	for i, record := range records {
		record.Joined = record.Joined || record.LegacyAttached
		record.LegacyAttached = false
		current[i] = record
	}
	return current
}
