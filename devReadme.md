# WP Performance Tools — Developer Reference

Architecture, design decisions, and instructions for extending the tool.
The layout deliberately follows aura-devshield: `cmd/` + `internal/`, a `Scanner` interface, a shared `Finding` type, and output kept separate from analysis.

---

## Repository layout

```
WP-performance-tools/
├── cmd/
│   └── wp-perf/
│       └── main.go              # CLI entry point — subcommands, flags, target resolution, scanner loop
├── internal/
│   ├── scanner/
│   │   ├── finding.go           # Finding struct + Severity enum + NewFinding()
│   │   ├── fingerprint.go       # SHA256 fingerprint (ID + target + path)
│   │   └── scanner.go           # Scanner interface
│   ├── config/
│   │   └── config.go            # ~/.wp-perf/config.json: Target definitions
│   ├── wpcli/
│   │   ├── runner.go            # Runs WP-CLI locally / over SSH / in Docker; Probe() runs embedded PHP
│   │   └── shell.go             # Shell quoting helpers
│   ├── probe/
│   │   ├── probe.go             # go:embed of php/*.php + result-marker extraction
│   │   └── php/
│   │       ├── env.php          # site inventory
│   │       ├── database.php     # table sizes, meta keys, autoload, clutter
│   │       ├── media.php        # uploads walk
│   │       ├── discover.php     # representative URLs
│   │       ├── bench.php        # render one page and measure it
│   │       └── boot.php         # WordPress load time
│   ├── site/
│   │   └── site.go              # site.Info (decoded env probe), shared by scanners
│   ├── database/                # Scanner: types.go, findings.go, scanner.go
│   ├── media/                   # Scanner: media.go
│   ├── bootstrap/               # Scanner: measure.go (paired runs), findings.go, scanner.go
│   ├── pages/                   # Scanner: types.go, bench.go (discover, runs, medians), findings.go, scanner.go
│   ├── frontend/                # Scanner: assets.go (parse + HTTP fetch), findings.go, scanner.go
│   ├── archive/                 # .wpress checker: wpress.go, findings.go (no target needed)
│   ├── units/
│   │   └── units.go             # Bytes(), Int (JSON number-or-string), Median()
│   └── output/
│       ├── tui.go               # ANSI primitives, TTY detection, layout helpers
│       ├── terminal.go          # Human-readable output (findings, bench table, startup costs, archive)
│       └── json.go              # JSON output
├── config.example.json
├── CHANGELOG.md
├── devReadme.md                 # this file
├── functional-drawing.md
├── Makefile
└── README.md
```

---

## How it works, in one paragraph

`main.go` resolves a **target** (config entry and/or flags) into a `config.Target`, builds a `wpcli.Runner` for it, and loads `site.Info` through the `env` probe, which doubles as the connectivity check. It then runs each selected **scanner**. A scanner asks the runner to execute one or more **probes**: PHP files embedded in the binary that are piped into `wp eval-file -` on the site's machine. Each probe prints a marker line followed by JSON. The scanner decodes the JSON into Go structs and passes them through pure **finding functions** that compare numbers with thresholds and return `[]scanner.Finding`. `output` prints findings and raw measurements as a terminal report or JSON.

---

## Core data types

### `scanner.Finding`

The universal output unit. Every scanner produces `[]scanner.Finding`.

```go
type Finding struct {
    ID          string            // namespaced: "pages.repeated_option_lookups"
    Fingerprint string            // SHA256 of ID + Target + Path
    Severity    Severity          // SeverityLow | SeverityMedium | SeverityHigh
    Title       string            // short human label
    Description string            // what was measured + how to fix it
    Target      string            // what it points at (page, plugin, table, file)
    Path        string            // filesystem path, if applicable
    Metadata    map[string]string // the numbers behind it (per_page, heaviest_keys, …)
}
```

**Always build findings with `scanner.NewFinding(...)`**, which fills in the fingerprint. **ID convention:** `<scanner>.<finding_type>`, snake_case. **Always use the severity constants**, never string literals.

### `scanner.Scanner`

```go
type Scanner interface {
    Name() string
    Scan() ([]Finding, error)
}
```

Scanners receive their dependencies (`*wpcli.Runner`, `*site.Info`, options) as struct fields. Scanners that produce raw measurements keep them on the struct after `Scan()` (`pages.Scanner.Results`, `bootstrap.Scanner.Report`, `media.Scanner.Report`, `frontend.Scanner.Assets`) so `main.go` can print or export them. That keeps `Scan()`'s signature uniform.

### `config.Target`

Where a site lives. The combinations: nothing set (local `wp`), `SSH`, `DockerContainer`, or both. `IsCopy` disables the production prompt. `SkipPlugins` are excluded from every measurement (use it for Query Monitor or migration plugins that do not exist in production).

### Probes

