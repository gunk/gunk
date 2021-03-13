package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/log"
)

type GrpcPython struct{}

func (g GrpcPython) Name() string {
	return "grpc-python"
}

func (g GrpcPython) Download(version string, p Paths) (string, error) {
	log.Printf("Downloading and building grpc-python. This can take about 15 minutes.")
	if strings.HasPrefix(version, "0.") {
		return "", fmt.Errorf("cannot use 0.x version %s", version)
	}
	if _, err := exec.LookPath("cmake"); err != nil {
		return "", fmt.Errorf("cmake is not installed")
	}
	repoPath := `https://github.com/grpc/grpc`
	cmdArgs := []string{"clone", "--depth", "1", "--branch", version, repoPath, p.buildDir}
	log.Printf("Cloning main repo.")
	gitCmd := log.ExecCommand("git", cmdArgs...)
	err := gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}
	log.Printf("Cloning submodules.")
	cmdArgs = []string{"submodule", "foreach", `git config -f .gitmodules submodule.$sm_path.shallow true`}
	gitCmd = log.ExecCommand("git", cmdArgs...)
	gitCmd.Dir = p.buildDir
	err = gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}
	cmdArgs = []string{"submodule", "update", "--init", "--jobs=6"}
	gitCmd = log.ExecCommand("git", cmdArgs...)
	gitCmd.Dir = p.buildDir
	err = gitCmd.Run()
	if err != nil {
		all := "git " + strings.Join(cmdArgs, " ")
		return "", log.ExecError(all, err)
	}
	// remove .git to save space, but only after checking submodules
	rmCmd := log.ExecCommand("rm", "-rf", ".git")
	rmCmd.Dir = p.buildDir
	err = rmCmd.Run()
	if err != nil {
		all := "rm -rf .git"
		return "", log.ExecError(all, err)
	}
	log.Printf("Running cmake.")
	cmakeDir := filepath.Join(p.buildDir, "cmake", "build")
	if err := os.MkdirAll(cmakeDir, 0755); err != nil {
		return "", err
	}
	cmakeCmd := log.ExecCommand("cmake", "../..")
	cmakeCmd.Dir = cmakeDir
	err = cmakeCmd.Run()
	if err != nil {
		all := "cmake ../.."
		return "", log.ExecError(all, err)
	}
	log.Printf("Running make, building grpc_python_plugin")
	buildCmd := log.ExecCommand("make", "-j", "2", "grpc_python_plugin")
	buildCmd.Dir = cmakeDir
	err = buildCmd.Run()
	if err != nil {
		all := "make -j 2 grpc_python_plugin"
		return "", log.ExecError(all, err)
	}
	return path.Join(cmakeDir, "grpc_python_plugin"), nil
}
