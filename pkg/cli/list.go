package cli

import (
	"flag"
	"fmt"
	"strings"

	"bikegeo/pkg/catalog"
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
	if err := a.printDocument(compare.BuildFrameListDocument(f), nil); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	return 0
}

func (a *App) runForks(args []string) int {
	fs := flag.NewFlagSet("forks", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	forks := catalog.CommonForkCatalog
	headers := []string{"Fork ID", "Name", "Travel", "Standard", "Options (A2C / Rake)"}
	var rows [][]string

	for _, f := range forks {
		travelStr := "Rigid (0mm)"
		if f.TravelMm > 0 {
			travelStr = fmt.Sprintf("%.0f mm", f.TravelMm)
		}
		stdStr := "—"
		if f.Standard {
			stdStr = "yes"
		}
		var optStrs []string
		for _, o := range f.Options {
			optStrs = append(optStrs, fmt.Sprintf("%.0f/%.0f", o.A2cMm, o.RakeMm))
		}
		rows = append(rows, []string{
			f.ID,
			f.Name,
			travelStr,
			stdStr,
			strings.Join(optStrs, ", "),
		})
	}

	title := fmt.Sprintf("Standard Fork Catalog (%d)", len(forks))
	if err := a.render(title, "", headers, rows); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	return 0
}

func (a *App) runStems(args []string) int {
	fs := flag.NewFlagSet("stems", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	stems := catalog.CommonStemCatalog
	headers := []string{"Stem Set ID", "Length Range", "Angles", "Total Stems"}
	var rows [][]string

	for _, s := range stems {
		rangeStr := "—"
		if len(s.LengthRangeMm) == 2 {
			rangeStr = fmt.Sprintf("%.0f–%.0f mm", s.LengthRangeMm[0], s.LengthRangeMm[1])
		}
		var angStrs []string
		for _, ang := range s.AnglesDeg {
			angStrs = append(angStrs, fmt.Sprintf("%.0f°", ang))
		}
		totalStems := len(s.Expand())
		rows = append(rows, []string{
			s.ID,
			rangeStr,
			strings.Join(angStrs, ", "),
			fmt.Sprintf("%d", totalStems),
		})
	}

	title := fmt.Sprintf("Standard Stem Catalog (%d)", len(stems))
	if err := a.render(title, "", headers, rows); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	return 0
}
