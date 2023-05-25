package utils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Open attempts to open the URL in the browser.
//
// Return true if the browser was opened successfully.
func Open(url string) bool {
	if os.Getenv("AP_BROWSER") == "none" {
		return false
	}

	err := open(runtime.GOOS, url)
	return err == nil
}

func StudioURL(webHost, studioPort, page string) string {
	return fmt.Sprintf("https://%s/studio%s?__airplane_host=http://localhost:%s", webHost, page, studioPort)
}

// open attempts to open the browser for `os` at `url`.
func open(os, url string) error {
	var cmd string
	var args []string

	switch os {
	case "darwin":
		cmd = "open"
		args = append(args, url)

	case "windows":
		cmd = "cmd"
		r := strings.NewReplacer("&", "^&")
		args = append(args, "/c", "start", r.Replace(url))

	default:
		cmd = "xdg-open"
		args = append(args, url)
	}

	bin, err := exec.LookPath(cmd)
	if err != nil {
		return err
	}

	return exec.Command(bin, args...).Run()
}
