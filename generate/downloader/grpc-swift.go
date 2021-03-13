package downloader

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/log"
)

type GrpcSwift struct{}

func (g GrpcSwift) Name() string {
	return "grpc-swift"
}

func (g GrpcSwift) Download(version string, p Paths) (string, error) {
	version = strings.TrimPrefix(version, "v")
	if strings.HasPrefix(version, "0.") {
		return "", fmt.Errorf("cannot use 0.x version %s", version)
	}
	if _, err := exec.LookPath("swift"); err != nil {
		return "", fmt.Errorf("swift is not installed. See https://swift.org/download/")
	}
	repoPath := `https://github.com/grpc/grpc-swift`
	binaryPath := filepath.Join(p.buildDir, "protoc-gen-grpc-swift")
	cmdArgs := []string{"clone", "--depth", "1", "--branch", version, repoPath, p.buildDir}
	gitCmd := log.ExecCommand("git", cmdArgs...)
	err := gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}
	buildCmd := log.ExecCommand("make", "plugins")
	buildCmd.Dir = p.buildDir
	var stderr bytes.Buffer
	buildCmd.Stderr = &stderr
	err = buildCmd.Run()
	if err != nil {
		all := "make plugins"
		return "", log.ExecError(all, err)
	}
	return binaryPath, nil
}
