package wpcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Aura-Plugins/wp-performance-tools/internal/config"
	"github.com/Aura-Plugins/wp-performance-tools/internal/probe"
)

// opcacheDir is where PHP keeps compiled scripts between WP-CLI runs, on the machine running WordPress.
// Without it every run recompiles WordPress and the timings look far worse than under a web server.
const opcacheDir = "/tmp/wp-perf-opcache"

// Runner executes WP-CLI commands against one target.
type Runner struct {
	Target config.Target
}

// Options are the WP-CLI global flags a measurement may need.
type Options struct {
	User           string   // --user: render as this WordPress user
	SkipPlugins    []string // added to Target.SkipPlugins
	SkipAllPlugins bool     // --skip-plugins with no list
	SkipThemes     bool     // --skip-themes
	Exec           string   // --exec: PHP run before WordPress loads (e.g. define SAVEQUERIES)
}

func New(t config.Target) *Runner {
	return &Runner{Target: t}
}

// Run executes `wp <args>` with stdin attached and returns its stdout.
func (r *Runner) Run(stdin string, args []string) (string, error) {
	cmd := r.command(r.wpScript(args))
	cmd.Stdin = strings.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := lastLines(stderr.String()+"\n"+stdout.String(), 6)
		return stdout.String(), fmt.Errorf("wp %s: %w\n%s", firstArg(args), err, detail)
	}
	return stdout.String(), nil
}

// Probe runs an embedded PHP probe through `wp eval-file -` and decodes its JSON result into out.
// probeArgs reach the PHP code as $args.
func (r *Runner) Probe(name string, probeArgs []string, opts Options, out any) error {
	src, err := probe.Source(name)
	if err != nil {
		return err
	}

	args := r.globalFlags(opts)
	args = append(args, "eval-file", "-")
	args = append(args, probeArgs...)

	stdout, err := r.Run(src, args)
	if err != nil {
		return fmt.Errorf("probe %s: %w", name, err)
	}

	payload, err := probe.Extract(stdout)
	if err != nil {
		return fmt.Errorf("probe %s: %w\n%s", name, err, lastLines(stdout, 6))
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("probe %s: decoding result: %w", name, err)
	}
	return nil
}

// globalFlags turns Options (plus the target's defaults) into WP-CLI global flags.
func (r *Runner) globalFlags(opts Options) []string {
	var args []string
	if r.Target.WPPath != "" {
		args = append(args, "--path="+r.Target.WPPath)
	}
	if opts.User != "" {
		args = append(args, "--user="+opts.User)
	}

	if opts.SkipAllPlugins {
		args = append(args, "--skip-plugins")
	} else {
		skip := append(append([]string{}, r.Target.SkipPlugins...), opts.SkipPlugins...)
		if len(skip) > 0 {
			args = append(args, "--skip-plugins="+strings.Join(skip, ","))
		}
	}

	if opts.SkipThemes {
		args = append(args, "--skip-themes")
	}
	if opts.Exec != "" {
		args = append(args, "--exec="+opts.Exec)
	}
	return args
}

// wpScript builds the shell line that runs WP-CLI on the WordPress machine.
// With OPcache on, it runs the WP-CLI phar through `php -d opcache...` — but only when the `wp`
// binary really is a PHP script; wrappers (shell scripts) are run as-is.
func (r *Runner) wpScript(args []string) string {
	bin := r.Target.WPBinary
	if bin == "" {
		bin = "wp"
	}
	wpArgs := shellJoin(args)

	if !r.Target.OpcacheEnabled() {
		return shellQuote(bin) + " " + wpArgs
	}

	php := r.Target.PHPBinary
	if php == "" {
		php = "php"
	}
	phpFlags := "-d opcache.enable_cli=1 -d opcache.file_cache=" + opcacheDir + " -d opcache.file_cache_only=1 -d memory_limit=1G"

	return fmt.Sprintf(
		`WP=$(command -v %s); if [ -n "$WP" ] && head -n 1 "$WP" | grep -q php; then mkdir -p %s 2>/dev/null; %s %s "$WP" %s; else %s %s; fi`,
		shellQuote(bin), opcacheDir, shellQuote(php), phpFlags, wpArgs, shellQuote(bin), wpArgs,
	)
}

// command wraps the WP-CLI shell line for where the site lives: local, Docker, SSH, or SSH + Docker.
func (r *Runner) command(script string) *exec.Cmd {
	t := r.Target

	var argv []string // the command line on the machine that runs WordPress
	if t.DockerContainer != "" {
		user := t.DockerUser
		if user == "" {
			user = "www-data"
		}
		argv = []string{"docker", "exec", "-i", "-u", user, t.DockerContainer, "sh", "-c", script}
	} else {
		argv = []string{"sh", "-c", script}
	}

	if t.SSH != "" {
		// BatchMode: never hang on a password prompt; use keys or an ssh-agent.
		return exec.Command("ssh", "-o", "BatchMode=yes", t.SSH, shellJoin(argv))
	}
	return exec.Command(argv[0], argv[1:]...)
}

func firstArg(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}
