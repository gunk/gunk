package reflectutil

import (
	"fmt"
	"go/ast"
	"reflect"
	"sort"
	"strconv"
	"strings"

	protop "github.com/emicklei/proto"
	"github.com/golang/protobuf/proto"
)

func UnmarshalProto(v interface{}, lit *protop.Literal) {
	value := reflect.Indirect(reflect.ValueOf(v))
	typ := value.Type()

	switch typ.Kind() {
	case reflect.Struct:
		for _, elem := range lit.OrderedMap {
			setField(value, elem.Name, elem)
		}
	default:
		panic(fmt.Errorf("unsupported type: %s", typ))
	}
}

func UnmarshalAST(v interface{}, expr ast.Expr) {
	value := reflect.Indirect(reflect.ValueOf(v))
	typ := value.Type()

	switch typ.Kind() {
	case reflect.Struct:
		expr := expr.(*ast.CompositeLit)
		for _, elt := range expr.Elts {
			kv := elt.(*ast.KeyValueExpr)
			setField(value, kv.Key.(*ast.Ident).Name, kv.Value)
		}
	default:
		panic(fmt.Errorf("unsupported type: %s", typ))
	}
}

func setField(structVal reflect.Value, name string, value interface{}) {
	// Performs a case insensitive search because generated structs by
	// protoc don't follow the best practices from the go naming convention:
	// initialisms should be all capitals.
	// Remove underscores too, so that foo_bar matches the field FooBar.
	name = strings.ReplaceAll(name, "_", "")
	typ := structVal.Type()
	field, ok := typ.FieldByNameFunc(func(s string) bool {
		return strings.EqualFold(s, name)
	})
	if !ok {
		panic(fmt.Errorf("%s was not found in %s", name, typ))
	}
	fval := structVal.FieldByIndex(field.Index)

	val := valueFor(field.Type, field.Tag, value)
	// Merge slices and maps. For example, maps are often decoded one
	// key-value element at a time for backwards compatibility, so we must
	// add the elements incrementally.
	switch field.Type.Kind() {
	case reflect.Slice:
		val = reflect.AppendSlice(fval, val)
	case reflect.Map:
		if fval.IsNil() {
			fval.Set(reflect.MakeMap(field.Type))
		}
		iter := val.MapRange()
		for iter.Next() {
			fval.SetMapIndex(iter.Key(), iter.Value())
		}
		return
	}
	fval.Set(val)
}

func valueFor(typ reflect.Type, tag reflect.StructTag, value interface{}) reflect.Value {
	if named, ok := value.(*protop.NamedLiteral); ok {
		// We don't care about the name here.
		value = named.Literal
	}

	switch typ.Kind() {
	case reflect.Ptr:
		return valueFor(typ.Elem(), tag, value).Addr()
	case reflect.Struct:
		strc := reflect.New(typ).Elem()
		switch value := value.(type) {
		case *ast.CompositeLit:
			for _, elt := range value.Elts {
				kv := elt.(*ast.KeyValueExpr)
				setField(strc, kv.Key.(*ast.Ident).Name, kv.Value)
			}
		case *protop.Literal:
			for _, lit := range value.OrderedMap {
				setField(strc, lit.Name, lit)
			}
		default:
			panic(fmt.Sprintf("%T is not a valid value for %s", value, typ))
		}
		return strc
	case reflect.Map:
		mp := reflect.MakeMap(typ)
		switch value := value.(type) {
		case *ast.CompositeLit:
			for _, elt := range value.Elts {
				kv := elt.(*ast.KeyValueExpr)
				key := valueFor(typ.Key(), "", kv.Key)
				val := valueFor(typ.Elem(), "", kv.Value)
				mp.SetMapIndex(key, val)
			}
		case *protop.Literal:
			key := valueFor(typ.Key(), "", value.OrderedMap[0])
			if key.Interface() == "empty" {
				// TODO(mvdan): figure out why this happens
				break
			}
			val := valueFor(typ.Elem(), "", value.OrderedMap[1])
			mp.SetMapIndex(key, val)
		default:
			panic(fmt.Sprintf("%T is not a valid value for %s", value, typ))
		}
		return mp
	case reflect.Slice:
		etyp := typ.Elem()
		list := reflect.MakeSlice(typ, 0, 0)
		switch value := value.(type) {
		case *ast.CompositeLit:
			for _, elt := range value.Elts {
				list = reflect.Append(list, valueFor(etyp, tag, elt))
			}
		case *protop.Literal:
			// convert string to slice of bytes, uint8 and byte are the same kind
			if etyp.Kind() == reflect.Uint8 && value.IsString {
				return reflect.ValueOf([]byte(value.SourceRepresentation()))
			}
			if value.Array == nil {
				list = reflect.Append(list, valueFor(etyp, tag, value))
				break
			}
			for _, lit := range value.Array {
				list = reflect.Append(list, valueFor(etyp, tag, lit))
			}
		default:
			panic(fmt.Sprintf("%T is not a valid value for %s", value, typ))
		}
		return list
	}

	valueStr := ""
	switch x := value.(type) {
	case *ast.Ident:
		valueStr = x.Name
	case *ast.BasicLit:
		valueStr = x.Value
	case *ast.SelectorExpr:
		valueStr = x.Sel.Name
	case *protop.Literal:
		valueStr = x.SourceRepresentation()
	default:
		panic(fmt.Sprintf("%T contains no name or string value", value))
	}
	value = reflect.Value{} // ensure we just use valueStr from this point

	// If the field is an enum, decode it, and store it via a conversion
	// from int32 to the named enum type.
	for _, kv := range strings.Split(tag.Get("protobuf"), ",") {
		kv := strings.SplitN(kv, "=", 2)
		if kv[0] != "enum" {
			continue
		}
		enumMap := proto.EnumValueMap(kv[1])
		val, ok := enumMap[valueStr]
		if !ok {
			panic(fmt.Errorf("%q is not a valid %s", valueStr, kv[1]))
		}
		return reflect.ValueOf(val).Convert(typ)
	}

	var v interface{}
	var err error
	switch typ.Kind() {
	case reflect.String:
		v, err = strconv.Unquote(valueStr)
	case reflect.Float64:
		v, err = strconv.ParseFloat(valueStr, 64)
	case reflect.Bool:
		v, err = strconv.ParseBool(valueStr)
	case reflect.Uint64:
		v, err = strconv.ParseUint(valueStr, 10, 64)
	}
	if err != nil {
		panic(err)
	}
	if v == nil {
		panic(fmt.Sprintf("%T is not a valid value for %s", valueStr, typ))
	}
	return reflect.ValueOf(v)
}

// SortValues sorts vals in alphabetical order.
func SortValues(vals []reflect.Value) {
	if len(vals) == 0 {
		return
	}
	switch vals[0].Kind() {
	case reflect.String:
		sort.Slice(vals, func(i, j int) bool {
			return vals[i].Interface().(string) < vals[j].Interface().(string)
		})
	default:
		panic(fmt.Sprintf("TODO: reflect sort type: %s", vals[0].Type()))
	}
}
