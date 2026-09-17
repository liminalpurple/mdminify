// This is a deliberately separate module so that the goldmark dependency
// required for HTML-equivalence testing never enters the mdminify module.
// A directory containing its own go.mod is excluded from the parent module's
// package patterns, so `go build ./...` and `go test ./...` at the repository
// root do not see this package.
module github.com/liminalpurple/mdminify/conformance

go 1.26.0

replace github.com/liminalpurple/mdminify => ../

require (
	github.com/liminalpurple/mdminify v0.0.0-00010101000000-000000000000
	github.com/yuin/goldmark v1.8.6
	golang.org/x/net v0.59.0
)
