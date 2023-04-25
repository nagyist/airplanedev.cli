package initcmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/airplanedev/cli/pkg/deploy/taskdir/definitions"
	"golang.org/x/exp/slices"
)

type InitResponse struct {
	WorkingDirectory  string
	filenames         map[string]fileStatus
	NewTaskDefinition *definitions.Definition
	NewViewDefinition *definitions.ViewDefinition
}

type fileStatus string

const (
	fileStatusCreated  fileStatus = "created"
	fileStatusModified fileStatus = "modified"
)

type FilenameWithStatus struct {
	Filename   string     `json:"filename"`
	FileStatus fileStatus `json:"fileStatus"`
}

func newInitResponse(wd string) (InitResponse, error) {
	absWd, err := filepath.Abs(wd)
	if err != nil {
		return InitResponse{}, err
	}
	return InitResponse{
		WorkingDirectory: absWd,
		filenames:        map[string]fileStatus{},
	}, nil
}

func (ir *InitResponse) AddCreatedFile(filename string) {
	filename = ir.normalizeFilename(filename)

	ir.filenames[filename] = fileStatusCreated
}

func (ir *InitResponse) AddModifiedFile(filename string) {
	filename = ir.normalizeFilename(filename)

	if fileStatus, ok := ir.filenames[filename]; ok && fileStatus == fileStatusCreated {
		return
	}

	ir.filenames[filename] = fileStatusModified
}

func (ir *InitResponse) AddFile(created bool, filename string) {
	if created {
		ir.AddCreatedFile(filename)
	} else {
		ir.AddModifiedFile(filename)
	}
}

func (ir InitResponse) GetCreatedFiles() []string {
	filenames := []string{}
	for fn, status := range ir.filenames {
		if status == fileStatusCreated {
			filenames = append(filenames, fn)
		}
	}
	slices.SortFunc(filenames, sortFilenames)

	return filenames
}

func (ir InitResponse) GetModifiedFiles() []string {
	filenames := []string{}
	for fn, status := range ir.filenames {
		if status == fileStatusModified {
			filenames = append(filenames, fn)
		}
	}
	slices.SortFunc(filenames, sortFilenames)

	return filenames
}

func (ir InitResponse) GetFilenamesWithStatus() []FilenameWithStatus {
	filenames := []string{}
	for fn := range ir.filenames {
		filenames = append(filenames, fn)
	}
	slices.SortFunc(filenames, sortFilenames)

	filenamesWithStatus := []FilenameWithStatus{}
	for _, fn := range filenames {
		filenamesWithStatus = append(filenamesWithStatus, FilenameWithStatus{
			Filename:   fn,
			FileStatus: ir.filenames[fn],
		})
	}
	return filenamesWithStatus
}

func (ir InitResponse) String() string {
	filenames := []string{}
	for fn := range ir.filenames {
		filenames = append(filenames, fn)
	}
	slices.SortFunc(filenames, sortFilenames)

	b := strings.Builder{}
	for _, fn := range filenames {
		status := "C"
		if ir.filenames[fn] == fileStatusModified {
			status = "M"
		}
		fmt.Fprintf(&b, "- [%s] %s\n", status, fn)
	}

	return b.String()
}

func (ir InitResponse) normalizeFilename(filename string) string {
	if filepath.IsAbs(filename) {
		filename = strings.TrimPrefix(filename, ir.WorkingDirectory+"/")
	}
	return filename
}

func sortFilenames(a string, b string) bool {
	aInWd := !filepath.IsAbs(a)
	bInWd := !filepath.IsAbs(b)
	if aInWd == bInWd {
		if a < b {
			return true
		} else {
			return false
		}
	}
	if aInWd {
		return true
	} else {
		return false
	}
}
