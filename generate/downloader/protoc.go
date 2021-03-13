package downloader

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gunk/gunk/log"
	"github.com/rogpeppe/go-internal/lockedfile"
	"golang.org/x/sys/unix"
)

const defaultProtocVersion = "v3.9.1"

// CheckOrDownloadProtoc downloads protoc to the specified path, unless it's already
// been downloaded. If no path is provided, it uses an OS-appropriate user cache.
// If the version is not specified, the latest version is fetched from GitHub.
// If both version and path are specified and a file already exists at the path,
// it checks whether the output of `protoc --version` is an exact match.
//
// Note that this code is safe for concurrent use between multiple goroutines or
// processes, since it uses a lock file on disk.
func CheckOrDownloadProtoc(path, version string) (string, error) {
	if version == "" {
		version = defaultProtocVersion
	}
	// note - functionality is shared partly with getPaths in download.go
	// but as that does not test existing binaries (as protoc-gen- binaries do not need to return version)
	// let's keep it separate
	dstPath := path
	if dstPath == "" {
		// Get the OS-specific cache directory.
		cachePath, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		if dir := os.Getenv("GUNK_CACHE_DIR"); dir != "" {
			// Allow overriding the cache dir entirely. Mainly for
			// the tests.
			cachePath = dir
		}
		cacheDir := filepath.Join(cachePath, "gunk")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return "", err
		}
		// The proto command path to use or download to.
		dstPath = filepath.Join(cacheDir, fmt.Sprintf("protoc-%s", version))
	}
	dstDir, _ := filepath.Split(dstPath)
	if unix.Access(dstDir, unix.W_OK) != nil {
		// we use unwritable dstPath (system protoc),
		// let's not do any of the locking/downloading and just test it
		if err := verifyProtocBinary(dstPath, version); err != nil {
			return "", err
		}
		return dstPath, nil
	}
	// First, grab a lock separate from the destination file. The
	// destination file is a binary we'll want to execute, so using it
	// directly as the lock can lead to "text file busy" errors.
	unlock, err := lockedfile.MutexAt(dstPath + ".lock").Lock()
	if err != nil {
		return "", err
	}
	defer unlock()
	// We are the only goroutine with access to dstPath. Check if it already
	// exists. Using lockedfile.OpenFile allows us to do an atomic write
	// when it doesn't exist yet.
	// TODO: isn't this lock entirely superfluous? The first lock already blocks the whole time
	dstFile, err := lockedfile.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0775)
	if os.IsExist(err) {
		// It exists. Because of O_EXCL, we haven't actually opened the
		// file. Just verify that protoc works and return.
		if err := verifyProtocBinary(dstPath, version); err != nil {
			return "", err
		}
		return dstPath, nil
	}
	if err != nil {
		return "", err
	}
	defer dstFile.Close()
	// The file does not exist. Download it, using dstFile.
	url, err := protocDownloadURL(runtime.GOOS, runtime.GOARCH, version)
	if err != nil {
		return "", fmt.Errorf("downloading protoc: %w", err)
	}
	// Download protoc since we were unable to find a usable
	// protoc installation.
	cl := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	res, err := cl.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("could not retrieve %q (%d)", url, res.StatusCode)
	}
	// Check the contents of the zipped folder for the protoc binary.
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	buf := bytes.NewReader(b)
	rdr, err := zip.NewReader(buf, res.ContentLength)
	if err != nil {
		return "", err
	}
	// Search in the zip download for the 'protoc' command.
	for _, f := range rdr.File {
		if f.Name != "bin/protoc" {
			continue
		}
		fc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer fc.Close()
		// Write protoc command to cache.
		if _, err := io.Copy(dstFile, fc); err != nil {
			return "", err
		}
		if err := dstFile.Close(); err != nil {
			return "", err
		}
		log.Verbosef("downloaded protoc to %s", dstPath)
		if err := verifyProtocBinary(dstPath, version); err != nil {
			return "", err
		}
		return dstPath, nil
	}
	return "", fmt.Errorf("unable to download and extract protoc")
}

func verifyProtocBinary(path, version string) error {
	cmd := log.ExecCommand(path, "--version")
	out, err := cmd.Output()
	if err != nil {
		return log.ExecError(path, err)
	}
	versionOutput := string(out)
	if versionOutput[:9] != "libprotoc" {
		return fmt.Errorf("%q was not a valid protoc binary", path)
	}
	// NOTE: the output of protoc --version doesn't include a 'v',
	// but the release tags do
	gotVersion := strings.TrimSpace(versionOutput[10:])
	short := version[1:]
	// split "-rc"
	split := strings.Split(short, "-")
	if gotVersion != split[0] {
		return fmt.Errorf("want protoc version %q got %q", split[0], gotVersion)
	}
	return nil
}

// protocDownloadURL builds a URL for retrieving for the protoc tool artifact
// from GitHub for use with current Go runtime's GOOS and GOARCH combination.
//
// Supported os + arch variants:
//
// 	osx-x86_32
// 	osx-x86_64
// 	linux-x86_32
// 	linux-x86_64
// 	linux-aarch64
// 	win32
// 	win64
//
// Example: https://github.com/protocolbuffers/protobuf/releases/download/v3.9.1/protoc-3.9.1-linux-x86_64.zip
func protocDownloadURL(os, arch, version string) (string, error) {
	// retrieve the specified version's release assets
	const protocGitHubRepo = "protocolbuffers/protobuf"
	if !strings.HasPrefix(version, "v") {
		return "", fmt.Errorf("invalid version: %s", version)
	}
	// determine the platform
	var platform string
	switch {
	case os == "darwin" && arch == "386":
		platform = "osx-x86_32"
	case os == "darwin" && arch == "amd64":
		platform = "osx-x86_64"
	case os == "linux" && arch == "386":
		platform = "linux-x86_32"
	case os == "linux" && arch == "amd64":
		platform = "linux-x86_64"
	case os == "linux" && arch == "arm64":
		platform = "linux-aarch_64"
	case os == "windows" && arch == "386":
		platform = "win32"
	case os == "windows" && arch == "amd64":
		platform = "win64"
	default:
		return "", fmt.Errorf("unknown os %q and arch %q", os, arch)
	}
	// the version string is guaranteed to starts with "v", removing it
	short := version[1:]
	short = strings.ReplaceAll(short, "rc", "rc-")
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/protoc-%s-%s.zip",
		protocGitHubRepo,
		version,
		short,
		platform,
	), nil
}
