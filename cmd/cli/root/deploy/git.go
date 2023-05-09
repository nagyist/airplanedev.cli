package deploy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/airplanedev/cli/pkg/cli/analytics"
	"github.com/airplanedev/cli/pkg/cli/apiclient/cliapi"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
)

type GitRepoGetter interface {
	GetGitRepo(file string) (*git.Repository, error)
}
type FileGitRepoGetter struct{}

// GetGitRepo gets a git repo that tracks the input file or directory. If the file is not in a git repo, the
// returned repo will be nil.
func (gh *FileGitRepoGetter) GetGitRepo(fileOrDir string) (*git.Repository, error) {
	fileInfo, err := os.Stat(fileOrDir)
	if err != nil {
		return nil, err
	}

	possibleRepoDir := fileOrDir
	if !fileInfo.IsDir() {
		possibleRepoDir = filepath.Dir(fileOrDir)
	}

	repo, err := git.PlainOpenWithOptions(possibleRepoDir, &git.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, nil
		}
		return nil, err
	}
	return repo, nil

}

type MockGitRepoGetter struct {
	Repo *git.Repository
}

func (gh *MockGitRepoGetter) GetGitRepo(file string) (*git.Repository, error) {
	return gh.Repo, nil
}

func GetEntrypointRelativeToGitRoot(repo *git.Repository, taskFilePath string) (string, error) {
	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}
	return filepath.Rel(w.Filesystem.Root(), taskFilePath)
}

func GetGitMetadata(repo *git.Repository) (api.GitMetadata, error) {
	meta := api.GitMetadata{}

	w, err := repo.Worktree()
	if err != nil {
		return meta, err
	}

	// Worktree.Status() is very slow so fall back to the command line instead.
	// https://github.com/go-git/go-git/issues/181
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = w.Filesystem.Root()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	if cmd.Run() != nil {
		analytics.ReportMessage(fmt.Sprintf("failed to get git status: %v", stdout.String()))
	} else {
		meta.IsDirty = stdout.Len() > 0
	}

	h, err := repo.Head()
	if err != nil {
		return meta, err
	}

	commit, err := repo.CommitObject(h.Hash())
	if err != nil {
		return meta, err
	}
	meta.CommitHash = commit.Hash.String()
	meta.CommitMessage = commit.Message
	if meta.User != "" {
		meta.User = commit.Author.Name
	}

	ref := h.Name().String()
	if h.Name().IsBranch() {
		ref = strings.TrimPrefix(ref, "refs/heads/")
	}
	meta.Ref = ref

	remotes, err := repo.Remotes()
	if err != nil {
		return meta, errors.Wrap(err, "listing remotes")
	}
	var remote *git.Remote
	for _, r := range remotes {
		// If there is a remote called origin, use that. Origin is the default
		// name for the remote, so it's our best guess for the correct remote.
		if r.Config().Name == "origin" {
			remote = r
			break
		}
	}
	if remote == nil && len(remotes) > 0 {
		// If there is no origin, use the first remote.
		remote = remotes[0]
	}
	if remote != nil {
		// URLs will always be non-empty. Use the first URL which is used
		// by git for fetching from a remote.
		remoteURL := remote.Config().URLs[0]
		repoOwner, repoName, vendor, err := parseRemote(remoteURL)
		if err != nil {
			return meta, errors.Wrapf(err, "parsing remote %s", remote.Config().URLs[0])
		}
		meta.RepositoryOwnerName = repoOwner
		meta.RepositoryName = repoName
		meta.Vendor = vendor
	}

	return meta, nil
}

var (
	githubHTTPRegex, _ = regexp.Compile(`^https:\/\/github\.com\/(.+)\/(.+?)(\.git)?$`)
	githubSSHRegex, _  = regexp.Compile(`^git@github\.com:(.+)\/(.+?)(\.git)?$`)
)

func parseRemote(remote string) (repoOwner, repoName string, vendor api.GitVendor, err error) {
	switch {
	case githubHTTPRegex.MatchString(remote):
		matches := githubHTTPRegex.FindStringSubmatch(remote)
		if len(matches) < 3 {
			return "", "", "", errors.Errorf("invalid github http remote %s", remote)
		}
		return matches[1], matches[2], api.GitVendorGitHub, nil
	case githubSSHRegex.MatchString(remote):
		matches := githubSSHRegex.FindStringSubmatch(remote)
		if len(matches) < 3 {
			return "", "", "", errors.Errorf("invalid github ssh remote %s", remote)
		}
		return matches[1], matches[2], api.GitVendorGitHub, nil
	default:
		return "", "", "", nil
	}
}
