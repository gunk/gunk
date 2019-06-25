package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin_go "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/gunk/gunk/docgen/extract"
	"github.com/gunk/gunk/plugin"
)

func main() {
	plugin.RunMain(new(docPlugin))
}

type docPlugin struct{}

func (p *docPlugin) Generate(req *plugin_go.CodeGeneratorRequest) (*plugin_go.CodeGeneratorResponse, error) {
	// For now, we only care about all.proto
	var source *google_protobuf.FileDescriptorProto
	for _, f := range req.GetProtoFile() {
		if strings.Contains(f.GetName(), "all.proto") {
			source = f
			break
		}
	}

	if source == nil {
		return nil, fmt.Errorf("no file to generate")
	}

	dest := proto.String(filepath.Join(filepath.Dir(source.GetName()), "messages.pot"))

	e, err := extract.Run(source)
	if err != nil {
		return nil, err
	}

	return &plugin_go.CodeGeneratorResponse{
		File: []*plugin_go.CodeGeneratorResponse_File{
			{
				Name:    dest,
				Content: proto.String(e.String()),
			},
		},
	}, nil
}
