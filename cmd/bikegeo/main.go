package main

import (
	"os"

	"bikegeo/pkg/cli"
	"bikegeo/pkg/web"
)

func main() {
	// `serve` lives here rather than in pkg/cli because pkg/web renders
	// reports by calling pkg/cli; the dependency only runs one way.
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Exit(web.RunServe(os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
