package gojcbmock

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Downloads and caches the mock server, so that it is retrievable
// for automatic testing

const defaultMockFile = "CouchbaseMock-1.4.0.jar"
const defaultMockUrl = "http://packages.couchbase.com/clients/c/mock/" + defaultMockFile

// Ensures that the mock path is available
func GetMockPath() (path string, err error) {
	var url string
	if path = os.Getenv("GOCB_MOCK_PATH"); path == "" {
		path = strings.Join([]string{os.TempDir(), defaultMockFile}, string(os.PathSeparator))
	}
	if url = os.Getenv("GOCB_MOCK_URL"); url == "" {
		url = defaultMockUrl
	}

	path, err = filepath.Abs(path)
	if err != nil {
		throwMockError("Couldn't get absolute path (!)", err)
	}

	info, err := os.Stat(path)
	if err == nil && info.Size() > 0 {
		return path, nil
	} else if err != nil && !os.IsNotExist(err) {
		throwMockError("Couldn't resolve existing path", err)
	}

	os.Remove(path)
	log.Printf("Downloading %s to %s", url, path)

	resp, err := http.Get(defaultMockUrl)
	if err != nil {
		throwMockError("Couldn't create HTTP request (or other error)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		throwMockError(fmt.Sprintf("Got HTTP %d from URL", resp.StatusCode), errors.New("non-200 response"))
	}

	out, err := os.Create(path)
	if err != nil {
		throwMockError("Couldn't open output file", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		throwMockError("Couldn't write response", err)
	}

	return path, nil
}
