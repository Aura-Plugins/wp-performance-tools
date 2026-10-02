package pages

// Page is one URL to benchmark.
type Page struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

// Metrics are the numeric measurements of one render. Every field is median-able.
type Metrics struct {
	TotalMs       float64 `json:"total_ms"`
	BootstrapMs   float64 `json:"bootstrap_ms"`
	RenderMs      float64 `json:"render_ms"`
	Queries       float64 `json:"queries"`
	QueryMs       float64 `json:"query_ms"`
	PeakMemMB     float64 `json:"peak_mem_mb"`
	HTMLKB        float64 `json:"html_kb"`
	MetaPosts     float64 `json:"meta_posts"`
	MetaKB        float64 `json:"meta_kb"`
	OptionLookups float64 `json:"option_lookups"`
	PostLoads     float64 `json:"post_loads"`
	MetaLoads     float64 `json:"meta_loads"`
}

// Run is the decoded result of one bench probe execution.
type Run struct {
	Metrics
	Path           string         `json:"path"`
	Status         int            `json:"status"`
	Template       string         `json:"template"`
	Redirect       string         `json:"redirect"`
	MetaTracked    bool           `json:"meta_tracked"`
	TopMetaKeys    []NamedBytes   `json:"top_meta_keys"`
	OptionFamilies []OptionFamily `json:"option_families"`
	QueryPatterns  []NamedCount   `json:"query_patterns"`
	Slowest        []SlowQuery    `json:"slowest"`
	HTML           string         `json:"html,omitempty"` // base64, only when requested
}

type NamedBytes struct {
	Name  string  `json:"name"`
	Bytes float64 `json:"bytes"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type OptionFamily struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Caller string `json:"caller"`
}

type SlowQuery struct {
	Ms  float64 `json:"ms"`
	SQL string  `json:"sql"`
}

// Result is a benchmarked page: median metrics over all measured runs, plus the detail
// (patterns, slowest queries…) from the first measured run.
type Result struct {
	Page   Page    `json:"page"`
	Runs   int     `json:"runs"`
	Median Metrics `json:"median"`
	Sample Run     `json:"sample"`
}
