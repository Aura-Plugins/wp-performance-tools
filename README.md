# WP Performance Tools

A standalone command-line tool that measures why a WordPress site is slow, and shows what to fix.

It runs from your machine, reaches the site through WP-CLI (locally, over SSH, inside Docker, or both), and runs small read-only PHP probes inside the site's own WordPress. **Nothing is installed on the site.** No plugin, no files left behind.

---

## What it checks

| Scanner | What it measures | Typical findings |
|---|---|---|
| `database` | Table sizes, heaviest post-meta keys, autoloaded options, clutter | One meta key holding most of `wp_postmeta`; autoload above 800 KB; expired transients |
| `media` | Uploads folder size, file types, originals above 1 MB, files outside the media library | Hundreds of MB of leftovers from removed plugins; oversized uploads |
| `bootstrap` | Startup time of WordPress core, the theme and each plugin | A plugin adding 40 ms to every request |
| `pages` | Renders representative pages N times: time, queries, repeated-query patterns, memory, post meta loaded | Options read one query at a time (e.g. ACF option-page menus); posts loaded one by one (N+1); megabytes of meta loaded per page |
| `frontend` | Downloads every image, stylesheet and script of a page | 8 MB of images on the home page; images printed at full size; no lazy loading |

There is also `check-archive`, which validates an All-in-One WP Migration `.wpress` backup **before** you spend 10 minutes uploading it.

---

## Requirements

- **On the site's machine:** WP-CLI (`wp`), and shell access to it: SSH, `docker exec`, or a local shell such as Local's *Open site shell*. If a container has no WP-CLI, download the phar to `/tmp` and point to it with `--wp-binary /tmp/wp-cli.phar` (see **Tips**).
- **To build from source:** Go 1.26+.

Works on any host where you can run WP-CLI. Most shared hosts with SSH qualify; for hosts without a shell, copy the site to Local or a sandbox first. That is the recommended way anyway (see **Safety**).

---

## Install

### Homebrew (macOS and Linux)

```bash
brew tap aura-plugins/tap
brew install wp-perf
```

### Script (macOS and Linux)

```bash
curl -sSfL https://raw.githubusercontent.com/Aura-Plugins/wp-performance-tools/main/install.sh | bash
```

Installs to `/usr/local/bin` and verifies the SHA256 checksum. Override the directory with `INSTALL_DIR=~/.local/bin`.

### Download a binary

