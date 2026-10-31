package daemon

import "time"

// ShortenStartWait makes a test's daemon wait for at most d to start, or for a process that is no run daemon to go.
func ShortenStartWait(d time.Duration) func() {
	was := startWait
	startWait = d
	return func() { startWait = was }
}
