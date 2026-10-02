package output

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/archive"
	"github.com/Aura-Plugins/wp-performance-tools/internal/bootstrap"
	"github.com/Aura-Plugins/wp-performance-tools/internal/pages"
	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/site"
	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
)

// SiteHeader prints the site identity line shown above every scan or bench.
func SiteHeader(info *site.Info, where string) {
	cache := "no object cache"
	if info.ObjectCache {
		cache = "object cache"
	}
	tuiHeader("WP Performance Tools",
		info.HomeURL+"  ·  "+where,
		fmt.Sprintf("WordPress %s  ·  PHP %s  ·  %d plugins  ·  theme %s %s  ·  %s",
			info.WPVersion, info.PHPVersion, len(info.ActivePlugins), info.Theme, info.ThemeVersion, cache),
	)
}

// PrintFindings renders all findings in the TUI style, grouped High → Medium → Low.
func PrintFindings(findings []scanner.Finding) {
	var high, medium, low int
	for _, f := range findings {
		switch f.Severity {
		case scanner.SeverityHigh:
			high++
		case scanner.SeverityMedium:
			medium++
		case scanner.SeverityLow:
			low++
		}
	}

	tuiSummary(len(findings), high, medium, low)
	if len(findings) == 0 {
		return
	}

	for _, sev := range []scanner.Severity{
		scanner.SeverityHigh,
		scanner.SeverityMedium,
		scanner.SeverityLow,
	} {
		var group []scanner.Finding
		for _, f := range findings {
			if f.Severity == sev {
				group = append(group, f)
			}
		}
		if len(group) == 0 {
			continue
		}
		tuiSectionHeader(strings.ToUpper(string(sev)), sev)
		for _, f := range group {
			tuiFinding(f)
		}
	}

	tuiDivider("")
	fmt.Println()
}

// PrintBench renders the per-page measurement table (medians).
func PrintBench(results []pages.Result) {
	if len(results) == 0 {
		return
	}
	fmt.Println()
	tuiDivider(fmt.Sprintf("pages · median of %d runs", results[0].Runs))
	fmt.Println()
	fmt.Printf("  %s\n", col(fmt.Sprintf("%-28s %7s %7s %7s %8s %8s %9s %7s", "page", "total", "boot", "render", "queries", "opt.1x1", "meta", "mem"), ansiGray))
	for _, r := range results {
		label := r.Page.Label
		if len(label) > 28 {
			label = label[:27] + "…"
		}
		fmt.Printf("  %-28s %5.0fms %5.0fms %5.0fms %8.0f %8.0f %9s %5.0fMB\n",
			label, r.Median.TotalMs, r.Median.BootstrapMs, r.Median.RenderMs,
			r.Median.Queries, r.Median.OptionLookups, units.Bytes(int64(r.Median.MetaKB*1024)), r.Median.PeakMemMB)
	}
	fmt.Println()
	fmt.Printf("  %s\n", col("opt.1x1 = options fetched one query at a time · meta = post meta loaded into memory", ansiDim))
}

// PrintBootstrap renders startup cost per component.
func PrintBootstrap(rep *bootstrap.Report, theme string) {
	if rep == nil {
		return
	}
	plugins := append([]bootstrap.Cost(nil), rep.Plugins...)
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Ms > plugins[j].Ms })

	rows := [][2]string{
		{"total (core + theme + plugins)", ms(rep.AllMs)},
		{"WordPress core", ms(rep.CoreMs)},
		{"theme " + theme, ms(rep.ThemeMs)},
	}
	for _, p := range plugins {
		rows = append(rows, [2]string{p.Component, ms(p.Ms)})
	}
	width := 0
	for _, r := range rows {
		width = max(width, len(r[0]))
	}

	fmt.Println()
	tuiDivider("startup cost per request")
	fmt.Println()
	for _, r := range rows {
		fmt.Printf("  %-*s %8s\n", width, r[0], r[1])
	}
	fmt.Printf("\n  %s\n", col("Per-component costs are paired differences; ±5 ms is noise (±10 ms on a busy server).", ansiDim))
}

// ms formats milliseconds, showing values that round to zero as "0 ms" (never "-0 ms").
func ms(v float64) string {
	if v > -0.5 && v < 0.5 {
		v = 0
	}
	return fmt.Sprintf("%.0f ms", v)
}

// PrintArchive renders an archive check.
func PrintArchive(rep *archive.Report) {
	status := col("✓ complete", ansiBoldGreen)
	if !rep.Complete {
		status = col("✕ incomplete", ansiBoldRed)
	}
	tuiHeader("WP Performance Tools · archive check", rep.Path)
	fmt.Printf("\n  %s   %d entries   %s\n", status, rep.Entries, units.Bytes(rep.SizeBytes))
	if rep.Manifest != nil {
		fmt.Println()
		for _, k := range []string{"SiteURL", "WordPress", "PHP", "Database", "Template", "Plugins"} {
			if v, ok := rep.Manifest[k]; ok {
				fmt.Printf("  %s %s\n", col(fmt.Sprintf("%-10s", k), ansiGray), compact(v))
			}
		}
	}
}

func compact(v any) string {
	switch t := v.(type) {
	case map[string]any:
		if ver, ok := t["Version"]; ok {
			return fmt.Sprint(ver)
		}
	case []any:
		parts := make([]string, len(t))
		for i, p := range t {
			parts[i] = fmt.Sprint(p)
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
