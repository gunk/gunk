package generate

import (
	"encoding/json"
	"io"

	_ "github.com/golang/protobuf/protoc-gen-go/grpc"

	"github.com/gunk/gunk/scopegen/parser"
)

// JSON generates mapping between service full method names and its OAuth2 scope in JSON format.
func JSON(w io.Writer, f *parser.File) error {
	var result = make(map[string][]string, len(f.Methods))
	for _, method := range f.Methods {
		result[method.Name] = method.Scopes
	}
	return json.NewEncoder(w).Encode(result)
}
