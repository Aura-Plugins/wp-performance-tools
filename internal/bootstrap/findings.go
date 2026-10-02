package bootstrap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
)

// Thresholds in milliseconds of startup per request. Measurement noise is roughly ±5 ms
// (±10 ms on a busy shared server), so keep pluginMediumMs well above it.
const (
	pluginMediumMs = 25
	pluginHighMs   = 75
	themeMediumMs  = 30
	totalMediumMs  = 250
)

// FindExpensiveComponents flags plugins and the theme that add a lot of startup time to every request.
func FindExpensiveComponents(rep *Report, theme string) []scanner.Finding {
	var out []scanner.Finding

	plugins := append([]Cost(nil), rep.Plugins...)
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Ms > plugins[j].Ms })

	for _, p := range plugins {
		if p.Ms < pluginMediumMs {
			continue
		}
		sev := scanner.SeverityMedium
		if p.Ms >= pluginHighMs {
			sev = scanner.SeverityHigh
		}
		out = append(out, scanner.NewFinding(
			"bootstrap.expensive_plugin", sev,
			"Plugin adds noticeable startup time",
			fmt.Sprintf("About %.0f ms is added to every request (front end, admin, AJAX, REST) just by loading this plugin. Check whether all its features are used, or whether a lighter alternative exists.", p.Ms),
			p.Component,
			map[string]string{"ms": fmt.Sprintf("%.1f", p.Ms)},
		))
	}

	if rep.ThemeMs >= themeMediumMs {
		out = append(out, scanner.NewFinding(
			"bootstrap.expensive_theme", scanner.SeverityMedium,
			"Theme adds noticeable startup time",
			fmt.Sprintf("About %.0f ms before any page is built. Look for work done on every request in functions.php or on the init hook (queries, file reads, remote calls) that could be cached or moved to a cron job.", rep.ThemeMs),
			theme,
			map[string]string{"ms": fmt.Sprintf("%.1f", rep.ThemeMs)},
		))
	}

	if rep.AllMs >= totalMediumMs {
		out = append(out, scanner.NewFinding(
			"bootstrap.slow_bootstrap", scanner.SeverityMedium,
			"Slow WordPress startup",
			fmt.Sprintf("%.0f ms to load WordPress before any page is built (core %.0f ms, theme %.0f ms, plugins %.0f ms). Every request pays this.", rep.AllMs, rep.CoreMs, rep.ThemeMs, rep.AllMs-rep.NoPluginsMs),
			"WordPress bootstrap",
			map[string]string{"breakdown": breakdown(rep)},
		))
	}
	return out
}

func breakdown(rep *Report) string {
	parts := []string{fmt.Sprintf("core %.0f ms", rep.CoreMs), fmt.Sprintf("theme %.0f ms", rep.ThemeMs)}
	for _, p := range rep.Plugins {
		parts = append(parts, fmt.Sprintf("%s %.0f ms", p.Component, p.Ms))
	}
	return strings.Join(parts, "; ")
}
