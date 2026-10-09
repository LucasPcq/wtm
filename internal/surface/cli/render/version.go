package output

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
)

func WriteVersionJSON(w io.Writer, report domain.VersionReport) error {
	return encodeJSON(w, report)
}

func VersionLine(w io.Writer, version string) {
	fmt.Fprintf(w, domain.VersionLineFmt+"\n", domain.AppName, version)
}
