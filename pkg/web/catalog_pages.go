package web

import (
	"fmt"
	"strings"

	"bikegeo/pkg/catalog"
)

func (db *docb) forks() int {
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
	if err := db.render(title, "", headers, rows); err != nil {
		db.fail("Error: %v", err)
		return 1
	}
	return 0
}

func (db *docb) stems() int {
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
	if err := db.render(title, "", headers, rows); err != nil {
		db.fail("Error: %v", err)
		return 1
	}
	return 0
}
