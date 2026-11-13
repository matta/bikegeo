package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"bikegeo/pkg/compare"
	"bikegeo/pkg/fit"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/report"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(val string) error {
	*s = append(*s, val)
	return nil
}

func (a *App) runFit(args []string) int {
	fs := flag.NewFlagSet("fit", flag.ContinueOnError)

	var detailFlag bool
	var tryFlags stringSliceFlag

	fs.BoolVar(&detailFlag, "detail", false, "Show detailed per-size residual table for each combination in --try sweeps")
	fs.Var(&tryFlags, "try", "Combinatorial trial parameter, e.g. --try fork.a2c=470,472 or --try XL:stack=655,660 (repeatable)")

	usage := func(w io.Writer) {
		fmt.Fprintln(w, "Usage: bikegeo fit <frame[:size]> [--try key=v1,v2,...] [--detail]")
		fmt.Fprintln(w, "Analyzes CAD residuals across all sizes; optional :size sets the default target size for unqualified --try fields.")
		fs.SetOutput(w)
		fs.PrintDefaults()
	}
	if ok, code := a.parseSubcommandFlags(fs, args, map[string]bool{"detail": true}, false, usage, usage); !ok {
		return code
	}

	posArgs := fs.Args()
	if len(posArgs) < 1 {
		usage(a.Err)
		return 2
	}

	allFrames := frames.AllFrames()
	framePtr, impSz, err := compare.ResolveFrameByToken(allFrames, posArgs[0])
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	frame := *framePtr

	targetSize := frame.GetDefaultSize()
	if impSz != "" {
		matchedSz, err := compare.ResolveFrameSize(frame, impSz)
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		targetSize = matchedSz
	}

	const configID = "stock"

	var doc report.Document
	if len(tryFlags) > 0 {
		combos, err := fit.RunSweeps(frame, configID, targetSize, tryFlags)
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		doc = fit.BuildSweepDocument(frame, combos, detailFlag)
	} else {
		doc, err = fit.BuildAnalysisDocument(frame, configID, targetSize)
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
	}
	a.printDocument(doc)
	return 0
}
