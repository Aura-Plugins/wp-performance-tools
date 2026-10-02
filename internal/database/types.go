package database

import "github.com/Aura-Plugins/wp-performance-tools/internal/units"

// Report is the decoded result of the database probe.
type Report struct {
	Tables          []Table     `json:"tables"`
	MetaTotalBytes  int64       `json:"meta_total_bytes"`
	MetaKeys        []MetaKey   `json:"meta_keys"`
	Autoload        Autoload    `json:"autoload"`
	AutoloadTop     []NamedSize `json:"autoload_top"`
	Transients      int64       `json:"transients"`
	ExpiredTimeouts int64       `json:"expired_timeouts"`
	OrphanMeta      int64       `json:"orphan_meta"`
	Posts           []PostCount `json:"posts"`
}

// MySQL returns numbers as strings in some drivers; units.Int accepts both.
type Table struct {
	Name       string    `json:"name"`
	RowCount   units.Int `json:"row_count"`
	DataBytes  units.Int `json:"data_bytes"`
	IndexBytes units.Int `json:"index_bytes"`
}

func (t Table) Bytes() int64 { return int64(t.DataBytes) + int64(t.IndexBytes) }

type MetaKey struct {
	Name     string    `json:"name"`
	RowCount units.Int `json:"row_count"`
	Bytes    units.Int `json:"bytes"`
}

type Autoload struct {
	OptionCount units.Int `json:"option_count"`
	Bytes       units.Int `json:"bytes"`
}

type NamedSize struct {
	Name  string    `json:"name"`
	Bytes units.Int `json:"bytes"`
}

type PostCount struct {
	Type   string    `json:"type"`
	Status string    `json:"status"`
	Count  units.Int `json:"count"`
}
