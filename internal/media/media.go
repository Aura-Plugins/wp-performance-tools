package media

import (
	"fmt"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Thresholds.
const (
	oversizedMediumCount = 20       // originals above 1 MB
	outsideMinBytes      = 50 << 20 // files in uploads that are not in the media library
)

// Report is the decoded result of the media probe.
type Report struct {
	UploadsDir      string      `json:"uploads_dir"`
	TotalBytes      units.Int   `json:"total_bytes"`
	TotalFiles      units.Int   `json:"total_files"`
	LibraryItems    units.Int   `json:"library_items"`
	ByExtension     []FileGroup `json:"by_extension"`
	OversizedCount  units.Int   `json:"oversized_count"`
	OversizedBytes  units.Int   `json:"oversized_bytes"`
	OversizedTop    []FileGroup `json:"oversized_top"`
	OutsideFiles    units.Int   `json:"outside_files"`
	OutsideBytes    units.Int   `json:"outside_bytes"`
	OutsideByFolder []FileGroup `json:"outside_by_folder"`
}

// FileGroup is a file or a group of files (by extension or folder) with its size.
type FileGroup struct {
	Name  string    `json:"name"`
	Files units.Int `json:"files,omitempty"`
	Bytes units.Int `json:"bytes"`
}

// Scanner walks the uploads folder. Read-only. After Scan, Report holds the raw numbers.
type Scanner struct {
	Runner *wpcli.Runner
	Report *Report
}

func (s *Scanner) Name() string { return "media" }

func (s *Scanner) Scan() ([]scanner.Finding, error) {
	var r Report
	if err := s.Runner.Probe("media", nil, wpcli.Options{}, &r); err != nil {
		return nil, err
	}
	s.Report = &r

	var findings []scanner.Finding
	findings = append(findings, FindOversizedOriginals(&r)...)
	findings = append(findings, FindFilesOutsideLibrary(&r)...)
	return findings, nil
}

// FindOversizedOriginals flags library images whose original file is above 1 MB.
// They matter when the theme outputs the original instead of a generated size.
func FindOversizedOriginals(r *Report) []scanner.Finding {
	if int64(r.OversizedCount) < oversizedMediumCount {
		return nil
	}
	return []scanner.Finding{scanner.NewFinding(
		"media.oversized_originals", scanner.SeverityMedium,
		"Many images uploaded above 1 MB",
		fmt.Sprintf("%d library images are larger than 1 MB (%s in total). Harmless if the theme always uses generated sizes; costly if it outputs originals (see frontend.full_size_images). Consider resizing/compressing on upload.",
			int64(r.OversizedCount), units.Bytes(int64(r.OversizedBytes))),
		r.UploadsDir,
		map[string]string{"largest": groupList(r.OversizedTop, 5)},
	)}
}

// FindFilesOutsideLibrary flags uploads content no attachment points to (old plugins, imports, orphans).
func FindFilesOutsideLibrary(r *Report) []scanner.Finding {
	if int64(r.OutsideBytes) < outsideMinBytes {
		return nil
	}
	return []scanner.Finding{scanner.NewFinding(
		"media.files_outside_library", scanner.SeverityLow,
		"Files in uploads that are not in the media library",
		fmt.Sprintf("%d files (%s) are not attached to any media item: often leftovers from removed plugins, old imports or caches. They make backups bigger but do not slow pages down.",
			int64(r.OutsideFiles), units.Bytes(int64(r.OutsideBytes))),
		r.UploadsDir,
		map[string]string{"by_folder": groupList(r.OutsideByFolder, 8)},
	)}
}

func groupList(groups []FileGroup, n int) string {
	var parts []string
	for _, g := range groups[:min(n, len(groups))] {
		parts = append(parts, fmt.Sprintf("%s (%s)", g.Name, units.Bytes(int64(g.Bytes))))
	}
	return strings.Join(parts, "; ")
}
