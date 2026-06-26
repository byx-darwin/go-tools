module github.com/byx-darwin/go-tools/example

go 1.25

replace (
	github.com/byx-darwin/go-tools/go-auth => ../go-auth
	github.com/byx-darwin/go-tools/go-common => ../go-common
	github.com/byx-darwin/go-tools/go-framework => ../go-framework
	github.com/byx-darwin/go-tools/go-middleware => ../go-middleware
)
