package main

import (
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/gunk/gunk/generate"
)

var (
	app = kingpin.New("gunk", "Gunk Unified N-terface Kompiler command-line tool.")

	gen         = app.Command("generate", "Generate code.")
	genPatterns = gen.Arg("patterns", "patterns of Gunk packages").Strings()
)

func main() {
	os.Exit(main1())
}

func main1() int {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	// Register user
	case gen.FullCommand():
		if err := generate.Generate("", *genPatterns...); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
	}

	return 0
}
