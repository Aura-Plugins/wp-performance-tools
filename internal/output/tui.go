package output

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
)

// ANSI escape codes — only applied when stdout is a real terminal.
const (
	ansiReset      = "\033[0m"
	ansiBold       = "\033[1m"
	ansiDim        = "\033[2m"
	ansiYellow     = "\033[33m"
	ansiGray       = "\033[90m"
	ansiBoldRed    = "\033[1;31m"
	ansiBoldYellow = "\033[1;33m"
	ansiBoldCyan   = "\033[1;36m"
	ansiBoldGreen  = "\033[1;32m"
)

var (
	ttyOnce sync.Once
	ttyVal  bool
)

// isTTY reports whether stdout is an interactive terminal.
// Cached after first call so we only stat once per run.
func isTTY() bool {
	ttyOnce.Do(func() {
		fi, err := os.Stdout.Stat()
		ttyVal = err == nil && fi.Mode()&os.ModeCharDevice != 0
	})
	return ttyVal
}

// col wraps s in an ANSI code when writing to a terminal.
// When output is piped, returns s unchanged so files stay clean.
func col(s, code string) string {
	if !isTTY() {
		return s
	}
	return code + s + ansiReset
}

func sevColor(sev scanner.Severity) string {
	switch sev {
	case scanner.SeverityHigh:
		return ansiBoldRed
	case scanner.SeverityMedium:
		return ansiBoldYellow
	default:
		return ansiBoldCyan
	}
}

func sevIcon(sev scanner.Severity) string {
	switch sev {
	case scanner.SeverityHigh:
		return "✕"
	case scanner.SeverityMedium:
		return "▲"
	default:
		return "●"
	}
}

// ── Layout primitives ─────────────────────────────────────────────────────────

func tuiHeader(title string, lines ...string) {
	fmt.Println()
	fmt.Printf("  %s\n", col(title, ansiBold))
	for _, l := range lines {
		fmt.Printf("  %s\n", col(l, ansiGray))
	}
	fmt.Printf("  %s\n", col(strings.Repeat("─", 64), ansiGray))
}

func tuiSummary(total, high, medium, low int) {
	fmt.Println()
	if total == 0 {
		fmt.Printf("  %s\n\n", col("✓  No findings", ansiBoldGreen))
		return
	}
	line := col(fmt.Sprintf("%d findings", total), ansiBold)
	if high > 0 {
		line += "   " + col(fmt.Sprintf("✕ %d high", high), ansiBoldRed)
	}
	if medium > 0 {
		line += "   " + col(fmt.Sprintf("▲ %d medium", medium), ansiBoldYellow)
	}
	if low > 0 {
		line += "   " + col(fmt.Sprintf("● %d low", low), ansiBoldCyan)
	}
	fmt.Printf("  %s\n", line)
}

func tuiSectionHeader(label string, sev scanner.Severity) {
	fmt.Printf("\n  %s %s %s\n\n",
		col("──", ansiGray),
		col(label, sevColor(sev)),
		col(strings.Repeat("─", 54), ansiGray),
	)
}

// tuiFinding renders a single finding with indent, icon, and colour.
func tuiFinding(f scanner.Finding) {
	icon := sevIcon(f.Severity)
	color := sevColor(f.Severity)

	fmt.Printf("  %s  %s  %s\n", col(icon, color), col(f.Title, ansiBold), col(f.ID, ansiGray))
	fmt.Printf("     %s\n", col(f.Target, ansiYellow))

	// Path only when it differs from target (avoids repeating paths twice).
	if f.Path != "" && f.Path != f.Target {
		fmt.Printf("     %s\n", col(f.Path, ansiDim))
	}

	fmt.Printf("     %s\n", f.Description)

	for _, k := range sortedKeys(f.Metadata) {
		fmt.Printf("     %s %s\n", col(k+":", ansiGray), col(f.Metadata[k], ansiGray))
	}

	fmt.Println()
}

func tuiDivider(label string) {
	if label == "" {
		fmt.Printf("  %s\n", col(strings.Repeat("─", 64), ansiGray))
	} else {
		fmt.Printf("  %s %s %s\n",
			col("──", ansiGray),
			col(label, ansiGray),
			col(strings.Repeat("─", max(2, 60-len(label))), ansiGray),
		)
	}
}
