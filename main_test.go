package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gunk/gunk/loader"
)

var write = flag.Bool("w", false, "overwrite testdata output files")

func TestMain(m *testing.M) {
	flag.Parse()

	// Don't put the binaries in a temporary directory to delete, as that
	// means we have to re-link them every single time. That's quite
	// expensive, at around half a second per 'go test' invocation.
	binDir, err := filepath.Abs(".bin")
	if err != nil {
		panic(err)
	}
	os.Setenv("GOBIN", binDir)
	os.Setenv("PATH", binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
	cmd := exec.Command("go", "install", "-ldflags=-w -s",
		"github.com/golang/protobuf/protoc-gen-go",
		"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway",
	)
	if err := cmd.Run(); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

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
	if *write {
		// don't check that the output files match
		return
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
