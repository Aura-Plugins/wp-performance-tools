package pages

import (
	"fmt"

	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// saveQueries makes $wpdb record every query; it must be defined before WordPress loads.
const saveQueries = "define('SAVEQUERIES', true);"

// Discover asks the site for representative URLs (home, latest of each public post type, search),
// capped at max pages.
func Discover(r *wpcli.Runner, max int) ([]Page, error) {
	var res struct {
		Pages []Page `json:"pages"`
	}
	if err := r.Probe("discover", nil, wpcli.Options{}, &res); err != nil {
		return nil, err
	}
	if max > 0 && len(res.Pages) > max {
		// Keep Search (last) even when capping.
		search := res.Pages[len(res.Pages)-1]
		res.Pages = append(res.Pages[:max-1], search)
	}
	return res.Pages, nil
}

// RenderOnce renders a page once. html=true also returns the page HTML (base64 in Run.HTML).
func RenderOnce(r *wpcli.Runner, user, path string, html bool) (Run, error) {
	args := []string{path}
	if html {
		args = append(args, "html")
	}
	var run Run
	err := r.Probe("bench", args, wpcli.Options{User: user, Exec: saveQueries}, &run)
	return run, err
}

// Benchmark renders each page warmup+runs times and keeps the median of the measured runs.
// The warm-up run fills OPcache and is discarded. progress (may be nil) is called before each page.
func Benchmark(r *wpcli.Runner, user string, list []Page, runs int, progress func(i, total int, p Page)) ([]Result, error) {
	if runs < 1 {
		runs = 1
	}
	var results []Result
	for i, p := range list {
		if progress != nil {
			progress(i+1, len(list), p)
		}
		if _, err := RenderOnce(r, user, p.Path, false); err != nil { // warm-up
			return results, fmt.Errorf("%s (%s): %w", p.Label, p.Path, err)
		}

		var measured []Run
		for n := 0; n < runs; n++ {
			run, err := RenderOnce(r, user, p.Path, false)
			if err != nil {
				return results, fmt.Errorf("%s (%s): %w", p.Label, p.Path, err)
			}
			measured = append(measured, run)
		}
		results = append(results, Result{Page: p, Runs: runs, Median: medianOf(measured), Sample: measured[0]})
	}
	return results, nil
}

// medianOf takes the median of every metric independently.
func medianOf(runs []Run) Metrics {
	pick := func(f func(Metrics) float64) float64 {
		v := make([]float64, len(runs))
		for i, r := range runs {
			v[i] = f(r.Metrics)
		}
		return units.Median(v)
	}
	return Metrics{
		TotalMs:       pick(func(m Metrics) float64 { return m.TotalMs }),
		BootstrapMs:   pick(func(m Metrics) float64 { return m.BootstrapMs }),
		RenderMs:      pick(func(m Metrics) float64 { return m.RenderMs }),
		Queries:       pick(func(m Metrics) float64 { return m.Queries }),
		QueryMs:       pick(func(m Metrics) float64 { return m.QueryMs }),
		PeakMemMB:     pick(func(m Metrics) float64 { return m.PeakMemMB }),
		HTMLKB:        pick(func(m Metrics) float64 { return m.HTMLKB }),
		MetaPosts:     pick(func(m Metrics) float64 { return m.MetaPosts }),
		MetaKB:        pick(func(m Metrics) float64 { return m.MetaKB }),
		OptionLookups: pick(func(m Metrics) float64 { return m.OptionLookups }),
		PostLoads:     pick(func(m Metrics) float64 { return m.PostLoads }),
		MetaLoads:     pick(func(m Metrics) float64 { return m.MetaLoads }),
	}
}
