// +build tools

package tools

import (
	// required for assets/doc.go generate
	_ "github.com/shurcooL/vfsgen"

	// required by the tests + code generation
	_ "mvdan.cc/gofumpt"
)
