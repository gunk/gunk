package extract

import (
	"reflect"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger/options"

	"github.com/gunk/gunk/docgen/pot"
	"github.com/gunk/gunk/reflectutil"
)

// Run extract all string from swagger options and store them into pot builder.
func Run(file *google_protobuf.FileDescriptorProto) (pot.Builder, error) {
	pb := pot.NewBuilder()

	if proto.HasExtension(file.GetOptions(), options.E_Openapiv2Swagger) {
		ext, err := proto.GetExtension(file.GetOptions(), options.E_Openapiv2Swagger)
		if err != nil {
			return nil, err
		}
		pb.AddTranslations(toTranslate(ext))
	}

	for _, s := range file.GetService() {
		for _, m := range s.GetMethod() {
			ext, err := proto.GetExtension(m.GetOptions(), options.E_Openapiv2Operation)
			if err != nil {
				if err == proto.ErrMissingExtension {
					continue
				}
				return nil, err
			}
			pb.AddTranslations(toTranslate(ext))
		}
	}

	return pb, nil
}

func toTranslate(val interface{}) []string {
	v := reflect.ValueOf(val)
	t := v.Type()

	if t.Kind() == reflect.Ptr {
		v = reflect.Indirect(v)
		t = t.Elem()
	}

	var res []string
	for i := 0; i < t.NumField(); i++ {
		name := t.Field(i).Name
		value := v.FieldByName(name)

		if !value.IsValid() || reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface()) {
			continue
		}

		switch value.Kind() {
		case reflect.Map:
			keys := value.MapKeys()
			reflectutil.SortValues(keys)
			for _, key := range keys {
				c := value.MapIndex(key)
				switch c.Type().Kind() {
				case reflect.Ptr:
					if !c.IsNil() {
						t := reflect.Indirect(c).Interface()
						res = append(res, toTranslate(t)...)
					}
				case reflect.String:
					res = append(res, c.String())
				}
			}
		case reflect.Ptr:
			if !value.IsNil() {
				res = append(res, toTranslate(reflect.Indirect(value).Interface())...)
			}
		case reflect.String:
			res = append(res, value.String())
		}
	}
	return res
}
