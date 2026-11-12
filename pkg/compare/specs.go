// Package compare holds the comparison engine: spec parsing, column
// resolution, ranking/pruning, and report-document construction. It is shared
// by the CLI (pkg/cli) and the HTTP server (pkg/web).
package compare

import (
	"fmt"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/catalog"
	"bikegeo/pkg/model"
)

// ResolveFrameByToken finds a FrameModel by exact ID, legacy size-suffixed ID, or unique substring match.
// If the token included a size suffix (e.g. "chumba-sendero-l" or "chumba-sendero:L"), inferredSize is returned.
func ResolveFrameByToken(frames []model.FrameModel, token string) (*model.FrameModel, string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, "", fmt.Errorf("empty frame identifier")
	}

	var explicitSize string
	if strings.Contains(token, ":") {
		parts := strings.SplitN(token, ":", 2)
		token = parts[0]
		explicitSize = parts[1]
	}

	lower := strings.ToLower(token)

	// 1. Exact frame ID match
	for i := range frames {
		if strings.ToLower(frames[i].ID) == lower {
			return &frames[i], explicitSize, nil
		}
	}

	// 2. Exact match on frame ID + "-<size>"
	for i := range frames {
		for _, sz := range frames[i].GetSizes() {
			cand := strings.ToLower(fmt.Sprintf("%s-%s", frames[i].ID, sz))
			if cand == lower {
				if explicitSize == "" {
					explicitSize = sz
				}
				return &frames[i], explicitSize, nil
			}
		}
	}

	// 3. Exact match on Model or Manufacturer + Model
	for i := range frames {
		full := strings.ToLower(fmt.Sprintf("%s %s", frames[i].Manufacturer, frames[i].Model))
		if strings.ToLower(frames[i].Model) == lower || full == lower {
			return &frames[i], explicitSize, nil
		}
	}

	// 4. Substring match across ID, Manufacturer, Model
	var matches []*model.FrameModel
	for i := range frames {
		haystack := strings.ToLower(fmt.Sprintf("%s %s %s", frames[i].ID, frames[i].Manufacturer, frames[i].Model))
		if strings.Contains(haystack, lower) {
			matches = append(matches, &frames[i])
		}
	}

	if len(matches) == 1 {
		return matches[0], explicitSize, nil
	}
	if len(matches) > 1 {
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, "", fmt.Errorf("ambiguous frame %q matches multiple frames: %s", token, strings.Join(ids, ", "))
	}

	var available []string
	for _, f := range frames {
		available = append(available, f.ID)
	}
	return nil, "", fmt.Errorf("frame %q not found (available: %s)", token, strings.Join(available, ", "))
}

// ParseComparisonSpecs parses a CLI token of the form "frame[:size][@mod1][@mod2]..."
// If the frame token is "all", it expands across all registered frames in memory.
// If the size is specified as "all", it expands across all declared sizes.
// If the fork modifier is "@all", it expands across all plausible forks and sags.
func ParseComparisonSpecs(
	frames []model.FrameModel,
	spec string,
) ([]model.ComparisonSetup, error) {
	parts := strings.Split(spec, "@")
	frameToken := strings.TrimSpace(parts[0])

	var explicitSize string
	if strings.Contains(frameToken, ":") {
		subParts := strings.SplitN(frameToken, ":", 2)
		frameToken = subParts[0]
		explicitSize = subParts[1]
	}

	if strings.EqualFold(frameToken, "all") {
		var allSetups []model.ComparisonSetup
		mods := parts[1:]
		if len(mods) == 0 && (explicitSize == "" || strings.EqualFold(explicitSize, "all")) {
			explicitSize = "all"
			mods = []string{"all"}
		}

		for _, f := range frames {
			setups, err := parseFrameSetups(f, explicitSize, mods)
			if err != nil {
				// Frame did not support the requested size or fork modifier; skip gracefully
				continue
			}
			allSetups = append(allSetups, setups...)
		}
		if len(allSetups) == 0 {
			return nil, fmt.Errorf("no frames matched comparison specification %q", spec)
		}
		return allSetups, nil
	}

	frame, implicitSize, err := ResolveFrameByToken(frames, frameToken)
	if err != nil {
		return nil, err
	}
	if explicitSize == "" {
		explicitSize = implicitSize
	}
	return parseFrameSetups(*frame, explicitSize, parts[1:])
}

