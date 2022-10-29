package latest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/airplanedev/cli/pkg/analytics"
	"github.com/airplanedev/cli/pkg/conf"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/cli/pkg/version"
)

const releaseURL = "https://api.github.com/repos/airplanedev/cli/releases?per_page=1"

type release struct {
	Name       string `json:"name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// CheckLatest queries the GitHub API for newer releases and prints a warning if the CLI is outdated.
func CheckLatest(ctx context.Context, userConfig *conf.UserConfig) bool {
	if version.Get() == "<unknown>" || version.Prerelease() {
		// Pass silently if we don't know the current CLI version or are on a pre-release.
		return true
	}

	if userConfig != nil && userConfig.LatestVersion.Version != "" &&
		userConfig.LatestVersion.Updated.After(time.Now().AddDate(0, 0, -1)) {
		// We only want to log about newer CLI versions once a day.
		return true
	}

	latest, err := getLatest(ctx)
	if err != nil {
		analytics.ReportError(err)
		logger.Debug("An error ocurred checking for the latest version: %s", err)
		return true
	} else if latest == "" {
		// Pass silently if we can't identify the latest version.
		return true
	}
	if userConfig != nil {
		userConfig.LatestVersion = conf.VersionUpdate{
			Version: latest,
			Updated: time.Now().UTC(),
		}
		if err := conf.WriteDefaultUserConfig(*userConfig); err != nil {
			// Do nothing
		}
	}

	latestWithoutPrefix := strings.TrimPrefix(latest, "v")
	// Assumes not matching latest means you are behind:
	if latestWithoutPrefix != version.Get() {
		logger.Warning("A newer CLI version is available (%s -> %s). To upgrade, run", version.Get(), latestWithoutPrefix)
		logger.Log(logger.Gray("  " + getUpgradeCommand()))
		logger.Log("")
		return false
	}
	return true
}

func getUpgradeCommand() string {
	curlCmd := "curl -L https://github.com/airplanedev/cli/releases/latest/download/install.sh | sh"
	os := runtime.GOOS
	switch os {
	case "windows":
		return "iwr https://github.com/airplanedev/cli/releases/latest/download/install.ps1 -useb | iex"
	case "darwin":
		cmd := curlCmd
		_, err := exec.Command("brew", "list", "airplane").Output()
		if err == nil {
			cmd = "brew update && brew upgrade airplanedev/tap/airplane"
		}
		return cmd
	default:
		return curlCmd
	}
}

func getLatest(ctx context.Context) (string, error) {
	curTime := time.Now()
	defer func() {
		totalTime := time.Since(curTime)
		logger.Debug("Time to get latest version: %s", totalTime)
	}()
	// GitHub heavily rate limits this endpoint. We should proxy this through our API and use an API key.
	// https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting
	req, err := http.NewRequestWithContext(ctx, "GET", releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// e.g. {"message":"API rate limit ..."}
		var ghError struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ghError); err != nil {
			analytics.ReportError(err)
			logger.Debug("Unable to decode GitHub %s API response: %s", resp.Status, err)
		}
		return "", fmt.Errorf("HTTP %s: %s", resp.Status, ghError.Message)
	}

	var releases []release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", err
	}
	if len(releases) == 0 {
		return "", nil
	}
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		return release.Name, nil
	}
	return "", nil
}
