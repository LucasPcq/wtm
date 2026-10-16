package worktree

import "slices"

const mainOrdinal = 0

func FreeOrdinal(taken []int) int {
	candidate := mainOrdinal + 1
	for slices.Contains(taken, candidate) {
		candidate++
	}
	return candidate
}
