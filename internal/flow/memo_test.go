package flow

import "testing"

func TestSetMemoLoadsOncePerSelection(t *testing.T) {
	var memo SetMemo[int]
	loads := 0
	load := func() int { loads++; return loads }

	first := memo.Get([]string{"a", "b"}, load)
	again := memo.Get([]string{"a", "b"}, load)
	other := memo.Get([]string{"a"}, load)

	if first != 1 || again != 1 || other != 2 {
		t.Errorf("got %d, %d, %d, want the same selection read once", first, again, other)
	}
}
