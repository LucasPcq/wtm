package main

import "golang.org/x/tools/go/analysis"

var analyzers = []*analysis.Analyzer{
	layersAnalyzer,
	domainAnalyzer,
	stylesAnalyzer,
	typeassertAnalyzer,
	yesflagAnalyzer,
	mutationAnalyzer,
	glyphAnalyzer,
	tuistyleAnalyzer,
	mutedlineAnalyzer,
	fontcoverAnalyzer,
}
