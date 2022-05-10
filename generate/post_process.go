package generate

import (
	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/loader"
	"mvdan.cc/gofumpt/format"
)

// postProcess processes the input file before writing to output file.
func postProcess(input []byte, gen config.Generator, mainPkgPath string, pkgs map[string]*loader.GunkPackage) ([]byte, error) {
	code := gen.Code()
	if code == "go" && gen.JSONPostProc {
		var err error
		input, err = jsonTagPostProcessor(input)
		if err != nil {
			return input, err
		}
		// gofumpt is not idempotent: https://github.com/mvdan/gofumpt/issues/132
		input, err = format.Source(input, format.Options{LangVersion: "1.14"})
		if err != nil {
			return input, err
		}
	}
	switch code {
	case "go", "grpc-gateway", "go-grpc":
		return format.Source(input, format.Options{LangVersion: "1.14"})
	case "js":
		if gen.FixPaths {
			return jsPathProcessor(input, mainPkgPath, pkgs)
		}
	case "ts":
		if gen.FixPaths {
			return tsPathProcessor(input, mainPkgPath, pkgs)
		}
	}
	return input, nil
}
