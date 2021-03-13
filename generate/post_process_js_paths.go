package generate

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/loader"
)

func pathToRoot(r string) string {
	s := strings.Split(r, "/")
	for i := range s {
		s[i] = ".."
	}
	return strings.Join(s, "/")
}

func pathFromTo(from, to string) string {
	if from == to {
		return "."
	}
	join := func(p []string) string {
		if len(p) == 0 {
			return "/"
		}
		return "/" + strings.Join(p, "/") + "/"
	}
	fromSplit := strings.Split(filepath.Clean(from), "/")
	toSplit := strings.Split(filepath.Clean(to), "/")
	var path []string
	// first go up
	for !strings.HasPrefix(join(toSplit), join(fromSplit)) {
		fromSplit = fromSplit[0 : len(fromSplit)-1]
		path = append(path, "..")
	}
	// then remove the prefix from toPath
	toWithout := strings.TrimPrefix(join(toSplit), join(fromSplit))
	k := "./" + filepath.Clean(strings.Join(path, "/")+"/"+toWithout)
	k = strings.ReplaceAll(k, "//", "/")
	return k
}

func jsPathProcessor(input []byte, mainPkgPath string, pkgs map[string]*loader.GunkPackage) ([]byte, error) {
	lines := bytes.Split(input, []byte{'\n'})
	fLines := make([][]byte, 0, len(lines))
LINES:
	for _, l := range lines {
		if bytes.Contains(l, []byte("annotations_pb")) {
			continue LINES
		}
		if !bytes.Contains(l, []byte("require('./")) {
			fLines = append(fLines, l)
			continue LINES
		}
		for pkgPath, pkg := range pkgs {
			require := []byte(fmt.Sprintf("require('./%s/all_pb.js')", pkgPath))
			if bytes.Contains(l, require) {
				thisPkgDir := pkgs[mainPkgPath].Dir
				otherPkgDir := pkg.Dir
				replacement := []byte(fmt.Sprintf("require('%s/all_pb.js')", pathFromTo(thisPkgDir, otherPkgDir)))
				l = bytes.ReplaceAll(l, require, replacement)
			}
		}
		fLines = append(fLines, l)
	}
	return bytes.Join(fLines, []byte{'\n'}), nil
}

func tsPathProcessor(input []byte, mainPkgPath string, pkgs map[string]*loader.GunkPackage) ([]byte, error) {
	lines := bytes.Split(input, []byte{'\n'})
	fLines := make([][]byte, 0, len(lines))
LINES:
	for _, l := range lines {
		if bytes.Contains(l, []byte("annotations_pb")) {
			continue LINES
		}
		if !(bytes.Contains(l, []byte("require(\"../")) || bytes.Contains(l, []byte(" from \".."))) {
			fLines = append(fLines, l)
			continue LINES
		}
		for pkgPath, pkg := range pkgs {
			require := []byte(fmt.Sprintf("require(\"%s/%s/all_pb\")", pathToRoot(mainPkgPath), pkgPath))
			if bytes.Contains(l, require) {
				thisPkgDir := pkgs[mainPkgPath].Dir
				otherPkgDir := pkg.Dir
				replacement := []byte(fmt.Sprintf("require(\"%s/all_pb\")", pathFromTo(thisPkgDir, otherPkgDir)))
				l = bytes.ReplaceAll(l, require, replacement)
			}
			impor := []byte(fmt.Sprintf(" from \"%s/%s/all_pb\"", pathToRoot(mainPkgPath), pkgPath))
			if bytes.Contains(l, impor) {
				thisPkgDir := pkgs[mainPkgPath].Dir
				otherPkgDir := pkg.Dir
				replacement := []byte(fmt.Sprintf(" from \"%s/all_pb\"", pathFromTo(thisPkgDir, otherPkgDir)))
				l = bytes.ReplaceAll(l, impor, replacement)
			}
		}
		fLines = append(fLines, l)
	}
	return bytes.Join(fLines, []byte{'\n'}), nil
}
