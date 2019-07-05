package generate

import (
	"fmt"
	"io"
	"io/ioutil"
	"text/template"

	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/knq/snaker"

	"github.com/gunk/gunk/assets"
	"github.com/gunk/gunk/docgen/parser"
	"github.com/gunk/gunk/docgen/pot"
)

// Run generates a markdown file describing the API
// and a messages.pot containing all sentences that need to be
// translated.
func Run(w io.Writer, f *google_protobuf.FileDescriptorProto, lang []string) (pot.Builder, error) {
	pb := pot.NewBuilder()

	b, err := loadTemplate()
	if err != nil {
		return nil, err
	}

	tmpl := template.Must(template.New("api.md").
		Funcs(template.FuncMap{
			"GetText": func(txt string) string {
				if txt != "" {
					pb.AddTranslations([]string{txt})
				}
				return fmt.Sprintf("%s", txt)
			},
			"AddSnippet": func(name string) string {
				return fmt.Sprintf("{{snippet %s %v}}", snaker.CamelToSnake(name), lang)
			},
		}).
		Parse(string(b)))

	api, err := parser.ParseFile(f)
	if err != nil {
		return nil, err
	}

	if err := tmpl.Execute(w, api); err != nil {
		return nil, err
	}
	return pb, nil
}

func loadTemplate() ([]byte, error) {
	file, err := assets.Assets.Open("api.md")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ioutil.ReadAll(file)
}
