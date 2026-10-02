package pages

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
)

// Thresholds, per page (median). A well-built WordPress page usually makes 30–100 queries.
const (
	queriesMedium       = 150
	queriesHigh         = 300
	optionLookupsMedium = 50
	optionLookupsHigh   = 150
	postLoadsMedium     = 20
	metaLoadsMedium     = 20
	metaKBMedium        = 1024
	metaKBHigh          = 2048
	totalMsMedium       = 400 // server-dependent: compare relative to the same server
	totalMsHigh         = 800
	memMBMedium         = 128
	slowQueryMs         = 50
)

// worst returns the result with the highest value of metric, and a "Label: value" list of all pages.
func worst(results []Result, metric func(Metrics) float64) (Result, string) {
	sorted := append([]Result(nil), results...)
	sort.Slice(sorted, func(i, j int) bool { return metric(sorted[i].Median) > metric(sorted[j].Median) })
	var parts []string
	for _, r := range sorted {
		parts = append(parts, fmt.Sprintf("%s: %.0f", r.Page.Label, metric(r.Median)))
	}
	return sorted[0], strings.Join(parts, "; ")
}

func target(r Result) string {
	if r.Page.Label == r.Page.Path {
		return r.Page.Path
	}
	return fmt.Sprintf("%s (%s)", r.Page.Label, r.Page.Path)
}

// FindHighQueryCount flags pages that make too many database queries.
func FindHighQueryCount(results []Result) []scanner.Finding {
	if len(results) == 0 {
		return nil
	}
	w, all := worst(results, func(m Metrics) float64 { return m.Queries })
	if w.Median.Queries < queriesMedium {
		return nil
	}
	sev := scanner.SeverityMedium
	if w.Median.Queries >= queriesHigh {
		sev = scanner.SeverityHigh
	}
	var patterns []string
	for _, p := range w.Sample.QueryPatterns[:min(3, len(w.Sample.QueryPatterns))] {
		patterns = append(patterns, fmt.Sprintf("%d× %s", p.Count, p.Name))
	}
	return []scanner.Finding{scanner.NewFinding(
		"pages.high_query_count", sev,
		"Pages make too many database queries",
		fmt.Sprintf("Up to %.0f queries per page (a well-built page usually needs 30–100). The most repeated query shapes are listed below; many identical shapes usually means something is loaded one item at a time.", w.Median.Queries),
		target(w),
		map[string]string{"per_page": all, "top_patterns": strings.Join(patterns, " | ")},
	)}
}

// FindRepeatedOptionLookups flags options read one by one (= not autoloaded), grouped into families.
func FindRepeatedOptionLookups(results []Result) []scanner.Finding {
	if len(results) == 0 {
		return nil
	}
	w, all := worst(results, func(m Metrics) float64 { return m.OptionLookups })
	if w.Median.OptionLookups < optionLookupsMedium {
		return nil
	}
	sev := scanner.SeverityMedium
	if w.Median.OptionLookups >= optionLookupsHigh {
		sev = scanner.SeverityHigh
	}
	var fams []string
	acf := false
	for _, f := range w.Sample.OptionFamilies {
		fams = append(fams, fmt.Sprintf("%s* (%d)", f.Name, f.Count))
		if strings.HasPrefix(f.Name, "options_") {
			acf = true
		}
	}
	desc := fmt.Sprintf("Up to %.0f options are fetched from the database one query at a time on every page, because they are not autoloaded.", w.Median.OptionLookups)
	if acf {
		desc += " Names starting with options_ are ACF options-page fields (often menus built with repeaters): enable autoload on that options page, or cache the rendered output."
	} else {
		desc += " Autoload the ones every page needs, or cache what is built from them."
	}
	meta := map[string]string{"per_page": all, "families": strings.Join(fams, "; ")}
	if len(w.Sample.OptionFamilies) > 0 && w.Sample.OptionFamilies[0].Caller != "" {
		meta["called_from"] = w.Sample.OptionFamilies[0].Caller
	}
	return []scanner.Finding{scanner.NewFinding("pages.repeated_option_lookups", sev, "Options read one query at a time", desc, target(w), meta)}
}

