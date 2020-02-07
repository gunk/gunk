package parser

import (
	"fmt"
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger/options"
)

// Scope represents an OAuth2 scope.
type Scope struct {
	Name  string
	Value string
}

// Method represents a gRPC method and its OAuth2 scopes.
type Method struct {
	Name   string
	Scopes [][]string
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

// parseScopes returns a map of security scheme name and OAuth2 scopes definitions.
func parseScopes(file *descriptor.FileDescriptorProto) (map[string][]Scope, error) {
	fileExt, err := proto.GetExtension(file.GetOptions(), options.E_Openapiv2Swagger)
	if err == proto.ErrMissingExtension {
		// no swagger option defined, generated nothing
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	swagger := fileExt.(*options.Swagger)

	if swagger.GetSecurityDefinitions() == nil {
		return nil, nil
	}

	var (
		result          = make(map[string][]Scope)
		securitySchemes = swagger.GetSecurityDefinitions().GetSecurity()
	)
	for name, securityScheme := range securitySchemes {
		if securityScheme.Type == options.SecurityScheme_TYPE_OAUTH2 {
			if securityScheme.Scopes == nil {
				continue
			}
			var scopeKeys []string
			for scopeName := range securityScheme.Scopes.Scope {
				scopeKeys = append(scopeKeys, scopeName)
			}
			sort.Strings(scopeKeys)

			var scopes []Scope
			for _, scopeName := range scopeKeys {
				scopes = append(scopes, Scope{
					Name:  scopeName,
					Value: securityScheme.Scopes.Scope[scopeName],
				})
			}
			result[name] = scopes
		}
	}

	return result, nil
}

// parseMethodsFromService parsed the given service and store its method OAuth2 scopes to method map.
// methods: full method name --> (security scheme name --> scopes)
func parseMethodsFromService(methods map[string]map[string][]string, packageName string, oauth2SecuritySchemes map[string][]Scope, service *descriptor.ServiceDescriptorProto) error {
	for _, method := range service.GetMethod() {
		methodExt, err := proto.GetExtension(method.GetOptions(), options.E_Openapiv2Operation)
		if err == proto.ErrMissingExtension {
			continue
		} else if err != nil {
			return err
		}
		op := methodExt.(*options.Operation)
		for _, securityRequirement := range op.GetSecurity() {
			for name, val := range securityRequirement.GetSecurityRequirement() {
				for schemeName := range oauth2SecuritySchemes {
					if name == schemeName {
						fullMethodName := generateFullMethodName(packageName, service.GetName(), method.GetName())
						methodScopes, ok := methods[fullMethodName]
						if !ok {
							methodScopes = make(map[string][]string)
						}
						methodScopes[schemeName] = val.GetScope()
						methods[fullMethodName] = methodScopes
					}
				}
			}
		}
	}
	return nil
}

func parseMethods(oauth2SecuritySchemes map[string][]Scope, file *descriptor.FileDescriptorProto) ([]Method, error) {
	var methods = make(map[string]map[string][]string)

	if len(oauth2SecuritySchemes) == 0 {
		return nil, nil
	}

	for _, service := range file.GetService() {
		if err := parseMethodsFromService(methods, file.GetPackage(), oauth2SecuritySchemes, service); err != nil {
			return nil, err
		}
	}

	result, err := parseMethodScopes(oauth2SecuritySchemes, methods)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// parseMethodScopes generates the list of method and its configured OAuth2 scopes
func parseMethodScopes(oauth2SecuritySchemes map[string][]Scope, methods map[string]map[string][]string) ([]Method, error) {
	var (
		result      []Method
		methodNames []string
	)

	for name := range methods {
		methodNames = append(methodNames, name)
	}
	// sorted method names by key before storing to *File, to guarantee order of method in output
	sort.Strings(methodNames)

	for _, name := range methodNames {
		methodScopes := methods[name]
		m := Method{
			Name:   name,
			Scopes: nil,
		}

		// sorted scheme names by key before storing to *File, to guarantee order of method in output
		var schemeNames []string
		for name := range methodScopes {
			schemeNames = append(schemeNames, name)
		}
		sort.Strings(schemeNames)

		for _, schemeName := range schemeNames {
			// validate if scope defined in oauth2SecuritySchemes
			definedScopes, ok := oauth2SecuritySchemes[schemeName]
			if !ok {
				return nil, fmt.Errorf("unknown security scheme %s", schemeName)
			}

			if err := validateScopes(methodScopes[schemeName], definedScopes, name); err != nil {
				return nil, err
			}

			m.Scopes = append(m.Scopes, methodScopes[schemeName])
		}
		result = append(result, m)
	}
	return result, nil
}

// validateScopes returns an error if any scope is undefined.
func validateScopes(scopes []string, definedScopes []Scope, methodName string) error {
	for _, scope := range scopes {
		defined := false
		for _, definedScope := range definedScopes {
			if definedScope.Name == scope {
				defined = true
				break
			}
		}
		if !defined {
			return fmt.Errorf("failed to parse, scope %q of method %q is undefined", scope, methodName)
		}
	}
	return nil
}

// ParseFile parsed the given proto file to extract its OAuth2 scopes information.
func ParseFile(file *descriptor.FileDescriptorProto) (*File, error) {
	f := &File{
		Package: file.GetPackage(),
	}

	oauth2SecuritySchemes, err := parseScopes(file)
	if err != nil {
		return nil, err
	}

	var schemeNames []string
	// sorted scheme names by key before storing to *File, to guarantee order of method in output
	for name := range oauth2SecuritySchemes {
		schemeNames = append(schemeNames, name)
	}
	sort.Strings(schemeNames)
	for _, name := range schemeNames {
		f.Scopes = append(f.Scopes, oauth2SecuritySchemes[name]...)
	}

	methods, err := parseMethods(oauth2SecuritySchemes, file)
	if err != nil {
		return nil, err
	}
	f.Methods = methods

	return f, nil
}
