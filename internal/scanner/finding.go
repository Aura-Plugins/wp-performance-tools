package scanner

/*
This file defines the Finding struct, the single output format shared by every scanner:
	database scanner
	pages scanner
	bootstrap scanner
	media scanner
	frontend scanner
	archive checker
*/

type Severity string

const (
	SeverityLow    Severity = "Low"
	SeverityMedium Severity = "Medium"
	SeverityHigh   Severity = "High"
)

type Finding struct {
	ID          string            `json:"id"`
	Fingerprint string            `json:"fingerprint"`
	Severity    Severity          `json:"severity"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Target      string            `json:"target"`
	Path        string            `json:"path,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewFinding builds a Finding and fills in its fingerprint.
// Scanners should always use this rather than a struct literal so no finding ships without one.
func NewFinding(id string, sev Severity, title, description, target string, metadata map[string]string) Finding {
	f := Finding{
		ID:          id,
		Severity:    sev,
		Title:       title,
		Description: description,
		Target:      target,
		Metadata:    metadata,
	}
	f.Fingerprint = Fingerprint(f)
	return f
}
