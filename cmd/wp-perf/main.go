package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/archive"
	"github.com/Aura-Plugins/wp-performance-tools/internal/bootstrap"
	"github.com/Aura-Plugins/wp-performance-tools/internal/config"
	"github.com/Aura-Plugins/wp-performance-tools/internal/database"
	"github.com/Aura-Plugins/wp-performance-tools/internal/frontend"
	"github.com/Aura-Plugins/wp-performance-tools/internal/media"
	"github.com/Aura-Plugins/wp-performance-tools/internal/output"
	"github.com/Aura-Plugins/wp-performance-tools/internal/pages"
	"github.com/Aura-Plugins/wp-performance-tools/internal/scanner"
	"github.com/Aura-Plugins/wp-performance-tools/internal/site"
	"github.com/Aura-Plugins/wp-performance-tools/internal/wpcli"
)

// version is set at build time via -ldflags "-X main.version=v1.0.0".
var version = "dev"

const usage = `Usage: wp-perf <command> [target] [flags]

  scan  [target]            Run all scanners and report findings
  bench [target]            Benchmark pages only and print the measurement table
  check-archive <file>      Check an All-in-One WP Migration .wpress file is complete
  targets                   List targets in ~/.wp-perf/config.json
  version                   Print the version

[target] is a name from ~/.wp-perf/config.json, or describe one with flags:
  --ssh host  --container name  --path /var/www/html  --user admin  --copy

Run "wp-perf <command> -h" for all flags of a command.
`

// allScanners is the order scanners run in: cheap and read-only first.
var allScanners = []string{"database", "media", "bootstrap", "pages", "frontend"}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch args[0] {
	case "scan":
		runScan(args[1:])
	case "bench":
		runBench(args[1:])
	case "check-archive":
		runCheckArchive(args[1:])
	case "targets":
		runTargets()
	case "version", "--version", "-version":
		fmt.Println("wp-perf", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n%s", args[0], usage)
		os.Exit(1)
	}
}

// runScan runs every selected scanner against a target and prints findings.
func runScan(args []string) {
	flags := flag.NewFlagSet("scan", flag.ExitOnError)
	tf := addTargetFlags(flags)
	jsonOutput := flags.Bool("json", false, "Output the full report as JSON")
	only := flags.String("only", "", "Comma-separated scanners to run: "+strings.Join(allScanners, ","))
	runs := flags.Int("runs", 5, "Measured runs per page / per startup configuration (median is reported)")
	maxPages := flags.Int("max-pages", 8, "Maximum auto-discovered pages to benchmark")
	frontPage := flags.String("frontend-page", "/", "Page whose assets the frontend scanner weighs")
	var urls stringList
	flags.Var(&urls, "url", "Page path to benchmark instead of auto-discovery (repeatable), e.g. --url / --url /news/")
	confirm := flags.Bool("confirm", false, "Skip the production warning prompt")
	positional := parseInterspersed(flags, args)

	selected, err := selectScanners(*only)
	if err != nil {
		fail(err)
	}

	ss := prepare(positional, tf, *confirm, *jsonOutput)

	pg := &pages.Scanner{Runner: ss.runner, User: ss.target.User, Paths: urls, MaxPages: *maxPages, Runs: *runs}
	bs := &bootstrap.Scanner{Runner: ss.runner, Info: ss.info, Runs: *runs}
	md := &media.Scanner{Runner: ss.runner}
	fe := &frontend.Scanner{Runner: ss.runner, Info: ss.info, User: ss.target.User, Path: *frontPage}
	if !*jsonOutput {
		pg.Progress = func(i, total int, p pages.Page) { progress("pages %d/%d  %s", i, total, p.Label) }
		bs.Progress = func(step string) { progress("startup  %s", step) }
	}

	byName := map[string]scanner.Scanner{
		"database":  &database.Scanner{Runner: ss.runner, Info: ss.info},
		"media":     md,
		"bootstrap": bs,
		"pages":     pg,
		"frontend":  fe,
	}

	var findings []scanner.Finding
	for _, name := range selected {
		s := byName[name]
		if !*jsonOutput {
			progress("%s…", s.Name())
		}
		f, err := s.Scan()
		if err != nil {
			clearProgress()
			fmt.Fprintf(os.Stderr, "Warning: %s scan: %v\n", s.Name(), err)
			continue
		}
		findings = append(findings, f...)
	}
	clearProgress()

	if *jsonOutput {
		report := map[string]any{
			"site":            ss.info,
			"target":          ss.name,
			"findings":        nonNil(findings),
			"pages":           pg.Results,
			"bootstrap":       bs.Report,
			"media":           md.Report,
			"frontend_assets": fe.Assets,
		}
		if err := output.PrintJSON(report); err != nil {
			fail(err)
		}
	} else {
		output.SiteHeader(ss.info, ss.target.Describe())
		output.PrintBench(pg.Results)
		output.PrintBootstrap(bs.Report, ss.info.Theme)
		output.PrintFindings(findings)
	}

	if len(findings) > 0 {
		os.Exit(1)
	}
}

