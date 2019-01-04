[![GoDoc](https://godoc.org/github.com/gunk/gunk?status.svg)](https://godoc.org/github.com/gunk/gunk)
[![Build Status](https://travis-ci.org/gunk/gunk.svg?branch=master)](https://travis-ci.org/gunk/gunk)

# Gunk

Gunk is an acronym for "Gunk Unified N-terface Kompiler".

Gunk primarily works as a frontend for `protobuf`'s, `protoc` compiler. `gunk`
aims is to provide a way to work with `protobuf` files / projects in the same
way as the Go programming language's toolchain allows for working with Go
projects.

Gunk provides an alternative Go-derived syntax for defining `protobuf`'s, that
is simpler and easier to work with. Additionally, for developers familar with
the Go programming language will be instantly comfortable with Gunk files and
syntax.

### Contents

* [Installing](#installing)
* [Usage](#usage)
* [Contributing](#contributing)
* [How it works](#how-it-works)

## Installing

Gunk requires [Go][go-project] to be installed on your machine. To install Go,
follow the [installation instructions][go-install] for your specific operating
systems.

Gunk can then be installed in the usual Go fashion:

	go get -u github.com/gunk/gunk

## Usage

### Syntax

The aim of Gunk is to provide Go-compatible syntax that can be natively read
and handled by the `go/*` package. As such, Gunk definitions are a subset of
the Go programming language:

	// examples/util/echo.gunk

### Working with `*.gunk` files

Working with the `gunk` command line tool should be instantly recognizable to
experienced Go developers:

	gunk generate ./examples/util

### More information

Please see [the GoDoc API page][godoc] for a full API listing.

## How it works

Gunk works by using Go's standard library `go/ast` package (and related
packages) to parse all `*.gunk` files in a "Gunk Package" (and applying the
same kinds of rules as Go package), building the appropos protobuf messages. In
turn, those are passed to `protoc-gen-*` tools, along with any passed options.

## Contributing

### Bug Reports & Feature Requests

Please use the [issue tracker][issue-tracker] to report any bugs or file feature requests.

### Developing

PRs are welcome. To begin developing, do this:

```bash
$ git clone https://github.com/gunk/gunk.git
$ export GO111MODULE=on
$ cd gunk/
$ go build
$ ./gunk
usage: gunk [<flags>] <command> [<args> ...]

Gunk Unified N-terface Kompiler command-line tool.

Flags:
  -h, --help  Show context-sensitive help (also try --help-long and --help-man).

Commands:
  help [<command>...]
    Show help.

  generate [<flags>] [<patterns>...]
    Generate code from Gunk packages.

  convert [<flags>] [<file>]
    Convert Proto file to Gunk file.

  format [<patterns>...]
    Format Gunk code.
```

This project uses [go modules][go-modules] for managing dependencies, which
comes with Go 1.11 and above. Before sending a PR, please do the following:

    GO111MODULE=on go mod tidy

[go-install]: https://golang.org/doc/install
[go-modules]: https://github.com/golang/go/wiki/Modules
[go-project]: https://golang.org/project
[issue-tracker]: https://github.com/gunk/gunk/issues
