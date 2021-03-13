package downloader

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gunk/gunk/log"
)

type Go struct{}

func (g Go) Name() string {
	return "go"
}

func (g Go) Download(version string, p Paths) (string, error) {
	repoPath, cmdBuildPath, err := g.repoPath(version)
	if err != nil {
		return "", err
	}
	cmdBuildPath = append([]string{p.buildDir}, cmdBuildPath...)
	binaryDir := filepath.Join(cmdBuildPath...)
	binaryPath := filepath.Join(binaryDir, "protoc-gen-go")
	cmdArgs := []string{"clone", "--depth", "1", "--branch", version, repoPath, p.buildDir}
	gitCmd := log.ExecCommand("git", cmdArgs...)
	err = gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}
	// protoc-gen-go versions 1.3.0 and older do not have go modules
	// so compilation would load new version of submodules and produce a hybrid
	// we need to init modules
	goModPath := path.Join(p.buildDir, "go.mod")
	_, fErr := os.Stat(goModPath)
	if fErr != nil {
		if !os.IsNotExist(fErr) {
			return "", fErr
		}
		goModInitCmd := log.ExecCommand("go", "mod", "init", "google.golang.org/protobuf")
		goModInitCmd.Dir = p.buildDir
		err = goModInitCmd.Run()
		if err != nil {
			all := "go mod init google.golang.org/protobuf"
			return "", log.ExecError(all, err)
		}
	}
	buildCmd := log.ExecCommand("go", "build")
	buildCmd.Dir = binaryDir
	err = buildCmd.Run()
	if err != nil {
		all := "go build"
		return "", log.ExecError(all, err)
	}
	return binaryPath, nil
}

func (Go) repoPath(version string) (string, []string, error) {
	version = strings.TrimPrefix(version, "v")
	split := strings.Split(version, ".")
	if len(split) != 3 {
		return "", nil, fmt.Errorf("cannot interpret %q as version number: not 3 parts", split)
	}
	major, err := strconv.Atoi(split[0])
	if err != nil {
		return "", nil, fmt.Errorf("cannot interpret %q as version number: major not a number", split)
	}
	if major > 1 {
		return `https://github.com/protocolbuffers/protobuf-go`, []string{`cmd`, `protoc-gen-go`}, nil
	}
	return "", nil, fmt.Errorf("unsupported version %q", version)
}
