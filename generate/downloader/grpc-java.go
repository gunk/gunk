package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

type GrpcJava struct{}

func (pd GrpcJava) Name() string {
	return "grpc-java"
}

func (pd GrpcJava) Download(version string, p Paths) (string, error) {
	// The file does not exist. Download it, using dstFile.
	url, err := pd.downloadURL(runtime.GOOS, runtime.GOARCH, version)
	if err != nil {
		return "", err
	}
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
	dstFile, err := os.OpenFile(p.binary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0775)
	if err != nil {
		return "", err
	}
	defer dstFile.Close()
	// Write command to cache.
	if _, err := io.Copy(dstFile, res.Body); err != nil {
		return "", err
	}
	if err := dstFile.Close(); err != nil {
		return "", err
	}
	return p.binary, nil
}

func (GrpcJava) downloadURL(os, arch, version string) (string, error) {
	if !strings.HasPrefix(version, "v") {
		return "", fmt.Errorf("invalid version: %s", version)
	}
	version = version[1:]
	// determine the platform
	var platform string
	switch {
	case os == "darwin" && arch == "386":
		return "", fmt.Errorf("macOS 386 not supported")
	case os == "darwin" && arch == "amd64":
		platform = "osx-x86_64"
	case os == "linux" && arch == "386":
		platform = "linux-x86_32"
	case os == "linux" && arch == "amd64":
		platform = "linux-x86_64"
	case os == "linux" && arch == "arm64":
		platform = "linux-aarch_64"
	case os == "windows" && arch == "386":
		platform = "windows-x86_32"
	case os == "windows" && arch == "amd64":
		platform = "windows-x86_64"
	default:
		return "", fmt.Errorf("unknown os %q and arch %q", os, arch)
	}
	const mavenRepo = `https://repo1.maven.org/maven2/io/grpc/protoc-gen-grpc-java`
	return fmt.Sprintf("%s/%s/protoc-gen-grpc-java-%s-%s.exe",
		mavenRepo,
		version,
		version,
		platform), nil
}
