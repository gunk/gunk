package format

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/loader"
	"github.com/kenshaw/snaker"
)

// Formatter is a struct that holds the state of the formatter.
// A new formatter should be initialized when using different config.
type Formatter struct {
	Config *config.Config

	snaker *snaker.Initialisms
}

// New creates a new instance of Formatter.
func New(cfg *config.Config) (*Formatter, error) {
	s := snaker.NewDefaultInitialisms()
	err := s.Add(cfg.Format.Initialisms...)
	if err != nil {
		return nil, err
	}
	return &Formatter{
		Config: cfg,
		snaker: s,
	}, nil
}

// Run formats Gunk files to be canonically formatted.
func Run(dir string, args ...string) error {
	if len(args) == 1 && args[0] == "-" {
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("error on loading: %w", err)
		}
		src, err := Source(buf)
		if err != nil {
			return fmt.Errorf("error on formatting: %w", err)
		}
		_, err = os.Stdout.Write(src)
		if err != nil {
			return fmt.Errorf("error on writing: %w", err)
		}
		return nil
	}
	fset := token.NewFileSet()
	l := loader.Loader{Dir: dir, Fset: fset}
	pkgs, err := l.Load(args...)
	if err != nil {
		return fmt.Errorf("error on loading: %w", err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("no Gunk packages to format")
	}
	if loader.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("encountered package loading errors")
	}
	for _, pkg := range pkgs {
		cfg, err := config.Load(pkg.Dir)
		if err != nil {
			return fmt.Errorf("unable to load gunkconfig: %w", err)
		}
		f, err := New(cfg)
		if err != nil {
			return fmt.Errorf("unable to initialize formatter: %w", err)
		}
		for i, file := range pkg.GunkSyntax {
			path := pkg.GunkFiles[i]
			orig, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error on reading: %w", err)
			}
			got, err := f.formatFile(fset, file)
			if err != nil {
				return fmt.Errorf("error on formating: %w", err)
			}
			if !bytes.Equal(orig, got) {
				if err := ioutil.WriteFile(path, got, 0o666); err != nil {
					return fmt.Errorf("error on writing: %w", err)
				}
			}
		}
	}
	return nil
}

// Source canonically formats a single Gunk file, returning the result and any
// error encountered.
func Source(src []byte) ([]byte, error) {
	f, err := New(&config.Config{})
	if err != nil {
		return nil, err
	}
	return f.Source(src)
}

// Source canonically formats a single Gunk file using the formatter's config,
// returning the result and any error encountered.
func (f *Formatter) Source(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return f.formatFile(fset, file)
}

func (f *Formatter) formatFile(fset *token.FileSet, file *ast.File) (_ []byte, formatErr error) {
	// Use custom panic values to report errors from the inspect func,
	// since that's the easiest way to immediately halt the process and
	// return the error.
	type inspectError struct{ err error }
	defer func() {
		if r := recover(); r != nil {
			if ierr, ok := r.(inspectError); ok {
				formatErr = ierr.err
			} else {
				panic(r)
			}
		}
	}()
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CommentGroup:
			if err := f.formatComment(fset, node); err != nil {
				panic(inspectError{err})
			}
		case *ast.StructType:
			if err := f.formatStruct(fset, node); err != nil {
				panic(inspectError{err})
			}
		}
		return true
	})
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (f *Formatter) formatComment(fset *token.FileSet, group *ast.CommentGroup) error {
	// Split the gunk tag ourselves, so we can support Source.
	doc, tags, err := loader.SplitGunkTag(nil, fset, group)
	if err != nil {
		return err
	}
	if len(tags) == 0 {
		// no gunk tags
		return nil
	}
	// If there is leading comments, add a new line
	// between them and the gunk tags.
	if doc != "" {
		doc += "\n\n"
	}
	for i, tag := range tags {
		var buf bytes.Buffer
		// Print with space indentation, since all comment lines begin
		// with "// " and we don't want to mix spaces and tabs.
		config := printer.Config{Mode: printer.UseSpaces, Tabwidth: 8}
		if err := config.Fprint(&buf, fset, tag.Expr); err != nil {
			return err
		}
		doc += "+gunk " + buf.String()
		if i < len(tags)-1 {
			doc += "\n"
		}
	}
	*group = *loader.CommentFromText(group, doc)
	return nil
}

