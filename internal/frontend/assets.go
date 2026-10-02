package frontend

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Asset is one file the browser downloads for the page.
type Asset struct {
	URL       string `json:"url"`
	Kind      string `json:"kind"` // css | js | img
	Status    int    `json:"status"`
	WireBytes int64  `json:"wire_bytes"` // as transferred (gzip/brotli when the server compresses)
	RawBytes  int64  `json:"raw_bytes"`  // uncompressed
	Lazy      bool   `json:"lazy,omitempty"`
	FullSize  bool   `json:"full_size,omitempty"` // <img class="… size-full …">
	HasSrcset bool   `json:"has_srcset,omitempty"`
	Error     string `json:"error,omitempty"`
}

var (
	tagRe      = regexp.MustCompile(`(?is)<(img|script|link)\b[^>]*>`)
	bgRe       = regexp.MustCompile(`(?i)url\(\s*(?:&quot;|["'])?(https?://[^)"'&\s]+)`)
	attrRes    = map[string]*regexp.Regexp{}
	httpClient = &http.Client{Timeout: 30 * time.Second}
)

func attr(tag, name string) (string, bool) {
	re, ok := attrRes[name]
	if !ok {
		re = regexp.MustCompile(`(?is)\s` + name + `\s*=\s*(?:"([^"]*)"|'([^']*)')`)
		attrRes[name] = re
	}
	m := re.FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return strings.ReplaceAll(m[1]+m[2], "&#038;", "&"), true
}

// ParseAssets extracts stylesheets, scripts and images from rendered HTML, resolved against base.
// It is a regex scan, not a full HTML parser: good enough for WordPress output, which is regular.
func ParseAssets(html string, base *url.URL) []Asset {
	seen := map[string]bool{}
	var out []Asset
	add := func(raw, kind string, a Asset) {
		u, err := base.Parse(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return
		}
		if seen[u.String()] {
			return
		}
		seen[u.String()] = true
		a.URL, a.Kind = u.String(), kind
		out = append(out, a)
	}

	for _, tag := range tagRe.FindAllString(html, -1) {
		switch strings.ToLower(tag[1:4]) {
		case "img":
			src, ok := attr(tag, "src")
			if !ok || strings.HasPrefix(src, "data:") {
				continue
			}
			loading, _ := attr(tag, "loading")
			class, _ := attr(tag, "class")
			_, srcset := attr(tag, "srcset")
			add(src, "img", Asset{
				Lazy:      strings.EqualFold(loading, "lazy"),
				FullSize:  strings.Contains(" "+class+" ", " size-full "),
				HasSrcset: srcset,
			})
		case "scr":
			if src, ok := attr(tag, "src"); ok {
				add(src, "js", Asset{})
			}
		case "lin":
			rel, _ := attr(tag, "rel")
			if href, ok := attr(tag, "href"); ok && strings.Contains(strings.ToLower(rel), "stylesheet") {
				add(href, "css", Asset{})
			}
		}
	}
	for _, m := range bgRe.FindAllStringSubmatch(html, -1) {
		add(m[1], "img", Asset{})
	}
	return out
}

// Fetch downloads an asset and records its size. For CSS/JS it fetches twice (compressed and
// uncompressed) to see whether the server compresses. Setting Accept-Encoding ourselves stops
// Go from transparently decompressing, so the byte count is what travels over the wire.
func Fetch(a *Asset) {
	status, wire, err := download(a.URL, "gzip, br")
	if err != nil {
		a.Error = err.Error()
		return
	}
	a.Status, a.WireBytes, a.RawBytes = status, wire, wire
	if a.Kind == "css" || a.Kind == "js" {
		if _, raw, err := download(a.URL, "identity"); err == nil {
			a.RawBytes = raw
		}
	}
}

func download(u, encoding string) (int, int64, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Accept-Encoding", encoding)
	req.Header.Set("User-Agent", "wp-perf (+asset weight check)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, n, err
}
