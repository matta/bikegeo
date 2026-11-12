package cli

import (
	"flag"
	"fmt"
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
	fs.SetOutput(a.Err)

	var configFlag string
	var detailFlag bool
	var tryFlags stringSliceFlag

	fs.StringVar(&configFlag, "config", "", "Specific frame configuration ID to analyze (defaults to baseline)")
	fs.BoolVar(&detailFlag, "detail", false, "Show detailed per-size residual table for each combination in --try sweeps")
	fs.Var(&tryFlags, "try", "Combinatorial trial parameter, e.g. --try fork.a2c=470,472 or --try XL:stack=655,660 (repeatable)")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"detail": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	posArgs := fs.Args()
	if len(posArgs) < 1 {
		fmt.Fprintln(a.Err, "Usage: bikegeo fit <frame[:size]> [--try key=v1,v2,...] [--detail] [--config id]")
		return 2
	}

	allFrames := frames.AllFrames()
	framePtr, impSz, err := compare.ResolveFrameByToken(allFrames, posArgs[0])
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	frame := *framePtr

	targetSize := impSz
	if targetSize == "" {
		targetSize = frame.GetDefaultSize()
	}

	configID := configFlag
	if configID == "" {
		configID = "stock"
	}

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
