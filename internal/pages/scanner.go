package pages

import (
	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Scanner benchmarks representative pages and turns the measurements into findings.
// After Scan, Results holds the raw measurements (used by the `bench` command and JSON output).
type Scanner struct {
	Runner   *wpcli.Runner
	User     string   // WordPress user to render as ("" = logged out)
	Paths    []string // explicit paths; empty = auto-discover
	MaxPages int
	Runs     int
	Progress func(i, total int, p Page)

	Results []Result
}

func (s *Scanner) Name() string { return "pages" }

func (s *Scanner) Scan() ([]scanner.Finding, error) {
	list, err := s.pages()
	if err != nil {
		return nil, err
	}

	s.Results, err = Benchmark(s.Runner, s.User, list, s.Runs, s.Progress)
	if err != nil {
		return nil, err
	}

	var findings []scanner.Finding
	findings = append(findings, FindHighQueryCount(s.Results)...)
	findings = append(findings, FindRepeatedOptionLookups(s.Results)...)
	findings = append(findings, FindNPlusOne(s.Results)...)
	findings = append(findings, FindLargeMetaLoaded(s.Results)...)
	findings = append(findings, FindSlowPages(s.Results)...)
	findings = append(findings, FindSlowQueries(s.Results)...)
	return findings, nil
}

func (s *Scanner) pages() ([]Page, error) {
	if len(s.Paths) == 0 {
		return Discover(s.Runner, s.MaxPages)
	}
	list := make([]Page, len(s.Paths))
	for i, p := range s.Paths {
		list[i] = Page{Label: p, Path: p}
	}
	return list, nil
}