func parseFrameSetups(
	frame model.FrameModel,
	explicitSize string,
	rawMods []string,
) ([]model.ComparisonSetup, error) {
	var forkToken string
	var sagOverride *float64
	var allForks bool
	sizeToken := explicitSize

	for _, rawMod := range rawMods {
		mod := strings.TrimSpace(rawMod)
		if mod == "" {
			continue
		}
		low := strings.ToLower(mod)

		// 0. "all", "all-forks", "all-sizes", "forks"
		if low == "all" {
			if sizeToken != "" {
				allForks = true
			} else {
				sizeToken = "all"
			}
			continue
		}
		if low == "all-forks" || low == "forks" || low == "all-configs" {
			allForks = true
			continue
		}
		if low == "all-sizes" {
			sizeToken = "all"
			continue
		}

		// 1. Direct A2C syntax: a2c=NNN/MM or a2c=NNN
		if strings.HasPrefix(low, "a2c=") {
			forkToken = mod
			continue
		}

		// 2. Sag modifier (e.g. "20%", "sag=25")
		if strings.HasSuffix(low, "%") || strings.HasPrefix(low, "sag=") {
			clean := strings.TrimSuffix(strings.TrimPrefix(low, "sag="), "%")
			val, err := strconv.ParseFloat(clean, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid sag modifier %q in %q", mod, frame.ID)
			}
			if val > 0 && !canonicalizer.IsAllowedSag(val) {
				return nil, fmt.Errorf("invalid sag modifier %q in %q; allowed sags are 20%%, 25%%, 30%%", mod, frame.ID)
			}
			sagOverride = &val
			continue
		}

		// 3. Rigid fork modifier
		if low == "rigid" || low == "0mm" || low == "0" || strings.HasPrefix(low, "rigid") {
			forkToken = "rigid"
			continue
		}

		// 4. Exact match on one of the frame's sizes
		matchedSize := ""
		for _, sz := range frame.GetSizes() {
			if strings.EqualFold(sz, mod) {
				matchedSize = sz
				break
			}
		}
		if matchedSize != "" {
			sizeToken = matchedSize
			continue
		}

		// 5. Fork travel modifier ending with "mm" (e.g. "120mm", "150mm")
		if strings.HasSuffix(low, "mm") {
			forkToken = mod
			continue
		}

		// 6. Pure numeric modifier: >= 60 is fork travel (mm); < 60 is sag (%)
		if val, err := strconv.ParseFloat(low, 64); err == nil {
			if val >= 60 {
				forkToken = fmt.Sprintf("%.0fmm", val)
			} else {
				if val > 0 && !canonicalizer.IsAllowedSag(val) {
					return nil, fmt.Errorf("invalid sag modifier %q in %q; allowed sags are 20%%, 25%%, 30%%", mod, frame.ID)
				}
				sagOverride = &val
			}
			continue
		}

		// 7. Fork preset ID or name in catalog
		if _, ok := canonicalizer.GetCatalogFork(mod); ok {
			forkToken = mod
			continue
		}

		// If modifier looks like a size token (e.g. XS, S, M, L, XL, XXL), let size validation report it
		if isLikelySizeToken(low) {
			sizeToken = mod
			continue
		}

		return nil, fmt.Errorf(
			"unrecognized modifier %q for frame \"%s %s\" (sizes: %s; supported forks: %s; sag: e.g. 20%%)",
			mod, frame.Manufacturer, frame.Model,
			strings.Join(frame.GetSizes(), ", "),
			strings.Join(frame.GetSupportedForkOptions(), ", "),
		)
	}

	// Determine sizes
	var sizes []string
	if strings.EqualFold(sizeToken, "all") {
		sizes = frame.GetSizes()
		if len(sizes) == 0 {
			return nil, fmt.Errorf("frame %q has no declared sizes", frame.ID)
		}
	} else if sizeToken != "" {
		matched := ""
		for _, sz := range frame.GetSizes() {
			if strings.EqualFold(sz, sizeToken) {
				matched = sz
				break
			}
		}
		if matched == "" {
			return nil, fmt.Errorf(
				"frame \"%s %s\" (%s) does not have size %q (available sizes: %s)",
				frame.Manufacturer, frame.Model, frame.ID, sizeToken, strings.Join(frame.GetSizes(), ", "),
			)
		}
		sizes = []string{matched}
	} else {
		sizes = []string{frame.GetDefaultSize()}
	}

	// Determine fork setups
	var forkSetups []model.ComparisonSetup
	if allForks {
		expForks, err := canonicalizer.ExpandPlausibleForkSetups(frame, sagOverride)
		if err != nil {
			return nil, err
		}
		forkSetups = expForks
	} else {
		targetA2c, targetRake, _, displayLabel, err := canonicalizer.ResolveForkToken(forkToken, sagOverride, frame)
		if err != nil {
			return nil, err
		}
		sagVal := 0.0
		if sagOverride != nil {
			sagVal = *sagOverride
		}
		forkSetups = []model.ComparisonSetup{
			{
				FrameID:      frame.ID,
				A2cMm:        targetA2c,
				RakeMm:       targetRake,
				DisplayLabel: displayLabel,
				SagPct:       sagVal,
				AutoExpanded: false,
			},
		}
	}

	var setups []model.ComparisonSetup
	for _, sz := range sizes {
		for _, fs := range forkSetups {
			s := fs
			s.Size = sz
			setups = append(setups, s)
		}
	}

	return setups, nil
}

