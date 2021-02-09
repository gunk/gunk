package generate

import (
	"github.com/gunk/gunk/config"
)

// postProcess processes the input file before writing to output file.
func postProcess(input []byte, gen config.Generator) ([]byte, error) {
	if gen.IsGo() {
		if gen.JSONPostProc {
			return jsonTagPostProcessor(input)
		}
	}

	return input, nil
}
