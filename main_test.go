package main

import (
	"go/build"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGunk(t *testing.T) {
	// TODO: this test likely won't pass on windows
	pkgs := []string{
		"util", "util/imp-arg",
		"github.com/gunk/opt", "github.com/gunk/opt/http",
	}
	outPaths := []string{
		"testdata/src/util/echo.pb.go",
		"testdata/src/util/types.pb.go",
		"testdata/src/util/imp-arg/imp.pb.go",
		"testdata/src/util/imp-noarg/imp.pb.go",
		"testdata/src/util/imp-noarg/imp.pb.go",
		"testdata/src/github.com/gunk/opt/opt.pb.go",
		"testdata/src/github.com/gunk/opt/http/http.pb.go",
	}
	orig := make(map[string]string)
	for _, outPath := range outPaths {
		orig[outPath] = mayReadFile(t, outPath)
		os.Remove(outPath)
	}
	gopath, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	if err := runPaths(gopath, pkgs...); err != nil {
		t.Fatal(err)
	}
	for _, outPath := range outPaths {
		want := orig[outPath]
		got := mayReadFile(t, outPath)
		if got != want {
			t.Fatalf("%s was modified", outPath)
		}
	}
	if testing.Short() {
		t.Skip(`skipping "go build" check in short mode`)
	}
	cmd := exec.Command("go", append([]string{"build"}, pkgs...)...)
	cmd.Env = []string{"GOPATH=" + gopath + ":" + build.Default.GOPATH}
	if _, err := cmd.Output(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s", e.Stderr)
		}
		t.Fatalf("%v", err)
	}
}

func mayReadFile(t *testing.T, path string) string {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bs)
}
