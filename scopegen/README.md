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
```
