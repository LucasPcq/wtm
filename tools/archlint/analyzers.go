package main

import "golang.org/x/tools/go/analysis"

var analyzers = append(internalOnly(
	layersAnalyzer,
	domainAnalyzer,
	stylesAnalyzer,
	yesflagAnalyzer,
	chokepointAnalyzer,
	glyphAnalyzer,
	tuistyleAnalyzer,
	mutedlineAnalyzer,
	servicedagAnalyzer,
	daemonblindAnalyzer,
	metawriterAnalyzer,
	emitsAnalyzer,
	publishAnalyzer,
	statewriteAnalyzer,
	stateownerAnalyzer,
	pkgdocAnalyzer,
	rootfreezeAnalyzer,
),
	typeassertAnalyzer,
	fontcoverAnalyzer,
	nextAnalyzer,
	codeprefixAnalyzer,
	subtestnameAnalyzer,
	testlayoutAnalyzer,
	portfileAnalyzer,
	envreadAnalyzer,
	slicefilesAnalyzer,
)

func internalOnly(written ...*analysis.Analyzer) []*analysis.Analyzer {
	scoped := make([]*analysis.Analyzer, 0, len(written))
	for _, original := range written {
		copied := *original
		copied.Run = func(pass *analysis.Pass) (any, error) {
			if internalPath(pass.Pkg.Path()) == "" {
				return nil, nil
			}
			return original.Run(pass)
		}
		scoped = append(scoped, &copied)
	}
	return scoped
}
