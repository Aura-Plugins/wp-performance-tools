package units

/*
Small shared helpers with no internal imports: human-readable sizes, a JSON integer that
also accepts numeric strings, and medians for repeated measurements.
*/

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Bytes formats a byte count as B, KB, MB or GB (1024-based).
func Bytes(n int64) string {
	const k = 1024
	switch {
	case n >= k*k*k:
		return fmt.Sprintf("%.1f GB", float64(n)/(k*k*k))
	case n >= k*k:
		return fmt.Sprintf("%.1f MB", float64(n)/(k*k))
	case n >= k:
		return fmt.Sprintf("%.0f KB", float64(n)/k)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// Int decodes a JSON number or a numeric string. $wpdb returns every column as a
// string, so "1234" and 1234 must both work.
type Int int64

func (i *Int) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*i = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		var raw json.Number
		if jerr := json.Unmarshal(b, &raw); jerr != nil {
			return err
		}
		if n, err = raw.Float64(); err != nil {
			return err
		}
	}
	*i = Int(n)
	return nil
}

// Median returns the median of values (0 for an empty slice). The input is not modified.
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}
