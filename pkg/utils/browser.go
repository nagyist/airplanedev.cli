package utils

import (
	"fmt"
	"os"
	"runtime"

	"github.com/airplanedev/cli/pkg/browser"
)

// Open attempts to open the URL in the browser.
//
// Return true if the browser was opened successfully.
func Open(url string) bool {
	if os.Getenv("AP_BROWSER") == "none" {
		return false
	}

	err := browser.Open(runtime.GOOS, url)
	return err == nil
}

func StudioURL(webHost, studioPort, page string) string {
	return fmt.Sprintf("https://%s/studio%s?__airplane_host=http://localhost:%s", webHost, page, studioPort)
}
