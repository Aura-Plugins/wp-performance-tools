package probe

/*
Probes are small PHP files embedded in the binary. The runner pipes one into
`wp eval-file -`, so it runs inside the site's own WordPress with full access to
$wpdb, the object cache and the theme — and nothing is ever copied to the server.

Every probe ends by printing Marker followed by one line of JSON. Anything printed
before the marker (PHP notices, theme output) is ignored.
*/

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
)

//go:embed php/*.php
var files embed.FS

// Marker separates a probe's JSON result from anything else printed on stdout.
const Marker = "@@WPPERF@@"

// Source returns the PHP source of a probe by name (the file name without .php).
func Source(name string) (string, error) {
	b, err := files.ReadFile("php/" + name + ".php")
	if err != nil {
		return "", fmt.Errorf("unknown probe %q", name)
	}
	return string(b), nil
}

// Extract returns the JSON printed after the last marker in a probe's stdout.
func Extract(stdout string) ([]byte, error) {
	out := []byte(stdout)
	i := bytes.LastIndex(out, []byte(Marker))
	if i < 0 {
		return nil, errors.New("no result marker in output (did WordPress load? is the site reachable?)")
	}
	payload := bytes.TrimSpace(out[i+len(Marker):])
	if nl := bytes.IndexByte(payload, '\n'); nl >= 0 {
		payload = payload[:nl]
	}
	return payload, nil
}
