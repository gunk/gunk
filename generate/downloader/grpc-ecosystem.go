package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gunk/gunk/log"
)

type GrpcEcosystem struct {
	Type string
}

func (ged GrpcEcosystem) Name() string {
	return ged.Type
}

func (ged GrpcEcosystem) Download(version string, p Paths) (string, error) {
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
		// old versions do not have releases, try to build
		b, err := ged.buildGithub(version, p)
		if err != nil {
			return "", err
		}
		return b, nil
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

func (ged GrpcEcosystem) buildGithub(version string, p Paths) (string, error) {
	const repoPath = `https://github.com/grpc-ecosystem/grpc-gateway`

	cmdArgs := []string{"clone", "--depth", "1", "--branch", version, repoPath, p.buildDir}

	gitCmd := log.ExecCommand("git", cmdArgs...)

	err := gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}

	// older version don't even have go.mod
	goModPath := path.Join(p.buildDir, "go.mod")
	_, fErr := os.Stat(goModPath)
	if fErr != nil {
		if !os.IsNotExist(fErr) {
			return "", fErr
		}
		goModInitCmd := log.ExecCommand("go", "mod", "init", "github.com/grpc-ecosystem/grpc-gateway")
		goModInitCmd.Dir = p.buildDir
		err = goModInitCmd.Run()
		if err != nil {
			all := "go mod init github.com/grpc-ecosystem/grpc-gateway"
			return "", log.ExecError(all, err)
		}
	}

	binaryDir := filepath.Join(p.buildDir, fmt.Sprintf("protoc-gen-%s", ged.Type))

	buildCmd := log.ExecCommand("go", "build")
	buildCmd.Dir = binaryDir

	err = buildCmd.Run()
	if err != nil {
		all := "go build"
		return "", log.ExecError(all, err)
	}

	return filepath.Join(binaryDir, fmt.Sprintf("protoc-gen-%s", ged.Type)), err
}

func (ged GrpcEcosystem) downloadURL(os, arch, version string) (string, error) {
	if arch != "amd64" {
		return "", fmt.Errorf("only 64bit supported")
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
