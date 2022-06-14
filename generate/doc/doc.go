package doc

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/loader"
	"github.com/kenshaw/snaker"
)

type Doc struct {
	pkg *loader.GunkPackage

	services map[string]*Service // service types
	types    map[string]Type     // data types

	inService map[string][]*Endpoint // types used as service params or return type
	inField   map[string]bool        // types defined as used in fields of other types
}

// Generate generates the JSON documentation.
func Generate(pkg *loader.GunkPackage, genCfg config.Generator) (p *Package, err error) {
	doc := &Doc{
		pkg:       pkg,
		services:  make(map[string]*Service),
		types:     make(map[string]Type),
		inService: make(map[string][]*Endpoint),
		inField:   make(map[string]bool),
	}

	type bailout struct{}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(bailout); ok {
				return
			}
			panic(r)
		}
	}()
	var pkgDesc string
	// collect types and services
	for _, v := range pkg.GunkSyntax {
		if v.Doc.Text() != "" {
			pkgDesc = v.Doc.Text()
		}
		for _, w := range v.Decls {
			ast.Inspect(w, func(n ast.Node) bool {
				switch n := n.(type) {
				default:
					return false
				case *ast.GenDecl, *ast.StructType, *ast.FieldList:
					return true
				// Type definitions such as enum, struct and interface.
				case *ast.TypeSpec:
					err = doc.addType(n)
				// Possible values of enums.
				case *ast.ValueSpec:
					err = doc.addEnum(n)
				}
				if err != nil {
					panic(bailout{})
				}
				return false
			})
		}
	}
	// cleanup
	for k, v := range doc.types {
		m, ok := v.(*Message)
		if !ok {
			continue
		}
		if doc.inService[k] != nil && !doc.inField[k] {
			// Remove data types that are only part of a service.
			delete(doc.types, k)
		}
		// replace ref types with the message itself
		for _, s := range doc.inService[k] {
			// replace request type
			if req, ok := s.Request.(*Ref); ok && req.Name == k {
				s.Request = m
				// fixup body field name
				if s.BodyField != "" && s.BodyField != "*" {
					for _, f := range m.Fields {
						if f.GunkName == s.BodyField {
							s.BodyField = f.Name
							break
						}
					}
				}
				// fix path field names
				s.Path = processPath(m, s.Path)
			}
			// replace response type
			if res, ok := s.Response.(*Ref); ok && res.Name == k {
				s.Response = m
			}
		}
	}
	// create package
	services := make([]*Service, 0, len(doc.services))
	for _, s := range doc.services {
		services = append(services, s)
	}
	// sort services
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	return &Package{
		Name:        pkg.Name,
		ID:          pkg.Types.Path(),
		Description: pkgDesc,
		Services:    services,
		Types:       doc.types,
	}, nil
}

func (doc *Doc) addType(n *ast.TypeSpec) error {
	switch nn := n.Type.(type) {
	case *ast.StructType:
		return doc.addMessage(n, nn)
	case *ast.InterfaceType:
		return doc.addService(n, nn)
	case *ast.Ident:
		if nn.Name == "int" {
			qName := doc.qualifiedTypeName(n.Name.Name, doc.pkg.Types)
			if _, ok := doc.types[qName]; !ok {
				// Create an enum if it doesn't exist.
				// It might be created by addEnum if the value is declared
				// before the type.
				doc.types[qName] = &Enum{}
			}
			enum := doc.types[qName].(*Enum)
			enum.Name = n.Name.Name
			enum.Description = cleanDescription(n.Name.Name, n.Doc.Text())
		}
	}
	return nil
}

