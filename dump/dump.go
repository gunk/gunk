package dump

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/gunk/gunk/generate"
)

// Run will generate the FileDescriptorSet for a Gunk package, and
// output it as required.
func Run(format, dir string, patterns ...string) error {
	// Load the Gunk package and generate the FileDescriptorSet for the
	// Gunk package.
	fds, err := generate.FileDescriptorSet(dir, patterns...)
	if err != nil {
		return err
	}

	// Format the FileDescriptorSet.
	var bs []byte
	switch format {
	case "json":
		bs, err = json.Marshal(fds)
	case "", "raw":
		// The default format.
		bs, err = proto.Marshal(fds)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown output format %q", format)
	}

	// Otherwise, output to stdout
	_, err = os.Stdout.Write(bs)
	return err
}
