package site

import (
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Info is the site inventory every scanner can read: versions, URLs, plugins, cache setup.
type Info struct {
	WPVersion     string   `json:"wp_version"`
	PHPVersion    string   `json:"php_version"`
	DBVersion     string   `json:"db_version"`
	SiteURL       string   `json:"site_url"`
	HomeURL       string   `json:"home_url"`
	TablePrefix   string   `json:"table_prefix"`
	Multisite     bool     `json:"multisite"`
	ActivePlugins []string `json:"active_plugins"`
	Theme         string   `json:"theme"`
	ThemeVersion  string   `json:"theme_version"`
	ObjectCache   bool     `json:"object_cache"`
	Opcache       bool     `json:"opcache"`
	Permalinks    string   `json:"permalinks"`
}

// Load runs the env probe. It is also the connectivity check: if this fails,
// nothing else will work, so callers should stop with its error.
func Load(r *wpcli.Runner) (*Info, error) {
	var info Info
	if err := r.Probe("env", nil, wpcli.Options{}, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// MeasuredPlugins returns the active plugins minus those the target excludes from measurements.
func (i *Info) MeasuredPlugins(skip []string) []string {
	excluded := make(map[string]bool, len(skip))
	for _, s := range skip {
		excluded[s] = true
	}
	var out []string
	for _, p := range i.ActivePlugins {
		if !excluded[p] {
			out = append(out, p)
		}
	}
	return out
}
