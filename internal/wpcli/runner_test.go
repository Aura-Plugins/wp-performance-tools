package wpcli

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Aura-Plugins/wp-performance-tools/internal/config"
)

func TestShellQuoteRoundTrip(t *testing.T) {
	// The quoted string must reach a real shell unchanged, including quotes and $.
	in := `it's "$HOME" & more`
	out, err := exec.Command("sh", "-c", "printf %s "+shellQuote(in)).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Errorf("got %q, want %q", out, in)
	}
}

func TestGlobalFlags(t *testing.T) {
	r := New(config.Target{WPPath: "/var/www/html", SkipPlugins: []string{"query-monitor"}})

	got := strings.Join(r.globalFlags(Options{User: "admin", SkipPlugins: []string{"acf"}, Exec: "x();"}), " ")
	want := "--path=/var/www/html --user=admin --skip-plugins=query-monitor,acf --exec=x();"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}

	got = strings.Join(r.globalFlags(Options{SkipAllPlugins: true, SkipThemes: true}), " ")
	if got != "--path=/var/www/html --skip-plugins --skip-themes" {
		t.Errorf("skip-all: got %q", got)
	}
}

func TestCommandWrapping(t *testing.T) {
	cases := []struct {
		t    config.Target
		want string // prefix of the argv joined with spaces
	}{
		{config.Target{}, "sh -c"},
		{config.Target{DockerContainer: "wp1"}, "docker exec -i -u www-data wp1 sh -c"},
		{config.Target{SSH: "box"}, "ssh -o BatchMode=yes box 'sh' '-c'"},
		{config.Target{SSH: "box", DockerContainer: "wp1", DockerUser: "app"}, "ssh -o BatchMode=yes box 'docker' 'exec' '-i' '-u' 'app' 'wp1'"},
	}
	for _, c := range cases {
		cmd := New(c.t).command("wp --info")
		got := strings.Join(cmd.Args, " ")
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("%+v:\n got  %q\n want prefix %q", c.t, got, c.want)
		}
	}
}

func TestWPScriptOpcache(t *testing.T) {
	off := false
	plain := New(config.Target{Opcache: &off}).wpScript([]string{"eval-file", "-"})
	if plain != "'wp' 'eval-file' '-'" {
		t.Errorf("no-opcache script: %q", plain)
	}
	withCache := New(config.Target{}).wpScript([]string{"eval-file", "-"})
	if !strings.Contains(withCache, "opcache.enable_cli=1") || !strings.Contains(withCache, "grep -q php") {
		t.Errorf("opcache script should enable OPcache only for PHP launchers: %q", withCache)
	}
}
