package rules

import "testing"

func TestAllocateOrdinal(t *testing.T) {
	cases := []struct {
		name  string
		taken []int
		want  int
	}{
		{"aucun worktree", nil, 1},
		{"que le principal", []int{0}, 1},
		{"contigu", []int{1, 2, 3}, 4},
		{"trou recyclé", []int{1, 3, 4}, 2},
		{"trou en tête", []int{2, 3}, 1},
		{"doublons", []int{1, 1, 2}, 3},
		{"désordonné", []int{5, 1, 4, 2}, 3},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AllocateOrdinal(c.taken); got != c.want {
				t.Errorf("AllocateOrdinal(%v) = %d, want %d", c.taken, got, c.want)
			}
		})
	}
}

func TestKeepsOrdinal(t *testing.T) {
	cases := []struct {
		name    string
		branch  string
		ordinal int
		others  []OrdinalHolder
		want    bool
	}{
		{"non attribué", "feat/b", 0, nil, false},
		{"seul à le porter", "feat/b", 2, []OrdinalHolder{{Branch: "feat/a", Ordinal: 1}}, true},
		{"partagé, branche la plus basse", "feat/a", 1, []OrdinalHolder{{Branch: "feat/b", Ordinal: 1}}, true},
		{"partagé, branche la plus haute", "feat/b", 1, []OrdinalHolder{{Branch: "feat/a", Ordinal: 1}}, false},
		{"un autre porte un numéro différent", "feat/b", 1, []OrdinalHolder{{Branch: "feat/a", Ordinal: 2}}, true},
		{"le principal ne dispute rien", "feat/b", 1, []OrdinalHolder{{Branch: "main", Ordinal: 0}}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := KeepsOrdinal(KeepsOrdinalParams{Branch: c.branch, Ordinal: c.ordinal, Others: c.others})
			if got != c.want {
				t.Errorf("KeepsOrdinal(%s, %d) = %v, want %v", c.branch, c.ordinal, got, c.want)
			}
		})
	}
}

// Exactly one of a set of duplicates keeps the number, whoever is asked first.
func TestKeepsOrdinalPicksExactlyOneWinner(t *testing.T) {
	holders := []OrdinalHolder{
		{Branch: "feat/c", Ordinal: 3},
		{Branch: "feat/a", Ordinal: 3},
		{Branch: "feat/b", Ordinal: 3},
	}

	winners := 0
	for i, holder := range holders {
		others := append(append([]OrdinalHolder{}, holders[:i]...), holders[i+1:]...)
		if KeepsOrdinal(KeepsOrdinalParams{Branch: holder.Branch, Ordinal: holder.Ordinal, Others: others}) {
			winners++
		}
	}

	if winners != 1 {
		t.Errorf("%d worktrees kept ordinal 3, want exactly 1", winners)
	}
}
