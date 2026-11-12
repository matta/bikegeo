package cli

import (
	"flag"
	"strings"

	"bikegeo/pkg/compare"
)

func (a *App) runList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var f compare.ListFilter
	fs.StringVar(&f.Query, "q", "", "Search/filter by manufacturer, model, or ID")
	fs.StringVar(&f.Query, "search", "", "Search/filter by manufacturer, model, or ID")
	fs.StringVar(&f.Category, "category", "", "Filter by category (hardtail, rigid, gravel, full-suspension)")
	fs.StringVar(&f.Size, "size", "", "Filter frames that offer a specific size")
	fs.BoolVar(&f.AllSizes, "all-sizes", false, "Expand every size of multi-size frames into its own row")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"all-sizes": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}
	if f.Query == "" && fs.NArg() > 0 {
		f.Query = strings.Join(fs.Args(), " ")
	}
	a.printDocument(compare.BuildFrameListDocument(f))
	return 0
}
