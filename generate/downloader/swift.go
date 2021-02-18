package downloader

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/log"
)

type Swift struct{}

func (g Swift) Name() string {
	return "swift"
}

func (g Swift) Download(version string, p Paths) (string, error) {
	version = strings.TrimPrefix(version, "v")

	if _, err := exec.LookPath("swift"); err != nil {
		return "", fmt.Errorf("swift is not installed. See https://swift.org/download/")
	}

	repoPath := `https://github.com/apple/swift-protobuf`

	binaryPath := filepath.Join(p.buildDir, ".build", "release", "protoc-gen-swift")

	cmdArgs := []string{"clone", "--depth", "1", "--branch", version, repoPath, p.buildDir}

	gitCmd := log.ExecCommand("git", cmdArgs...)

	err := gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}

	buildCmd := log.ExecCommand("swift", "build", "-c", "release")
	buildCmd.Dir = p.buildDir

	var stderr bytes.Buffer
	buildCmd.Stderr = &stderr

	err = buildCmd.Run()
	if err != nil {
		all := "swift build -c release"
		return "", log.ExecError(all, err)
	}

	return binaryPath, nil
}
