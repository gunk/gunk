# about testdata

testdata uses a [proxy](https://godoc.org/github.com/rogpeppe/go-internal/goproxytest) server to serve Go modules for local testing. Go modules are served as [txtar](https://godoc.org/github.com/rogpeppe/go-internal/cmd/txtar-addmod)

## Testing a new gunk/opt

To test a new option added to the gunk/opt repository, you will need to generate a new txtar go module. You can do so by following the steps below:

1/ download `go get https://github.com/rogpeppe/go-internal`

2/ build `go build cmd/txtar-addmod/addmod.go`

3/ run `txtar-addmod -all dir path@version`, `all` to include all source files, `dir` being where you want to create your txtar (`testdata/mod`), `path@version` being the module and the version you want to add (`github.com/gunk/opt@latest`)

4/ use the generated version in your test script by adding :

```
require (
	github.com/gunk/opt <VERSION>
)
```

`<VERSION>` is the version number you can find in the newly generated txtar file. 