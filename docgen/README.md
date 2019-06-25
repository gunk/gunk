# Docgen

Docgen is a plugin of gunk. For now, it goes through the same gunk package used
in `gunk generate` command and generates a `messages.pot`.
The `messages.pot` files contains all strings from the openapi annotations.

## Installation

Use the following command to install docgen:

```sh
go get -u github.com/gunk/gunk/docgen
```

This will place `docgen` in your `$GOBIN`

## Usage

In your `.gunkconfig` add the following:

```ini
[generate]
command=docgen
out=examples/util/v1/
```
