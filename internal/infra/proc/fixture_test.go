package proc

import "time"

const (
	readerGoneEvery = 10 * time.Millisecond
	daemonHelperEnv = "WTM_TEST_DAEMON_HELPER"
)
