# Gunk [![GoDoc][godoc]][godoc-link]

[godoc]: https://godoc.org/github.com/gunk/gunk?status.svg "GoDoc"
[godoc-link]: https://godoc.org/github.com/gunk/gunk

Gunk is a modern frontend and syntax for [Protocol Buffers][protobuf].

[Quickstart][] | [Installing][] | [Syntax][] | [Configuring][] | [About][] | [Releases][]

[quickstart]: #quickstart "Quickstart"
[installing]: #installing "Installing"
[syntax]: #protocol-types-and-messages "Protocol Types and Messages Syntax"
[configuring]: #project-configuration-files "Project Configuration Files"
[about]: #about "About"
[releases]: https://github.com/gunk/gunk/releases "Releases"

## Overview

Gunk provides a modern project-based workflow along with a [Go-derived][go-project]
syntax for defining types and services for use with [Protocol Buffers][protobuf].
Gunk is designed to integrate cleanly with existing [`protoc`][protobuf] based
build pipelines, while standardizing workflows in a way that is
familiar/accessible to Go developers.

## Quickstart

Create a working directory for a project:

```sh
$ mkdir -p ~/src/example && cd ~/src/example
```

[Install `gunk`][installing] and place the following [Gunk definitions][syntax]
in `example/util.gunk`:

[gunk definition]: #syntax "Gunk Protocol Syntax"

```go
package util

// Util is a utility service.
type Util interface {
	// Echo returns the passed message.
	Echo(Message) Message
}

// Message contains an echo message.
type Message struct {
	// Msg is a message from a client.
	Msg string `pb:"1"`
}
```

Create the corresponding [project configuration][] in `example/.gunkconfig`:

[project configuration]: #gunk-project-config "Gunk Project Configuration File"

```ini
[generate go]

[generate js]
import_style=commonjs
binary
```

Then, generate [protocol buffer][protobuf] definitions/code:

```sh
$ ls -A
.gunkconfig  util.gunk

$ gunk generate

$ ls -A
all.pb.go  all_pb.js  .gunkconfig  util.gunk
```

As seen above, `gunk` generated the corresponding Go and JavaScript [protobuf
code][protobuf] using the options defined in the `.gunkconfig`.

#### End-to-end Example

A end-to-end example gRPC server implementation, using Gunk definitions [is
available for review][gunk-example-server].

#### Debugging `protoc` commands

Underlying commands executed by `gunk` can be viewed with the following:

```sh
$ gunk generate -x
protoc-gen-go
protoc --js_out=import_style=commonjs,binary:/home/user/example --descriptor_set_in=/dev/stdin all.proto
```

## Installing

The `gunk` command-line tool can be installed [via Release][], [via Homebrew][], [via Scoop][] or [via Go][]:

[via release]: #installing-via-release
[via homebrew]: #installing-via-homebrew-macos
[via scoop]: #installing-via-scoop-windows
[via go]: #installing-via-go

### Installing via Release

1. [Download a release for your platform][releases]
2. Extract the `gunk` or `gunk.exe` file from the `.tar.bz2` or `.zip` file
3. Move the extracted executable to somewhere on your `$PATH` (Linux/macOS) or
   `%PATH%` (Windows)

### Installing via Homebrew (macOS)

`gunk` is available in the [`gunk/gunk` tap][gunk-tap], and can be installed in
the usual way with the [`brew` command][homebrew]:

```sh
# add tap
$ brew tap gunk/gunk

# install gunk
$ brew install gunk
```

### Installing via Scoop (Windows)

