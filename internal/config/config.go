package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Target describes where a WordPress site lives and how to reach its WP-CLI.
//
// The three ways of reaching a site combine freely:
//   - nothing set            → run `wp` on this machine (e.g. inside Local's "Open site shell")
//   - SSH set                → run on a remote host via the system `ssh` (honours ~/.ssh/config aliases)
//   - DockerContainer set    → run inside that container with `docker exec` (locally or on the SSH host)
type Target struct {
	SSH             string   `json:"ssh,omitempty"`              // ssh host or alias, e.g. "my-vps"
	DockerContainer string   `json:"docker_container,omitempty"` // e.g. "mysite-wp"
	DockerUser      string   `json:"docker_user,omitempty"`      // default "www-data"
	WPPath          string   `json:"wp_path,omitempty"`          // passed as --path; empty = WP-CLI's default
	WPBinary        string   `json:"wp_binary,omitempty"`        // default "wp"
	PHPBinary       string   `json:"php_binary,omitempty"`       // default "php"; used to enable OPcache for WP-CLI
	User            string   `json:"user,omitempty"`             // WordPress login used to render pages (needed behind login walls)
	URL             string   `json:"url,omitempty"`              // public base URL for fetching assets, if it differs from home_url
	IsCopy          bool     `json:"is_copy"`                    // true = a copy/sandbox; skips the production warning
	SkipPlugins     []string `json:"skip_plugins,omitempty"`     // plugins excluded from every measurement (e.g. query-monitor)
	Opcache         *bool    `json:"opcache,omitempty"`          // default true: measure with OPcache on, like PHP under a web server
}

// OpcacheEnabled reports whether measurements should run with OPcache (default: yes).
func (t Target) OpcacheEnabled() bool {
	return t.Opcache == nil || *t.Opcache
}

// Describe returns a short human label for where the target lives.
func (t Target) Describe() string {
	where := "local"
	if t.SSH != "" {
		where = "ssh:" + t.SSH
	}
	if t.DockerContainer != "" {
		where += " → docker:" + t.DockerContainer
	}
	return where
}

type Config struct {
	Targets map[string]Target `json:"targets"`
}

// DefaultPath is ~/.wp-perf/config.json, or $WP_PERF_CONFIG when set.
func DefaultPath() (string, error) {
	if p := os.Getenv("WP_PERF_CONFIG"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wp-perf", "config.json"), nil
}

// Load reads the config file. A missing file is not an error: it returns an empty config,
// so the tool works with ad-hoc flags alone.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Targets: map[string]Target{}}, nil
		}
		return nil, err
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if c.Targets == nil {
		c.Targets = map[string]Target{}
	}
	return &c, nil
}

// TargetNames returns the configured target names, sorted.
func (c *Config) TargetNames() []string {
	names := make([]string, 0, len(c.Targets))
	for n := range c.Targets {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
