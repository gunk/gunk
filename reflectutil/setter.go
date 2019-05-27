package reflectutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// SetValue assign value to the field
// name of the struct structPtr.
// Returns an error if name is not found or structPtr cannot
// be addressed.
func SetValue(structPtr interface{}, name, value string) {
	// Performs a case insensitive search because generated structs by
	// protoc don't follow the best practices from the go naming convention:
	// initialisms should be all capitals
	tmp := reflect.Indirect(reflect.ValueOf(structPtr)).FieldByNameFunc(func(s string) bool {
		return strings.EqualFold(s, name)
	})
	if !tmp.IsValid() {
		panic(fmt.Errorf("%s was not found in %T", name, structPtr))
	}
	if !tmp.CanAddr() {
		panic(fmt.Errorf("%s from %T cannot be addressed", name, structPtr))
	}
	var v interface{}
	var err error
	switch k := tmp.Type().Kind(); k {
	case reflect.String:
		v = value
	case reflect.Float64:
		v, err = strconv.ParseFloat(value, 64)
	case reflect.Bool:
		v, err = strconv.ParseBool(value)
	case reflect.Uint64:
		v, err = strconv.ParseUint(value, 10, 64)
	default:
		panic(fmt.Errorf("%s not supported", k.String()))
	}
	if err != nil {
		panic(err)
	}
	tmp.Set(reflect.ValueOf(v))
}
