package frontend

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/units"
)

// Thresholds for one page.
const (
	imagesMediumBytes   = 1 << 20
	imagesHighBytes     = 3 << 20
	fullSizeMin         = 3 // images printed at their original size
	eagerImagesAllowed  = 3 // the first few (above the fold) should not be lazy
	cssMediumWireBytes  = 150 << 10
	jsMediumWireBytes   = 300 << 10
	uncompressedMinSize = 20 << 10
)

// Summary totals a page's assets by kind.
type Summary struct {
	Count     map[string]int   `json:"count"`
	WireBytes map[string]int64 `json:"wire_bytes"`
	RawBytes  map[string]int64 `json:"raw_bytes"`
}

func Summarise(assets []Asset) Summary {
	s := Summary{Count: map[string]int{}, WireBytes: map[string]int64{}, RawBytes: map[string]int64{}}
	for _, a := range assets {
		if a.Error != "" || a.Status >= 400 {
			continue
		}
		s.Count[a.Kind]++
		s.WireBytes[a.Kind] += a.WireBytes
		s.RawBytes[a.Kind] += a.RawBytes
	}
	return s
}

// FindHeavyImages flags pages whose images add up to megabytes.
func FindHeavyImages(page string, assets []Asset) []scanner.Finding {
	var imgs []Asset
	var total int64
	for _, a := range assets {
		if a.Kind == "img" && a.Error == "" && a.Status < 400 {
			imgs = append(imgs, a)
			total += a.WireBytes
		}
	}
	if total < imagesMediumBytes {
		return nil
	}
	sev := scanner.SeverityMedium
	if total >= imagesHighBytes {
		sev = scanner.SeverityHigh
	}
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].WireBytes > imgs[j].WireBytes })
	var top []string
	for _, a := range imgs[:min(5, len(imgs))] {
		top = append(top, fmt.Sprintf("%s (%s)", shortURL(a.URL), units.Bytes(a.WireBytes)))
	}
	return []scanner.Finding{scanner.NewFinding(
		"frontend.heavy_images", sev,
		"Page downloads megabytes of images",
		fmt.Sprintf("%d images, %s in total (measured at the src URL; with srcset a small screen may download less). Use generated image sizes, compress, consider WebP/AVIF.", len(imgs), units.Bytes(total)),
		page,
		map[string]string{"largest": strings.Join(top, "; ")},
	)}
}

// FindImageMarkup flags full-size images and missing lazy loading.
func FindImageMarkup(page string, assets []Asset) []scanner.Finding {
	var out []scanner.Finding
	var full, imgs, eager int
	for _, a := range assets {
		if a.Kind != "img" {
			continue
		}
		imgs++
		if a.FullSize {
			full++
		}
		if !a.Lazy {
			eager++
		}
	}
	if full >= fullSizeMin {
		out = append(out, scanner.NewFinding(
			"frontend.full_size_images", scanner.SeverityMedium,
			"Images printed at their original size",
			fmt.Sprintf("%d of %d images use the 'full' size (class size-full). WordPress already generates smaller versions; ask for 'large' / 'medium_large' in the theme (wp_get_attachment_image, get_the_post_thumbnail) and pass a realistic 'sizes' attribute.", full, imgs),
			page, nil,
		))
	}
	if eager > eagerImagesAllowed {
		out = append(out, scanner.NewFinding(
			"frontend.missing_lazy_loading", scanner.SeverityMedium,
			"Images without lazy loading",
			fmt.Sprintf("%d of %d images load immediately, including those off-screen or in hidden slides. Add loading=\"lazy\" to everything below the first screen.", eager, imgs),
			page, nil,
		))
	}
	return out
}

// FindHeavyCode flags large CSS/JS payloads and assets served without compression.
func FindHeavyCode(page string, assets []Asset) []scanner.Finding {
	var out []scanner.Finding
	s := Summarise(assets)
	if s.WireBytes["css"] >= cssMediumWireBytes {
		out = append(out, scanner.NewFinding(
			"frontend.heavy_css", scanner.SeverityMedium, "Large stylesheets",
			fmt.Sprintf("%d stylesheets, %s transferred (%s uncompressed).", s.Count["css"], units.Bytes(s.WireBytes["css"]), units.Bytes(s.RawBytes["css"])),
			page, nil,
		))
	}
	if s.WireBytes["js"] >= jsMediumWireBytes {
		out = append(out, scanner.NewFinding(
			"frontend.heavy_js", scanner.SeverityMedium, "Large JavaScript",
			fmt.Sprintf("%d scripts, %s transferred (%s uncompressed).", s.Count["js"], units.Bytes(s.WireBytes["js"]), units.Bytes(s.RawBytes["js"])),
			page, nil,
		))
	}
	var plain []string
	for _, a := range assets {
		if (a.Kind == "css" || a.Kind == "js") && a.Error == "" && a.RawBytes >= uncompressedMinSize && a.WireBytes >= a.RawBytes {
			plain = append(plain, shortURL(a.URL))
		}
	}
	if len(plain) > 0 {
		out = append(out, scanner.NewFinding(
			"frontend.uncompressed_assets", scanner.SeverityLow, "CSS/JS served without compression",
			fmt.Sprintf("%d files are sent uncompressed. Enable gzip or brotli on the web server or CDN.", len(plain)),
			page, map[string]string{"files": strings.Join(plain[:min(5, len(plain))], "; ")},
		))
	}
	return out
}

func shortURL(u string) string {
	if i := strings.Index(u, "/wp-content/"); i >= 0 {
		u = u[i:]
	}
	if i := strings.IndexByte(u, '?'); i >= 0 {
		u = u[:i]
	}
	return u
}
