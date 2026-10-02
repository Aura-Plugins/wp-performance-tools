package scanner

// Scanner is implemented by every analysis domain (database, pages, bootstrap, media, frontend).
// Name returns the domain identifier, which is also the prefix of every finding ID it produces.
type Scanner interface {
	Name() string
	Scan() ([]Finding, error)
}
