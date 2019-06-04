package reflectutil

import (
	"fmt"
	"go/ast"
	"reflect"
	"strconv"
	"strings"
)

type protoLiteral interface {
	SourceRepresentation() string
}

// SetValue assign value to the field name of the struct structPtr.
// Returns an error if name is not found or structPtr cannot
// be addressed.
func SetValue(structPtr interface{}, name, value interface{}) {
	nameStr := ""
	switch name := name.(type) {
	case *ast.Ident:
		nameStr = name.Name
	case string:
		nameStr = name
	default:
		panic(fmt.Errorf("invalid name type: %T", name))
	}

	// Performs a case insensitive search because generated structs by
	// protoc don't follow the best practices from the go naming convention:
	// initialisms should be all capitals.
	// Remove underscores too, so that foo_bar matches the field FooBar.
	nameStr = strings.ReplaceAll(nameStr, "_", "")
	tmp := reflect.ValueOf(structPtr).Elem().FieldByNameFunc(func(s string) bool {
		return strings.EqualFold(s, nameStr)
	})
	if !tmp.IsValid() {
		panic(fmt.Errorf("%s was not found in %T", nameStr, structPtr))
	}

	switch x := value.(type) {
	case *ast.Ident:
		value = x.Name
	case *ast.BasicLit:
		value = x.Value
	case protoLiteral:
		value = x.SourceRepresentation()
	}

	var v interface{}
	var err error
	switch k := tmp.Type(); k.Kind() {
	case reflect.String:
		switch value := value.(type) {
		case string:
			v, err = strconv.Unquote(value)
		}
	case reflect.Float64:
		switch value := value.(type) {
		case float64:
			v = value
		case string:
			v, err = strconv.ParseFloat(value, 64)
		}
	case reflect.Bool:
		switch value := value.(type) {
		case bool:
			v = value
		case string:
			v, err = strconv.ParseBool(value)
		}
	case reflect.Int32:
		switch value := value.(type) {
		case int32:
			v = value
		case string:
			v, err = strconv.ParseInt(value, 10, 32)
		}
	case reflect.Uint64:
		switch value := value.(type) {
		case uint64:
			v = value
		case string:
			v, err = strconv.ParseUint(value, 10, 64)
		}
	case reflect.Slice:
		switch k.Elem().Kind() {
		case reflect.String:
			v = reflect.Append(tmp, reflect.ValueOf(value)).Interface()
		default:
			panic(fmt.Errorf("slice of %s not supported", k.Elem().String()))
		}
	}
	if err != nil {
		panic(err)
	}
	if v == nil {
		panic(fmt.Sprintf("%T is not a valid value for %s", value, tmp.Type()))
	}
	val := reflect.ValueOf(v)
	if t := tmp.Type(); val.Type().ConvertibleTo(t) {
		val = val.Convert(t)
	}
	tmp.Set(val)
}
