package pages

import (
	"strings"
	"testing"

	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
)

func result(label string, m Metrics, families ...OptionFamily) Result {
	return Result{Page: Page{Label: label, Path: "/" + label}, Runs: 1, Median: m, Sample: Run{Metrics: m, OptionFamilies: families}}
}

func TestHighQueryCountPicksWorstPage(t *testing.T) {
	f := FindHighQueryCount([]Result{
		result("home", Metrics{Queries: 535}),
		result("page", Metrics{Queries: 80}),
	})
	if len(f) != 1 {
		t.Fatalf("want 1 finding, got %d", len(f))
	}
	if f[0].Severity != scanner.SeverityHigh || f[0].Target != "home (/home)" {
		t.Errorf("got severity %s target %q", f[0].Severity, f[0].Target)
	}
}

func TestHighQueryCountQuietBelowThreshold(t *testing.T) {
	if f := FindHighQueryCount([]Result{result("page", Metrics{Queries: queriesMedium - 1})}); len(f) != 0 {
		t.Errorf("want no finding below threshold, got %d", len(f))
	}
}

func TestOptionLookupsDetectsACF(t *testing.T) {
	f := FindRepeatedOptionLookups([]Result{
		result("home", Metrics{OptionLookups: 352}, OptionFamily{Name: "options_nav", Count: 215, Caller: "get_field → get_option"}),
	})
	if len(f) != 1 || f[0].Severity != scanner.SeverityHigh {
		t.Fatalf("want 1 high finding, got %+v", f)
	}
	if f[0].Metadata["called_from"] == "" {
		t.Error("want the calling code in metadata")
	}
	if want := "ACF"; !strings.Contains(f[0].Description, want) {
		t.Errorf("description should mention %s for options_* families: %q", want, f[0].Description)
	}
}

func TestEmptyResultsProduceNoFindings(t *testing.T) {
	for name, fn := range map[string]func([]Result) []scanner.Finding{
		"queries": FindHighQueryCount, "options": FindRepeatedOptionLookups, "n+1": FindNPlusOne,
		"meta": FindLargeMetaLoaded, "slow": FindSlowPages, "slowq": FindSlowQueries,
	} {
		if f := fn(nil); len(f) != 0 {
			t.Errorf("%s: want no findings for no results", name)
		}
	}
}
