package main

import (
	"fmt"
	"os"

	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/gunk/gunk/convert"
	"github.com/gunk/gunk/dump"
	"github.com/gunk/gunk/format"
	"github.com/gunk/gunk/generate"
	"github.com/gunk/gunk/log"
)

var (
	version = "v0.0.0-dev"

	app = kingpin.New("gunk", "The modern frontend and syntax for Protocol Buffers.")

	gen         = app.Command("generate", "Generate code from Gunk packages.")
	genPatterns = gen.Arg("patterns", "patterns of Gunk packages").Strings()

	conv                    = app.Command("convert", "Convert Proto file to Gunk file.")
	convProtoFilesOrFolders = conv.Arg("files_or_folders", "Proto files or folders to convert to Gunk").Strings()
	convOverwriteGunkFile   = conv.Flag("overwrite", "overwrite the converted Gunk file if it exists.").Bool()

	frmt         = app.Command("format", "Format Gunk code.")
	frmtPatterns = frmt.Arg("patterns", "patterns of Gunk packages").Strings()

	dmp         = app.Command("dump", "Write a FileDescriptorSet (a protocol buffer, defined in descriptor.proto)")
	dmpPatterns = dmp.Arg("patterns", "patterns of Gunk packages").Strings()
	dmpFormat   = dmp.Flag("format", "output format to write FileDescriptorSet as (options are 'raw' or 'json'").String()

	ver = app.Command("version", "Show Gunk version.")
)

func main() {
	os.Exit(main1())
}

func main1() int {
	app.HelpFlag.Short('h') // allow -h as well as --help

	gen.Flag("print-commands", "print the commands").Short('x').BoolVar(&log.PrintCommands)
	gen.Flag("verbose", "print the names of packages as they are generated").Short('v').BoolVar(&log.Verbose)

	var err error
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case ver.FullCommand():
		fmt.Fprintf(os.Stdout, "gunk %s\n", version)
	case gen.FullCommand():
		err = generate.Run("", *genPatterns...)
	case conv.FullCommand():
		err = convert.Run(*convProtoFilesOrFolders, *convOverwriteGunkFile)
	case frmt.FullCommand():
		err = format.Run("", *frmtPatterns...)
	case dmp.FullCommand():
		err = dump.Run(*dmpFormat, "", *dmpPatterns...)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
