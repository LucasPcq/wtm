package output

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAHookThatSucceedsLeavesOneResultLine(t *testing.T) {
	var out bytes.Buffer
	view := newHookView(hookViewParams{W: &out, Label: "on_create · feat/x", LogPath: "/state/hooks/x.log"})

	_, _ = view.Write([]byte("one\ntwo\n"))
	view.finish(hookEnd{Label: "on_create · feat/x", Duration: 1200 * time.Millisecond})

	assert.Equal(t, []string{"  ✓ on_create · feat/x (1.2s)"}, finalScreen(out.String()))
}

func TestAHookThatFailsKeepsItsTailAndPointsAtItsLog(t *testing.T) {
	var out bytes.Buffer
	view := newHookView(hookViewParams{W: &out, Label: "on_create · feat/x", LogPath: "/state/hooks/x.log"})

	for i := range 12 {
		_, _ = view.Write([]byte("line " + strconv.Itoa(i) + "\n"))
	}
	_, _ = view.Write([]byte("progress 10%\rprogress 90%\rbroken"))
	view.finish(hookEnd{Label: "on_create · feat/x", Duration: 300 * time.Millisecond, Failed: true})

	assert.Equal(t, []string{
		"  ✗ on_create · feat/x (300ms)",
		"    line 5", "    line 6", "    line 7", "    line 8", "    line 9", "    line 10", "    line 11",
		"    broken",
		"  full output:  /state/hooks/x.log",
	}, finalScreen(out.String()))
}

func TestARunningHookShowsItsBoundedTail(t *testing.T) {
	var out bytes.Buffer
	view := newHookView(hookViewParams{W: &out, Label: "on_create · feat/x"})

	_, _ = view.Write([]byte("a\r\nb\n"))

	assert.Equal(t, []string{"  → on_create · feat/x", "    a", "    b"}, finalScreen(out.String()))
	view.close()
	assert.Empty(t, finalScreen(out.String()))
}
