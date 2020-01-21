# About

`scopegen` is a [Gunk][gunk] plugin that reads OAuth2 scope definitions from
Gunk options in a `.gunk` source file, and generates code for supported
languages.

## Supported Languages

Currently, `scopegen` supports:

- Go
- JSON

## Installation

Use the following command to install scopegen:

```sh
$ go get -u github.com/gunk/gunk/scopegen
```

This will place `scopegen` in your `$GOBIN`

## Usage

In your project's `.gunkconfig` add the following:

```ini
[generate]
    command=scopegen
    go=true
    json=true
    output_version=2
```


## Output Version

Output version is configured with `output_version` configuration key, valid values are: 1, 2.
Output version 2 generates codes in a simplified form that doesn't use any custom type and preserve the scope name, 
description.
Output version 1 is kept only for backward compatible, and will be deprecated soon.

Example:

Output version 1:

```json
{"/test.Service/GetMessage":["read","write"],"/test.Service/GetMessage3":["read"]}
```

Output version 2:

```json
{"scopes":{"admin":"Grants read and write access to administrative information","read":"Grants read access","write":"Grants write access"},"auth_scopes":{"/test.Service/GetMessage":["read","write"],"/test.Service/GetMessage3":["read"]}}
```
