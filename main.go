package main

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/gunk/gunk/convert"
	"github.com/gunk/gunk/format"
	"github.com/gunk/gunk/generate"
)

var (
	app = kingpin.New("gunk", "Gunk Unified N-terface Kompiler command-line tool.")

	gen         = app.Command("generate", "Generate code from Gunk packages.")
	genPatterns = gen.Arg("patterns", "patterns of Gunk packages").Strings()

	conv                  = app.Command("convert", "Convert Proto file to Gunk file.")
	convProtoFile         = conv.Arg("file", "Proto file to convert to Gunk").String()
	convOverwriteGunkFile = conv.Flag("overwrite", "overwrite the converted Gunk file if it exists.").Bool()

	frmt         = app.Command("format", "Format Gunk code.")
	frmtPatterns = frmt.Arg("patterns", "patterns of Gunk packages").Strings()
)

func main() {
	os.Exit(main1())
}

func main1() int {
	var err error
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case gen.FullCommand():
		err = generate.Generate("", *genPatterns...)
	case conv.FullCommand():
		err = convert.Convert(*convProtoFile, *convOverwriteGunkFile)
	case frmt.FullCommand():
		err = format.Format("", *frmtPatterns...)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
