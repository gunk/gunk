// +build tools

package tools

import (
	// required by the tests
	_ "github.com/Kunde21/pulpMd"
	_ "github.com/golang/protobuf/protoc-gen-go"
	_ "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway"

	// required for assets/doc.go generate
	_ "github.com/shurcooL/vfsgen"

	// required by the tests + code generation
	_ "mvdan.cc/gofumpt"
)
