package parser

import (
	"fmt"
	"sort"

	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Scope represents an OAuth2 scope.
type Scope struct {
	Name  string
	Value string
}

// Method represents a gRPC method and its OAuth2 scopes.
type Method struct {
	Name   string
	Scopes []string
}

// File represents a proto file with all OAuth2 information parsed.
// Scopes and methods are converted from map to sorted slice of struct
// to have determinate generated codes.
type File struct {
	Package string
	// Scopes is list of scopes defined for the service.
	Scopes []Scope
	// Methods is mapping between full method name and its scopes.
	Methods []Method
}

func (f *File) validate() error {
	for _, method := range f.Methods {
		for _, scope := range method.Scopes {
			defined := false
			for _, definedScope := range f.Scopes {
				if definedScope.Name == scope {
					defined = true
					break
				}
			}
			if !defined {
				return fmt.Errorf("failed to parse, scope %q of method %q is undefined", scope, method.Name)
			}
		}
	}
	return nil
}

// generateFullMethodName generates the full method name of a gRPC service method.
// It is reserve-engineered from protoc-gen-go.
// Reference: https://github.com/golang/protobuf/blob/822fe56949f5d56c9e2f02367c657e0e9b4d27d1/protoc-gen-go/grpc/grpc.go
func generateFullMethodName(pkg, servName, methName string) string {
	fullServName := servName
	if pkg != "" {
		fullServName = pkg + "." + fullServName
	}
	return fmt.Sprintf("/%s/%s", fullServName, methName)
}

func parseScopes(file *descriptorpb.FileDescriptorProto) (string, []Scope, error) {
	var (
		oauth2SecuritySchemeName string
		scopes                   []Scope
	)
	fileExt := proto.GetExtension(file.GetOptions(), options.E_Openapiv2Swagger)
	if fileExt == nil {
		// no swagger option defined, generated nothing
		return "", nil, nil
	}
	swagger := fileExt.(*options.Swagger)
	if swagger.GetSecurityDefinitions() == nil {
		return "", nil, nil
	}
	for securitySchemeName, securityScheme := range swagger.GetSecurityDefinitions().GetSecurity() {
		if securityScheme.Type == options.SecurityScheme_TYPE_OAUTH2 {
			if securityScheme.Scopes == nil {
				continue
			}
			var scopeKeys []string
			for scopeName := range securityScheme.Scopes.Scope {
				scopeKeys = append(scopeKeys, scopeName)
			}
			sort.Strings(scopeKeys)
			for _, scopeName := range scopeKeys {
				scopes = append(scopes, Scope{
					Name:  scopeName,
					Value: securityScheme.Scopes.Scope[scopeName],
				})
			}
			oauth2SecuritySchemeName = securitySchemeName
			// for now we only handle first OAuth2 security scheme definition,
			// supports for generating multiple schemes might come later
			break
		}
	}
	return oauth2SecuritySchemeName, scopes, nil
}

// parseMethodsFromService parsed the given service and store its method OAuth2 scopes to method map.
func parseMethodsFromService(methods map[string][]string, packageName, securitySchemeName string, service *descriptorpb.ServiceDescriptorProto) error {
	for _, method := range service.GetMethod() {
		methodExt := proto.GetExtension(method.GetOptions(), options.E_Openapiv2Operation)
		if methodExt == nil {
			continue
		}
		op := methodExt.(*options.Operation)
		for _, securityRequirement := range op.GetSecurity() {
			for name, val := range securityRequirement.GetSecurityRequirement() {
				if name == securitySchemeName {
					fullMethodName := generateFullMethodName(packageName, service.GetName(), method.GetName())
					methods[fullMethodName] = val.GetScope()
				}
			}
		}
	}
	return nil
}

func parseMethods(securitySchemeName string, file *descriptorpb.FileDescriptorProto) ([]Method, error) {
	var (
		methods = make(map[string][]string)
		result  []Method
	)
	if securitySchemeName == "" {
		return nil, nil
	}
	for _, service := range file.GetService() {
		if err := parseMethodsFromService(methods, file.GetPackage(), securitySchemeName, service); err != nil {
			return nil, err
		}
	}
	// sorted methods by key before storing to *File, to guarantee order of method in output
	var methodNames []string
	for name := range methods {
		methodNames = append(methodNames, name)
	}
	sort.Strings(methodNames)
	for _, name := range methodNames {
		methodScopes := methods[name]
		sort.Strings(methodScopes)
		result = append(result, Method{
			Name:   name,
			Scopes: methodScopes,
		})
	}
	return result, nil
}

// ParseFile parsed the given proto file to extract its OAuth2 scopes information.
func ParseFile(file *descriptorpb.FileDescriptorProto) (*File, error) {
	f := &File{
		Package: file.GetPackage(),
	}
	oauth2SecuritySchemeName, scopes, err := parseScopes(file)
	if err != nil {
		return nil, err
	}
	f.Scopes = scopes
	methods, err := parseMethods(oauth2SecuritySchemeName, file)
	if err != nil {
		return nil, err
	}
	f.Methods = methods
	if err = f.validate(); err != nil {
		return nil, err
	}
	return f, nil
}
