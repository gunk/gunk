package generate

import (
	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/loader"
	"mvdan.cc/gofumpt/format"
)

// postProcess processes the input file before writing to output file.
func postProcess(input []byte, gen config.Generator, mainPkgPath string, pkgs map[string]*loader.GunkPackage) ([]byte, error) {
	code := gen.Code()
	if code == "go" {
		if gen.JSONPostProc {
			return jsonTagPostProcessor(input)
		}
	}

	if code == "js" {
		if gen.FixPaths {
			return jsPathProcessor(input, mainPkgPath, pkgs)
		}
	}

	if code == "ts" {
		if gen.FixPaths {
			return tsPathProcessor(input, mainPkgPath, pkgs)
		}
	}

	if code == "go" || code == "grpc-gateway" {
		return format.Source(input, format.Options{LangVersion: "1.14"})
	}

	return input, nil
}
