package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gunk/gunk/loader"
)

func main() {
	os.Exit(main1())
}

func main1() int {
	flag.Parse()

	if err := loader.Load("", flag.Args()...); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}
