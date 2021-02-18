package downloader

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/log"
)

type Ts struct{}

func (g Ts) Name() string {
	return "ts"
}

func (g Ts) Download(version string, p Paths) (string, error) {
	version = strings.TrimPrefix(version, "v")

	if _, err := exec.LookPath("npm"); err != nil {
		return "", fmt.Errorf("node is not installed. See https://nodejs.org/en/download/")
	}

	if err := os.MkdirAll(p.buildDir, 0755); err != nil {
		return "", err
	}

	npmCmd := log.ExecCommand("npm", "init", "-y")
	npmCmd.Dir = p.buildDir

	err := npmCmd.Run()
	if err != nil {
		all := "npm init -y"
		return "", log.ExecError(all, err)
	}

	npmCmd = log.ExecCommand("npm", "install", "ts-protoc-gen@"+version)
	npmCmd.Dir = p.buildDir

	err = npmCmd.Run()
	if err != nil {
		all := "npm install ts-protoc-gen@" + version
		return "", log.ExecError(all, err)
	}

	// in order to be reproducible, install the *minimal* versions of everything
	// required by ts-protoc-gen (just google-protobuf?)
	type packageJSON struct {
		Dependencies map[string]string `json:"dependencies"`
	}

	protocJSONBytes, err := ioutil.ReadFile(filepath.Join(p.buildDir, "node_modules", "ts-protoc-gen", "package.json"))
	if err != nil {
		return "", fmt.Errorf("cannot read ts-protoc-gen package.json: %w", err)
	}

	var protocJSON packageJSON
	err = json.Unmarshal(protocJSONBytes, &protocJSON)
	if err != nil {
		return "", fmt.Errorf("cannot parse ts-protoc-gen package.json: %w", err)
	}

	for k, v := range protocJSON.Dependencies {
		if strings.HasPrefix(v, "^") {
			vv := strings.TrimPrefix(v, "^")
			npmCmd := log.ExecCommand("npm", "install", fmt.Sprintf("%s@%s", k, vv))
			npmCmd.Dir = p.buildDir

			err := npmCmd.Run()
			if err != nil {
				all := "npm install " + fmt.Sprintf("%s@%s", k, vv)
				return "", log.ExecError(all, err)
			}
		}
	}

	binaryPath := filepath.Join(p.buildDir, "node_modules", ".bin", "protoc-gen-ts")

	return binaryPath, nil
}
