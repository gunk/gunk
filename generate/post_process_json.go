package generate

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// jsonNameFromProtobufTag retrieves JSON name from the provided value of the
// protobuf tag. If no JSON names are defined, an empty string is returned.
//
//     Example: protobuf:"bytes,1,opt,name=FirstName,json=first_name,proto3"
//     Result:  first_name
func jsonNameFromProtobufTag(tag string) string {
	const jsonPartName = "json="
	if len(tag) == 0 {
		return ""
	}
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		if strings.HasPrefix(part, jsonPartName) {
			return strings.TrimPrefix(part, jsonPartName)
		}
	}
	return ""
}

// jsonNameFromJSONTag returns the JSON name from the provided value of the
// JSON tag. If no JSON names are defined, an empty string is returned.
//
//     Example: json:"sample,omitempty"
//     Result:  sample
func jsonNameFromJSONTag(tag string) string {
	if len(tag) == 0 {
		return ""
	}
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// jsonTagPostProcessor replaces all JSON names in struct tags with the JSON
// names declared in the protobuf tag.
func jsonTagPostProcessor(input []byte) ([]byte, error) {
	const (
		jsonTagName     = "json"
		protobufTagName = "protobuf"
	)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", input, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	ast.Inspect(f, func(node ast.Node) bool {
		structDecl, ok := node.(*ast.StructType)
		if !ok {
			return true
		}
		for i, field := range structDecl.Fields.List {
			if field.Tag == nil {
				continue
			}
			tagValue := field.Tag.Value
			tagValue = strings.Trim(tagValue, "`")
			tag := reflect.StructTag(tagValue)
			jsonName := jsonNameFromJSONTag(tag.Get(jsonTagName))
			protobufJSONName := jsonNameFromProtobufTag(tag.Get(protobufTagName))
			if jsonName != "" && protobufJSONName != "" && jsonName != protobufJSONName {
				text := strings.Replace(field.Tag.Value, `json:"`+jsonName, `json:"`+protobufJSONName, 1)
				structDecl.Fields.List[i].Tag = &ast.BasicLit{
					ValuePos: field.Tag.Pos(),
					Kind:     field.Tag.Kind,
					Value:    text,
				}
			}
		}
		return true
	})
	var output bytes.Buffer
	if err = format.Node(&output, fset, f); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