`gunk` can be installed using [Scoop](https://scoop.sh):

```powershell
# install scoop if not already installed
iex (new-object net.webclient).downloadstring('https://get.scoop.sh')

scoop install gunk
```

### Installing via Go

`gunk` can be installed in the usual Go fashion:

```sh
# install gunk
$ go get -u github.com/gunk/gunk
```

## Protobuf Dependency and Caching

The `gunk` command-line tool uses the `protoc` command-line tool. `gunk` can be configured
to use `protoc` at a specified path. If it isn't available, `gunk` will
[download the latest protobuf release][protobuf-releases] to the user's cache,
for use. It's also possible to pin a specific version, see the section on [protoc configuration][].

[protoc configuration]: #section-protoc

## Protocol Types and Messages

Gunk provides an alternate, Go-derived syntax for defining [protocol
buffers][protobuf]. As such, Gunk definitions are a subset of the Go
programming language. Additionally, a special `+gunk` annotation is recognized
by `gunk`, to allow the declaration of [protocol buffer options][protobuf-options]:

```go
package message

import "github.com/gunk/opt/http"

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
	// 	Method:	"POST",
	// 	Path:	"/v1/echo",
	// 	Body:	"*",
	// }
	Echo(Message) Message
}
```

Technically speaking, gunk is not actually strict subset of go, as gunk allows
unused imports; it actually requires them for some features.

See the example above;
in pure go, this would not be a valid go code, as `http` is not used outside of the comment.

### Scalars

Gunk's Go-derived syntax uses the canonical [Go scalar types][protobuf-types]
of the `proto3` syntax, defined by the [protocol buffer project][protobuf]:

| Proto3 Type | Gunk Type |
| ----------- | --------- |
| `double`    | `float64` |
| `float`     | `float32` |
| `int32`     | `int`     |
| `int32`     | `int32`   |
| `int64`     | `int64`   |
| `uint32`    | `uint`    |
| `uint32`    | `uint32`  |
| `uint64`    | `uint64`  |
| `bool`      | `bool`    |
| `string`    | `string`  |
| `bytes`     | `[]byte`  |

**Note:** Variable-length scalars will be enabled in the future using a tag
parameter.

[gunk ons]: #gunk-annotations "Gunk Annotation Syntax"

### Messages

Gunk's Go-derived syntax uses Go's `struct` type declarations for declaring
messages, and require a `pb:"<field_number>"` tag to indicate the field number:

```go
type Message struct {
	FieldA string `pb:"1"`
}

type Envelope struct {
	Message Message  `pb:"1" json:"msg"`
}
```

There are additional tags (for example, the `json:` tag above), that will be
recognized by `gunk format`, and passed on to generators, where possible.

**Note:** When using [`gunk format`][], a valid `pb:"<field_number>"` tag will be automatically
inserted if not declared.

[`gunk format`]: #formatting-gunk-files

### Services

Gunk's Go-derived syntax uses Go's `interface` syntax for declaring services:

```go
type SearchService interface {
	Search(SearchRequest) SearchResponse
}
```

The above is equivalent to the following protobuf syntax:

```proto3
service SearchService {
  rpc Search (SearchRequest) returns (SearchResponse);
}
```

### Enums

Gunk's Go-derived syntax uses Go `const`'s for declaring enums:

```go
type MyEnum int

const (
	MYENUM MyEnum = iota
	MYENUM2
)
```

**Note:** values can also be fixed numeric values or a calculated value (using
`iota`).

### Maps

Gunk's Go-derived syntax uses Go `map`'s for declaring `map` fields:

```go
type Project struct {
	ProjectID string `pb:"1" json:"project_id"`
}

type GetProjectResponse struct {
	Projects map[string]Project `pb:"1"`
}
```

### Repeated Values

Gunk's Go-derived syntax uses Go's slice syntax (`[]`) for declaring a
repeated field:

```go
type MyMessage struct {
	FieldA []string `pb:"1"`
}
```

### Message Streams

Gunk's Go-derived syntax uses Go `chan` syntax for declaring streams:

```go
type MessageService interface {
	List(chan Message) chan Message
}
```

The above is equivalent to the following protobuf syntax:

```proto3
service MessageService {
  rpc List(stream Message) returns (stream Message);
}
```

### Protocol Options

[Protocol buffer options][protobuf-options] are standard messages (ie, a
`struct`), and can be attached to any service, message, enum, or other other
type declaration in a Gunk file via the doccomment preceding the type, field,
or service:

```go
// MyOption is an option.
type MyOption struct {
	Name string `pb:"1"`
}

// +gunk MyOption {
// 	Name: "test",
// }
type MyMessage struct {
	/* ... */
}
```

## Project Configuration Files

Gunk uses a top-level `.gunkconfig` configuration file for managing the Gunk
protocol definitons for a project:

```ini
# Example .gunkconfig for Go, grpc-gateway, Python and JS, TypeScript (CommonJS), TypeScript

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

[generate ts]
service=true
plugin_version=v0.12.0
fix_paths_post_proc=true

[generate ts_proto]
service=true
plugin_version=v1.110.4

[generate protobuf-ts]
ts_nocheck
eslint_disable
optimize_code_size
plugin_version= v2.9.4

```

- `[generate ts]` uses [ts-protoc-gen](https://www.npmjs.com/package/ts-protoc-gen)
- `[generate ts_proto]` uses [ts-proto](https://www.npmjs.com/package/ts-proto)
- `[generate protobuf-ts]` uses [@protobuf-ts/plugin](https://www.npmjs.com/package/@protobuf-ts/plugin)

### Project Search Path

When `gunk` is invoked from the command-line, it searches the passed package
spec (or current working directory) for a `.gunkconfig` file, and walks up the
directory hierarchy until a `.gunkconfig` is found, or the project's root is
encountered. The project root is defined as the top-most directory containing a
`.git` subdirectory, or where a `go.mod` file is located.

### Format

The `.gunkconfig` file format is compatible with [Git config syntax][git-config],
and in turn is compatible with the INI file format:

```ini
[generate]
command=protoc-gen-go

[generate]
out=v1/js
protoc=js
```

### Global section

- `import_path` - see "Converting Existing Protobuf Files"

- `strip_enum_type_names` - with this option on, enums with their type prefixed
  will be renamed to the version without prefix.

  Note that this might produce invalid protobuf that stops compiling in 1.4.\*
  protoc-gen-go, if the enum names clash.

### Section `[format]`

The configuration options for formatting Gunk files where formatting options
that may break program behavior can be enabled.

#### Parameters

- `snake_case_json` - automatically sets all field tags of JSON to their snake
  cased name if enabled

- `initialisms` - comma-separated list of initialisms to be used when
  formatting JSON names

- `reorder_pb` - automatically sets pb according to the field's order,
  overwriting previous pb fields

### Section `[protoc]`

The path where to check for (or where to download) the `protoc` binary can be configured.
The version can also be pinned.

#### Parameters

- `version` - the version of protoc to use. If unspecified, defaults
  to the latest release available. Otherwise, gunk will either download the specified
  version, or check that the version of `protoc` at the specified path matches what was
  configured.

- `path` - the path to check for the `protoc` binary. If unspecified, defaults appropriate user
  cache directory for the user's OS. If no file exists at the path, `gunk` will attempt to download
  protoc.

### Section `[generate[ <type>]]`

Each `[generate]` or `[generate <type>]` section in a `.gunkconfig` corresponds
to a invocation of the `protoc-gen-<type>` tool.

#### Parameters

Each `name[=value]` parameter defined within a `[generate]` section will be
passed as a parameter to the `protoc-gen-<type>` tool, with the exception of
the following special parameters that override the behavior of the `gunk generate` tool:

- `command` - overrides the `protoc-gen-*` command executable used by
  `gunk generate`. The executable must be findable on `$PATH` (Linux/macOS) or
  `%PATH%` (Windows), or may be the full path to the executable. If not
  defined, then `command` will be `protoc-gen-<type>`, when `<type>` is the
  value in `[generate <type>]`.

- `protoc` - overrides the `<type>` value, causing `gunk generate` to use the
  `protoc` value in place of `<type>`.

- `out` - overrides the output path of `protoc`. If not defined, output will be
  the same directory as the location of the `.gunk` files.

- `plugin_version` - specify version of plugin. The plugin is downloaded
  from github/maven, built in cache and used. It is _not_ installed in $PATH.
  This currently works with the following plugins:

  - `protoc-gen-go`
  - `protoc-gen-grpc-java`
  - `protoc-gen-grpc-gateway`
  - `protoc-gen-openapiv2` (`protoc-gen-swagger` support is deprecated)
  - `protoc-gen-swift` (installing swift itself first is necessary)
  - `protoc-gen-grpc-swift` (installing swift itself first is necessary)
  - `protoc-gen-ts` (installing node and npm first is necessary)
  - `protoc-gen-grpc-python` (cmake, gcc is necessary; takes ~10 minutes to clone build)

  It is recommended to use this function everywhere, for reproducible builds,
  together with `version` for protoc.

- `json_tag_postproc` - uses `json` tags defined in gunk file also for go-generated
  file

- `fix_paths_postproc` - for `js` and `ts` - by default, gunk generates wrong
  paths for other imported gunk packages, because of the way gunk moves files
  around.
  Works only if `js` also has `import_style=commonjs` option.

- `generate_single`- runs the generator with all files at once instead of per
  package.
  The `out` parameter must be set when enabled.

All other `name[=value]` pairs specified within the `generate` section will be
passed as plugin parameters to `protoc` and the `protoc-gen-<type>` generators.

#### Short Form

The following `.gunkconfig`:

```ini
[generate go]

[generate js]
out=v1/js
```

is equivalent to:

```ini
[generate]
command=protoc-gen-go

[generate]
out=v1/js
protoc=js
```

#### Different forms of invocation

There are three different forms of gunkconfig sections that have three
different semantics.

```ini
[generate]
command=protoc-gen-go

[generate]
protoc=go

[generate go]
```

The first one uses protoc-gen-go plugin directly, without using protoc.
It also attempts to move files to the same directory as the gunk file.

The second one uses protoc and does not attempt to move any files.
Protoc attempts to load plugin from $PATH, if it is not one of the
built-in protoc plugins; this will _not_ work together with pinned version
and other gunk features and is not recommended outside of built-in
protoc generators.

The third version is reccomended. It will try to detect whether language
is one of built-in
protoc generators, in that case behaves like the second way, otherwise
behaves like the first.

The built-in protoc generators are:

- cpp
- java
- python
- php
- ruby
- csharp
- objc
- js

## Third-Party Protobuf Options

Gunk provides the [`+gunk` annotation syntax][] for declaring [protobuf
options][protobuf-options], and specially recognizes some third-party API
annotations, such as Google HTTP options, including all builtin/standard
`protoc` options for code generation:

[`+gunk` annotation syntax]: #protocol-options "Gunk Annotation Syntax"

```go
// +gunk java.Package("com.example.message")
// +gunk java.MultipleFiles(true)
package message

import (
	"github.com/gunk/opt/http"
	"github.com/gunk/opt/file/java"
)

type Util interface {
	// +gunk http.Match{
	// 	Method:	"POST",
	// 	Path:	"/v1/echo",
	// 	Body:	"*",
	// }
	Echo()
}
```

Further documentation on available options can be found at the
[Gunk options project][gunk-options].

## Formatting Gunk Files

Gunk provides the `gunk format` command to format `.gunk` files (akin to `gofmt`):

```sh
$ gunk format /path/to/file.gunk
$ gunk format <pathspec>
```

## Converting Existing Protobuf Files

Gunk provides the `gunk convert` command that will converting existing `.proto`
files (or a directory) to the Go-derived Gunk syntax:

```sh
$ gunk convert /path/to/file.proto
$ gunk convert /path/to/protobuf/directory
```

If your `.proto` is referencing another `.proto` from another directory,
you can add `import_path` in the global section of your `.gunkconfig`.
If you don't provide `import_path` it will only search in the root directory.

```ini
import_path=relative/path/to/protobuf/directory
```

> The path to provide is relative from the `.gunkconfig` location.

Furthermore, the referenced files must contain:

```proto
option go_package="path/of/go/package";
```

The resulting `.gunk` file will contain the import path as defined in `go_package`:

```go
import (
	name "path/of/go/package"
)
```

## About

Gunk is developed by the team at [Brankas][brankas], and was designed to
streamline API design and development.

### History

From the beginning of the company, the Brankas team defined API types and
services in `.proto` files, leveraging ad-hoc `Makefile`'s, shell scripts, and
other non-standardized mechanisms for generating [Protocol Buffer code][protobuf].

As development exploded in 2017 (and beyond) with continued addition of backend
microservices/APIs, more code repositories and projects, and team members, it
became necessary to standardize tooling for the organization as well as reduce
the cognitive load of developers (who for the most part were working almost
exclusively with Go) when declaring gRPC and REST services.

### Naming

The Gunk name has a cheeky, backronym "Gunk Unified N-terface Kompiler",
however the name was chosen because it was possible to secure the GitHub `gunk`
project name, was short, concise, and not used by other projects.

Additionally, "gunk" is an apt description for the "gunk" surrounding protocol
definition, generation, compilation, and delivery.

## Contributing

Issues, Pull Requests, and other contributions are greatly welcomed and
appreciated! Get started with building and running `gunk`:

```sh
# clone source repository
$ git clone https://github.com/gunk/gunk.git && cd gunk

# force GO111MODULES
$ export GO111MODULE=on

# build and run
$ go build && ./gunk
```

### Dependency Management

Gunk uses [Go modules][go-modules] for dependency management, and as such
requires Go 1.11+. Please run `go mod tidy` before submitting any PRs:

```sh
$ export GO111MODULE=on
$ cd gunk && go mod tidy
```

[brankas]: https://brank.as/
[git-config]: https://git-scm.com/docs/git-config
[go-modules]: https://github.com/golang/go/wiki/Modules
[go-project]: https://golang.org/project
[gunk-options]: https://github.com/gunk/opt
[gunk-example-server]: https://github.com/gunk/gunk-example-server
[gunk-tap]: https://github.com/gunk/homebrew-gunk
[homebrew]: https://brew.sh/
[protobuf]: https://developers.google.com/protocol-buffers/
[protobuf-options]: https://developers.google.com/protocol-buffers/docs/proto#options
[protobuf-releases]: https://github.com/protocolbuffers/protobuf/releases
[protobuf-types]: https://developers.google.com/protocol-buffers/docs/proto3#scalar
