package main

import (
	"os"

	"bikegeo/internal/cli"
	"bikegeo/internal/web"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, web.RunServe))
}