func (doc *Doc) addMessage(n *ast.TypeSpec, st *ast.StructType) error {
	msg := &Message{
		Name:        n.Name.Name,
		Description: cleanDescription(n.Name.Name, n.Doc.Text()),
	}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			return fmt.Errorf("field %s has no name", field.Type)
		}
		if len(field.Names) > 1 {
			return fmt.Errorf("field %s has multiple names", field.Names)
		}
		ftype := doc.pkg.TypesInfo.TypeOf(field.Type)
		typ, err := doc.convertType(ftype, false)
		if err != nil {
			return err
		}
		var tag reflect.StructTag
		if field.Tag != nil {
			// we already know the tag must be valid
			raw, _ := strconv.Unquote(field.Tag.Value)
			tag = reflect.StructTag(raw)
		}
		json := tag.Get("json")
		name := field.Names[0].Name
		if json == "" {
			json = snaker.DefaultInitialisms.CamelToSnake(name)
		}
		msg.Fields = append(msg.Fields, &Field{
			Name:        json,
			GunkName:    name,
			Description: cleanDescription(name, field.Doc.Text()),
			Type:        typ,
		})
	}
	qName := doc.qualifiedTypeName(n.Name.Name, doc.pkg.Types)
	doc.types[qName] = msg
	return nil
}

func (doc *Doc) addService(n *ast.TypeSpec, ifc *ast.InterfaceType) error {
	service := &Service{
		Name:        n.Name.Name,
		Description: cleanDescription(n.Name.Name, n.Doc.Text()),
	}
	for _, v := range ifc.Methods.List {
		if len(v.Names) != 1 {
			return fmt.Errorf("methods must have exactly one name")
		}
		endpoint := &Endpoint{
			Name:        v.Names[0].Name,
			Description: cleanDescription(v.Names[0].Name, v.Doc.Text()),
		}
		for _, tag := range doc.pkg.GunkTags[v] {
			switch tag.Type.String() {
			case "github.com/gunk/opt/http.Match":
				for _, elt := range tag.Expr.(*ast.CompositeLit).Elts {
					kv := elt.(*ast.KeyValueExpr)
					val, _ := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
					switch name := kv.Key.(*ast.Ident).Name; name {
					case "Method":
						endpoint.Method = val
					case "Path":
						endpoint.Path = val
					case "Body":
						endpoint.BodyField = val
					}
				}
			case "github.com/gunk/opt/doc.Embed":
			}
		}
		sign := doc.pkg.TypesInfo.TypeOf(v.Type).(*types.Signature)
		var err error
		endpoint.Request, endpoint.StreamingRequest, err = doc.convertParam(endpoint, sign.Params())
		if err != nil {
			return fmt.Errorf("%s: %s", v.Names[0].Name, err)
		}
		endpoint.Response, endpoint.StreamingResponse, err = doc.convertParam(endpoint, sign.Results())
		if err != nil {
			return fmt.Errorf("%s: %s", v.Names[0].Name, err)
		}
		service.Endpoints = append(service.Endpoints, endpoint)
	}
	doc.services[n.Name.Name] = service
	return nil
}

// processPath processes the provided path by mapping the names in the path to
// their JSON names based on the provided Message.
func processPath(m *Message, val string) string {
	formatted := ""
	jsonName := make(map[string]string)
	for _, f := range m.Fields {
		jsonName[f.GunkName] = f.Name
	}
	for {
		i := strings.Index(val, "{")
		if i == -1 {
			break
		}
		formatted += val[:i]
		val = val[i:]
		j := strings.Index(val, "}")
		if j == -1 {
			break
		}
		pathVar := val[1:j]
		if v, ok := jsonName[pathVar]; ok {
			formatted += "{" + v + "}"
		} else {
			// Leave unchanged if not found.
			formatted += "{" + pathVar + "}"
		}
		val = val[j+1:]
	}
	formatted += val
	return formatted
}

func (doc *Doc) convertParam(e *Endpoint, params *types.Tuple) (Type, bool, error) {
	switch params.Len() {
	case 0:
		return nil, false, nil
	case 1:
		// below
	default:
		return nil, false, fmt.Errorf("multiple parameters are not supported")
	}
	param := params.At(0).Type()
	var streaming bool
	var typ Type
	var err error
	if p, ok := param.(*types.Chan); ok {
		typ, err = doc.convertType(p.Elem(), true)
		streaming = true
	} else {
		typ, err = doc.convertType(param, true)
	}
	if err != nil {
		return nil, false, err
	}
	if _, ok := typ.(*Ref); !ok {
		return nil, false, fmt.Errorf("unsupported parameter type: %v", typ)
	}
	ref := typ.(*Ref)
	doc.inService[ref.Name] = append(doc.inService[ref.Name], e)
	return ref, streaming, nil
}

