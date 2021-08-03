package generate

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/gunk/gunk/docgen/parser"
	"github.com/gunk/gunk/docgen/pot"
	"github.com/gunk/gunk/docgen/templates"
)

var ErrNoServices = fmt.Errorf("file has no services")

type CustomHeaderIdsOpt int

const (
	CustomHeaderIdsOptDisabled CustomHeaderIdsOpt = iota
	CustomHeaderIdsOptEnabled
)

// Run generates a markdown file describing the API and a messages.pot
// containing all sentences that need to be translated.
func Run(w io.Writer, f *parser.FileDescWrapper, lang []string,
	customHeaderIds CustomHeaderIdsOpt, onlyExternal parser.OnlyExternalOpt) (pot.Builder, error) {
	pb := pot.NewBuilder()
	api, err := parser.ParseFile(f, onlyExternal)
	if err != nil {
		return nil, err
	}
	if !api.HasServices() {
		return nil, ErrNoServices
	}
	if err := api.CheckSwagger(); err != nil {
		name := ""
		if f.Name != nil {
			name = *f.Name
		}
		return nil, fmt.Errorf("swagger error in file %s: %w", name, err)
	}

	tpl := template.Must(template.New("doc").
		Funcs(template.FuncMap{
			"CustomHeaderId": func(txt ...string) string {
				if customHeaderIds != CustomHeaderIdsOptEnabled {
					return ""
				}
				return fmt.Sprintf("{#%s}", strings.Join(txt, ""))
			},
			"GetText": func(txt string) string {
				if txt != "" {
					pb.AddTranslation(txt)
				}
				return txt
			},
			"AddSnippet": func(name string) (string, error) {
				return addSnippet(name, lang)
			},
			"mdType": func(txt string) string {
				return strings.ReplaceAll(txt, "[", "\\[")
			},
		}).Parse(templates.API))
	if err := tpl.Execute(w, api); err != nil {
		return nil, err
	}
	return pb, nil
}
