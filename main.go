package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gunk/gunk/loader"
)

func main() {
	flag.Parse()

	if err := loader.Load(flag.Args()...); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
