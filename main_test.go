package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/goproxytest"
	"github.com/rogpeppe/go-internal/testscript"

	"github.com/gunk/gunk/generate"
)

var write = flag.Bool("w", false, "overwrite testdata output files")

func TestMain(m *testing.M) {
	if os.Getenv("TESTSCRIPT_COMMAND") == "" {
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
			"github.com/golang/protobuf/protoc-gen-go",
			"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway",
			"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger",
			"./docgen/",
			"./scopegen/",
			"github.com/Kunde21/pulpMd",
			"./testdata/protoc-gen-strict",
		)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			panic(err)
		}

		// Start the Go proxy server running for all tests.
		srv, err := goproxytest.NewServer("testdata/mod", "")
		if err != nil {
			log.Fatalf("cannot start proxy: %v", err)
		}
		proxyURL = srv.URL
	}

	os.Exit(testscript.RunMain(m, map[string]func() int{
		"gunk": main1,
	}))
}

var proxyURL string

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

var rxGeneratedFile = regexp.MustCompile(`\.pb.*\.go$`)

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
	home, err := filepath.Abs(filepath.Join(".cache", "home"))
	if err != nil {
		t.Fatal(err)
	}
	cachePath, err := userCachePath()
	if err != nil {
		t.Fatal(err)
	}

	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "scripts"),
		Setup: func(e *testscript.Env) error {
			e.Vars = append(e.Vars, "GOPROXY="+proxyURL)
			e.Vars = append(e.Vars, "GONOSUMDB=*")
			e.Vars = append(e.Vars, "HOME="+home)
			e.Vars = append(e.Vars, "CACHEPATH="+cachePath)
			return nil
		},
	})
}

// userCacheDir returns relative path of user cached data directory from home directory.
// We need to use relative path, because the absolute user cache directory will be different
// for each OS and in some tests we use a isolated $HOME to avoid caching.
func userCachePath() (string, error) {
	realUserCacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	realHomeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(realUserCacheDir, realHomeDir), nil
}
