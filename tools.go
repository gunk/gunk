// +build tools

package tools

import (
	// required by the tests
	_ "github.com/Kunde21/pulpMd"

	// required for assets/doc.go generate
	_ "github.com/shurcooL/vfsgen"

	// required by the tests + code generation
	_ "mvdan.cc/gofumpt"
)
