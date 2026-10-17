package branch

func (ab AheadBehind) Divergence() Divergence {
	switch {
	case ab.Ahead > 0 && ab.Behind > 0:
		return Diverged
	case ab.Behind > 0:
		return Behind
	case ab.Ahead > 0:
		return Ahead
	}
	return UpToDate
}
