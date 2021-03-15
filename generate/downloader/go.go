package downloader

import (
	"os"
	"path/filepath"

	"github.com/gunk/gunk/log"
)

type Go struct{}

func (g Go) Name() string {
	return "go"
}

func (pd Go) Download(version string, p Paths) (string, error) {
	if err := os.MkdirAll(p.buildDir, 0o755); err != nil {
		return "", err
	}

	buildCmd := log.ExecCommand(
		"go",
		"install",
		"google.golang.org/protobuf/cmd/protoc-gen-go@"+version)
	buildCmd.Dir = p.buildDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = append(buildCmd.Env,
		"GOBIN="+p.buildDir,
		"GOPATH="+os.Getenv("GOPATH"),
		"HOME="+os.Getenv("HOME"),
	)
	err := buildCmd.Run()
	if err != nil {
		all := "GOBIN=" + p.buildDir + " go install google.golang.org/protobuf/cmd/protoc-gen-go@" + version
		return "", log.ExecError(all, err)
	}

	return filepath.Join(p.buildDir, "protoc-gen-go"), nil
}
