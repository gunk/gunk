// Plugin provides a helper methods to build custom protoc code generators.
// It is used for gunk-related repos, but can be used outside of gunk.
package plugin

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// Plugin provides plugin method.
type Plugin interface {
	Generate(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error)
}

func RunMain(p Plugin) {
	req, err := readRequest(os.Stdin)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	resp, err := p.Generate(req)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeResponse(os.Stdout, resp); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}

func readRequest(r io.Reader) (*pluginpb.CodeGeneratorRequest, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	req := new(pluginpb.CodeGeneratorRequest)
	if err = proto.Unmarshal(data, req); err != nil {
		return nil, err
	}
	if len(req.GetFileToGenerate()) == 0 {
		return nil, fmt.Errorf("no files were supplied to the generator")
	}
	return req, nil
}

func writeResponse(w io.Writer, resp *pluginpb.CodeGeneratorResponse) error {
	data, err := proto.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}
