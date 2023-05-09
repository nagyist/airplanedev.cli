package conf

import (
	"os"
	"strings"

	"github.com/pkg/errors"
)

var (
	// ErrMissing is returned when the config file does not exist.
	ErrMissing = errors.New("conf: config file does not exist")
)

// GetAPIKey gets an Airplane API key from an env var, if one exists.
func GetAPIKey() string {
	return os.Getenv("AP_API_KEY")
}

// GetTeamID gets an Airplane team ID from an env var, if one exists.
func GetTeamID() string {
	return os.Getenv("AP_TEAM_ID")
}

type GetGitRepoReponse struct {
	OwnerName string
	RepoName  string
}

// GetGitRepo gets a git repo from an env var, if one exists.
func GetGitRepo() GetGitRepoReponse {
	repo := os.Getenv("AP_GIT_REPO")
	repoSplit := strings.Split(repo, "/")
	if len(repoSplit) == 2 {
		return GetGitRepoReponse{
			OwnerName: repoSplit[0],
			RepoName:  repoSplit[1],
		}
	}
	return GetGitRepoReponse{}
}

// GetGitUser gets a git user from an env var, if one exists.
func GetGitUser() string {
	return os.Getenv("AP_GIT_USER")
}

// GetSource gets the source from an env var, if it exists.
func GetSource() string {
	return os.Getenv("AP_SOURCE")
}
