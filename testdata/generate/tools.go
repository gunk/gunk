// build +tools

// this is here for "go mod tidy". Otherwise it would get removed, but then gunk requires it.

package util

import (
	_ "github.com/gunk/opt/openapiv2"
)
