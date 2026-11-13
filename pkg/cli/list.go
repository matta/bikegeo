package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"bikegeo/pkg/compare"
)

func (a *App) runList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

	var f compare.ListFilter
	fs.StringVar(&f.Query, "q", "", "Search/filter by manufacturer, model, or ID (alias for --search)")
	fs.StringVar(&f.Query, "search", "", "Search/filter by manufacturer, model, or ID")
	fs.StringVar(&f.Category, "category", "", fmt.Sprintf("Filter by category (%s)", strings.Join(compare.AvailableCategories(), ", ")))
	fs.StringVar(&f.Size, "size", "", "Filter frames that offer a specific size")
	fs.BoolVar(&f.AllSizes, "all-sizes", false, "Expand every size of multi-size frames into its own row")

	usage := func(w io.Writer) {
		fmt.Fprintln(w, "Usage: bikegeo list [flags] [query]")
		fs.SetOutput(w)
		fs.PrintDefaults()
	}
	if ok, code := a.parseSubcommandFlags(fs, args, map[string]bool{"all-sizes": true}, false, usage, usage); !ok {
		return code
	}
	if f.Query == "" && fs.NArg() > 0 {
		f.Query = strings.Join(fs.Args(), " ")
	}
	doc, err := compare.BuildFrameListDocument(f)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	a.printDocument(doc)
	return 0
}
