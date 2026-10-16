package envcopy

//go-sumtype:decl Warning

type Warning interface{ warning() }

type NoTemplate struct{ Target string }

type SourceMissing struct {
	Target string
	Dir    string
}

type ParentFallback struct{ Target string }

func (NoTemplate) warning()     {}
func (SourceMissing) warning()  {}
func (ParentFallback) warning() {}
