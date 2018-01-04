# About gunk

Gunk is an acronym for "Gunk Unified N-terface Kompiler."

Gunk primarily works as a frontend for `protobuf`'s, `protoc` compiler. `gunk`
aims is to provide a way to work with `protobuf` files / projects in the same
way as the Go programming language's toolchain allows for working with Go
projects.

Gunk provides an alternative Go-derived syntax for defining `protobufs`, that
is simpler and easier to work with. Additionally, for developers building Go
projects, the syntax will be instantly recognizable and require almost no
learning curve.

## Installing

Gunk can be installed in the usual Go fashion:

```sh
$ go get -u github.com/gunk/gunk/cmd/gunk
```

## Syntax

The aim of Gunk is to provide Go-compatible syntax that can be natively read
and handled by the `go/ast` package. As such, Gunk definitions are a subset of
the Go programming language:

```go
// examples/echo/echo.gunk
```

## Working with `*.gunk` files

Working with the `gunk` command line tool should be instantly recognizable to
experienced Go developers:

```sh
$ gunk
```

## Overview

Gunk works by using Go's standard library `go/ast` package (and related
packages) to parse all `*.gunk` files in a "Gunk Package" (and applying the
same kinds of rules as Go package), building the appropos protobuf messages. In
turn, those are passed to `protoc-gen-*` tools, along with any passed options.
