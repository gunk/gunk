package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestGunk(t *testing.T) {
	matches, _ := filepath.Glob(filepath.Join("testdata", "src", "util", "*.pb.go"))
	orig := make(map[string]string)
	for _, outPath := range matches {
		orig[outPath] = readFile(t, outPath)
		os.Remove(outPath)
	}
	if err := runPkg("util", "testdata"); err != nil {
		t.Fatal(err)
	}
	for _, outPath := range matches {
		want := orig[outPath]
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
