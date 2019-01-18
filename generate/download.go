package generate

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/gunk/gunk/log"
)

const (
	protocVersion = "3.6.1"
	// https://github.com/protocolbuffers/protobuf/releases/download/v3.6.1/protoc-3.6.1-linux-x86_32.zip
	protocURL = "https://github.com/protocolbuffers/protobuf/releases/download/v%s/protoc-%s-%s.zip"
)

// checkOrDownloadProtoc will check the $PATH for 'protoc', if it doesn't
// exist it checks to see if it has previously been downloaded to the
// gunk cache, if not download protoc.
func checkOrDownloadProtoc() (string, error) {
	// First check $PATH for protoc
	if path, err := exec.LookPath("protoc"); err == nil && path != "" {
		return path, nil
	}

	// Get the os specific cache directory.
	cachePath, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	gunkCachePath := filepath.Join(cachePath, "gunk")
	// Make the gunk cache path.
	if err := os.MkdirAll(gunkCachePath, 0755); err != nil {
		return "", err
	}

	// The proto command path to use or download to.
	protocCachePath := filepath.Join(gunkCachePath, "protoc")

	// Check the cache path to see if it has been previously
	// downloaded by gunk.
	if _, err := os.Stat(protocCachePath); err == nil {
		return protocCachePath, nil
	}

	if err := downloadAndExtractProtoc(protocCachePath); err != nil {
		return "", err
	}
	log.DownloadedProtoc(protocCachePath)
	return protocCachePath, nil
}

// downloadAndExtractProtoc will download protoc, and extract the protoc
// binary to the specified location on disk.
func downloadAndExtractProtoc(protocCachePath string) error {
	url, err := protocDownloadURL()
	if err != nil {
		return err
	}

	// Download protoc since we were unable to find a usable
	// protoc installation.
	cl := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	res, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return fmt.Errorf("could not retrieve %q (%d)", url, res.StatusCode)
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// Check the contents of the zipped folder for the protoc
	// binary.
	buf := bytes.NewReader(b)
	rdr, err := zip.NewReader(buf, res.ContentLength)
	if err != nil {
		return err
	}

	// Search in the zip download for the 'protoc' command.
	for _, f := range rdr.File {
		if f.Name != "bin/protoc" {
			continue
		}

		fc, err := f.Open()
		if err != nil {
			return err
		}
		defer fc.Close()

		b, err := ioutil.ReadAll(fc)
		if err != nil {
			return err
		}

		// Write protoc command to cache.
		if err = ioutil.WriteFile(protocCachePath, b, f.FileInfo().Mode()); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unable to download and extract protoc")
}

// protoDownloadURL will generate a protoc url to download for the current
// GOOS and GOARCH. The following are the protoc supported GOOS + GOARCH
// combinations:
// 	- linux-aarch64
// 	- linux-x86_32
// 	- linux-x86_64
// 	- osx-x86_32
// 	- osx-x86_64
// 	- win32
func protocDownloadURL() (string, error) {

	// Translate the arch to what the protoc download expects.
	var arch string
	switch a := runtime.GOARCH; a {
	case "386":
		arch = "x86_32"
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}

	// Verify and translate the os to what protoc download expects.
	// Check that we have a valid GOOS + GOARH combintation.
	var osAndArch string
	switch o := runtime.GOOS; o {
	case "darwin":
		switch arch {
		case "x86_32", "x86_64":
			osAndArch = "osx-" + arch
		default:
			return "", fmt.Errorf("%q is not a supported arch for darwin protoc download", arch)
		}
	case "windows":
		if arch != "x86_32" {
			return "", fmt.Errorf("%q is not a supported arch for windows protoc download", arch)
		}
		osAndArch = "win32"
	case "linux":
		switch arch {
		case "x86_32", "x86_64", "aarch64":
			osAndArch = "linux-" + arch
		default:
			return "", fmt.Errorf("%q is not a supported arch for linux protoc download", arch)
		}
	default:
		return "", fmt.Errorf("unsupported os %q for protoc download", o)
	}
	return fmt.Sprintf(protocURL, protocVersion, protocVersion, osAndArch), nil
}
