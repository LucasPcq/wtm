package daemon

import "time"

// ShortenStartWait makes a test's daemon wait for at most d to start, or for a process that is not wtm's daemon to go.
func ShortenStartWait(d time.Duration) func() {
	was := startWait
	startWait = d
	return func() { startWait = was }
}

// ShortenStoppingWait makes a test's client wait for at most d for a daemon stopping to let its lock go.
func ShortenStoppingWait(d time.Duration) func() {
	was := stoppingWait
	stoppingWait = d
	return func() { stoppingWait = was }
}