// ParseComparisonSpec parses a CLI token of the form "frame[:size][@mod1][@mod2]..."
// where modifiers can be a size (e.g. "L"), a fork travel/type (e.g. "120mm", "rigid"), or a sag % (e.g. "20%").
// If the token resolves to multiple sizes (e.g. ":all"), the first setup is returned.
func ParseComparisonSpec(
	frames []model.FrameModel,
	spec string,
) (model.ComparisonSetup, error) {
	setups, err := ParseComparisonSpecs(frames, spec)
	if err != nil {
		return model.ComparisonSetup{}, err
	}
	if len(setups) == 0 {
		return model.ComparisonSetup{}, fmt.Errorf("no comparison setups resolved for %q", spec)
	}
	return setups[0], nil
}

func isLikelySizeToken(low string) bool {
	switch low {
	case "xs", "s", "m", "ml", "m/l", "l", "xl", "xxl", "s1", "s2", "s3", "s4", "s5", "s6":
		return true
	}
	return false
}

func ResolveAllColumns(
	frames []model.FrameModel,
	state model.SavedComparisonState,
) ([]model.CanonicalComparisonRow, []model.ComparisonSetup, error) {
	var rows []model.CanonicalComparisonRow
	var normalized []model.ComparisonSetup

	for _, col := range state.Columns {
		frame, _, err := ResolveFrameByToken(frames, col.FrameID)
		if err != nil {
			return nil, nil, err
		}
		row, err := canonicalizer.ResolveFrameForkSetupWithSaddle(*frame, col, state.RiderProfile, state.Saddle)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, row)
		normalized = append(normalized, row.Setup)
	}

	return rows, normalized, nil
}

// DefaultComparisonState returns the compiled baseline comparison configuration.
func DefaultComparisonState() model.SavedComparisonState {
	defSaddle := catalog.DefaultSaddle()
	defBar := catalog.DefaultHandlebar()
	return model.SavedComparisonState{
		RiderProfile: catalog.DefaultRiderProfile(),
		Transpose:    true,
		RefIndex:     1,
		IdealProfile: "idmatch",
		SaddleID:     defSaddle.ID,
		Saddle:       defSaddle,
		HandlebarID:  defBar.ID,
		Handlebar:    defBar,
		Columns: []model.ComparisonSetup{
			{FrameID: "binary-havok", Size: "XL", A2cMm: 486, RakeMm: 44, DisplayLabel: "100mm @ 20% sag"},
			{FrameID: "otso-fenrir-stainless", Size: "L", A2cMm: 499, RakeMm: 44, DisplayLabel: "110mm @ 20% sag"},
			{FrameID: "otso-fenrir-stainless", Size: "L", A2cMm: 495, RakeMm: 54, DisplayLabel: "Rigid (495mm)"},
			{FrameID: "otso-fenrir-stainless", Size: "XL", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "esker-hayduke", Size: "S3", A2cMm: 499, RakeMm: 44, DisplayLabel: "110mm @ 20% sag"},
			{FrameID: "revel-el-jefe", Size: "L", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "revel-el-jefe", Size: "XL", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "surly-krampus", Size: "L", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "surly-krampus", Size: "XL", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "binary-kinetik", Size: "L", A2cMm: 499, RakeMm: 44, DisplayLabel: "110mm @ 20% sag"},
			{FrameID: "turner-venn-ti", Size: "L", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "otso-fenrir-stainless", Size: "L", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "fairlight-holt-2", Size: "L", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "fairlight-holt-2", Size: "XL", A2cMm: 506, RakeMm: 44, DisplayLabel: "120mm @ 20% sag"},
			{FrameID: "sklar-pbj", Size: "L", A2cMm: 499, RakeMm: 44, DisplayLabel: "110mm @ 20% sag"},
			{FrameID: "sklar-pbj", Size: "XL", A2cMm: 499, RakeMm: 44, DisplayLabel: "110mm @ 20% sag"},
			{FrameID: "salsa-fargo", Size: "L", A2cMm: 483, RakeMm: 51, DisplayLabel: "Rigid (483mm)"},
			{FrameID: "salsa-fargo", Size: "XL", A2cMm: 483, RakeMm: 51, DisplayLabel: "Rigid (483mm)"},
			{FrameID: "salsa-fargo", Size: "L", A2cMm: 486, RakeMm: 44, DisplayLabel: "100mm @ 20% sag"},
			{FrameID: "salsa-fargo", Size: "XL", A2cMm: 486, RakeMm: 44, DisplayLabel: "100mm @ 20% sag"},
		},
	}
}
