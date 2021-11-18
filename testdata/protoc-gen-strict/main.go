// protoc-gen-strict is like a protoc-gen-* program, but it just runs a number
// of sanity checks on the CodeGeneratorRequest. It outputs an empty
// CodeGeneratorResponse to stdout, to signal that there's nothing to generate.
package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gunk/gunk/protoutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	in, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(in, &req); err != nil {
		return err
	}
	if err := check(&req); err != nil {
		return err
	}
	var res pluginpb.CodeGeneratorResponse
	out, err := protoutil.MarshalDeterministic(&res)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func check(req *pluginpb.CodeGeneratorRequest) error {
	seen := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, f := range req.ProtoFile {
		for _, dep := range f.Dependency {
			if seen[dep] == nil {
				return fmt.Errorf("%s has unknown dep %s", *f.Name, dep)
			}
		}
		seen[*f.Name] = f
	}
	return nil
}
