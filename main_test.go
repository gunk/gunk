package main

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestGunk(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("testdata", "*.pb.go"))
	if err != nil {
		t.Fatal(err)
	}
	orig := make(map[string]string)
	for _, outPath := range matches {
		orig[outPath] = readFile(t, outPath)
	}
	if err := runPkg("./testdata"); err != nil {
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