From [Releases](https://github.com/Aura-Plugins/wp-performance-tools/releases): `wp-perf-darwin-arm64`, `-darwin-amd64`, `-linux-amd64`, `-linux-arm64`, `-windows-amd64.exe`.

### From source

```bash
git clone https://github.com/Aura-Plugins/wp-performance-tools && cd wp-performance-tools
make install
```

---

## Quick start

**1. Check a backup before importing it**

```bash
wp-perf check-archive ~/Downloads/site-backup.wpress
```

**2. Scan a site** (described with flags):

```bash
wp-perf scan --ssh my-vps --container mysite-wp --user admin --copy
```

**3. Or save it as a target** in `~/.wp-perf/config.json` (see `config.example.json`) and use its name:

```bash
wp-perf scan mysite
wp-perf bench mysite --url / --url /news/
wp-perf scan mysite --only pages,frontend --json > reports/mysite.report.json
```

---

## Commands

```
wp-perf scan  [target] [flags]     All scanners; exits 1 when there are findings
wp-perf bench [target] [flags]     Page benchmark table only
wp-perf check-archive <file>       Validate a .wpress file (exits 1 if incomplete)
wp-perf targets                    List configured targets
wp-perf version
```

### Target flags (scan, bench)

| Flag | Meaning |
|---|---|
| `--ssh host` | SSH host or `~/.ssh/config` alias (key-based; password prompts are disabled) |
| `--container name` | Docker container running WordPress |
| `--docker-user u` | User inside the container (default `www-data`) |
| `--path /dir` | WordPress path (WP-CLI `--path`) |
| `--wp-binary cmd` | WP-CLI command or path on the site's machine (default `wp`) |
| `--user login` | Render pages as this WordPress user (needed behind login walls) |
| `--site-url url` | Base URL to download assets from, if different from the site's `home_url` |
| `--skip-plugins a,b` | Exclude plugins from all measurements (e.g. `query-monitor`) |
| `--copy` | Declare the target a copy; skips the production warning |
| `--no-opcache` | Measure without OPcache (not representative of a web server) |

### Scan flags

| Flag | Default | Meaning |
|---|---|---|
| `--only a,b` | all | Scanners to run: `database,media,bootstrap,pages,frontend` |
| `--runs N` | 5 | Measured runs per page / per startup configuration (median reported) |
| `--max-pages N` | 8 | Cap on auto-discovered pages |
| `--url /path` | auto | Benchmark these pages instead (repeatable) |
| `--frontend-page /path` | `/` | Page whose assets are weighed |
| `--json` | | Full report as JSON (site info, findings and all raw measurements) |
| `--confirm` | | Skip the production prompt |

---

## Config file

`~/.wp-perf/config.json` (or the path in `$WP_PERF_CONFIG`):

```json
{
  "targets": {
    "mysite": {
      "ssh": "my-vps",
      "docker_container": "mysite-wp",
      "user": "admin",
      "is_copy": true,
      "skip_plugins": ["query-monitor", "all-in-one-wp-migration"]
    }
  }
}
```

Flags given on the command line override the saved target.

---

## Safety

- **Read-only analysis:** `database`, `media`, `check-archive` and the inventory only read.
- **Rendering runs the site's code:** `pages`, `frontend` and `bootstrap` load WordPress and render pages, exactly as a visit would. Anything the site does on page load (sending emails, running cron, writing to the database) will happen. **Run against a copy, not production.** Targets not marked `is_copy` ask for confirmation first.
- **Logged-in rendering** uses WP-CLI's `--user`, so no passwords or cookies are ever created or sent.
- **No telemetry.** The only network traffic is to your site (SSH, and HTTP to download its assets).

---

## Tips

**Container without WP-CLI** (e.g. the official `wordpress` image): put the phar in `/tmp` for the duration of the scan. It is outside the site files and disappears on the next redeploy.

```bash
ssh my-vps 'docker exec -u www-data mysite-wp sh -c "curl -sSL -o /tmp/wp-cli.phar https://raw.githubusercontent.com/wp-cli/builds/gh-pages/phar/wp-cli.phar && chmod +x /tmp/wp-cli.phar"'
wp-perf scan --ssh my-vps --container mysite-wp --wp-binary /tmp/wp-cli.phar --copy
ssh my-vps 'docker exec mysite-wp rm -rf /tmp/wp-cli.phar /tmp/wp-perf-opcache'
```

**Before and after a fix:** save `--json` reports and compare them; finding fingerprints stay stable across runs.

---

## Reading the numbers

- **Times are server-side** (no browser, no network). Absolute milliseconds depend on the server; compare runs on the same machine. Query counts, repeated-query patterns and data volumes do not depend on the server.
- **Medians of N runs**, after a warm-up run that is discarded. Startup costs per plugin are *paired differences*, so values within ±5 ms of zero (±10 ms on a busy server) are noise.
- **Images are measured at their `src` URL.** With `srcset`, a small screen may download a smaller file.
- **The front end covers weight, not rendering.** For Core Web Vitals, add Lighthouse or PageSpeed Insights.

---

## Finding IDs

| ID | Severity | Meaning |
|---|---|---|
| `database.oversized_meta_key` | Medium/High | One meta key > 10 MB and > 25% of all post meta |
| `database.large_autoload` | Medium/High | Autoloaded options > 400 KB / 800 KB |
| `database.large_table` | Medium | Non-core table > 100 MB |
| `database.expired_transients` · `orphaned_postmeta` · `many_revisions` | Low | Housekeeping |
| `database.no_object_cache` | Low | No persistent object cache |
| `media.oversized_originals` | Medium | ≥ 20 library images above 1 MB |
| `media.files_outside_library` | Low | > 50 MB in uploads not attached to media items |
| `bootstrap.expensive_plugin` | Medium/High | Plugin adds ≥ 25 / 75 ms per request |
| `bootstrap.expensive_theme` · `slow_bootstrap` | Medium | Theme ≥ 30 ms; total ≥ 250 ms |
| `pages.high_query_count` | Medium/High | ≥ 150 / 300 queries on a page |
| `pages.repeated_option_lookups` | Medium/High | ≥ 50 / 150 options read one query at a time |
| `pages.repeated_post_loads` · `repeated_meta_loads` | Medium | ≥ 20 posts / meta loaded one at a time |
| `pages.large_meta_loaded` | Medium/High | ≥ 1 / 2 MB of post meta loaded per page |
| `pages.slow_generation` · `high_memory` · `slow_query` | Medium | ≥ 400 ms; ≥ 128 MB; a query ≥ 50 ms |
| `frontend.heavy_images` | Medium/High | ≥ 1 / 3 MB of images on the page |
| `frontend.full_size_images` · `missing_lazy_loading` | Medium | ≥ 3 full-size images; > 3 images without lazy loading |
| `frontend.heavy_css` · `heavy_js` | Medium | ≥ 150 KB CSS / 300 KB JS transferred |
| `frontend.uncompressed_assets` | Low | CSS/JS served without gzip/brotli |
| `archive.corrupted` | High | `.wpress` file incomplete |
| `archive.encrypted` | Low | `.wpress` is password-protected |

Thresholds are constants at the top of each scanner's `findings.go`. See `devReadme.md`.
