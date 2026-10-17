package hooks

type unguardedSink struct {
	lines int
	body  []byte
}

func (s *unguardedSink) Write(p []byte) (int, error) {
	// Unprotected, as every surface's sink is: under -race a test fails unless the runner serializes the two streams.
	s.body = append(s.body, p...)
	s.lines++
	return len(p), nil
}
