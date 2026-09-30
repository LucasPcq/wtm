package rules

import "testing"

func column(jobs []string, current string, rows int) LogsJobRange {
	return LogsJobColumn(LogsJobColumnParams{Jobs: jobs, Current: current, Rows: rows})
}

func TestLogsJobColumnShowsEverythingThatFits(t *testing.T) {
	got := column([]string{"web", "api", "db"}, "api", 10)
	if got.Start != 0 || got.End != 3 {
		t.Errorf("got %+v, want the whole list", got)
	}
}

func TestLogsJobColumnKeepsTheCurrentJobVisible(t *testing.T) {
	jobs := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	for _, current := range jobs {
		got := column(jobs, current, 4)
		index := 0
		for i, job := range jobs {
			if job == current {
				index = i
			}
		}
		if index < got.Start || index >= got.End {
			t.Errorf("current %q at %d is outside %+v", current, index, got)
		}
		if got.End-got.Start != 4 {
			t.Errorf("column of %+v, want exactly 4 rows filled", got)
		}
	}
}

func TestLogsJobColumnStaysAtTheTopForTheFirstJobs(t *testing.T) {
	got := column([]string{"a", "b", "c", "d", "e"}, "a", 3)
	if got.Start != 0 {
		t.Errorf("got %+v, want the column at the top", got)
	}
}

func TestLogsJobColumnStopsAtTheBottom(t *testing.T) {
	got := column([]string{"a", "b", "c", "d", "e"}, "e", 3)
	if got.End != 5 || got.Start != 2 {
		t.Errorf("got %+v, want the last three", got)
	}
}

func TestLogsJobColumnIsEmptyWithoutJobs(t *testing.T) {
	if got := column(nil, "", 5); got.Start != 0 || got.End != 0 {
		t.Errorf("got %+v, want an empty range", got)
	}
}

func width(names []string, total int) int {
	return LogsJobColumnWidth(LogsJobColumnWidthParams{Names: names, Total: total, Glyph: 2, Gap: 1, Max: 24, TailMin: 20})
}

func TestLogsJobColumnWidthFitsTheLongestName(t *testing.T) {
	if got := width([]string{"web", "api-gateway"}, 100); got != len("api-gateway")+2 {
		t.Errorf("got %d, want the longest name plus its glyph", got)
	}
}

func TestLogsJobColumnWidthNeverTakesMoreThanHalf(t *testing.T) {
	if got := width([]string{"a-very-long-job-name"}, 20); got > 10 {
		t.Errorf("got %d, want at most half of 20", got)
	}
}

func TestLogsJobColumnWidthGivesUpOnANarrowBody(t *testing.T) {
	if got := width([]string{"web"}, 4); got != 0 {
		t.Errorf("got %d, want no column at all", got)
	}
}

// Short job names used to lose the column entirely on a wide terminal: the
// guard measured the names instead of the room left to the tail.
func TestLogsJobColumnWidthKeepsAColumnForShortNames(t *testing.T) {
	if got := width([]string{"ui", "db", "pg"}, 200); got == 0 {
		t.Error("a 200-column body with short job names must still draw the column")
	}
}
