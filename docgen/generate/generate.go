package generate

import (
	"fmt"
	"io"
	"io/ioutil"
	"text/template"

	"github.com/knq/snaker"

	"github.com/gunk/gunk/assets"
	"github.com/gunk/gunk/docgen/parser"
	"github.com/gunk/gunk/docgen/pot"
)

// Run generates a markdown file describing the API
// and a messages.pot containing all sentences that need to be
// translated.
func Run(w io.Writer, f *parser.FileDescWrapper, lang []string) (pot.Builder, error) {
	pb := pot.NewBuilder()

	api, err := parser.ParseFile(f)
	if err != nil {
		return nil, err
	}

	tplName := "annex.md"
	if api.HasServices() {
		tplName = "api.md"
	}

	b, err := loadTemplate(tplName)
	if err != nil {
		return nil, err
	}

	tmpl := template.Must(template.New("doc").
		Funcs(template.FuncMap{
			"GetText": func(txt string) string {
				if txt != "" {
					pb.AddTranslation(txt)
				}
				return txt
			},
			"AddSnippet": func(name string) string {
				return fmt.Sprintf("{{snippet %s %v}}", snaker.CamelToSnake(name), lang)
			},
		}).
		Parse(string(b)))

	if err := tmpl.Execute(w, api); err != nil {
		return nil, err
	}
	return pb, nil
}

func loadTemplate(name string) ([]byte, error) {
	file, err := assets.Assets.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}