A probe is a PHP file in `internal/probe/php/`. Rules:

1. It runs **inside WordPress** (WP-CLI has already loaded it), so use `$wpdb`, `get_option()` and so on directly. Never shell out to `mysql`, because many containers do not have it.
2. It reads its arguments from `$args` (positional args after `wp eval-file -`).
3. It ends with exactly one `echo "\n@@WPPERF@@" . wp_json_encode( $out ) . "\n";`. Anything printed before that is ignored.
4. Lists go out as **arrays of objects** (`[{name, bytes}]`), not associative arrays, so order is preserved in JSON.
5. Read-only unless the probe's whole purpose is to render (bench). Rendering probes must say so in their header comment.
6. Use `{$wpdb->prefix}` / `{$wpdb->postmeta}` and so on, never a hard-coded `wp_`.

---

## Data flow

```
main() — subcommand routing
  │
  ├── scan / bench
  │     ├── selectScanners(--only)           validate before touching the network
  │     ├── prepare()
  │     │     ├── loadConfig()               ~/.wp-perf/config.json (or $WP_PERF_CONFIG)
  │     │     ├── resolveTarget()            config entry + flag overrides → config.Target
  │     │     ├── production guard           prompt unless is_copy / --copy / --confirm
  │     │     └── site.Load(runner)          env probe = connectivity check
  │     ├── for each scanner: Scan()         errors become warnings, other scanners still run
  │     │     └── runner.Probe(name, args, opts, &out)
  │     │           └── [ssh host] [docker exec -i] sh -c '<wp …> eval-file -'  ← PHP on stdin
  │     └── output.SiteHeader / PrintBench / PrintBootstrap / PrintFindings   or   output.PrintJSON
  │
  └── check-archive → archive.Check(file) → archive.Findings → output
```

---

## Adding a new scanner

Example: a `cron` scanner that flags overdue or very frequent WP-Cron events.

### Step 1: create the probe

`internal/probe/php/cron.php`:

```php
<?php
/*
 * cron — scheduled events: how many, how overdue, how frequent.
 * Args: none. Read-only.
 */
$events = array();
foreach ( (array) _get_cron_array() as $ts => $hooks ) {
	foreach ( $hooks as $hook => $instances ) {
		foreach ( $instances as $e ) {
			$events[] = array( 'hook' => $hook, 'next' => (int) $ts, 'interval' => isset( $e['interval'] ) ? (int) $e['interval'] : 0 );
		}
	}
}
echo "\n@@WPPERF@@" . wp_json_encode( array( 'now' => time(), 'events' => $events ) ) . "\n";
```

The `go:embed php/*.php` directive picks it up automatically.

### Step 2: create the package

```
internal/cron/
├── types.go       # Report struct matching the probe's JSON
├── findings.go    # thresholds (constants) + pure Find…(report) []scanner.Finding functions
└── scanner.go     # implements scanner.Scanner
```

### Step 3: implement `scanner.Scanner`

```go
package cron

type Scanner struct {
    Runner *wpcli.Runner
}

func (s *Scanner) Name() string { return "cron" }

func (s *Scanner) Scan() ([]scanner.Finding, error) {
    var r Report
    if err := s.Runner.Probe("cron", nil, wpcli.Options{}, &r); err != nil {
        return nil, err
    }
    return FindOverdueEvents(&r), nil
}
```

Keep the finding functions **pure** (report in, findings out). That's what makes them unit-testable without a WordPress site. See `internal/pages/findings_test.go`.

### Step 4: wire it into `main.go`

1. Add `"cron"` to `allScanners` (order = run order; read-only scanners first).
2. Add it to the `byName` map in `runScan`.
3. If it keeps raw measurements on the struct, add them to the JSON report map and, if useful, a print function in `internal/output/terminal.go`.

### Step 5: document

Add the finding IDs to the table in `README.md` and an entry to `CHANGELOG.md`.

---

## Changing thresholds

Every threshold is a named constant at the top of the scanner's `findings.go`, with a comment. Change the number, rebuild, and run `go test ./...`; tests that pin a threshold boundary will tell you if you changed behaviour they rely on.

---

## Design decisions and rationale

### Standalone CLI, not a WordPress plugin

- **No observer effect.** A plugin is part of what it measures and loads in sequence with the others. From outside, `--skip-plugins` isolates each plugin's cost.
- **Simulation before change.** WP-CLI's `--exec` runs code before WordPress loads, which lets you test "what if" fixes in memory.
- **Controlled conditions.** Fresh process per run, warm-up discarded, medians.
- **Nothing installed on client sites.** No code footprint and no exposed SQL.
- **Outside PHP's web limits.** Walking 40,000 uploads or checking a 6 GB archive doesn't hit `max_execution_time`.

### Zero external dependencies

