package cli

import (
	"flag"
	"fmt"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
)

func (a *App) runList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var query string
	var category string
	var sizeFilter string
	var allSizes bool

	fs.StringVar(&query, "q", "", "Search/filter by manufacturer, model, or ID")
	fs.StringVar(&query, "search", "", "Search/filter by manufacturer, model, or ID")
	fs.StringVar(&category, "category", "", "Filter by category (hardtail, rigid, gravel, full-suspension)")
	fs.StringVar(&sizeFilter, "size", "", "Filter frames that offer a specific size")
	fs.BoolVar(&allSizes, "all-sizes", false, "Expand every size of multi-size frames into its own row")

	reordered := reorderFlagsForStdlib(fs, args, map[string]bool{"all-sizes": true})
	if err := fs.Parse(reordered); err != nil {
		return 2
	}
	if query == "" && fs.NArg() > 0 {
		query = strings.Join(fs.Args(), " ")
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	frames, err := store.GetAllFrames()
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	szF := strings.ToLower(strings.TrimSpace(sizeFilter))

	var filteredFrames []model.FrameModel
	for _, f := range frames {
		if q != "" {
			hay := strings.ToLower(fmt.Sprintf("%s %s %s %s", f.ID, f.Manufacturer, f.Model, strings.Join(f.GetSizes(), " ")))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		if cat != "" && strings.ToLower(string(f.Category)) != cat {
			continue
		}
		if szF != "" {
			hasSz := false
			for _, sz := range f.GetSizes() {
				if strings.ToLower(sz) == szF {
					hasSz = true
					break
				}
			}
			if !hasSz {
				continue
			}
		}
		filteredFrames = append(filteredFrames, f)
	}

	if allSizes {
		headers := []string{"Frame ID", "Manufacturer & Model", "Size", "Reach", "Stack", "HTA", "STA", "BB Drop", "Supported Forks"}
		var rows [][]string
		var bikesOut []model.BikeGeometry

		for _, f := range filteredFrames {
			for _, sz := range f.GetSizes() {
				if szF != "" && strings.ToLower(sz) != szF {
					continue
				}
				b, err := geometry.MaterializeBikeGeometry(f, sz)
				if err != nil {
					continue
				}
				bikesOut = append(bikesOut, b)
				rows = append(rows, []string{
					f.ID,
					fmt.Sprintf("%s %s", f.Manufacturer, f.Model),
					sz,
					fmt.Sprintf("%.1f mm", b.Reach),
					fmt.Sprintf("%.1f mm", b.Stack),
					fmt.Sprintf("%.1f°", b.HtaDeg),
					fmt.Sprintf("%.1f°", b.StaDeg),
					fmt.Sprintf("%.1f mm", b.BbDrop),
					strings.Join(f.GetSupportedForkOptions(), ", "),
				})
			}
		}

		title := fmt.Sprintf("Stored Bikes (%d sizes across %d frames)", len(rows), len(filteredFrames))
		if err := RenderOutput(a.Out, a.Format, title, "", headers, rows, bikesOut); err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	headers := []string{"ID", "Manufacturer & Model", "Category", "Sizes (Default)", "Supported Forks", "Default Fork", "Headset"}
	var rows [][]string
	for _, f := range filteredFrames {
		defSz := f.GetDefaultSize()
		var formattedSizes []string
		for _, sz := range f.GetSizes() {
			if sz == defSz && len(f.GetSizes()) > 1 {
				formattedSizes = append(formattedSizes, sz+"*")
			} else {
				formattedSizes = append(formattedSizes, sz)
			}
		}
		cup := f.HeadsetCupType.CupStackMm()
		headsetStr := string(f.HeadsetCupType)
		if headsetStr == "" {
			headsetStr = "Standard"
		}
		headsetStr = fmt.Sprintf("%s (+%.0fmm, CAD %.0fmm)", headsetStr, cup, f.GetGeoHeadsetSpacerMm())

		rows = append(rows, []string{
			f.ID,
			fmt.Sprintf("%s %s", f.Manufacturer, f.Model),
			string(f.Category),
			strings.Join(formattedSizes, ", "),
			strings.Join(f.GetSupportedForkOptions(), ", "),
			f.GetDefaultForkSpecString(),
			headsetStr,
		})
	}

	title := fmt.Sprintf("Stored Frame Models (%d)", len(filteredFrames))
	if err := RenderOutput(a.Out, a.Format, title, "", headers, rows, filteredFrames); err != nil {
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
	if err := RenderOutput(a.Out, a.Format, title, "", headers, rows, forks); err != nil {
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
	if err := RenderOutput(a.Out, a.Format, title, "", headers, rows, stems); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	return 0
}
