package version

import "sync"

// Set by Go Releaser.
var (
	version    string = "<unknown>"
	date       string = "<unknown>"
	prerelease string = ""
)

func Get() string {
	return version
}

func Prerelease() bool {
	return prerelease != ""
}

func Date() string {
	return date
}

type Metadata struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	IsLatest bool   `json:"isLatest"`
}

// We cache the CLI's version since github rate limits checks
// When we add hot reload and it's long running, we should expire/periodically refresh this.
type Cache struct {
	mu      sync.Mutex
	Version *Metadata
}

func (c *Cache) Add(response Metadata) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Version = &response
}
