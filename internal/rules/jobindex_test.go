package rules_test

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func TestClassifyIndexVersionReadsItsOwnFormat(t *testing.T) {
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{File: 2, Binary: 2})

	if !access.Read || access.Freeze {
		t.Fatalf("access = %+v, want the entries read and nothing frozen", access)
	}
}

func TestClassifyIndexVersionReplacesAnAbandonedFormat(t *testing.T) {
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{File: 1, Binary: 2})

	if access.Read {
		t.Fatal("an older format holds nothing this binary can read")
	}
	if access.Freeze {
		t.Fatal("freezing on a stale file disables the index outright: nothing is recorded and nothing is ever picked back up")
	}
}

func TestClassifyIndexVersionFreezesOnANewerBinarysIndex(t *testing.T) {
	access := rules.ClassifyIndexVersion(rules.IndexVersionParams{File: 3, Binary: 2})

	if !access.Freeze {
		t.Fatal("overwriting a newer binary's index would destroy its record of what it started")
	}
	if access.Read {
		t.Fatal("a format from the future is not one this binary can read")
	}
}

func TestJoinedStatusReadsTheLegacyAttachedSpelling(t *testing.T) {
	cases := map[domain.JobStatus]domain.JobStatus{
		domain.JobStatusLegacyAttached: domain.JobStatusJoined,
		domain.JobStatusJoined:         domain.JobStatusJoined,
		domain.JobStatusRunning:        domain.JobStatusRunning,
	}
	for in, want := range cases {
		if got := rules.CurrentJobStatus(in); got != want {
			t.Errorf("CurrentJobStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCurrentJobRecordsFoldTheLegacyAttachedKey(t *testing.T) {
	records := rules.CurrentJobRecords([]domain.JobRecord{
		{Name: "db", LegacyAttached: true},
		{Name: "api"},
	})
	if !records[0].Joined || records[0].LegacyAttached {
		t.Errorf("legacy claim = %+v, want Joined and no legacy key", records[0])
	}
	if records[1].Joined {
		t.Errorf("a process of its own became a claim: %+v", records[1])
	}
}

func TestCurrentJobInfosReadTheLegacyAttachedStatus(t *testing.T) {
	jobs := rules.CurrentJobInfos([]domain.JobInfo{{Name: "db", Status: domain.JobStatusLegacyAttached}})
	if jobs[0].Status != domain.JobStatusJoined {
		t.Errorf("status = %q, want joined", jobs[0].Status)
	}
}
