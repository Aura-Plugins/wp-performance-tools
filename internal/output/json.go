package output

import (
	"encoding/json"
	"os"
)

// PrintJSON writes any report as indented JSON to stdout.
func PrintJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(v)
}
