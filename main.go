// mdminify is a command-line tool that minifies markdown files without
// changing their rendered meaning. See the [minify] package for the
// core library.
package main

import (
	"os"

	"github.com/liminalpurple/mdminify/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
