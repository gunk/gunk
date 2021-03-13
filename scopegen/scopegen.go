package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gunk/gunk/plugin"
	"github.com/gunk/gunk/scopegen/generate"
	"github.com/gunk/gunk/scopegen/parser"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
	"mvdan.cc/gofumpt/format"
)

func main() {
	plugin.RunMain(new(scopePlugin))
}

type scopePlugin struct{}

func (s *scopePlugin) Generate(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	const (
		jsonLangName         = "json"
		goLangName           = "go"
		defaultOutputVersion = 1
	)
	var (
		langs         = make(map[string]struct{})
		baseDir       string
		outputVersion int
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
			case "output_version":
				var err error
				if outputVersion, err = strconv.Atoi(v); err != nil {
					return nil, fmt.Errorf("unknown output version: err=%s", err.Error())
				}
			default:
				return nil, fmt.Errorf("unknown parameter: %s", k)
			}
		}
	}
	// find the source by looping through the proto files and finding the one that matches the FileToGenerate
	var f *descriptorpb.FileDescriptorProto
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
	if outputVersion == 0 {
		outputVersion = defaultOutputVersion
	}
	resp := &pluginpb.CodeGeneratorResponse{}
	for lang := range langs {
		buf := bytes.NewBuffer(nil)
		var res string
		switch lang {
		case jsonLangName:
			if err = generate.JSON(buf, parsed, outputVersion); err != nil {
				return nil, fmt.Errorf("failed to generate scopes: lang=%s err=%s", lang, err.Error())
			}
			res = buf.String()
		case goLangName:
			if err = generate.Go(buf, parsed, outputVersion); err != nil {
				return nil, fmt.Errorf("failed to generate scopes: lang=%s err=%s", lang, err.Error())
			}
			formatted, err := format.Source(buf.Bytes(), format.Options{LangVersion: "1.14"})
			if err != nil {
				return nil, fmt.Errorf("failed to generate scopes: lang=%s err=%s", lang, err.Error())
			}
			res = string(formatted)
		default:
			// this should never be reached, but be defensive
			return nil, fmt.Errorf("unsupported language: %s", lang)
		}
		resp.File = append(resp.File, newCodeGeneratorFile(baseDir, lang, res))
	}
	return resp, nil
}

func newCodeGeneratorFile(baseDir, lang, content string) *pluginpb.CodeGeneratorResponse_File {
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String(filepath.Join(baseDir, "all.scopes."+lang)),
		Content: proto.String(content),
	}
}
