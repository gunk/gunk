package generate

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/gunk/gunk/log"
	"github.com/rogpeppe/go-internal/lockedfile"
)

const protocGitHubRepo = "ProtocolBuffers/protobuf"

// CheckOrDownloadProtoc downloads protoc to a cache folder, unless it's already
// been downloaded. The user can pin a version in their config; otherwise, the
// latest version is fetched from GitHub.
func CheckOrDownloadProtoc() (string, error) {
	// Get the os specific cache directory.
	cachePath, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	cacheDir := filepath.Join(cachePath, "gunk")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	version := "latest"
	// TODO: support specifying the version in the config

	// The proto command path to use or download to.
	dstPath := filepath.Join(cacheDir, fmt.Sprintf("protoc-%s", version))

	// Check the cache path to see if it has been previously
	// downloaded by gunk.
	dstFile, err := lockedfile.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0775)
	if os.IsExist(err) {
		// already exists; nothing to do
		return dstPath, nil
	}
	if err != nil {
		return "", err
	}
	defer dstFile.Close()

	url, err := protocDownloadURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
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
		return dstPath, nil
	}
	return "", fmt.Errorf("unable to download and extract protoc")
}

// githubAsset wraps asset information for a github release.
type githubAsset struct {
	BrowserDownloadURL string `json:"browser_download_url"`
	Name               string `json:"name"`
	ContentType        string `json:"content_type"`
}

// githubLatestAssets retrieves the latest release assets from the named repo.
func githubLatestAssets(repo string) (string, []githubAsset, error) {
	urlstr := "https://api.github.com/repos/" + repo + "/releases/latest"

	// create request
	req, err := http.NewRequest("GET", urlstr, nil)
	if err != nil {
		return "", nil, err
	}

	// do request
	cl := &http.Client{}
	res, err := cl.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer res.Body.Close()

	var release struct {
		Name   string        `json:"name"`
		Assets []githubAsset `json:"assets"`
	}
	if err := json.NewDecoder(res.Body).Decode(&release); err != nil {
		return "", nil, err
	}

	return release.Name, release.Assets, nil
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
// See: https://github.com/protocolbuffers/protobuf/releases/latest
func protocDownloadURL(os, arch string) (string, error) {
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

	// retrieve the latest release assets
	ver, assets, err := githubLatestAssets(protocGitHubRepo)
	if err != nil {
		return "", err
	}

	// find the asset
	nameRE := regexp.MustCompile(`^protoc-[0-9]+\.[0-9]+\.[0-9]+-` + platform + `\.zip$`)
	for _, asset := range assets {
		if nameRE.MatchString(asset.Name) {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("could not find platform release asset for %q", ver)
}
