package generate

import (
	"encoding/json"
	"fmt"
	"io"

	_ "github.com/golang/protobuf/protoc-gen-go/grpc"
	"github.com/gunk/gunk/scopegen/parser"
)

// jsonV1 generates mapping between service full method names and its OAuth2 scope in JSON format.
func jsonV1(w io.Writer, f *parser.File) error {
	result := make(map[string][]string, len(f.Methods))
	for _, method := range f.Methods {
		result[method.Name] = method.Scopes
	}
	return json.NewEncoder(w).Encode(result)
}

type model struct {
	Scopes     map[string]string   `json:"scopes"`
	AuthScopes map[string][]string `json:"auth_scopes"`
}

// jsonV2 generates mapping between service full method names and its OAuth2 scope in JSON format.
func jsonV2(w io.Writer, f *parser.File) error {
	m := &model{
		Scopes:     make(map[string]string, len(f.Scopes)),
		AuthScopes: make(map[string][]string, len(f.Methods)),
	}
	for _, method := range f.Methods {
		m.AuthScopes[method.Name] = method.Scopes
	}
	for _, scope := range f.Scopes {
		m.Scopes[scope.Name] = scope.Value
	}
	return json.NewEncoder(w).Encode(m)
}

// JSON generates mapping between service full method names and its OAuth2 scope in JSON format.
func JSON(w io.Writer, f *parser.File, outputVersion int) error {
	switch outputVersion {
	case 1:
		return jsonV1(w, f)
	case 2:
		return jsonV2(w, f)
	default:
		return fmt.Errorf("unknown output version: %d", outputVersion)
	}
}
