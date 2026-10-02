# Changelog

All notable changes to WP Performance Tools are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

---

## [0.1.0]

### Added

- **`scan`** — runs five scanners against a WordPress site and reports findings grouped High → Medium → Low. Exits `1` when findings are present.
  - `database` — oversized meta keys, autoload size, large non-core tables, expired transients, orphaned meta, revisions, missing object cache.
  - `media` — uploads size, originals above 1 MB, files not attached to the media library (grouped by folder).
  - `bootstrap` — startup cost of WordPress core, the theme and each plugin, measured with paired runs.
  - `pages` — benchmarks auto-discovered or given pages (median of N runs): time, queries, one-at-a-time option/post/meta loads, post meta loaded into memory, slow queries.
  - `frontend` — weighs every image, stylesheet and script on a page over HTTP; flags heavy images, full-size images, missing lazy loading, uncompressed assets.
- **`bench`** — page benchmark only, with the measurement table.
- **`check-archive`** — validates an All-in-One WP Migration `.wpress` file without extracting it; supports both end-block formats (zeros, and empty name + content size in newer AI1WM versions). Strips secrets from the printed manifest.
- **`targets`** — lists targets from `~/.wp-perf/config.json` (override with `WP_PERF_CONFIG`).
- **Targets** reachable locally, over SSH (system `ssh`, honours `~/.ssh/config`), inside Docker, or SSH + Docker.
- **OPcache-on measurements** by default, so CLI timings resemble PHP under a web server (`--no-opcache` to disable).
- **Production guard** — targets not marked `is_copy` prompt `[Y/n]` before rendering pages; only an uppercase `Y` proceeds. `--confirm` skips the prompt.
- **`--json`** on every command.
- **`--wp-binary`** for sites whose container has no `wp` on the PATH (e.g. a phar in `/tmp`).
- **Release tooling** — GitHub Actions release on `v*` tags (5 platforms + checksums), `install.sh`, Homebrew formula.
