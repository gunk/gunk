package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"
	plugin_go "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/gunk/gunk/docgen/generate"
	"github.com/gunk/gunk/docgen/parser"
	"github.com/gunk/gunk/log"
	"github.com/gunk/gunk/plugin"
)

const modeAppend = "append"

func main() {
	plugin.RunMain(new(docPlugin))
}

type docPlugin struct{}

func (p *docPlugin) Generate(req *plugin_go.CodeGeneratorRequest) (*plugin_go.CodeGeneratorResponse, error) {
	var lang []string
	var mode, dir string
	if param := req.GetParameter(); param != "" {
		ps := strings.Split(param, ",")
		for _, p := range ps {
			s := strings.Split(p, "=")
			if len(s) != 2 {
				return nil, fmt.Errorf("could not parse parameter: %s", p)
			}
			k, v := s[0], s[1]
			switch k {
			case "lang":
				lang = append(lang, v)
			case "mode":
				if v != modeAppend {
					return nil, fmt.Errorf("unknown mode: %s", v)
				}
				mode = v
			case "out":
				dir = v
			default:
				return nil, fmt.Errorf("unknown parameter: %s", k)
			}
		}
	}

	// find the source by looping through the protofiles and finding the one that matches the FileToGenerate
	var source *parser.FileDescWrapper
	for _, f := range req.GetProtoFile() {
		if strings.Contains(f.GetName(), "all.proto") {
			for _, fileToGenerate := range req.FileToGenerate{
				if fileToGenerate == f.GetName(){
					source = &parser.FileDescWrapper{FileDescriptorProto: f}
					break
				}
			}
		}
	}
	if source == nil {
		return nil, fmt.Errorf("no file to generate")
	}
	base := filepath.Join(filepath.Dir(source.GetName()))
	source.DependencyMap = parser.GenerateDependencyMap(source.FileDescriptorProto,req.GetProtoFile())

	var buf bytes.Buffer
	pb, err := generate.Run(&buf, source, lang)
	if err != nil {
		return nil, fmt.Errorf("failed markdown generation: %v", err)
	}

	if mode == modeAppend {
		// Load content from existing all.md
		e, err := ioutil.ReadFile(filepath.Join(dir, "all.md"))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		buf = *bytes.NewBuffer(append(e, buf.Bytes()...))

		// Get existing entries from messages.pot
		if err := pb.AddFromFile(filepath.Join(dir, "messages.pot")); err != nil {
			return nil, err
		}
	}

	// execute pulpMd to inject code snippets for examples.
	cmd := log.ExecCommand("pulpMd", "--stdin=true")
	cmd.Stdin = &buf
	out, err := cmd.Output()
	if err != nil {
		return nil, log.ExecError("pulpMd", err)
	}
	buf = *bytes.NewBuffer(out)

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
