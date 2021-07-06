package loader

import (
	"errors"
	"strconv"
	"strings"
)

// copied from go vet source code
// https://github.com/golang/tools/blob/master/go/analysis/passes/structtag/structtag.go

var (
	errTagSyntax      = errors.New("bad syntax for struct tag pair")
	errTagKeySyntax   = errors.New("bad syntax for struct tag key")
	errTagValueSyntax = errors.New("bad syntax for struct tag value")
	errTagValueSpace  = errors.New("suspicious space in struct tag value")
	errTagSpace       = errors.New("key:\"value\" pairs not separated by spaces")
)

// validateStructTag parses the struct tag and returns an error if it is not
// in the canonical format, which is a space-separated list of key:"value"
// settings. The value may contain spaces.
func validateStructTag(tag string) error {
	// This code is based on the StructTag.Get code in package reflect.

	n := 0
	for ; tag != ""; n++ {
		if n > 0 && tag != "" && tag[0] != ' ' {
			// More restrictive than reflect, but catches likely mistakes
			// like `x:"foo",y:"bar"`, which parses as `x:"foo" ,y:"bar"` with second key ",y".
			return errTagSpace
		}
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 {
			return errTagKeySyntax
		}
		if i+1 >= len(tag) || tag[i] != ':' {
			return errTagSyntax
		}
		if tag[i+1] != '"' {
			return errTagValueSyntax
		}
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			return errTagValueSyntax
		}
		qvalue := tag[:i+1]
		tag = tag[i+1:]

		value, err := strconv.Unquote(qvalue)
		if err != nil {
			return errTagValueSyntax
		}

		if strings.IndexByte(value, ' ') >= 0 {
			return errTagValueSpace
		}
	}
	return nil
}
