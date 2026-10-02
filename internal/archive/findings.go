package archive

import (
	"fmt"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
)

// Findings turns an archive report into findings.
func Findings(rep *Report) []scanner.Finding {
	var out []scanner.Finding
	if !rep.Complete {
		out = append(out, scanner.NewFinding(
			"archive.corrupted", scanner.SeverityHigh,
			"Archive is incomplete — import will fail",
			fmt.Sprintf("%s. All-in-One WP Migration will refuse it (\"archive appears to be corrupted\"). Download or copy it again and compare the byte size with the original. %d complete entries were found before the problem.", rep.Problem, rep.Entries),
			rep.Path,
			map[string]string{"size_bytes": fmt.Sprint(rep.SizeBytes)},
		))
	}
	if rep.Encrypted {
		out = append(out, scanner.NewFinding(
			"archive.encrypted", scanner.SeverityLow,
			"Archive is password-protected",
			"The export was encrypted. Have the export password ready; the import will ask for it.",
			rep.Path, nil,
		))
	}
	return out
}