// runBench benchmarks pages only and prints the measurement table.
func runBench(args []string) {
	flags := flag.NewFlagSet("bench", flag.ExitOnError)
	tf := addTargetFlags(flags)
	jsonOutput := flags.Bool("json", false, "Output results as JSON")
	runs := flags.Int("runs", 5, "Measured runs per page (median is reported)")
	maxPages := flags.Int("max-pages", 8, "Maximum auto-discovered pages")
	var urls stringList
	flags.Var(&urls, "url", "Page path to benchmark (repeatable); default: auto-discover")
	confirm := flags.Bool("confirm", false, "Skip the production warning prompt")
	positional := parseInterspersed(flags, args)

	ss := prepare(positional, tf, *confirm, *jsonOutput)

	pg := &pages.Scanner{Runner: ss.runner, User: ss.target.User, Paths: urls, MaxPages: *maxPages, Runs: *runs}
	if !*jsonOutput {
		pg.Progress = func(i, total int, p pages.Page) { progress("pages %d/%d  %s", i, total, p.Label) }
	}
	findings, err := pg.Scan()
	clearProgress()
	if err != nil {
		fail(err)
	}

	if *jsonOutput {
		if err := output.PrintJSON(map[string]any{"site": ss.info, "pages": pg.Results, "findings": nonNil(findings)}); err != nil {
			fail(err)
		}
		return
	}
	output.SiteHeader(ss.info, ss.target.Describe())
	output.PrintBench(pg.Results)
	output.PrintFindings(findings)
}

// runCheckArchive checks a .wpress file locally; no target needed.
func runCheckArchive(args []string) {
	flags := flag.NewFlagSet("check-archive", flag.ExitOnError)
	jsonOutput := flags.Bool("json", false, "Output the result as JSON")
	positional := parseInterspersed(flags, args)
	if len(positional) != 1 {
		fail(fmt.Errorf("usage: wp-perf check-archive <file.wpress> [--json]"))
	}

	rep, err := archive.Check(positional[0])
	if err != nil {
		fail(err)
	}
	findings := archive.Findings(rep)

	if *jsonOutput {
		if err := output.PrintJSON(map[string]any{"archive": rep, "findings": nonNil(findings)}); err != nil {
			fail(err)
		}
	} else {
		output.PrintArchive(rep)
		output.PrintFindings(findings)
	}
	if !rep.Complete {
		os.Exit(1)
	}
}

func runTargets() {
	cfgPath, cfg := loadConfig()
	fmt.Printf("\n  %s\n\n", cfgPath)
	if len(cfg.Targets) == 0 {
		fmt.Println("  No targets yet. See config.example.json in the repository.")
		fmt.Println()
		return
	}
	for _, name := range cfg.TargetNames() {
		t := cfg.Targets[name]
		kind := "PRODUCTION?"
		if t.IsCopy {
			kind = "copy"
		}
		fmt.Printf("  %-24s %-44s %s\n", name, t.Describe(), kind)
	}
	fmt.Println()
}

// ── shared setup ─────────────────────────────────────────────────────────────

// scanState is everything a command needs once the target is resolved and reachable.
type scanState struct {
	name   string
	target config.Target
	runner *wpcli.Runner
	info   *site.Info
}

// prepare resolves the target, applies the production guard, and checks connectivity
// by loading the site inventory. It exits on any failure.
func prepare(positional []string, tf *targetFlags, confirm, quiet bool) *scanState {
	_, cfg := loadConfig()
	name, t, err := resolveTarget(cfg, positional, tf)
	if err != nil {
		fail(err)
	}

	if !t.IsCopy && !confirm {
		fmt.Fprintln(os.Stderr, "\n  ⚠  This target is not marked as a copy (\"is_copy\": true / --copy).")
		fmt.Fprintln(os.Stderr, "     Benchmarking renders pages, which runs the site's own code for them —")
		fmt.Fprintln(os.Stderr, "     anything it does on page load (emails, cron, writes) will happen.")
		fmt.Fprintln(os.Stderr, "     Run this against a copy of the site, not production.")
		if !promptConfirm("\n  Continue anyway? [Y/n] ") {
			fmt.Fprintln(os.Stderr, "  Aborted.")
			os.Exit(1)
		}
	}

	runner := wpcli.New(t)
	if !quiet {
		progress("connecting to %s…", t.Describe())
	}
	info, err := site.Load(runner)
	clearProgress()
	if err != nil {
		fail(fmt.Errorf("could not load WordPress on %s: %w", t.Describe(), err))
	}
	return &scanState{name: name, target: t, runner: runner, info: info}
}

