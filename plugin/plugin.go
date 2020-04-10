package plugin

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/gogo/protobuf/proto"
	plugin_go "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/gunk/gunk/protoutil"
)

// Plugin provides plugin method.
type Plugin interface {
	Generate(req *plugin_go.CodeGeneratorRequest) (*plugin_go.CodeGeneratorResponse, error)
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

func readRequest(r io.Reader) (*plugin_go.CodeGeneratorRequest, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	req := new(plugin_go.CodeGeneratorRequest)
	if err = proto.Unmarshal(data, req); err != nil {
		return nil, err
	}

	if len(req.GetFileToGenerate()) == 0 {
		return nil, fmt.Errorf("no files were supplied to the generator")
	}

	return req, nil
}

func writeResponse(w io.Writer, resp *plugin_go.CodeGeneratorResponse) error {
	data, err := protoutil.MarshalDeterministic(resp)
	if err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}

	return nil
}