// FindNPlusOne flags posts and post meta loaded one item at a time.
func FindNPlusOne(results []Result) []scanner.Finding {
	var out []scanner.Finding
	if len(results) == 0 {
		return out
	}
	if w, all := worst(results, func(m Metrics) float64 { return m.PostLoads }); w.Median.PostLoads >= postLoadsMedium {
		out = append(out, scanner.NewFinding(
			"pages.repeated_post_loads", scanner.SeverityMedium,
			"Posts loaded one at a time",
			fmt.Sprintf("%.0f separate `SELECT * FROM posts WHERE ID = …` queries. Typical cause: a query with 'fields' => 'ids' followed by get_post() per item. Query the posts directly, or prime the cache with _prime_post_caches().", w.Median.PostLoads),
			target(w), map[string]string{"per_page": all},
		))
	}
	if w, all := worst(results, func(m Metrics) float64 { return m.MetaLoads }); w.Median.MetaLoads >= metaLoadsMedium {
		out = append(out, scanner.NewFinding(
			"pages.repeated_meta_loads", scanner.SeverityMedium,
			"Post meta loaded one post at a time",
			fmt.Sprintf("%.0f separate meta queries for single posts. Load the posts with a normal WP_Query (it primes meta in one query) or call update_meta_cache('post', $ids).", w.Median.MetaLoads),
			target(w), map[string]string{"per_page": all},
		))
	}
	return out
}

// FindLargeMetaLoaded flags pages that pull megabytes of post meta into memory.
func FindLargeMetaLoaded(results []Result) []scanner.Finding {
	if len(results) == 0 {
		return nil
	}
	w, all := worst(results, func(m Metrics) float64 { return m.MetaKB })
	if w.Median.MetaKB < metaKBMedium {
		return nil
	}
	sev := scanner.SeverityMedium
	if w.Median.MetaKB >= metaKBHigh {
		sev = scanner.SeverityHigh
	}
	var keys []string
	for _, k := range w.Sample.TopMetaKeys {
		keys = append(keys, fmt.Sprintf("%s (%s)", k.Name, units.Bytes(int64(k.Bytes))))
	}
	return []scanner.Finding{scanner.NewFinding(
		"pages.large_meta_loaded", sev,
		"Pages load megabytes of post meta",
		fmt.Sprintf("Up to %s of post meta for %.0f posts is read into memory per page. WordPress loads every meta value of a post when any field is read; the heaviest keys are listed. If a heavy key is never used, removing it fixes this.", units.Bytes(int64(w.Median.MetaKB*1024)), w.Median.MetaPosts),
		target(w),
		map[string]string{"per_page_kb": all, "heaviest_keys": strings.Join(keys, "; ")},
	)}
}

// FindSlowPages flags slow generation time and high memory. Absolute times depend on the server.
func FindSlowPages(results []Result) []scanner.Finding {
	var out []scanner.Finding
	if len(results) == 0 {
		return out
	}
	if w, all := worst(results, func(m Metrics) float64 { return m.TotalMs }); w.Median.TotalMs >= totalMsMedium {
		sev := scanner.SeverityMedium
		if w.Median.TotalMs >= totalMsHigh {
			sev = scanner.SeverityHigh
		}
		out = append(out, scanner.NewFinding(
			"pages.slow_generation", sev,
			"Slow page generation",
			fmt.Sprintf("Up to %.0f ms to build a page on the server (%.0f ms bootstrap + %.0f ms render). Absolute times depend on the server; use the other findings to see why.", w.Median.TotalMs, w.Median.BootstrapMs, w.Median.RenderMs),
			target(w), map[string]string{"per_page_ms": all},
		))
	}
	if w, all := worst(results, func(m Metrics) float64 { return m.PeakMemMB }); w.Median.PeakMemMB >= memMBMedium {
		out = append(out, scanner.NewFinding(
			"pages.high_memory", scanner.SeverityMedium,
			"High memory per request",
			fmt.Sprintf("Up to %.0f MB of PHP memory per page. Limits how many requests the server can handle at once.", w.Median.PeakMemMB),
			target(w), map[string]string{"per_page_mb": all},
		))
	}
	return out
}

// FindSlowQueries flags individual queries above slowQueryMs.
func FindSlowQueries(results []Result) []scanner.Finding {
	var out []scanner.Finding
	seen := map[string]bool{}
	for _, r := range results {
		for _, q := range r.Sample.Slowest {
			if q.Ms < slowQueryMs || seen[q.SQL] {
				continue
			}
			seen[q.SQL] = true
			out = append(out, scanner.NewFinding(
				"pages.slow_query", scanner.SeverityMedium,
				"Slow database query",
				fmt.Sprintf("%.0f ms for a single query. Check for missing indexes, LIKE '%%…%%' searches or meta_query on large tables.", q.Ms),
				target(r), map[string]string{"sql": q.SQL},
			))
		}
	}
	return out
}
