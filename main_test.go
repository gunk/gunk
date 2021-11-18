package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gunk/gunk/generate"
	"github.com/rogpeppe/go-internal/gotooltest"
	"github.com/rogpeppe/go-internal/testscript"
)

var write = flag.Bool("w", false, "overwrite testdata output files")

func TestMain(m *testing.M) {
	if os.Getenv("TESTSCRIPT_ON") == "" {
		flag.Parse()
		// Don't put the binaries in a temporary directory to delete, as that
		// means we have to re-link them every single time. That's quite
		// expensive, at around half a second per 'go test' invocation.
		binDir, err := filepath.Abs(".cache")
		if err != nil {
			panic(err)
		}
		os.Setenv("GOBIN", binDir)
		os.Setenv("PATH", binDir+string(filepath.ListSeparator)+os.Getenv("PATH"))
		cmd := exec.Command("go", "install", "-ldflags=-w -s",
			"./testdata/protoc-gen-strict",
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Printf("%+v\n", os.Environ())

			panic(err)
		}
	}
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"gunk": run,
	}))
}

func TestGenerate(t *testing.T) {
	t.Parallel()
	pkgs := []string{
		".", "./imported",
	}
	dir := filepath.Join("testdata", "generate")
	wantFiles, err := generatedFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	for path := range wantFiles {
		// make sure we're writing the files
		os.Remove(path)
	}
	if err := generate.Run(dir, pkgs...); err != nil {
		t.Fatal(err)
	}
	if *write {
		// don't check that the output files match
		return
	}
	gotFiles, err := generatedFiles(dir)
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

var rxGeneratedFile = regexp.MustCompile(`\.go|\.json|\.java|\.js$`)

func generatedFiles(dir string) (map[string]string, error) {
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
		if info.Name() == "tools.go" {
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

func TestScripts(t *testing.T) {
	t.Parallel()
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}

	goCache := filepath.Join(os.TempDir(), "gunk-test-go-cache")

	p := testscript.Params{
		Dir: filepath.Join("testdata", "scripts"),
		Setup: func(e *testscript.Env) error {
			cmd := exec.Command("cp",
				"testdata/prepare-go-sum/go.mod",
				"testdata/prepare-go-sum/go.sum",
				e.WorkDir)
			cmd.Stderr = os.Stderr
			if _, err := cmd.Output(); err != nil {
				return fmt.Errorf("failed to copy go.sum: %w", err)
			}

			e.Vars = append(e.Vars, "GONOSUMDB=*")
			e.Vars = append(e.Vars, "GUNK_CACHE_DIR="+cacheDir)
			e.Vars = append(e.Vars, "TESTSCRIPT_ON=on")

			e.Vars = append(e.Vars, "HOME="+goCache)
			return nil
		},
	}
	if err := gotooltest.Setup(&p); err != nil {
		t.Fatal(err)
	}
	testscript.Run(t, p)
}