func (f *Formatter) formatStruct(fset *token.FileSet, st *ast.StructType) error {
	if st.Fields == nil {
		return nil
	}
	// Figure out list of missing protobuf numbers.
	missingNum := make([]int, 0, len(st.Fields.List))
	if !f.Config.Format.PB { // Skip this if we are not going to use it anyways.
		// Find all unusedFields.
		unusedFields := make(map[int]bool, len(st.Fields.List))
		for i := 1; i <= len(st.Fields.List); i++ {
			unusedFields[i] = true
		}
		for _, field := range st.Fields.List {
			if field.Tag == nil {
				continue
			}
			tag, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				return err
			}
			pb, ok := reflect.StructTag(tag).Lookup("pb")
			if !ok {
				continue
			}
			pbNum, err := strconv.Atoi(pb)
			if err != nil {
				errorPos := fset.Position(field.Tag.Pos())
				// TODO: Add the same error checking in generate. Or, look at factoring
				// this code with the code in generate, they do very similar things?
				return fmt.Errorf("%s: struct field tag for pb contains a non-number %q", errorPos, pb)
			}
			delete(unusedFields, pbNum)
		}
		for k := range unusedFields {
			missingNum = append(missingNum, k)
		}
		sort.Ints(missingNum)
	}
	for i, field := range st.Fields.List {
		var key []string
		var value map[string]string
		if field.Tag != nil {
			tag, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				return err
			}
			key, value, err = parseTag(tag)
			if err != nil {
				// Don't touch tag if we can't read the tag.
				continue
			}
		}
		// Don't touch invalid code.
		if len(field.Names) != 1 {
			continue
		}
		// Insert JSON and protobuf key.
		entries := make([]string, 0, len(key))
		if f.Config.Format.PB {
			entries = append(entries, fmt.Sprintf("pb:%q", strconv.Itoa(i+1)))
		} else if _, ok := value["pb"]; ok {
			entries = append(entries, fmt.Sprintf("pb:%q", value["pb"]))
		} else {
			// Default behaviour: Add missing entries.
			entries = append(entries, fmt.Sprintf("pb:%q", strconv.Itoa(missingNum[0])))
			missingNum = missingNum[1:]
		}
		if f.Config.Format.JSON {
			entries = append(entries, fmt.Sprintf("json:%q", f.snaker.CamelToSnake(field.Names[0].Name)))
		} else if _, ok := value["json"]; ok {
			entries = append(entries, fmt.Sprintf("json:%q", value["json"]))
		}
		// Maintain other keys.
		for _, k := range key {
			if k == "pb" || k == "json" {
				// Skip pb and json as they have already been added to the start.
				continue
			}
			entries = append(entries, fmt.Sprintf("%s:%q", k, value[k]))
		}
		if len(entries) > 0 {
			field.Tag = &ast.BasicLit{
				ValuePos: field.Type.End() + 1,
				Kind:     token.STRING,
				Value:    "`" + strings.Join(entries, " ") + "`",
			}
		}
	}
	return nil
}

func parseTag(tag string) ([]string, map[string]string, error) {
	keys := make([]string, 0)
	values := make(map[string]string)
	for tag != "" {
		// skip leading space
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}
		// find colon separating key and value
		for i < len(tag) && tag[i] != ':' {
			i++
		}
		if i == len(tag) {
			return nil, nil, fmt.Errorf("unterminated key")
		}
		key := tag[:i]
		keys = append(keys, key)
		tag = tag[i+1:]
		// find end of value
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i == len(tag) {
			return nil, nil, fmt.Errorf("unterminated value")
		}
		value, err := strconv.Unquote(tag[:i+1])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid value")
		}
		values[key] = value
		tag = tag[i+1:]
	}

	return keys, values, nil
}
