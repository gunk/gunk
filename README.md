# About gunk

Gunk is an acronym for "Gunk Unified N-terface Kompiler."

Gunk primarily works as a frontend for `protobuf`'s, `protoc` compiler. `gunk`
aims is to provide a way to work with `protobuf` files / projects in the same
way as the Go programming language's toolchain allows for working with Go
projects.

Gunk provides an alternative Go-derived syntax for defining `protobuf`'s, that
is simpler and easier to work with. Additionally, for developers familar with
the Go programming language will be instantly comfortable with Gunk files and
syntax.

## Installing

Gunk can be installed in the usual Go fashion:

	go get -u github.com/gunk/gunk

## Syntax

The aim of Gunk is to provide Go-compatible syntax that can be natively read
and handled by the `go/*` package. As such, Gunk definitions are a subset of
the Go programming language:

	// examples/util/echo.gunk

## Working with `*.gunk` files

Working with the `gunk` command line tool should be instantly recognizable to
experienced Go developers:

	gunk ./examples/util

## Overview

Gunk works by using Go's standard library `go/ast` package (and related
packages) to parse all `*.gunk` files in a "Gunk Package" (and applying the
same kinds of rules as Go package), building the appropos protobuf messages. In
turn, those are passed to `protoc-gen-*` tools, along with any passed options.
