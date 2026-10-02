package units

import (
	"encoding/json"
	"testing"
)

func TestMedian(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{5}, 5},
		{[]float64{3, 1, 2}, 2},
		{[]float64{4, 1, 3, 2}, 2.5},
	}
	for _, c := range cases {
		if got := Median(c.in); got != c.want {
			t.Errorf("Median(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	in := []float64{3, 1, 2}
	Median(in)
	if in[0] != 3 {
		t.Error("Median must not reorder its input")
	}
}

func TestIntAcceptsStringsAndNumbers(t *testing.T) {
	var v struct {
		A Int `json:"a"`
		B Int `json:"b"`
		C Int `json:"c"`
	}
	if err := json.Unmarshal([]byte(`{"a":"1234","b":56,"c":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != 1234 || v.B != 56 || v.C != 0 {
		t.Errorf("got %+v", v)
	}
}

func TestBytes(t *testing.T) {
	cases := map[int64]string{512: "512 B", 2048: "2 KB", 5 << 20: "5.0 MB", 3 << 30: "3.0 GB"}
	for in, want := range cases {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, want %q", in, got, want)
		}
	}
}
