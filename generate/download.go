package generate

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

	"github.com/rogpeppe/go-internal/lockedfile"

	"github.com/gunk/gunk/log"
)

const defaultProtocVersion = "v3.9.1"

// CheckOrDownloadProtoc downloads protoc to the specified path, unless it's already
// been downloaded. If no path is provided, it uses an OS-appropriate user cache.
// If the version is not specified, the latest version is fetched from GitHub.
// If both version and path are specified and a file already exists at the path,
// it checks whether the output of `protoc --version` is an exact match.
func CheckOrDownloadProtoc(path, version string) (string, error) {
	if version == "" {
		version = defaultProtocVersion
	}

	dstPath := path
	if dstPath == "" {
		// Get the os specific cache directory.
		cachePath, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		cacheDir := filepath.Join(cachePath, "gunk")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return "", err
		}

		// The proto command path to use or download to.
		dstPath = filepath.Join(cacheDir, fmt.Sprintf("protoc-%s", version))
	}

	// Check the cache path to see if it has been previously
	// downloaded by gunk.
	dstFile, err := lockedfile.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0775)
	if os.IsExist(err) {
		if err := verifyProtocBinary(dstPath, version); err != nil {
			return "", err
		}
		return dstPath, nil
	}
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	url, err := protocDownloadURL(runtime.GOOS, runtime.GOARCH, version)
	if err != nil {
		return "", fmt.Errorf("downloading protoc: %v", err)
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
	if gotVersion != version[1:] {
		return fmt.Errorf("want protoc version %q got %q", version, gotVersion)
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

	return fmt.Sprintf("https://github.com/%s/releases/download/%s/protoc-%s-%s.zip",
		protocGitHubRepo,
		version,
		version[1:], // the version string is guaranteed to starts with "v", removing it
		platform,
	), nil
}
