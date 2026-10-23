package domain

// A Caller is where the caller of a command stands. Dir is the directory the command runs on: $WTM_PROJECT_DIR on the command line, else its working directory; a surface's call's directory, "" when the call names none. Named says whether the caller chose Dir rather than merely ran from it. Cwd is the directory the caller is in, its symbolic links resolved: a relative path it gives is read from it, and the worktree it stands in is found by it.
type Caller struct {
	Dir   string
	Named bool
	Cwd   string
}

// ReadsCaller is a command's request that reads where its caller stands; no field of its form says it, every surface sets it.
type ReadsCaller[R any] interface {
	From(caller Caller) R
}

// CalledBy is req asked by caller, when req reads where its caller stands; req as it is otherwise.
func CalledBy[R any](req R, caller Caller) R {
	if reads, ok := any(req).(ReadsCaller[R]); ok {
		return reads.From(caller)
	}
	return req
}
