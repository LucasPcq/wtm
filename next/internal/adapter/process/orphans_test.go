package process

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestAGroupIsIdentifiedByTheStartItsRecordSays(t *testing.T) {
	child, _ := spawned(t, "sleep 30", false)
	time.Sleep(50 * time.Millisecond)
	pgid := child.PID()

	assert.Equal(t, map[int]run.Group{pgid: {Alive: true, Identified: true}}, Orphans{}.Probe(map[int]time.Time{pgid: time.Now()}))
	assert.Equal(t, map[int]run.Group{pgid: {Alive: true}}, Orphans{}.Probe(map[int]time.Time{pgid: time.Now().Add(-time.Hour)}))
}

func TestAGroupGoneOrNotOursIsNotAlive(t *testing.T) {
	child, _ := spawned(t, "exit 0", false)
	child.Wait()

	groups := Orphans{}.Probe(map[int]time.Time{child.PID(): time.Now(), 1: time.Now(), 0: time.Now()})

	assert.Equal(t, map[int]run.Group{child.PID(): {}, 1: {}, 0: {}}, groups)
}

func TestReapStopsTheGroupsEvenDeafToSIGTERM(t *testing.T) {
	deaf, out := spawned(t, "trap '' TERM; echo ready; while :; do sleep 1; done", false)
	plain, _ := spawned(t, "sleep 30", false)
	assert.Eventually(t, func() bool { return out.String() == "ready\n" }, 5*time.Second, 10*time.Millisecond)

	Orphans{}.Reap([]int{deaf.PID(), plain.PID()})

	assert.Equal(t, -1, deaf.Wait())
	assert.Equal(t, -1, plain.Wait())
}

func TestElapsedIsReadAsPsWritesIt(t *testing.T) {
	cases := map[string]struct {
		etime string
		want  time.Duration
		ok    bool
	}{
		"minutes": {etime: "01:05", want: 65 * time.Second, ok: true},
		"hours":   {etime: "02:00:01", want: 2*time.Hour + time.Second, ok: true},
		"days":    {etime: "3-00:00:10", want: 72*time.Hour + 10*time.Second, ok: true},
		"garbage": {etime: "soon"},
		"seconds": {etime: "12"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := parseElapsed(c.etime)

			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.want, got)
		})
	}
}
