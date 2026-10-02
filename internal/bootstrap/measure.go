package bootstrap

import (
	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Cost is the measured startup cost of one component.
type Cost struct {
	Component string  `json:"component"` // plugin slug, "theme" or "core"
	Ms        float64 `json:"ms"`
}

// Report holds the baseline timings and the derived cost of each component.
type Report struct {
	AllMs       float64 `json:"all_ms"`        // core + plugins + theme
	NoPluginsMs float64 `json:"no_plugins_ms"` // core + theme
	CoreMs      float64 `json:"core_ms"`       // core only
	Plugins     []Cost  `json:"plugins"`
	ThemeMs     float64 `json:"theme_ms"`
}

// bootOnce runs the boot probe once and returns WordPress's load time in ms.
func bootOnce(r *wpcli.Runner, opts wpcli.Options) (float64, error) {
	var res struct {
		BootMs float64 `json:"boot_ms"`
	}
	err := r.Probe("boot", nil, opts, &res)
	return res.BootMs, err
}

// pairedDiff measures base and variant alternately, runs times, and returns the median of base
// and the median of the per-pair differences (base − variant).
//
// Alternating matters: server load drifts during a scan (other sites, cron, backups). Measuring
// all "with" runs and then all "without" runs bakes that drift into the difference; pairing
// neighbouring runs cancels most of it.
func pairedDiff(r *wpcli.Runner, base, variant wpcli.Options, runs int) (baseMs, diffMs float64, err error) {
	if _, err = bootOnce(r, variant); err != nil { // warm-up for this configuration
		return 0, 0, err
	}
	bases := make([]float64, 0, runs)
	diffs := make([]float64, 0, runs)
	for i := 0; i < runs; i++ {
		b, err := bootOnce(r, base)
		if err != nil {
			return 0, 0, err
		}
		v, err := bootOnce(r, variant)
		if err != nil {
			return 0, 0, err
		}
		bases = append(bases, b)
		diffs = append(diffs, b-v)
	}
	return units.Median(bases), units.Median(diffs), nil
}

// Measure isolates each component's cost with paired runs:
//
//	plugins  = all − (no plugins)
//	theme    = (no plugins) − (no plugins, no theme)
//	plugin X = all − (all with X skipped)
//
// Costs within a few ms of zero (even slightly negative) are noise.
func Measure(r *wpcli.Runner, plugins []string, runs int, progress func(step string)) (*Report, error) {
	step := func(s string) {
		if progress != nil {
			progress(s)
		}
	}
	if runs < 1 {
		runs = 1
	}
	all := wpcli.Options{}
	noPlugins := wpcli.Options{SkipAllPlugins: true}
	coreOnly := wpcli.Options{SkipAllPlugins: true, SkipThemes: true}

	if _, err := bootOnce(r, all); err != nil { // warm-up: fills OPcache
		return nil, err
	}

	var rep Report
	step("plugins vs none")
	allMs, _, err := pairedDiff(r, all, noPlugins, runs)
	if err != nil {
		return nil, err
	}
	step("theme vs core only")
	noPluginsMs, themeMs, err := pairedDiff(r, noPlugins, coreOnly, runs)
	if err != nil {
		return nil, err
	}
	rep.AllMs = allMs
	rep.NoPluginsMs = noPluginsMs
	rep.ThemeMs = themeMs
	rep.CoreMs = noPluginsMs - themeMs

	for _, p := range plugins {
		step("without " + p)
		_, cost, err := pairedDiff(r, all, wpcli.Options{SkipPlugins: []string{p}}, runs)
		if err != nil {
			return nil, err
		}
		rep.Plugins = append(rep.Plugins, Cost{Component: p, Ms: cost})
	}
	return &rep, nil
}
