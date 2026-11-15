package web

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/compare"
	"bikegeo/pkg/ideal"
)

func (db *docb) ideals(name string) int {
	if name == "" {
		return db.listIdeals()
	}
	return db.showIdeal(name)
}

func (db *docb) listIdeals() int {
	summaries := ideal.ListProfiles()

	headers := []string{"Name", "Description", "Saddle Height", "Attributes"}
	var rows [][]string
	for _, s := range summaries {
		shStr := "—"
		if sh, ok := s.SaddleHeightMm.Get(); ok {
			shStr = fmt.Sprintf("%.0fmm", sh)
		}
		attrsStr := fmt.Sprintf("%d (%d scored)", s.AttributeCount, s.ScoredCount)
		rows = append(rows, []string{
			s.Name,
			s.Description,
			shStr,
			attrsStr,
		})
	}

	title := fmt.Sprintf("Ideal Frame Profiles (%d)", len(summaries))
	subtitle := "Select a profile above or follow a profile link to view band costs, ranges, and sample evaluations"
	if err := db.render(title, subtitle, headers, rows); err != nil {
		db.fail("Error rendering output: %v", err)
		return 1
	}

	return 0
}

func (db *docb) showIdeal(name string) int {
	p, err := ideal.GetProfile(name)
	if err != nil {
		db.fail("Error: %v", err)
		return 1
	}

	title := fmt.Sprintf("Ideal Profile: %s", p.Name)
	subtitleParts := []string{}
	if p.Description != "" {
		subtitleParts = append(subtitleParts, p.Description)
	}
	if sh, ok := p.SaddleHeightMm.Get(); ok {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Designed for Saddle Height: %.0fmm", sh))
	}
	if p.TargetRef.IsValid() {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Cockpit Target: %s", p.TargetRef.Label()))
	}
	subtitle := strings.Join(subtitleParts, " | ")

	headers := []string{
		"Attribute",
		"Cost (points lost outside the ideal range)",
		"Ideal Range (free)",
	}

	var rows [][]string
	for _, attr := range p.Attributes {
		meta, ok := ideal.LookupAttribute(attr.ID)
		if !ok {
			continue
		}

		costStr := "display only"
		if attr.IsScored() {
			costStr = compare.FormatCost(attr, meta)
		}

		idealStr := ideal.FormatRange(attr.Ideal, meta.Unit, meta.Decimals)
		if idealStr == "" {
			idealStr = "—"
		}

		rows = append(rows, []string{
			fmt.Sprintf("%s (%s)", meta.Label, attr.ID),
			costStr,
			idealStr,
		})
	}

	if err := db.render(title, subtitle, headers, rows); err != nil {
		db.fail("Error rendering output: %v", err)
		return 1
	}

	// In table or markdown mode, also render sample evaluation points
	{
		sampleHeaders := []string{"Attribute", "Test Value", "Zone", "Points Lost", "Table Display"}
		var sampleRows [][]string

		for _, attr := range p.Attributes {
			// Oval Fits is priced on a setup's oval radius R, which needs a resolved row.
			if !attr.IsScored() || attr.ID == ideal.MetricHandlebarClampFit {
				continue
			}
			meta, _ := ideal.LookupAttribute(attr.ID)

			sampleVals := generateSampleValues(attr)
			for _, val := range sampleVals {
				ev := ideal.Evaluate(attr, meta, val, nil)
				scoreStr := fmt.Sprintf("%.1f", ev.PointsLost)
				sampleRows = append(sampleRows, []string{
					meta.Label,
					fmt.Sprintf("%.*f %s", meta.Decimals, val, meta.Unit),
					string(ev.Zone),
					scoreStr,
					ev.FormattedCell,
				})
			}
		}

		if len(sampleRows) > 0 {
			_ = db.render("Sample Evaluation Points", "Key checkpoints inside and outside the free (ideal) range", sampleHeaders, sampleRows)
		}
	}

	return 0
}

func generateSampleValues(spec ideal.AttributeSpec) []float64 {
	var vals []float64
	add := func(v float64) {
		for _, existing := range vals {
			if math.Abs(existing-v) < 0.001 {
				return
			}
		}
		vals = append(vals, v)
	}

	if spec.Ideal != nil {
		minVal, hasMin := spec.Ideal.Min.Get()
		maxVal, hasMax := spec.Ideal.Max.Get()
		if hasMin && hasMax {
			add(minVal)
			add((minVal + maxVal) / 2.0)
			add(maxVal)
		} else if hasMin {
			add(minVal)
			add(minVal + 10)
		} else if hasMax {
			add(maxVal)
			add(maxVal - 10)
		}
	}

	return vals
}
