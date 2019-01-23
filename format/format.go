package format

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"reflect"
	"strconv"

	"github.com/gunk/gunk/loader"
)

// Run formats Gunk files to be canonically formatted.
func Run(dir string, args ...string) error {
	fset := token.NewFileSet()
	l := loader.Loader{Dir: dir, Fset: fset}
	pkgs, err := l.Load(args...)
	if err != nil {
		return err
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
				return err
			}

			got, err := formatFile(fset, file)
			if err != nil {
				return err
			}

			if !bytes.Equal(orig, got) {
				if err := ioutil.WriteFile(path, got, 0666); err != nil {
					return err
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
		switch node.(type) {
		case *ast.CommentGroup:
			if err := formatComment(fset, node.(*ast.CommentGroup)); err != nil {
				panic(inspectError{err})
			}
		case *ast.StructType:
			if err := formatStruct(fset, node.(*ast.StructType)); err != nil {
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
	// Find which struct fields require sequence numbers, and
	// keep a record of which sequence numbers are already used.
	usedSequences := []int{}
	fieldsWithoutSequence := []*ast.Field{}
	for _, f := range st.Fields.List {
		tag := f.Tag
		if tag == nil {
			fieldsWithoutSequence = append(fieldsWithoutSequence, f)
			continue
		}
		// Can skip the error here because we've already parsed the file.
		str, _ := strconv.Unquote(tag.Value)
		stag := reflect.StructTag(str)
		val, ok := stag.Lookup("pb")
		// If there isn't a 'pb' tag present.
		if !ok {
			fieldsWithoutSequence = append(fieldsWithoutSequence, f)
			continue
		}
		// If there was a 'pb' tag, but it wasn't empty, return an error.
		// It is a bit difficult to add in the sequence number if the 'pb'
		// tag already exists.
		if ok && val == "" {
			errorPos := fset.Position(tag.Pos())
			return fmt.Errorf("%s: struct field tag for pb was empty, please remove or add sequence number", errorPos)
		}
		// If there isn't a number in 'pb' then return an error.
		i, err := strconv.Atoi(val)
		if err != nil {
			errorPos := fset.Position(tag.Pos())
			// TODO: Add the same error checking in generate. Or, look at factoring
			// this code with the code in generate, they do very similar things?
			return fmt.Errorf("%s: struct field tag for pb contains a non-number %q", errorPos, val)
		}
		usedSequences = append(usedSequences, i)
	}

	// Determine missing sequences.
	missingSequences := []int{}
	for i := 1; i < len(st.Fields.List)+1; i++ {
		found := false
		for _, u := range usedSequences {
			if u == i {
				found = true
				break
			}
		}
		if !found {
			missingSequences = append(missingSequences, i)
		}
	}

	// Add the sequence number to the field tag, creating a new
	// tag if one doesn't exist, or prepend the sequence number
	// to the tag that is already there.
	for i, f := range fieldsWithoutSequence {
		nextSequence := missingSequences[i]
		if f.Tag == nil {
			f.Tag = &ast.BasicLit{
				ValuePos: f.Type.End() + 1,
				Kind:     token.STRING,
				Value:    fmt.Sprintf("`pb:\"%d\"`", nextSequence),
			}
		} else {
			// Remove the string quoting around so it is easier to prepend
			// the sequence number.
			tagValueStr, _ := strconv.Unquote(f.Tag.Value)
			f.Tag.Value = fmt.Sprintf("`pb:\"%d\" %s`", nextSequence, tagValueStr)
		}
	}
	return nil
}
