package database

import (
	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/site"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Scanner analyses the database: heavy meta keys, autoload, large tables and clutter. Read-only.
type Scanner struct {
	Runner *wpcli.Runner
	Info   *site.Info
}

func (s *Scanner) Name() string { return "database" }

func (s *Scanner) Scan() ([]scanner.Finding, error) {
	var r Report
	if err := s.Runner.Probe("database", nil, wpcli.Options{}, &r); err != nil {
		return nil, err
	}

	var findings []scanner.Finding
	findings = append(findings, FindOversizedMetaKeys(&r)...)
	findings = append(findings, FindLargeAutoload(&r)...)
	findings = append(findings, FindLargeTables(&r, s.Info.TablePrefix)...)
	findings = append(findings, FindClutter(&r)...)
	findings = append(findings, FindNoObjectCache(s.Info.ObjectCache)...)
	return findings, nil
}
