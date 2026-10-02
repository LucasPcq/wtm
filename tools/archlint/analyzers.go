package main

import "golang.org/x/tools/go/analysis"

var analyzers = []*analysis.Analyzer{
	layersAnalyzer,
	domainAnalyzer,
	stylesAnalyzer,
	typeassertAnalyzer,
	yesflagAnalyzer,
	chokepointAnalyzer,
	glyphAnalyzer,
	tuistyleAnalyzer,
	mutedlineAnalyzer,
	fontcoverAnalyzer,
}
