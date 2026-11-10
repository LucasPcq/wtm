package run

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAShutdownStopsWhatTheDaemonRunsKeepsItsStacksAndLosesTheClaimsOnWhatStops(t *testing.T) {
	web := Job{Name: "web", WorkDir: "/a", Up: true, Hold: HoldOwn}
	db := Job{Name: "db", WorkDir: "/a", Up: true, Hold: HoldOwn, SharedDir: "/a"}
	dbHeld := Job{Name: "db", WorkDir: "/a/x", Up: true, Hold: HoldClaim, SharedDir: "/a"}
	cache := Job{Name: "cache", WorkDir: "/a", Up: true, Hold: HoldDetached, SharedDir: "/a"}
	cacheHeld := Job{Name: "cache", WorkDir: "/a/x", Up: true, Hold: HoldClaim, SharedDir: "/a"}
	crashed := Job{Name: "api", WorkDir: "/a", Hold: HoldOwn, State: Crashed}

	got := ShutdownOf([]Job{web, db, dbHeld, cache, cacheHeld, crashed})

	assert.Equal(t, Shutdown{Cut: []Job{web, db}, Kept: []Job{cache, cacheHeld}, Lost: []Job{dbHeld}}, got)
}
