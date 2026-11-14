package compare

import (
	"fmt"
	"math"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

func adaptRange(r *ideal.Range, fn func(float64) float64) *ideal.Range {
	if r == nil {
		return nil
	}
	return &ideal.Range{
		Min: r.Min.Map(fn),
		Max: r.Max.Map(fn),
	}
}

func AdaptProfileAttributeForSaddle(attr ideal.AttributeSpec, saddle model.SaddleSpec) ideal.AttributeSpec {
	if saddle.ID == "" || saddle.ID == "sqlab-610-ergolux" {
		return attr
	}
	refSQ := model.ReferenceIdmatchSaddle()
	lowID := strings.ToLower(attr.ID)
	if lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback" {
		fn := func(v float64) float64 {
			return saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(v))
		}
		copied := attr
		copied.Ideal = adaptRange(attr.Ideal, fn)
		copied.Acceptable = adaptRange(attr.Acceptable, fn)
		return copied
	}
	if lowID == "saddle_hb" || lowID == "saddlehb" {
		fn := func(v float64) float64 {
			tDist := saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, v, 3.0)
			return math.Round(tDist*10) / 10
		}
		copied := attr
		copied.Ideal = adaptRange(attr.Ideal, fn)
		copied.Acceptable = adaptRange(attr.Acceptable, fn)
		return copied
	}
	return attr
}

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
