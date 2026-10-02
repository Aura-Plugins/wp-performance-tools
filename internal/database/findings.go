package database

import (
	"fmt"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
)

// Thresholds. Change them here; every finding below reads from these.
const (
	metaKeyMinBytes      = 10 << 20  // a single meta key above 10 MB…
	metaKeyMinShare      = 0.25      // …that is also more than 25% of all post meta
	metaKeyHighBytes     = 50 << 20  // High above 50 MB
	autoloadMediumBytes  = 400 << 10 // WordPress Site Health warns at ~800 KB
	autoloadHighBytes    = 800 << 10
	expiredTransientsMin = 500
	orphanMetaMin        = 1000
	revisionsMin         = 5000
	largeTableBytes      = 100 << 20
)

// FindOversizedMetaKeys flags meta keys that dominate wp_postmeta. WordPress loads every meta
// value of a post as soon as any field is read, so a heavy key is paid for on every page that
// touches those posts, even if nothing ever reads it.
func FindOversizedMetaKeys(r *Report) []scanner.Finding {
	var out []scanner.Finding
	if r.MetaTotalBytes == 0 {
		return out
	}
	for _, k := range r.MetaKeys {
		bytes := int64(k.Bytes)
		share := float64(bytes) / float64(r.MetaTotalBytes)
		if bytes < metaKeyMinBytes || share < metaKeyMinShare {
			continue
		}
		sev := scanner.SeverityMedium
		if bytes >= metaKeyHighBytes {
			sev = scanner.SeverityHigh
		}
		out = append(out, scanner.NewFinding(
			"database.oversized_meta_key", sev,
			"One meta key holds most of the post meta",
			fmt.Sprintf("%s across %d rows (%.0f%% of all post meta, avg %s per row). WordPress loads all meta of a post at once, so this is read into memory whenever any field of those posts is used. If it is not needed (e.g. migration leftovers), archive and delete it, or move it to its own table.",
				units.Bytes(bytes), int64(k.RowCount), share*100, units.Bytes(bytes/max(int64(k.RowCount), 1))),
			k.Name,
			map[string]string{"bytes": fmt.Sprint(bytes), "rows": fmt.Sprint(int64(k.RowCount)), "share": fmt.Sprintf("%.2f", share)},
		))
	}
	return out
}

// FindLargeAutoload flags autoloaded options that are read on every request.
func FindLargeAutoload(r *Report) []scanner.Finding {
	bytes := int64(r.Autoload.Bytes)
	if bytes < autoloadMediumBytes {
		return nil
	}
	sev := scanner.SeverityMedium
	if bytes >= autoloadHighBytes {
		sev = scanner.SeverityHigh
	}
	var top []string
	for _, o := range r.AutoloadTop[:min(5, len(r.AutoloadTop))] {
		top = append(top, fmt.Sprintf("%s (%s)", o.Name, units.Bytes(int64(o.Bytes))))
	}
	return []scanner.Finding{scanner.NewFinding(
		"database.large_autoload", sev,
		"Autoloaded options are large",
		fmt.Sprintf("%s in %d options is loaded on every request. Set autoload off for big options that are only needed on some pages.", units.Bytes(bytes), int64(r.Autoload.OptionCount)),
		"wp_options",
		map[string]string{"largest": strings.Join(top, "; ")},
	)}
}

// FindClutter flags low-impact housekeeping: expired transients, orphaned meta, revisions.
func FindClutter(r *Report) []scanner.Finding {
	var out []scanner.Finding
	if r.ExpiredTimeouts >= expiredTransientsMin {
		out = append(out, scanner.NewFinding(
			"database.expired_transients", scanner.SeverityLow,
			"Many expired transients",
			fmt.Sprintf("%d expired transients are still stored (%d transient rows in total). Usually means WP-Cron is not running cleanup; `wp transient delete --expired` removes them.", r.ExpiredTimeouts, r.Transients),
			"wp_options", nil,
		))
	}
	if r.OrphanMeta >= orphanMetaMin {
		out = append(out, scanner.NewFinding(
			"database.orphaned_postmeta", scanner.SeverityLow,
			"Post meta without a post",
			fmt.Sprintf("%d meta rows belong to posts that no longer exist.", r.OrphanMeta),
			"wp_postmeta", nil,
		))
	}
	var revisions int64
	for _, p := range r.Posts {
		if p.Type == "revision" {
			revisions += int64(p.Count)
		}
	}
	if revisions >= revisionsMin {
		out = append(out, scanner.NewFinding(
			"database.many_revisions", scanner.SeverityLow,
			"Many post revisions",
			fmt.Sprintf("%d revisions stored. Consider limiting WP_POST_REVISIONS.", revisions),
			"wp_posts", nil,
		))
	}
	return out
}

// FindLargeTables flags big tables other than the core ones already analysed in detail.
func FindLargeTables(r *Report, prefix string) []scanner.Finding {
	core := map[string]bool{}
	for _, t := range []string{"posts", "postmeta", "options", "comments", "commentmeta", "terms", "termmeta", "term_taxonomy", "term_relationships", "users", "usermeta", "links"} {
		core[prefix+t] = true
	}
	var out []scanner.Finding
	for _, t := range r.Tables {
		if core[t.Name] || t.Bytes() < largeTableBytes {
			continue
		}
		out = append(out, scanner.NewFinding(
			"database.large_table", scanner.SeverityMedium,
			"Large non-core table",
			fmt.Sprintf("%s, about %d rows. Usually a plugin's log or cache table; check whether the plugin is still used and whether it can prune old rows.", units.Bytes(t.Bytes()), int64(t.RowCount)),
			t.Name, nil,
		))
	}
	return out
}

// FindNoObjectCache notes the absence of a persistent object cache.
func FindNoObjectCache(objectCache bool) []scanner.Finding {
	if objectCache {
		return nil
	}
	return []scanner.Finding{scanner.NewFinding(
		"database.no_object_cache", scanner.SeverityLow,
		"No persistent object cache",
		"Every request rebuilds options, posts and meta from the database. A persistent object cache (Redis/Memcached) helps most on sites with many logged-in users; fix repeated queries first.",
		"object cache", nil,
	)}
}
