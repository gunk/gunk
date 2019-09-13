# Scopegen

Scopegen is a plugin of gunk. It reads OAuth2 scope definitions in gunk options and generated codes for supported 
languages.

List of supported lanagues:

- go
- json

## Installation

Use the following command to install scopegen:

```sh
go get -u github.com/gunk/gunk/scopegen
```

This will place `scopegen` in your `$GOBIN`

## Usage

In your `.gunkconfig` add the following:

```ini
[generate]
command=scopegen
go=true
json=true
```