type targetFlags struct {
	ssh, container, dockerUser, path, wpBinary, user, url, skip *string
	copy, noOpcache                                             *bool
}

func addTargetFlags(fs *flag.FlagSet) *targetFlags {
	return &targetFlags{
		ssh:        fs.String("ssh", "", "SSH host or ~/.ssh/config alias where the site lives"),
		container:  fs.String("container", "", "Docker container running WordPress"),
		dockerUser: fs.String("docker-user", "", "User inside the container (default www-data)"),
		path:       fs.String("path", "", "WordPress path, passed to WP-CLI as --path"),
		wpBinary:   fs.String("wp-binary", "", "WP-CLI command or path on the site's machine (default wp), e.g. /tmp/wp-cli.phar"),
		user:       fs.String("user", "", "WordPress login to render pages as (needed behind login walls)"),
		url:        fs.String("site-url", "", "Public base URL for fetching assets, if different from home_url"),
		skip:       fs.String("skip-plugins", "", "Comma-separated plugins to exclude from measurements (e.g. query-monitor)"),
		copy:       fs.Bool("copy", false, "Declare the target a copy/sandbox (skips the production warning)"),
		noOpcache:  fs.Bool("no-opcache", false, "Measure without OPcache (not representative of a web server)"),
	}
}

// resolveTarget starts from a named config target (if given) and overlays any flags.
func resolveTarget(cfg *config.Config, positional []string, tf *targetFlags) (string, config.Target, error) {
	var name string
	var t config.Target
	if len(positional) > 0 {
		name = positional[0]
		var ok bool
		if t, ok = cfg.Targets[name]; !ok {
			return "", t, fmt.Errorf("unknown target %q (configured: %s)", name, strings.Join(cfg.TargetNames(), ", "))
		}
	}
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&t.SSH, *tf.ssh)
	set(&t.DockerContainer, *tf.container)
	set(&t.DockerUser, *tf.dockerUser)
	set(&t.WPPath, *tf.path)
	set(&t.WPBinary, *tf.wpBinary)
	set(&t.User, *tf.user)
	set(&t.URL, *tf.url)
	if *tf.skip != "" {
		t.SkipPlugins = strings.Split(*tf.skip, ",")
	}
	if *tf.copy {
		t.IsCopy = true
	}
	if *tf.noOpcache {
		off := false
		t.Opcache = &off
	}
	if name == "" {
		name = t.Describe()
	}
	return name, t, nil
}

func selectScanners(only string) ([]string, error) {
	if only == "" {
		return allScanners, nil
	}
	valid := map[string]bool{}
	for _, s := range allScanners {
		valid[s] = true
	}
	var out []string
	for _, s := range strings.Split(only, ",") {
		s = strings.TrimSpace(s)
		if !valid[s] {
			return nil, fmt.Errorf("unknown scanner %q (available: %s)", s, strings.Join(allScanners, ", "))
		}
		out = append(out, s)
	}
	return out, nil
}

func loadConfig() (string, *config.Config) {
	path, err := config.DefaultPath()
	if err != nil {
		fail(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		fail(err)
	}
	return path, cfg
}

// parseInterspersed lets positional arguments appear before, between or after flags
// (the stdlib flag package stops at the first positional argument).
func parseInterspersed(fs *flag.FlagSet, args []string) []string {
	var positional []string
	for {
		fs.Parse(args)
		args = fs.Args()
		if len(args) == 0 {
			return positional
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// promptConfirm asks a yes/no question. Only an uppercase "Y" proceeds; anything else aborts safely.
func promptConfirm(question string) bool {
	fmt.Fprint(os.Stderr, question)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}
	return strings.TrimSpace(line) == "Y"
}

// progress prints a transient status line on stderr when it is a terminal.
func progress(format string, a ...any) {
	if fi, err := os.Stderr.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\r\033[K  "+format, a...)
}

func clearProgress() {
	if fi, err := os.Stderr.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
}

func nonNil(f []scanner.Finding) []scanner.Finding {
	if f == nil {
		return []scanner.Finding{}
	}
	return f
}

func fail(err error) {
	clearProgress()
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
