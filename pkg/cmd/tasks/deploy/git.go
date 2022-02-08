package deploy

import (
	"path/filepath"
	"strings"

	"github.com/airplanedev/cli/pkg/api"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
)

type GitRepoGetter interface {
	GetGitRepo(file string) (*git.Repository, error)
}
type FileGitRepoGetter struct{}

// GetGitRepo gets a git repo that tracks the input file. If the file is not in a git repo, the
// returned repo will be nil.
func (gh *FileGitRepoGetter) GetGitRepo(file string) (*git.Repository, error) {
	repo, err := git.PlainOpenWithOptions(filepath.Dir(file), &git.PlainOpenOptions{
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

	status, err := w.Status()
	if err != nil {
		return meta, err
	}
	meta.IsDirty = !status.IsClean()

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

	return meta, nil
}
