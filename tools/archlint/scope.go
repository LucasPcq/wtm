package main

import "strings"

const (
	modulePath     = "github.com/LucasPcq/wtm/"
	internalPrefix = modulePath + "internal/"
)

func internalPath(pkgPath string) string {
	rest, ok := strings.CutPrefix(pkgPath, internalPrefix)
	if !ok {
		return ""
	}
	return rest
}

func layerOfPackage(pkgPath string) string {
	return strings.Split(internalPath(pkgPath), "/")[0]
}
