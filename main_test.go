package main

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestGunk(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("testdata", "*.gunk"))
	if err != nil {
		t.Fatal(err)
	}
	for _, inPath := range matches {
		outPath := strings.Replace(inPath, ".gunk", ".pb.go", 1)
		want := readFile(t, outPath)
		if err := runPaths(inPath); err != nil {
			t.Fatal(err)
		}
		got := readFile(t, outPath)
		if got != want {
			t.Fatalf("want:\n%s\ngot:\n%s", want, got)
		}
	}
}

func readFile(t *testing.T, path string) string {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(bs)
}
