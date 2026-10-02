package frontend

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/pages"
	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/site"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// Scanner renders a page through WP-CLI, then downloads every asset in it over HTTP
// from this machine to weigh what a browser would fetch.
// After Scan, Assets holds the per-file detail.
type Scanner struct {
	Runner *wpcli.Runner
	Info   *site.Info
	User   string
	Path   string // page to analyse; default "/"

	Assets []Asset
}

func (s *Scanner) Name() string { return "frontend" }

func (s *Scanner) Scan() ([]scanner.Finding, error) {
	path := s.Path
	if path == "" {
		path = "/"
	}

	run, err := pages.RenderOnce(s.Runner, s.User, path, true)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(run.HTML)
	if err != nil {
		return nil, fmt.Errorf("decoding rendered HTML: %w", err)
	}
	html := s.rewriteBase(string(raw))

	base, err := url.Parse(s.baseURL())
	if err != nil {
		return nil, fmt.Errorf("site URL %q: %w", s.baseURL(), err)
	}

	s.Assets = ParseAssets(html, base)
	for i := range s.Assets {
		Fetch(&s.Assets[i])
	}

	label := "Page " + path
	var findings []scanner.Finding
	findings = append(findings, FindHeavyImages(label, s.Assets)...)
	findings = append(findings, FindImageMarkup(label, s.Assets)...)
	findings = append(findings, FindHeavyCode(label, s.Assets)...)
	return findings, nil
}

// baseURL is where assets are fetched from: the target's URL override, else the site's home URL.
func (s *Scanner) baseURL() string {
	if s.Runner.Target.URL != "" {
		return strings.TrimRight(s.Runner.Target.URL, "/") + "/"
	}
	return s.Info.HomeURL
}

// rewriteBase swaps the site's own URL for the target URL override (e.g. when the copy is
// reachable at a different address than the one stored in the database).
func (s *Scanner) rewriteBase(html string) string {
	override := strings.TrimRight(s.Runner.Target.URL, "/")
	home := strings.TrimRight(s.Info.HomeURL, "/")
	if override == "" || override == home {
		return html
	}
	return strings.ReplaceAll(html, home, override)
}
