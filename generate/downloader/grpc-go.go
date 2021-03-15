package downloader

import (
	"os"
	"path/filepath"

	"github.com/gunk/gunk/log"
)

type GrpcGo struct{}

func (pd GrpcGo) Name() string {
	return "grpc-go"
}

func (pd GrpcGo) Download(version string, p Paths) (string, error) {
	if err := os.MkdirAll(p.buildDir, 0o755); err != nil {
		return "", err
	}

	buildCmd := log.ExecCommand(
		"go",
		"install",
		"google.golang.org/grpc/cmd/protoc-gen-go-grpc@"+version)
	buildCmd.Dir = p.buildDir
	buildCmd.Env = append(buildCmd.Env,
		"GOBIN="+p.buildDir,
		"GOPATH="+os.Getenv("GOPATH"),
		"HOME="+os.Getenv("HOME"),
	)
	err := buildCmd.Run()
	if err != nil {
		all := "GOBIN=" + p.buildDir + " go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@" + version
		return "", log.ExecError(all, err)
	}

	return filepath.Join(p.buildDir, "protoc-gen-go-grpc"), nil
}
