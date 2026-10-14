package hooks

// unguardedSink keeps state unprotected, as every surface's sink does: under -race a test fails unless the runner serializes the two streams.
type unguardedSink struct {
	lines int
	body  []byte
}

func (s *unguardedSink) Write(p []byte) (int, error) {
	s.body = append(s.body, p...)
	s.lines++
	return len(p), nil
}
