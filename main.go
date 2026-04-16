package main

import (
	"os"

	"github.com/liminalpurple/mdminify/cmd"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
