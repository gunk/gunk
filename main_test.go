package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/gunk/gunk/loader"
)

func TestGunk(t *testing.T) {
	pkgs := []string{
		".", "./imported",
	}
	outPaths := []string{
		"testdata/echo.pb.go",
		"testdata/types.pb.go",
		"testdata/imp-arg/imp.pb.go",
		"testdata/imp-noarg/imp.pb.go",
		"testdata/imp-noarg/imp.pb.go",
	}
	orig := make(map[string]string)
	for _, outPath := range outPaths {
		orig[outPath] = mayReadFile(outPath)
		// make sure we're writing the files
		os.Remove(outPath)
	}
	if err := loader.Load("testdata", pkgs...); err != nil {
		t.Fatal(err)
	}
	for _, outPath := range outPaths {
		want := orig[outPath]
		got := mayReadFile(outPath)
		if got != want {
			t.Fatalf("%s was modified", outPath)
		}
	}
	cmd := exec.Command("go", append([]string{"build"}, pkgs...)...)
	cmd.Dir = "testdata"
	if _, err := cmd.Output(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s", e.Stderr)
		}
		t.Fatalf("%v", err)
	}
}

func mayReadFile(path string) string {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bs)
}
