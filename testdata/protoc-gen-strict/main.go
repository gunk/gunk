// protoc-gen-strict is like a protoc-gen-* program, but it just runs a number
// of sanity checks on the CodeGeneratorRequest. It outputs an empty
// CodeGeneratorResponse to stdout, to signal that there's nothing to generate.
package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func main() {
	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var req plugin.CodeGeneratorRequest
	if err := proto.Unmarshal(input, &req); err != nil {
		panic(err)
	}
	if err := check(&req); err != nil {
		panic(err)
	}
	var resp plugin.CodeGeneratorResponse
	output, err := proto.Marshal(&resp)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stdout.Write(output); err != nil {
		panic(err)
	}
}

func check(req *plugin.CodeGeneratorRequest) error {
	seenFiles := make(map[string]*desc.FileDescriptorProto)
	for _, pfile := range req.ProtoFile {
		for _, dep := range pfile.Dependency {
			if seenFiles[dep] == nil {
				return fmt.Errorf("%s has unknown dep %s", *pfile.Name, dep)
			}
		}
		seenFiles[*pfile.Name] = pfile
	}
	return nil
}
