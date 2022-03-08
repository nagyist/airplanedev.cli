package version

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
