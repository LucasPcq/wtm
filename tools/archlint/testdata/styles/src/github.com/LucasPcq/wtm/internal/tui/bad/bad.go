package bad

import lg "github.com/charmbracelet/lipgloss"

var plain = lg.NewStyle() // want `only internal/styles may instantiate a lipgloss.Style`

var literal = lg.Style{} // want `only internal/styles may instantiate a lipgloss.Style`

var typed lg.Style
