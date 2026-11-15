package cli

import (
	"flag"
	"fmt"
	"io"

	"bikegeo/internal/catalog"
	"bikegeo/internal/compare"
	"bikegeo/internal/frames"
	"bikegeo/internal/validator"
)

func (a *App) runValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)

	var sizeFlag string
	var allFlag bool

	fs.StringVar(&sizeFlag, "size", "", "Specific frame size to validate (default: frame's default_size)")
	fs.BoolVar(&allFlag, "all", false, "Validate all frames")

	usage := func(w io.Writer) {
		fmt.Fprintln(w, "Usage: bikegeo validate [flags] [frame[:size]...]")
		fs.SetOutput(w)
		fs.PrintDefaults()
	}
	if ok, code := a.parseSubcommandFlags(fs, args, map[string]bool{"all": true}, false, usage, usage); !ok {
		return code
	}

	tokens := fs.Args()
	if allFlag {
		tokens = nil
	}
	targets, err := compare.ResolveValidationTargets(frames.AllFrames(), tokens, sizeFlag)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	doc, allValid, err := validator.BuildValidationDocument(targets, validator.Catalog{
		Forks: catalog.CommonForkCatalog,
		Stems: catalog.CommonStemCatalog,
	})
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	a.printDocument(doc)
	if !allValid {
		return 1
	}
	return 0
}
