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

## Required installs

    - Go (any specific version?)
    - Protoc

## Usage

### Syntax

The aim of Gunk is to provide Go-compatible syntax that can be natively read
and handled by the `go/*` package. As such, Gunk definitions are a subset of
the Go programming language:

```
// Message is a Echo message.
type Message struct {
	// Msg holds a message.
	Msg  string `pb:"1" json:"msg"`
	Code int    `pb:"2" json:"code"`
}

// Util is a utility service.
type Util interface {
	// Echo echoes a message.
	//
	// +gunk http.Match{
	//		Method:	"POST",
	// 		Path:	"/v1/echo",
	// 		Body:	"*",
	//	}
	Echo(Message) Message
}
```

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

## Gunk Config

Gunk is configurable by a `.gunkconfig` file. The `.gunkconfig` file or files
can be placed anywhere in the project and Gunk will attempt to find these
`.gunkconfig` files and merge them together. Gunk starts looking in the package
path and will continue looking up directories for any `.gunkconfig` until it
reaches the project root (the project root is where `.git` file or directory
or a `go.mod` file is).

Gunk uses an ini-style config file for declaring how Gunk should operate.
Each `[generate]` or `[generate <lang>]` section in the `.gunkconfig`
is a separate protoc generator run. A generate section can look like
the following:

```
[generate]
command=protoc-gen-go

[generate]
out=v1/js
protoc=js
```

`command` is a `protoc-gen-*` generator, it should be the binary name
of the generator to be run. The binary should be findable on your
`PATH`, otherwise the path to the binary can be used.

`protoc` will make Gunk call `protoc` with the out cli parameter
specified; `protoc=js` will call `protoc --js_out=OUT_PATH`

`out` is the file location the generated files should be outputted.

Any other key-values specified in a `generate` section will be
passed as plugin parameters to `protoc` or the `protoc-gen-*` generator.

You can also use a shortened generate section syntax:

```
[generate go]

[generate js]
out=v1/js
```

This is equivalent to:

```
[generate]
command=protoc-gen-go

[generate]
out=v1/js
protoc=js
```

### Example

```
# Example .gunkconfig for Go, grpc-gateway, Python and JS
[generate go]
out=v1/go
plugins=grpc

[generate]
out=v1/go
command=protoc-gen-grpc-gateway
logtostderr=true

[generate python]
out=v1/python

[generate js]
out=v1/js
import_style=commonjs
binary
```

## Convert

Gunk provides functionality to convert a proto file to a Gunk file.

    $ gunk convert /path/to/file.proto

This will convert your proto file to the equivalent Gunk file. Currently
this only works for single proto files.

## Gunk Annotations

[go-install]: https://golang.org/doc/install
[go-modules]: https://github.com/golang/go/wiki/Modules
[go-project]: https://golang.org/project
[issue-tracker]: https://github.com/gunk/gunk/issues
