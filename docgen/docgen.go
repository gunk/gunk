package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin_go "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/gunk/gunk/docgen/generate"
	"github.com/gunk/gunk/plugin"
)

func main() {
	plugin.RunMain(new(docPlugin))
}

type docPlugin struct{}

func (p *docPlugin) Generate(req *plugin_go.CodeGeneratorRequest) (*plugin_go.CodeGeneratorResponse, error) {
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

	base := filepath.Join(filepath.Dir(source.GetName()))

	var buf bytes.Buffer
	pb, err := generate.Run(&buf, source)
	if err != nil {
		return nil, fmt.Errorf("failed markdown generation: %v", err)
	}

	return &plugin_go.CodeGeneratorResponse{
		File: []*plugin_go.CodeGeneratorResponse_File{
			{
				Name:    proto.String(filepath.Join(base, "messages.pot")),
				Content: proto.String(pb.String()),
			},
			{
				Name:    proto.String(filepath.Join(base, "all.md")),
				Content: proto.String(buf.String()),
			},
		},
	}, nil
}