Standard library only (`os/exec`, `net/http`, `encoding/json`, `regexp`, `embed`, `flag`…). Single static binary, nothing to audit. SSH uses the system `ssh` binary rather than a Go SSH library, so `~/.ssh/config`, agents and jump hosts all just work.

**Rule:** do not add a dependency without a compelling reason. HTML is parsed with regexes in `frontend/assets.go`, which is adequate for WordPress's regular output. If that ever proves insufficient, `golang.org/x/net/html` is the one dependency worth considering.

### Probes embedded, piped via stdin

`wp eval-file -` reads PHP from stdin, so nothing is written to the server and there's no cleanup step. The probes ship inside the binary, so the PHP and the Go that decodes it can't drift apart.

### OPcache on by default

PHP-CLI disables OPcache, so every WP-CLI run would recompile WordPress and look 3–5× slower than under a web server. The runner launches the WP-CLI phar through `php -d opcache.enable_cli=1 -d opcache.file_cache=/tmp/wp-perf-opcache …`, which keeps compiled scripts between runs. It only does this when `wp` really is a PHP script; shell wrappers are run as-is.

### Medians and paired runs

Each page or startup configuration gets one warm-up run (discarded) plus N measured runs, and the median is reported. For startup costs, the "with" and "without" configurations are **alternated** and the cost is the median of the per-pair differences. On a shared server, load drifts during a scan, and measuring all "with" runs before all "without" runs bakes that drift into the result (this produced costs of −22 ms before pairing was introduced).

### Production guard

Rendering runs the site's own code. Targets not marked `is_copy` get a `[Y/n]` prompt where **only an uppercase `Y` proceeds** (the same convention as aura-devshield). `--confirm` exists for scripts.

### Exit codes

`scan` exits `1` when there are findings and `0` when clean, so it can gate CI. `check-archive` exits `1` for an incomplete archive. `bench` always exits `0` when it runs: it measures, it doesn't gate.

### Import dependency rules

The import graph must stay acyclic and layered:

```
main
  → output    → scanner, site, pages, bootstrap, archive, units
  → database, media, bootstrap, pages, frontend
              → wpcli, site, scanner, units          (frontend → pages for RenderOnce)
  → archive   → scanner
  → site      → wpcli
  → wpcli     → config, probe
  → config, probe, scanner, units   (no internal imports)
```

Scanner packages must not import `output`. `config`, `probe`, `scanner` and `units` must not import any other internal package.

---

## Development workflow

```bash
go build ./...                 # compile
go vet ./...                   # run before every commit
go test ./...                  # unit tests (no WordPress needed)
make build                     # ./wp-perf with version stamped from git
```

Run against the sandbox copy:

```bash
go run ./cmd/wp-perf scan --ssh my-vps --container mysite-wp --user admin --copy \
  --skip-plugins=query-monitor,all-in-one-wp-migration,all-in-one-wp-migration-unlimited-extension
```

Try a probe by hand (handy while writing a new one):

```bash
ssh my-vps 'docker exec -i -u www-data mysite-wp wp eval-file -' < internal/probe/php/env.php
```

Use a throwaway config without touching `~/.wp-perf`:

```bash
WP_PERF_CONFIG=./config.example.json go run ./cmd/wp-perf targets
```

---

## Distribution

| Deliverable | What it does |
|---|---|
| `Makefile` | `make build`, `make build-all` (5 targets into `dist/`), `make install`, `make vet`, `make test` |
| `.github/workflows/release.yml` | On a `v*` tag: vet → test → cross-compile → `checksums.txt` → GitHub Release |
| `install.sh` | Detects OS/arch, downloads the latest release, verifies SHA256, installs to `/usr/local/bin` |
| `Formula/wp-perf.rb` | Binary Homebrew formula, published in `github.com/Aura-Plugins/homebrew-tap` |
| `--version` | `wp-perf version` prints the tag injected at build time |

### Tagging a new release

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
# GitHub Actions builds and publishes the release (~1 min). Then update the formula:
gh release download vX.Y.Z -R Aura-Plugins/wp-performance-tools -p checksums.txt -O - 
# 1. Formula/wp-perf.rb: set version "X.Y.Z" and the four sha256 values from checksums.txt
# 2. Commit it here, and copy it to Formula/wp-perf.rb in Aura-Plugins/homebrew-tap
```

---

## Ideas / roadmap

| Idea | Notes |
|---|---|
| `compare a.json b.json` | Diff two `--json` reports by fingerprint: fixed / new / changed. Ideal for before-and-after evidence. |
| `simulate` | Generalise the in-memory fix simulation used in the first real-world analysis (`--exec` hooks that remove an action, preload options, or filter a query), with presets. |
| `cron` scanner | Overdue and very frequent WP-Cron events. See "Adding a new scanner". |
| Report export | `--html report.html` for non-technical readers. |
