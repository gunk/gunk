package convert

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/gunk/gunk/format"
	"github.com/gunk/gunk/loader"
)

// Run converts proto files or folders to gunk files, saving the files in
// the same folder as the proto file.
func Run(paths []string, overwrite bool) error {
	for _, path := range paths {
		if err := run(path, overwrite); err != nil {
			return err
		}
	}
	return nil
}

func run(path string, overwrite bool) error {

	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	// Determine whether the path is a file or a directory.
	// If it is a file convert the file.
	if !fi.IsDir() {
		return convertFile(path, overwrite)
	} else if filepath.Ext(path) == ".proto" {
		// If the path is a directory and has a .proto extension then error.
		return fmt.Errorf("%s is a directory, should be a proto file", path)
	}

	// Handle the case where it is a directory. Loop through
	// the files and if we have a .proto file attempt to
	// convert it.
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	for _, f := range files {
		// If the file is not a .proto file
		if f.IsDir() || filepath.Ext(f.Name()) != ".proto" {
			continue
		}
		if err := convertFile(filepath.Join(path, f.Name()), overwrite); err != nil {
			return err
		}
	}
	return nil
}

func convertFile(path string, overwrite bool) error {
	if filepath.Ext(path) != ".proto" {
		return fmt.Errorf("convert requires a .proto file")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to read file %q: %v", path, err)
	}
	defer file.Close()

	filename := filepath.Base(path)
	fileToWrite := strings.Replace(filename, ".proto", ".gunk", 1)
	fullpath := filepath.Join(filepath.Dir(path), fileToWrite)

	if _, err := os.Stat(fullpath); !os.IsNotExist(err) && !overwrite {
		return fmt.Errorf("path already exists %q, use --overwrite", fullpath)
	}

	w := &strings.Builder{}
	if err := loader.ConvertFromProto(w, file, filename); err != nil {
		return err
	}

	result := []byte(w.String())
	result, err = format.Source(result)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(fullpath, result, 0644); err != nil {
		return fmt.Errorf("unable to write to file %q: %v", fullpath, err)
	}
	return nil
}
