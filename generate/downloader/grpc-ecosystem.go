package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
)

type GrpcEcosystem struct {
	Type string
}

func (ged GrpcEcosystem) Name() string {
	return ged.Type
}

func (ged GrpcEcosystem) Download(version string, p Paths) (string, error) {
	if ged.Type == "swagger" {
		return "", fmt.Errorf("use protoc-gen-openapiv2 instead of protoc-gen-swagger")
	}
	url, err := ged.downloadURL(runtime.GOOS, runtime.GOARCH, version)
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
		return "", fmt.Errorf("download returns status 200")
	}
	dstFile, err := os.OpenFile(p.binary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o775)
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

func (ged GrpcEcosystem) downloadURL(os, arch, version string) (string, error) {
	if arch != "amd64" {
		if os != "darwin" {
			// can use rosetta
			return "", fmt.Errorf("only 64bit supported")
		}
	}
	const repo = "grpc-ecosystem/grpc-gateway"
	if os == "windows" {
		// TODO: any windows tests? :D
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/protoc-gen-%s-%s-%s-x86_64.exe",
			repo,
			version,
			ged.Type,
			version,
			os,
		), nil
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/protoc-gen-%s-%s-%s-x86_64",
		repo,
		version,
		ged.Type,
		version,
		os,
	), nil
}