func (doc *Doc) addEnum(n *ast.ValueSpec) error {
	if len(n.Names) == 0 {
		return nil
	}
	for _, ident := range n.Names {
		qName := doc.pkg.TypesInfo.TypeOf(ident).String()
		if _, ok := doc.types[qName]; !ok {
			// Create an enum if it has not been declared yet.
			doc.types[qName] = &Enum{}
		}
		enum, ok := doc.types[qName].(*Enum)
		if !ok {
			// Enforce that the type is an enum.
			return fmt.Errorf("cannot declare value %s for non-enum type %s", ident.Name, qName)
		}
		enum.Values = append(enum.Values, &EnumVal{
			Value:       ident.Name,
			Description: cleanDescription(ident.Name, n.Doc.Text()),
		})
	}
	return nil
}

func (doc *Doc) convertType(typ types.Type, inService bool) (Type, error) {
	switch typ := typ.(type) {
	case *types.Basic:
		switch typ.Kind() {
		case types.String:
			return &Basic{"String", ""}, nil
		case types.Int, types.Int32:
			return &Basic{"Integer", ""}, nil
		case types.Uint, types.Uint32:
			return &Basic{"Unsigned Integer", ""}, nil
		case types.Int64:
			return &Basic{"Integer(64)", ""}, nil
		case types.Uint64:
			return &Basic{"Unsigned Integer(64)", ""}, nil
		case types.Float32:
			return &Basic{"Float(32)", ""}, nil
		case types.Float64:
			return &Basic{"Float(64)", ""}, nil
		case types.Bool:
			return &Basic{"Boolean", ""}, nil
		}
	case *types.Slice:
		if eTyp, ok := typ.Elem().(*types.Basic); ok {
			if eTyp.Kind() == types.Byte {
				return &Basic{"Bytes", ""}, nil
			}
		}
		dtyp, err := doc.convertType(typ.Elem(), false)
		if err != nil {
			return nil, err
		}
		return &Array{dtyp}, nil
	case *types.Map:
		kTyp, err := doc.convertType(typ.Key(), false)
		if err != nil {
			return nil, err
		}
		vTyp, err := doc.convertType(typ.Elem(), false)
		if err != nil {
			return nil, err
		}
		return &Map{kTyp, vTyp}, nil
	case *types.Named:
		switch typ.String() {
		case "time.Time":
			return &Basic{"Timestamp", ""}, nil
		case "time.Duration":
			return &Basic{"Duration", ""}, nil
		case "encoding/json.RawMessage":
			return &Basic{"JSON Object", ""}, nil
		}
		obj := typ.Obj()
		fullName := doc.qualifiedTypeName(obj.Name(), obj.Pkg())
		if !inService {
			doc.inField[fullName] = true
		}
		return &Ref{fullName}, nil
	}
	return nil, fmt.Errorf("unknown type to convert: %T", typ)
}

func (doc *Doc) qualifiedTypeName(typeName string, pkg *types.Package) string {
	// If pkg is nil, we should format the type for the current package.
	if pkg == nil {
		pkg = doc.pkg.Types
	}
	return pkg.Path() + "." + typeName
}

// cleanDescription removes the leading "XYZ is" and the trailing dot from the
// description.
func cleanDescription(name string, desc string) string {
	// FIXME: Check for Deprecated:
	desc = strings.TrimSpace(desc)
	desc = strings.TrimPrefix(desc, name+" is ")
	desc = strings.TrimPrefix(desc, name+" are ")
	desc = strings.TrimSuffix(desc, ".")
	// NOTE: asdad
	// > adasad
	return desc
}
