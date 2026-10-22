package cli

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/ideal"
)

func (a *App) getIdealsDir() string {
	parent := filepathDirClean(a.DataDir)
	return filepathJoin(parent, "ideals")
}

func filepathDirClean(path string) string {
	cleaned := strings.TrimRight(path, "/\\")
	idx := strings.LastIndexAny(cleaned, "/\\")
	if idx == -1 {
		return "."
	}
	if idx == 0 {
		return "/"
	}
	return cleaned[:idx]
}

func filepathJoin(elem ...string) string {
	var parts []string
	for _, e := range elem {
		trimmed := strings.Trim(e, "/\\")
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, "/")
}

func (a *App) runIdeals(args []string) int {
	idealsDir := a.getIdealsDir()

	if len(args) == 0 {
		return a.listIdeals(idealsDir)
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "list", "ls":
		return a.listIdeals(idealsDir)
	case "show", "view", "get":
		if len(args) < 2 {
			fmt.Fprintf(a.Err, "Error: 'ideals show' requires a profile name (e.g. 'default')\n")
			return 2
		}
		return a.showIdeal(idealsDir, args[1])
	default:
		// Shorthand: 'bikegeo ideals default' -> show default
		return a.showIdeal(idealsDir, args[0])
	}
}

func (a *App) listIdeals(idealsDir string) int {
	summaries, err := ideal.ListProfiles(idealsDir)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	headers := []string{"Name", "Description", "Saddle Height", "Attributes"}
	var rows [][]string
	for _, s := range summaries {
		shStr := "—"
		if s.SaddleHeightMm != nil {
			shStr = fmt.Sprintf("%.0fmm", *s.SaddleHeightMm)
		}
		attrsStr := fmt.Sprintf("%d (%d scored)", s.AttributeCount, s.ScoredCount)
		rows = append(rows, []string{
			s.Name,
			s.Description,
			shStr,
			attrsStr,
		})
	}

	title := "Ideal Frame Profiles"
	subtitle := fmt.Sprintf("Directory: %s (use 'bikegeo ideals show <name>' to view details)", idealsDir)
	if err := RenderOutput(a.Out, a.Format, title, subtitle, headers, rows, summaries); err != nil {
		fmt.Fprintf(a.Err, "Error rendering output: %v\n", err)
		return 1
	}

	return 0
}

func (a *App) showIdeal(idealsDir string, name string) int {
	p, err := ideal.LoadProfile(idealsDir, name)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	title := fmt.Sprintf("Ideal Profile: %s", p.Name)
	subtitleParts := []string{}
	if p.Description != "" {
		subtitleParts = append(subtitleParts, p.Description)
	}
	if p.SaddleHeightMm != nil {
		subtitleParts = append(subtitleParts, fmt.Sprintf("Designed for Saddle Height: %.0fmm", *p.SaddleHeightMm))
	}
	subtitle := strings.Join(subtitleParts, " | ")

	headers := []string{
		"Attribute",
		"Weight",
		"Ideal Range",
		"Acceptable Range",
		"Edge Score",
		"Falloff (Below / Above)",
		"Floor",
	}

	var rows [][]string
	for _, attr := range p.Attributes {
		meta, ok := ideal.LookupAttribute(attr.ID)
		if !ok {
			continue
		}

		wtStr := "display only"
		if attr.Weight > 0 {
			wtStr = fmt.Sprintf("%.0f", attr.Weight)
		}

		idealStr := ideal.FormatRange(attr.Ideal, meta.Unit, meta.Decimals)
		if idealStr == "" {
			idealStr = "—"
		}

		accStr := ideal.FormatRange(attr.Acceptable, meta.Unit, meta.Decimals)
		if accStr == "" {
			accStr = "—"
		}

		edgeStr := "—"
		if attr.EdgeScore != nil {
			edgeStr = fmt.Sprintf("%.0f", *attr.EdgeScore)
		}

		falloffStr := "—"
		if attr.Falloff != nil {
			var parts []string
			if attr.Falloff.Below != nil {
				parts = append(parts, fmt.Sprintf("-%.1f%s", *attr.Falloff.Below, meta.Unit))
			} else {
				parts = append(parts, "—")
			}
			if attr.Falloff.Above != nil {
				parts = append(parts, fmt.Sprintf("+%.1f%s", *attr.Falloff.Above, meta.Unit))
			} else {
				parts = append(parts, "—")
			}
			falloffStr = strings.Join(parts, " / ")
		}

		floorStr := "—"
		if attr.Floor != nil {
			floorStr = fmt.Sprintf("%.0f", *attr.Floor)
		}

		rows = append(rows, []string{
			fmt.Sprintf("%s (%s)", meta.Label, attr.ID),
			wtStr,
			idealStr,
			accStr,
			edgeStr,
			falloffStr,
			floorStr,
		})
	}

	if err := RenderOutput(a.Out, a.Format, title, subtitle, headers, rows, p); err != nil {
		fmt.Fprintf(a.Err, "Error rendering output: %v\n", err)
		return 1
	}

	// In table or markdown mode, also render sample evaluation points
	if a.Format != FormatJSON && a.Format != FormatCSV {
		sampleHeaders := []string{"Attribute", "Test Value", "Zone", "Score", "Table Display"}
		var sampleRows [][]string

		for _, attr := range p.Attributes {
			if attr.Weight <= 0 || attr.Ideal == nil {
				continue
			}
			meta, _ := ideal.LookupAttribute(attr.ID)

			sampleVals := generateSampleValues(attr)
			for _, val := range sampleVals {
				ev := ideal.Evaluate(attr, meta, val, nil)
				scoreStr := fmt.Sprintf("%.0f", math.Round(ev.SubScore))
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
			fmt.Fprintln(a.Out)
			_ = RenderOutput(a.Out, a.Format, "Sample Evaluation Points", "Key checkpoints across ideal, acceptable, and falloff zones", sampleHeaders, sampleRows, nil)
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
		if spec.Ideal.Min != nil && spec.Ideal.Max != nil {
			add(*spec.Ideal.Min)
			add((*spec.Ideal.Min + *spec.Ideal.Max) / 2.0)
			add(*spec.Ideal.Max)
		} else if spec.Ideal.Min != nil {
			add(*spec.Ideal.Min)
			add(*spec.Ideal.Min + 10)
		} else if spec.Ideal.Max != nil {
			add(*spec.Ideal.Max)
			add(*spec.Ideal.Max - 10)
		}
	}

	if spec.Acceptable != nil {
		if spec.Acceptable.Min != nil {
			add(*spec.Acceptable.Min)
		}
		if spec.Acceptable.Max != nil {
			add(*spec.Acceptable.Max)
		}
	}

	if spec.Falloff != nil {
		if spec.Falloff.Below != nil && spec.Acceptable != nil && spec.Acceptable.Min != nil {
			add(*spec.Acceptable.Min - *spec.Falloff.Below)
		}
		if spec.Falloff.Above != nil && spec.Acceptable != nil && spec.Acceptable.Max != nil {
			add(*spec.Acceptable.Max + *spec.Falloff.Above)
		}
	}

	return vals
}
