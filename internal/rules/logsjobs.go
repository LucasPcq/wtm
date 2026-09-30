package rules

type LogsJobColumnParams struct {
	Jobs []string
	// Current is the job whose output is on screen; it always stays visible.
	Current string
	Rows    int
}

// LogsJobRange is half-open.
type LogsJobRange struct {
	Start int
	End   int
}

// LogsJobColumn keeps the current job inside the column and fills the rest with
// its neighbours, scrolling as a list does rather than windowing around a
// cursor: a column that jumps under the pointer is one a reader cannot aim at.
func LogsJobColumn(params LogsJobColumnParams) LogsJobRange {
	total := len(params.Jobs)
	if total == 0 || params.Rows <= 0 {
		return LogsJobRange{}
	}
	if total <= params.Rows {
		return LogsJobRange{Start: 0, End: total}
	}

	current := 0
	for index, job := range params.Jobs {
		if job == params.Current {
			current = index
			break
		}
	}

	// The current job is pulled to the middle only once it is far enough in for
	// a middle to exist; before that the column stays at the top, where the
	// first jobs are.
	start := current - params.Rows/2
	if start < 0 {
		start = 0
	}
	if start+params.Rows > total {
		start = total - params.Rows
	}
	return LogsJobRange{Start: start, End: start + params.Rows}
}

type LogsJobColumnWidthParams struct {
	Names []string
	// Total is the whole body's width, column and tail together.
	Total int
	Glyph int
	Gap   int
	// TailMin is what the output beside the column must keep. A column is worth
	// having only while the log it labels stays readable.
	TailMin int
	// Max caps the column so a long job name cannot squeeze the output it is
	// there to label.
	Max int
}

// LogsJobColumnWidth sizes the job column: wide enough for the longest name it
// holds, never past its cap nor past half the body. Zero means the body is too
// narrow for a column at all, and the tail then takes everything.
func LogsJobColumnWidth(params LogsJobColumnWidthParams) int {
	if len(params.Names) == 0 || params.Total <= 0 {
		return 0
	}

	widest := 0
	for _, name := range params.Names {
		if runes := len([]rune(name)); runes > widest {
			widest = runes
		}
	}

	width := widest + params.Glyph
	if params.Max > 0 && width > params.Max {
		width = params.Max
	}
	if half := params.Total / 2; width > half {
		width = half
	}
	// Measured against the space left to the tail, never against the names: short
	// job names make a narrow column, and a narrow column is still a column.
	if width+params.Gap+params.TailMin > params.Total {
		return 0
	}
	return width
}
