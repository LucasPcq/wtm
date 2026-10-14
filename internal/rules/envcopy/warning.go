package envcopy

//go-sumtype:decl Warning

// Warning is a file a copy could not provision as its strategy asked.
type Warning interface{ warning() }

// NoTemplate is a file the example strategy skipped: no template on disk.
type NoTemplate struct{ Target string }

// SourceMissing is a file skipped because Dir does not hold it.
type SourceMissing struct {
	Target string
	Dir    string
}

// ParentFallback is a file the parent worktree lacks, copied from the main one.
type ParentFallback struct{ Target string }

func (NoTemplate) warning()     {}
func (SourceMissing) warning()  {}
func (ParentFallback) warning() {}
