package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAStopStopsOwnJobsClearsCrashesAndReleasesWhatOthersHold(t *testing.T) {
	db := Job{Name: "db", WorkDir: "/main", Up: true, State: Running, Hold: HoldOwn, SharedDir: "/main"}
	claim := func(dir string) Job {
		return Job{Name: "db", WorkDir: dir, Up: true, Hold: HoldClaim, SharedDir: "/main"}
	}
	api := Job{Name: "api", WorkDir: "/a", Up: true, State: Running, Hold: HoldOwn}
	crashed := Job{Name: "worker", WorkDir: "/a", State: Crashed, Hold: HoldOwn}
	exited := Job{Name: "migrate", WorkDir: "/a", State: Exited, Hold: HoldOwn}
	mainHeld := db
	mainHeld.MainHolds = true
	cases := map[string]struct {
		jobs  []Job
		dirs  []string
		names []string
		want  []Stopping
	}{
		"own_and_crash": {jobs: []Job{api, crashed, exited}, dirs: []string{"/a"}, want: []Stopping{{Job: api, Fate: FateStop}, {Job: crashed, Fate: FateClear}}},
		"release_kept":  {jobs: []Job{db, claim("/a"), claim("/b")}, dirs: []string{"/a"}, want: []Stopping{{Job: claim("/a"), Fate: FateRelease, KeptBy: []string{"/b"}}}},
		"last_hold":     {jobs: []Job{db, claim("/a")}, dirs: []string{"/a"}, want: []Stopping{{Job: claim("/a"), Fate: FateStop}}},
		"every_hold":    {jobs: []Job{db, claim("/a"), claim("/b")}, dirs: []string{"/a", "/b"}, want: []Stopping{{Job: claim("/a"), Fate: FateStop}, {Job: claim("/b"), Fate: FateStop}}},
		"main_holds":    {jobs: []Job{mainHeld, claim("/a")}, dirs: []string{"/a"}, want: []Stopping{{Job: claim("/a"), Fate: FateRelease, KeptBy: []string{"/main"}}}},
		"main_releases": {jobs: []Job{mainHeld, claim("/a")}, dirs: []string{"/main"}, want: []Stopping{{Job: mainHeld, Fate: FateRelease, KeptBy: []string{"/a"}}}},
		"main_not_held": {jobs: []Job{db, claim("/a")}, dirs: []string{"/main"}},
		"names":         {jobs: []Job{api, crashed, db, claim("/a")}, dirs: []string{"/a"}, names: []string{"db"}, want: []Stopping{{Job: claim("/a"), Fate: FateStop}}},
		"order":         {jobs: []Job{claim("/a"), api, claim("/b")}, dirs: []string{"/b", "/a"}, want: []Stopping{{Job: claim("/b"), Fate: FateStop}, {Job: claim("/a"), Fate: FateStop}, {Job: api, Fate: FateStop}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, StoppingIn(c.jobs, c.dirs, c.names))
		})
	}
}
