package dev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/airplanedev/cli/pkg/api"
	"github.com/airplanedev/cli/pkg/logger"
	"github.com/airplanedev/lib/pkg/builtins"
	"github.com/mitchellh/hashstructure/v2"
	"github.com/pkg/errors"
	"google.golang.org/api/option"
)

var supportedOS = map[string]bool{"darwin": true, "linux": true}
var supportedArch = map[string]bool{"amd64": true, "arm64": true}

const (
	checksumFileName  = "checksum.txt"
	builtinsGCSBucket = "airplane-builtins-prod-a1a046b"
)

func isLocalExecutionSupported(opSystem, arch string) bool {
	_, osSupported := supportedOS[opSystem]
	_, archSupported := supportedArch[arch]
	return osSupported && archSupported
}

type StdAPIRequest struct {
	Namespace string                 `json:"namespace"`
	Name      string                 `json:"name"`
	Request   map[string]interface{} `json:"request"`
}

func BuiltinRequest(slug string, paramValues api.Values) (StdAPIRequest, error) {
	fs, err := builtins.GetBuiltinFunctionSpecification(slug)
	if err != nil {
		return StdAPIRequest{}, errors.New("invalid builtin slug")
	}
	req := StdAPIRequest{
		Namespace: fs.Namespace,
		Name:      fs.Name,
		Request:   paramValues,
	}
	return req, nil
}

func marshalBuiltinRequest(slug string, paramValues api.Values) (string, error) {
	req, err := BuiltinRequest(slug, paramValues)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}
	return string(b), nil
}

// TODO: refactor and share this logic with viewsDir
func tmpDirPath(fileName string) string {
	hash, err := hashstructure.Hash(fileName, hashstructure.FormatV2, nil)
	if err != nil {
		logger.Log("error with hashing tmpdir, using default hash value: %d", hash)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("airplane-builtins-%d", hash))
}

func createBuiltinDir(directory string) error {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		if err := os.Mkdir(directory, 0755); err != nil {
			return errors.Wrap(err, "creating temporary dir")
		}
		logger.Debug("created temporary dir for builtins: %s", directory)
		return nil
	}
	return nil
}

type BuiltinClient struct {
	fileName     string
	directory    string
	binaryPath   string
	checksumPath string
	client       *storage.Client
}

func NewBuiltinClient(opSystem, arch string) (*BuiltinClient, error) {
	if !isLocalExecutionSupported(opSystem, arch) {
		return nil, fmt.Errorf("Local builtins execution for %s %s systems is under development. Please reach out to support@airplane.dev for assistance.", opSystem, arch)
	}
	storageClient, err := storage.NewClient(context.Background(), option.WithoutAuthentication())
	if err != nil {
		return nil, errors.Wrap(err, "creating GCS client")
	}
	fileName := fmt.Sprintf("builtins-%s-%s", opSystem, arch)
	tmpDir := tmpDirPath(fileName)
	if err := createBuiltinDir(tmpDir); err != nil {
		return nil, errors.Wrap(err, "creating builtins directory")
	}

	client := &BuiltinClient{
		fileName:     fileName,
		client:       storageClient,
		directory:    tmpDir,
		checksumPath: filepath.Join(tmpDir, checksumFileName),
		binaryPath:   filepath.Join(tmpDir, fileName),
	}
	_, err = client.install()
	if err != nil {
		return nil, errors.Wrap(err, "installing builtins")
	}
	return client, nil
}

// Returns the exec.Cmd needed to call the builtin
func (b *BuiltinClient) Cmd(ctx context.Context, req string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, b.binaryPath, req), nil
}

// Downloads the latest version of builtins if it doesn't exist
// otherwise uses version installed in tmp directory
func (b *BuiltinClient) install() (string, error) {
	if _, err := os.Stat(b.binaryPath); os.IsNotExist(err) {
		logger.Debug("Builtins package not found: %s", b.binaryPath)
		return b.download()
	}
	if !b.isLatestVersion() {
		logger.Debug("Builtins package out of date. Getting latest version.")
		return b.download()
	}
	logger.Debug("Using cached builtins package: %s", b.binaryPath)
	return b.binaryPath, nil
}

func (b *BuiltinClient) getGCSObject() *storage.ObjectHandle {
	return b.client.Bucket(builtinsGCSBucket).Object(fmt.Sprintf("builtin-builds/%s", b.fileName))
}
func (b *BuiltinClient) isLatestVersion() bool {
	obj := b.getGCSObject()
	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		logger.Log("error getting builtins attributes: %v", err)
		return false
	}
	if _, err := os.Stat(b.checksumPath); os.IsNotExist(err) {
		logger.Debug("checksum does not exist: %v", err)
		return false
	}
	checksum, err := os.ReadFile(b.checksumPath)
	if err != nil {
		logger.Log("error reading checksum file: %v", err)
		return false
	}
	return bytes.Equal(checksum, attrs.MD5)
}

// Downloads the builtin binary from GCS
func (b *BuiltinClient) download() (string, error) {
	logger.Debug("Downloading builtins binary...")
	obj := b.getGCSObject()
	attrs, err := obj.Attrs(context.Background())
	if err != nil {
		return "", errors.Wrap(err, "getting builtins latest version")
	}
	rc, err := obj.NewReader(context.Background())
	if err != nil {
		return "", err
	}
	defer rc.Close()
	body, err := ioutil.ReadAll(rc)
	if err != nil {
		return "", err
	}
	file, err := os.Create(b.binaryPath)
	if err != nil {
		return "", errors.Wrap(err, "creating binary file")
	}
	defer file.Close()
	_, err = file.Write(body)
	if err != nil {
		return "", err
	}

	if err := file.Chmod(0766); err != nil {
		return "", errors.Wrap(err, "error granting file permissions")
	}
	logger.Debug("Builtins binary downloaded to: %s", b.binaryPath)
	checkSumFile, err := os.Create(b.checksumPath)
	if err != nil {
		return "", errors.Wrap(err, "creating checksum file")
	}
	defer checkSumFile.Close()
	_, err = checkSumFile.Write(attrs.MD5)
	if err != nil {
		return "", err
	}
	if err := checkSumFile.Chmod(0664); err != nil {
		return "", errors.Wrap(err, "error granting checksum file permissions")
	}
	logger.Debug("Checksum file created: %s", b.checksumPath)
	return b.binaryPath, nil
}
