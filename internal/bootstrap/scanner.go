package bootstrap

import (
	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/site"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Scanner measures how much startup time WordPress core, the theme and each plugin cost.
// After Scan, Report holds the raw timings.
type Scanner struct {
	Runner   *wpcli.Runner
	Info     *site.Info
	Runs     int
	Progress func(step string)

	Report *Report
}

func (s *Scanner) Name() string { return "bootstrap" }

func (s *Scanner) Scan() ([]scanner.Finding, error) {
	plugins := s.Info.MeasuredPlugins(s.Runner.Target.SkipPlugins)
	rep, err := Measure(s.Runner, plugins, s.Runs, s.Progress)
	if err != nil {
		return nil, err
	}
	s.Report = rep
	return FindExpensiveComponents(rep, s.Info.Theme), nil
}
