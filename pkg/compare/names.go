package compare

import (
	"fmt"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

// DisambiguateFrameNames returns a slice of display names corresponding to rows.
// When a (FrameID, Size) appears multiple times in the comparison, each instance
// is disambiguated with concise fork/sag modifiers (or fallback column index if identical).
func DisambiguateFrameNames(rows []model.CanonicalComparisonRow) []string {
	names := make([]string, len(rows))
	if len(rows) == 0 {
		return names
	}

	groups := make(map[string][]int)
	for i, r := range rows {
		key := fmt.Sprintf("%s|%s", r.Setup.FrameID, strings.ToLower(r.Setup.Size))
		groups[key] = append(groups[key], i)
	}

	for _, indices := range groups {
		baseName := fmt.Sprintf("%s %s (%s)", rows[indices[0]].Bike.Manufacturer, rows[indices[0]].Bike.Model, rows[indices[0]].Setup.Size)
		if len(indices) == 1 {
			names[indices[0]] = baseName
			continue
		}

		// Count fork types (e.g. "110mm", "rigid") within this duplicate group
		forkTypeCounts := make(map[string]int)
		for _, idx := range indices {
			ft := canonicalizer.FormatForkType(rows[idx])
			forkTypeCounts[ft]++
		}

		// Count fork + sag within this duplicate group in case identical setups exist
		forkSagCounts := make(map[string]int)
		for _, idx := range indices {
			fs := canonicalizer.FormatForkSag(rows[idx])
			forkSagCounts[fs]++
		}

		for _, idx := range indices {
			r := rows[idx]
			ft := canonicalizer.FormatForkType(r)
			fs := canonicalizer.FormatForkSag(r)

			var suffix string
			if forkTypeCounts[ft] == 1 {
				suffix = fmt.Sprintf("@ %s", ft)
			} else {
				suffix = fmt.Sprintf("@ %s", fs)
			}

			if forkSagCounts[fs] > 1 {
				suffix = fmt.Sprintf("%s (#%d)", suffix, idx+1)
			}

			names[idx] = fmt.Sprintf("%s %s", baseName, suffix)
		}
	}

	return names
}

func FormatAttrRanges(evalAttr ideal.AttributeSpec, meta ideal.AttributeMeta) (string, string) {
	return ideal.FormatRange(evalAttr.Ideal, meta.Unit, meta.Decimals), ideal.FormatRange(evalAttr.Acceptable, meta.Unit, meta.Decimals)
}
