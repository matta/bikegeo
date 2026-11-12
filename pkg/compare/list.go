package compare

import (
	"fmt"
	"strings"

	"bikegeo/pkg/frames"
	"bikegeo/pkg/geometry"
	"bikegeo/pkg/model"
	"bikegeo/pkg/report"
)

// ListFilter narrows the frame catalog. Empty fields match everything.
type ListFilter struct {
	Query, Category, Size string
	AllSizes              bool
}

// BuildFrameListDocument lays the filtered frame catalog out as one table:
// one row per frame, or one row per size with headline geometry when
// f.AllSizes is set.
func BuildFrameListDocument(f ListFilter) report.Document {
	q := strings.ToLower(strings.TrimSpace(f.Query))
	cat := strings.ToLower(strings.TrimSpace(f.Category))
	szF := strings.ToLower(strings.TrimSpace(f.Size))

	var kept []model.FrameModel
	for _, fm := range frames.AllFrames() {
		if q != "" {
			hay := strings.ToLower(fmt.Sprintf("%s %s %s %s", fm.ID, fm.Manufacturer, fm.Model, strings.Join(fm.GetSizes(), " ")))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		if cat != "" && strings.ToLower(string(fm.Category)) != cat {
			continue
		}
		if szF != "" && !hasSize(fm, szF) {
			continue
		}
		kept = append(kept, fm)
	}

	var doc report.Document
	if f.AllSizes {
		headers := []string{"Frame ID", "Manufacturer & Model", "Size", "Reach", "Stack", "HTA", "STA", "BB Drop", "Supported Forks"}
		var rows [][]string
		for _, fm := range kept {
			for _, sz := range fm.GetSizes() {
				if szF != "" && strings.ToLower(sz) != szF {
					continue
				}
				b, err := geometry.MaterializeBikeGeometry(fm, sz)
				if err != nil {
					continue
				}
				rows = append(rows, []string{
					fm.ID, fm.Manufacturer + " " + fm.Model, sz,
					fmt.Sprintf("%.1f mm", b.Reach),
					fmt.Sprintf("%.1f mm", b.Stack),
					fmt.Sprintf("%.1f°", b.HtaDeg),
					fmt.Sprintf("%.1f°", b.StaDeg),
					fmt.Sprintf("%.1f mm", b.BbDrop),
					strings.Join(fm.GetSupportedForkOptions(), ", "),
				})
			}
		}
		title := fmt.Sprintf("Stored Bikes (%d sizes across %d frames)", len(rows), len(kept))
		doc.AddTable(report.FromStrings(report.Slug(title), title, "", headers, rows))
		return doc
	}

	headers := []string{"ID", "Manufacturer & Model", "Category", "Sizes (Default)", "Supported Forks", "Default Fork", "Headset"}
	var rows [][]string
	for _, fm := range kept {
		defSz := fm.GetDefaultSize()
		var sizes []string
		for _, sz := range fm.GetSizes() {
			if sz == defSz && len(fm.GetSizes()) > 1 {
				sz += "*"
			}
			sizes = append(sizes, sz)
		}
		headset := string(fm.HeadsetCupType)
		if headset == "" {
			headset = "Standard"
		}
		headset = fmt.Sprintf("%s (+%.0fmm, CAD %.0fmm)", headset, fm.HeadsetCupType.CupStackMm(), fm.GetGeoHeadsetSpacerMm())
		rows = append(rows, []string{
			fm.ID, fm.Manufacturer + " " + fm.Model, string(fm.Category),
			strings.Join(sizes, ", "),
			strings.Join(fm.GetSupportedForkOptions(), ", "),
			fm.GetDefaultForkSpecString(), headset,
		})
	}
	title := fmt.Sprintf("Stored Frame Models (%d)", len(kept))
	doc.AddTable(report.FromStrings(report.Slug(title), title, "", headers, rows))
	return doc
}

func hasSize(fm model.FrameModel, lowerSize string) bool {
	for _, sz := range fm.GetSizes() {
		if strings.ToLower(sz) == lowerSize {
			return true
		}
	}
	return false
}
