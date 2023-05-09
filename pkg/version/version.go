package version

import "os"

// The following fields are set at build-time via ldflags.
var (
	version           = "(unknown)"
	date       string = "(unknown)"
	prerelease string = ""
)

// The following fields are cached to avoid unnecessary syscalls.
var (
	hostname = "(unknown)"
)

func init() {
	if h, err := os.Hostname(); err != nil {
		hostname = h
	}
}

func Get() string {
	return version
}

func Prerelease() bool {
	return prerelease != ""
}

func Date() string {
	return date
}

func Hostname() string {
	return hostname
}
