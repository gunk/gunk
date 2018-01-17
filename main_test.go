package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestGunk(t *testing.T) {
	pkgs := []string{"util", "util/imp"}
	outPaths := []string{
		"testdata/src/util/echo.pb.go",
		"testdata/src/util/types.pb.go",
		"testdata/src/util/imp/imp.pb.go",
	}
	orig := make(map[string]string)
	for _, outPath := range outPaths {
		orig[outPath] = mayReadFile(t, outPath)
		os.Remove(outPath)
	}
	if err := runPaths("testdata", pkgs...); err != nil {
		t.Fatal(err)
	}
	for _, outPath := range outPaths {
		want := orig[outPath]
		got := mayReadFile(t, outPath)
		if got != want {
			t.Fatalf("want:\n%s\ngot:\n%s", want, got)
		}
	}
}

func mayReadFile(t *testing.T, path string) string {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bs)
}
