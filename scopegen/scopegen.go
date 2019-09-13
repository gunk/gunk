package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/gunk/gunk/plugin"
	"github.com/gunk/gunk/scopegen/generate"
	"github.com/gunk/gunk/scopegen/parser"
)

func main() {
	plugin.RunMain(new(scopePlugin))
}

type scopePlugin struct{}

func (s *scopePlugin) Generate(req *plugin_go.CodeGeneratorRequest) (*plugin_go.CodeGeneratorResponse, error) {
	const (
		jsonLangName = "json"
		goLangName   = "go"
	)
	var (
		langs   = make(map[string]struct{})
		baseDir string
	)

	if param := req.GetParameter(); param != "" {
		ps := strings.Split(param, ",")
		for _, p := range ps {
			s := strings.Split(p, "=")
			if len(s) != 2 {
				return nil, fmt.Errorf("could not parse parameter: %s", p)
			}
			k, v := s[0], s[1]
			switch k {
			case jsonLangName, goLangName:
				boolVal, err := strconv.ParseBool(v)
				if err != nil {
					return nil, err
				}
				if boolVal {
					langs[k] = struct{}{}
				}
			case "out":
				baseDir = v
			default:
				return nil, fmt.Errorf("unknown parameter: %s", k)
			}
		}
	}

	// find the source by looping through the proto files and finding the one that matches the FileToGenerate
	var f *descriptor.FileDescriptorProto
	for _, descriptorProto := range req.GetProtoFile() {
		if strings.Contains(descriptorProto.GetName(), "all.proto") {
			for _, fileToGenerate := range req.FileToGenerate {
				if fileToGenerate == descriptorProto.GetName() {
					f = descriptorProto
					break
				}
			}
		}
	}
	if f == nil {
		return nil, fmt.Errorf("no file to generate")
	}

	parsed, err := parser.ParseFile(f)
	if err != nil {
		return nil, err
	}

	if baseDir == "" {
		baseDir = filepath.Dir(f.GetName())
	}

	var resp = &plugin_go.CodeGeneratorResponse{}

	for lang := range langs {
		var buf = bytes.NewBuffer(nil)
		switch lang {
		case jsonLangName:
			if err = generate.JSON(buf, parsed); err != nil {
				return nil, fmt.Errorf("failed to generate scopes: lang=%s err=%s", lang, err.Error())
			}
		case goLangName:
			if err = generate.Go(buf, parsed); err != nil {
				return nil, fmt.Errorf("failed to generate scopes: lang=%s err=%s", lang, err.Error())
			}
		default:
			// this should never be reached, but be defensive
			return nil, fmt.Errorf("unsupported language: %s", lang)
		}
		resp.File = append(resp.File, newCodeGeneratorFile(baseDir, lang, buf.String()))
	}

	return resp, nil
}

func newCodeGeneratorFile(baseDir, lang, content string) *plugin_go.CodeGeneratorResponse_File {
	return &plugin_go.CodeGeneratorResponse_File{
		Name:    proto.String(filepath.Join(baseDir, "all.scopes."+lang)),
		Content: proto.String(content),
	}
}
