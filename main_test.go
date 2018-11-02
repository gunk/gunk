package main

import (
	"flag"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

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

	os.Exit(testscript.RunMain(m, map[string]func() int{
		"gunk": main1,
	}))
}

func TestGenerate(t *testing.T) {
	pkgs := []string{
		".", "./imported",
	}
	dir := filepath.Join("testdata", "generate")
	wantFiles, err := generatedFiles(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	for path := range wantFiles {
		// make sure we're writing the files
		os.Remove(path)
	}

	if err := loader.Load(dir, pkgs...); err != nil {
		t.Fatal(err)
	}
	if *write {
		// don't check that the output files match
		return
	}

	gotFiles, err := generatedFiles(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	for path, got := range gotFiles {
		want := wantFiles[path]
		if got != want {
			t.Errorf("%s was modified", path)
		}
		delete(wantFiles, path)
	}
	for path := range wantFiles {
		t.Errorf("%s was not generated", path)
	}
	if testing.Short() {
		// the build shouldn't have broken if no files have changed
		return
	}
	cmd := exec.Command("go", append([]string{"build"}, pkgs...)...)
	cmd.Dir = dir
	if _, err := cmd.Output(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s", e.Stderr)
		}
		t.Fatalf("%v", err)
	}
}

var rxGeneratedFile = regexp.MustCompile(`\.pb.*\.go$`)

func generatedFiles(t *testing.T, dir string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !rxGeneratedFile.MatchString(info.Name()) {
			return nil
		}
		bs, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = string(bs)
		return nil
	})
	return files, err
}

func TestGenerateError(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "scripts"),
	})
}
