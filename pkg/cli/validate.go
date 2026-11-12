package cli

import (
	"flag"
	"fmt"

	"bikegeo/pkg/compare"
	"bikegeo/pkg/validator"
)

func (a *App) runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var sizeFlag string
	var configFlag string
	var allFlag bool

	fs.StringVar(&sizeFlag, "size", "", "Specific frame size to validate (default: frame's default_size)")
	fs.StringVar(&configFlag, "config", "", "Specific frame configuration ID to validate")
	fs.BoolVar(&allFlag, "all", false, "Validate all frames")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"all": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	tokens := fs.Args()
	if allFlag {
		tokens = nil
	}
	targets, err := compare.ResolveValidationTargets(tokens, sizeFlag)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	doc, allValid, err := validator.BuildValidationDocument(targets)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	if err := a.printDocument(doc, nil); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	if !allValid {
		return 1
	}
	return 0
}
