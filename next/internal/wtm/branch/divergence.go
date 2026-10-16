package branch

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const remotePrefix = "origin/"

func Divergence(ab wtm.AheadBehind) wtm.Divergence {
	switch {
	case ab.Ahead > 0 && ab.Behind > 0:
		return wtm.Diverged
	case ab.Behind > 0:
		return wtm.Behind
	case ab.Ahead > 0:
		return wtm.Ahead
	}
	return wtm.UpToDate
}

func IsRemote(name string) bool {
	return strings.HasPrefix(name, remotePrefix)
}
