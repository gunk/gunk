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
	"strconv"
	"strings"

	"github.com/gunk/gunk/loader"
	"github.com/kenshaw/snaker"
)

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
		for i, file := range pkg.GunkSyntax {
			path := pkg.GunkFiles[i]
			orig, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error on reading: %w", err)
			}
			got, err := formatFile(fset, file)
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
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return formatFile(fset, file)
}

func formatFile(fset *token.FileSet, file *ast.File) (_ []byte, formatErr error) {
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
			if err := formatComment(fset, node); err != nil {
				panic(inspectError{err})
			}
		case *ast.StructType:
			if err := formatStruct(fset, node); err != nil {
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

func formatComment(fset *token.FileSet, group *ast.CommentGroup) error {
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

func formatStruct(fset *token.FileSet, st *ast.StructType) error {
	if st.Fields == nil {
		return nil
	}
	for i, f := range st.Fields.List {
		var key []string
		var value map[string]string
		if f.Tag != nil {
			tag, err := strconv.Unquote(f.Tag.Value)
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
		if len(f.Names) != 1 {
			continue
		}
		// Insert JSON and protobuf key.
		entries := make([]string, 0, len(key))
		entries = append(entries, fmt.Sprintf("pb:%q", strconv.Itoa(i+1)))
		entries = append(entries, fmt.Sprintf("json:%q", snaker.CamelToSnake(f.Names[0].Name)))
		// Maintain other keys.
		for _, k := range key {
			if k == "pb" || k == "json" {
				// Skip pb and json as they have already been added to the start.
				continue
			}
			entries = append(entries, fmt.Sprintf("%s:%q", k, value[k]))
		}
		f.Tag = &ast.BasicLit{
			ValuePos: f.Type.End() + 1,
			Kind:     token.STRING,
			Value:    "`" + strings.Join(entries, " ") + "`",
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
