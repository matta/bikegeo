package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
	"bikegeo/pkg/scoring"
)

func (a *App) runCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	fs.Usage = func() {
		a.printCompareUsage(a.Out)
	}

	var transposeFlag bool
	var compactFlag bool
	var noScoreFlag bool
	var scoreDetailFlag bool
	var idealFlag string

	fs.BoolVar(&transposeFlag, "transpose", false, "Transpose table (Columns as rows, Metrics as columns)")
	fs.BoolVar(&compactFlag, "compact", false, "Compact/transposed table view (alias for --transpose)")
	fs.BoolVar(&noScoreFlag, "no-score", false, "Disable scoring and ranking display")
	fs.BoolVar(&scoreDetailFlag, "score-detail", false, "Display detailed scoring breakdown table")
	fs.StringVar(&idealFlag, "ideal", "", "Ideal frame profile name (default: 'default')")

	boolFlags := map[string]bool{
		"transpose":    true,
		"compact":      true,
		"no-score":     true,
		"score-detail": true,
	}
	reordered := reorderFlagsForStdlib(fs, args, boolFlags)
	if err := fs.Parse(reordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	explicitFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

	allFrames := frames.AllFrames()
	if len(allFrames) == 0 {
		fmt.Fprintf(a.Err, "Error: no bike frames found\n")
		return 1
	}

	state := DefaultComparisonState()

	if explicitFlags["transpose"] || explicitFlags["compact"] {
		state.Transpose = transposeFlag || compactFlag
	}

	posArgs := fs.Args()
	if len(posArgs) == 1 && (posArgs[0] == "help" || posArgs[0] == "-h" || posArgs[0] == "--help") {
		a.printCompareUsage(a.Out)
		return 0
	}

	if len(posArgs) > 0 {
		// Check if all positional arguments are incremental (+spec or -col_num / -frame)
		isIncremental := true
		for _, arg := range posArgs {
			if !strings.HasPrefix(arg, "+") && !strings.HasPrefix(arg, "-") {
				isIncremental = false
				break
			}
		}

		if isIncremental {
			for _, arg := range posArgs {
				if strings.HasPrefix(arg, "+") {
					specStr := strings.TrimPrefix(arg, "+")
					cols, err := ParseComparisonSpecs(allFrames, specStr)
					if err != nil {
						fmt.Fprintf(a.Err, "Error: %v\n", err)
						return 1
					}
					state.Columns = append(state.Columns, cols...)
				} else if strings.HasPrefix(arg, "-") {
					rmToken := strings.TrimPrefix(arg, "-")
					if idx, err := strconv.Atoi(rmToken); err == nil {
						colIdx := idx - 1
						if colIdx < 0 || colIdx >= len(state.Columns) {
							fmt.Fprintf(a.Err, "Error: column index %d out of range (1..%d)\n", idx, len(state.Columns))
							return 1
						}
						state.Columns = append(state.Columns[:colIdx], state.Columns[colIdx+1:]...)
					} else if strings.Contains(rmToken, "@") {
						targetCol, err := ParseComparisonSpec(allFrames, rmToken)
						if err != nil {
							fmt.Fprintf(a.Err, "Error: %v\n", err)
							return 1
						}
						var kept []model.ComparisonSetup
						for _, c := range state.Columns {
							if c.FrameID == targetCol.FrameID &&
								(targetCol.Size == "" || strings.EqualFold(c.Size, targetCol.Size)) &&
								(targetCol.A2cMm <= 0 || math.Abs(c.A2cMm-targetCol.A2cMm) < 0.1) {
								continue
							}
							kept = append(kept, c)
						}
						state.Columns = kept
					} else {
						f, sz, err := ResolveFrameByToken(allFrames, rmToken)
						if err != nil {
							fmt.Fprintf(a.Err, "Error: %v\n", err)
							return 1
						}
						var kept []model.ComparisonSetup
						for _, c := range state.Columns {
							if c.FrameID == f.ID && (sz == "" || strings.EqualFold(sz, "all") || strings.EqualFold(c.Size, sz)) {
								continue
							}
							kept = append(kept, c)
						}
						state.Columns = kept
					}
				}
			}
		} else {
			var newCols []model.ComparisonSetup
			var hasAllExpansion bool
			var singleExpandedFrame *model.FrameModel

			for _, specStr := range posArgs {
				specStr = strings.TrimPrefix(specStr, "+")
				cols, err := ParseComparisonSpecs(allFrames, specStr)
				if err != nil {
					fmt.Fprintf(a.Err, "Error: %v\n", err)
					return 1
				}
				if len(posArgs) == 1 && len(cols) > 1 {
					hasAllExpansion = true
					f, _, _ := ResolveFrameByToken(allFrames, strings.Split(specStr, "@")[0])
					singleExpandedFrame = f
				}
				newCols = append(newCols, cols...)
			}
			state.Columns = newCols
			state.RefIndex = 0
			if !explicitFlags["transpose"] && !explicitFlags["compact"] {
				state.Transpose = len(state.Columns) > 4
			}

			if hasAllExpansion && singleExpandedFrame != nil {
				defSize := singleExpandedFrame.GetDefaultSize()
				for i, c := range newCols {
					if c.FrameID == singleExpandedFrame.ID && strings.EqualFold(c.Size, defSize) {
						state.RefIndex = i
						break
					}
				}
			}
		}
	}

	if len(state.Columns) == 0 {
		fmt.Fprintf(a.Err, "Error: no columns selected for comparison\n")
		return 1
	}

	// Automatically enable compact/transpose view if more than 4 columns and not explicitly overridden
	if !explicitFlags["transpose"] && !explicitFlags["compact"] && len(state.Columns) > 4 {
		state.Transpose = true
	}

	idealName := idealFlag
	if idealName == "" {
		if state.IdealProfile != "" {
			idealName = state.IdealProfile
		} else {
			idealName = "default"
		}
	}

	prof, err := ideal.GetProfile(idealName)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	state.IdealProfile = prof.Name

	if prof.SaddleHeightMm != nil && *prof.SaddleHeightMm > 0 {
		state.SaddleHeightMm = *prof.SaddleHeightMm
	}

	// Resolve and validate all columns against frame constraints
	rows, normalizedCols, err := resolveAllColumns(allFrames, state)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	state.Columns = normalizedCols

	if err := a.renderComparisonMatrix(rows, state, prof, !noScoreFlag, scoreDetailFlag); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	return 0
}

func (a *App) printCompareUsage(w io.Writer) {
	fmt.Fprint(w, `BikeGeo Analyst CLI - Compare Geometry & Fit Scoring

Usage:
  bikegeo compare [flags] [spec...]
  bikegeo compare [flags] +<spec> ...
  bikegeo compare [flags] -<spec|col_num> ...

Overview:
  The compare command performs an apples-to-apples kinematic comparison of
  bicycle frame geometries, normalizing dynamic fork travel, axle-to-crown (A2C),
  rake, headset cup spacers, and suspension sag. Each setup is evaluated against
  the rider's biomechanical profile (saddle height, back angle, arm reach, and
  center of mass), computing key fit metrics (effective reach/stack, actual setback,
  AETT, spare vertical travel, front weight distribution) and scoring each setup
  against ideal fit criteria.

Comparison Specs:
  Comparison columns are defined using flexible spec tokens:
    [+|-]frame[:size][@mod1][@mod2]...
    (or using '@' throughout: frame[@size][@fork][@sag])

  Frame Identifier:
    - Exact frame ID (e.g. "seven-expedition-926f", "binary-havok")
    - Model name or unique substring match (e.g. "expedition", "fenrir", "krampus")

  Size Specifier:
    - Specific size (e.g. ":L", ":XL", ":S3", ":S 60cm"): Must match one of the
      frame's declared sizes.
    - Default size (omitted size): If no size is specified, the frame's declared
      default_size is automatically selected.
    - All sizes (":all" or "@all"): Expands the frame into individual comparison
      columns for EVERY size declared by the manufacturer.

  Modifiers (order-independent, separated by '@'):
    - Fork travel (e.g. "@120mm", "@100mm", or "@120"):
      Suspension travel in millimeters. Must be within the frame's supported forks.
    - Rigid fork ("@rigid" or "@0mm"):
      Configures the frame with its declared rigid fork.
      Only permitted if the frame supports rigid forks. Rigid forks require 0% sag.
    - Fork preset ID (e.g. "@enve_472", "@stock"):
      Selects a known preset fork from the catalog.
    - Sag percentage (e.g. "@20%", "@sag=25", or "@20"):
      Suspension sag percent. Pure numbers < 60 are treated as sag %; numbers >= 60
      are treated as fork travel in mm. If omitted, uses the frame's default sag.
    - Default fork (omitted fork):
      If no fork is specified, the frame's default fork (first entry in forks) is used.

Comparison State & Column Management:
  Default Comparison Baseline:
    - Running 'bikegeo compare' with no arguments evaluates the compiled default comparison setups.

  Full Replacement (Bare Specs):
    - Specifying arguments without '+' or '-' replaces all columns for that run.
      Example: bikegeo compare fenrir:L@110mm hayduke:S3@120mm

  Incremental Addition ('+spec'):
    - Prefixing a spec with '+' appends column(s) to the default comparison baseline.
      Example: bikegeo compare +seven-expedition-926f:all@120mm

  Incremental Removal ('-spec' or '-col_num'):
    - By column number: '-1', '-2', etc. removes the 1-based column from the default baseline.
    - By frame ID / substring: '-fenrir' or '-fenrir:all' removes all columns of that frame.
    - By frame size: '-fenrir:L' removes all columns of that frame in size L.
    - By specific fork/sag: '-fenrir:L@110mm' removes only that matching setup.

Table Layout & Auto-Transposition:
  - Standard View (Columns): Each bike is a vertical column; metrics are rows.
    Ideal and Acceptable ranges appear as reference columns.
  - Transposed View (Rows): Each bike is a row; metrics are columns.
    Ideal and Acceptable ranges appear as top header rows.
    Enabled with --transpose or --compact.
  - Automatic Transposition: When a comparison has more than 4 columns (such as
    expanding ':all'), the table automatically renders in transposed mode unless
    explicitly disabled with --transpose=false or --compact=false.

Scoring & Ranking:
  - Setups are scored against sweet-spot curves defined in an Ideal Profile (default: 'default').
  - Metrics display their evaluated zone (✓ ideal, ~ acceptable, ✗ outside) and distance to ideal.
  - Displayed rows are sorted descending by total score, placing the top-match #1 at the top.
  - Explanations section highlights Biggest Losses vs. Ideal and Head-to-Head Deciding Factors.
  - Use --score-detail to view the full criteria breakdown table.
  - Use --no-score to disable scoring and view columns in their original configuration order.

Flags:
  --ideal <name>               Select ideal profile ('default' or 'idmatch')
  -compact, --compact          Render compact/transposed table (Columns as rows, Metrics as columns)
  -transpose, --transpose      Alias for --compact
  -score-detail                Display detailed scoring breakdown table for each criterion
  -no-score                    Disable scoring and ranking display

Global Flags:
  -f, --format <format>     Output format: table (default), md, csv
`)
}

// ParseComparisonSpecs parses a CLI token of the form "frame[:size][@mod1][@mod2]..."
// If the size is specified as "all" (e.g. "frame:all" or "frame@all"), it expands to a ComparisonSetup
// for every size in frame.GetSizes(). Otherwise, it returns a single-element slice.
func ParseComparisonSpecs(
	frames []model.FrameModel,
	spec string,
) ([]model.ComparisonSetup, error) {
	parts := strings.Split(spec, "@")
	frameToken := strings.TrimSpace(parts[0])

	frame, implicitSize, err := ResolveFrameByToken(frames, frameToken)
	if err != nil {
		return nil, err
	}

	var forkToken string
	var sagOverride *float64
	sizeToken := implicitSize

	for _, rawMod := range parts[1:] {
		mod := strings.TrimSpace(rawMod)
		if mod == "" {
			continue
		}
		low := strings.ToLower(mod)

		// 0. "all" modifier representing all sizes
		if low == "all" {
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
				return nil, fmt.Errorf("invalid sag modifier %q in %q", mod, spec)
			}
			sagOverride = &val
			continue
		}

		// 3. Rigid fork modifier
		if low == "rigid" || low == "0mm" || strings.HasPrefix(low, "rigid") {
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

	targetA2c, targetRake, _, displayLabel, err := canonicalizer.ResolveForkToken(forkToken, sagOverride, *frame)
	if err != nil {
		return nil, err
	}

	setup := model.ComparisonSetup{
		FrameID:      frame.ID,
		Size:         sizeToken,
		A2cMm:        targetA2c,
		RakeMm:       targetRake,
		DisplayLabel: displayLabel,
	}

	if strings.EqualFold(setup.Size, "all") {
		sizes := frame.GetSizes()
		if len(sizes) == 0 {
			return nil, fmt.Errorf("frame %q has no declared sizes", frame.ID)
		}
		var setups []model.ComparisonSetup
		for _, sz := range sizes {
			s := setup
			s.Size = sz
			setups = append(setups, s)
		}
		return setups, nil
	}

	return []model.ComparisonSetup{setup}, nil
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

func resolveAllColumns(
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
		row, err := canonicalizer.ResolveFrameForkSetup(*frame, col, state.RiderProfile)
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
	return model.SavedComparisonState{
		RiderProfile: model.DefaultRiderProfile(),
		Transpose:    true,
		RefIndex:     1,
		IdealProfile: "default",
		Columns: []model.ComparisonSetup{
			{FrameID: "binary-havok", Size: "XL", A2cMm: 491, RakeMm: 44, DisplayLabel: "100mm @ 15% sag"},
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

type scoredRowItem struct {
	origIdx     int
	row         model.CanonicalComparisonRow
	score       scoring.Result
	displayName string
}

func (a *App) renderComparisonMatrix(
	rows []model.CanonicalComparisonRow,
	state model.SavedComparisonState,
	prof *ideal.Profile,
	scoringEnabled bool,
	scoreDetail bool,
) error {
	if len(rows) == 0 {
		return nil
	}

	if prof == nil {
		def, err := ideal.GetProfile("default")
		if err != nil {
			return err
		}
		prof = def
	}

	canonicalizer.ApplyIdealCockpitTargets(rows, prof)

	var scoreResults []scoring.Result
	if scoringEnabled {
		scoreResults = scoring.ScoreAllRows(rows, prof)
	}

	displayNames := disambiguateFrameNames(rows)

	scoredItems := make([]scoredRowItem, len(rows))
	scoredFramesForExplain := make([]scoring.ScoredFrame, len(rows))
	for i, r := range rows {
		name := displayNames[i]
		res := scoring.Result{}
		if scoringEnabled {
			res = scoreResults[i]
		}
		scoredItems[i] = scoredRowItem{
			origIdx:     i,
			row:         r,
			score:       res,
			displayName: name,
		}
		scoredFramesForExplain[i] = scoring.ScoredFrame{
			OriginalIndex: i,
			Name:          name,
			Row:           r,
			Score:         res,
		}
	}

	var expl scoring.Explanations
	var footers []FooterSection
	missingNotes := scoring.CollectMissingDataNotes(scoredFramesForExplain)

	if scoringEnabled && len(rows) > 0 {
		expl = scoring.ExplainProfile(scoredFramesForExplain, prof)
		footers = append(footers, FooterSection{
			Title: "Biggest Losses vs. Ideal",
			Lines: expl.Losses,
		})
		if len(rows) > 1 {
			footers = append(footers, FooterSection{
				Title: "Head-to-Head Deciding Factors",
				Lines: expl.HeadToHead,
			})
		}

		if len(missingNotes) > 0 {
			footers = append(footers, FooterSection{
				Title: "Data Notes",
				Lines: missingNotes,
			})
		}

		// Sort displayed rows/columns descending by score
		sort.SliceStable(scoredItems, func(i, j int) bool {
			return scoredItems[i].score.Total > scoredItems[j].score.Total
		})
	} else if len(missingNotes) > 0 {
		footers = append(footers, FooterSection{
			Title: "Data Notes",
			Lines: missingNotes,
		})
	}

	backAngle := 0.0
	if len(rows) > 0 {
		backAngle = rows[0].Canonical.BackAngleDeg
	}
	title := "Apples-to-Apples Geometry Comparison Matrix"
	subtitle := fmt.Sprintf(
		"Saddle Height: %.0fmm | Modeled Back Angle: %.1f° | Ideal Profile: %s (*max stay position)",
		state.SaddleHeightMm, backAngle, prof.Name,
	)

	// Prepare detail table if requested
	var detailHeaders []string
	var detailRows [][]string
	var detailFooters []FooterSection
	var detailSubtitle string
	if scoreDetail && scoringEnabled {
		detailHeaders = []string{"Rank", "#", "Frame"}
		var weightedAttrs []ideal.AttributeSpec
		var weightParts []string
		for _, attr := range prof.Attributes {
			if attr.Weight > 0 && attr.Ideal != nil {
				meta, _ := ideal.LookupAttribute(attr.ID)
				detailHeaders = append(detailHeaders, meta.Label)
				weightedAttrs = append(weightedAttrs, attr)
				weightParts = append(weightParts, fmt.Sprintf("%s %.0f", meta.Label, attr.Weight))
			}
		}
		detailHeaders = append(detailHeaders, "Total")
		detailSubtitle = fmt.Sprintf("Sub-scores (-200..+100) per criterion (points lost on total score in parens). Total = weighted average (weights: %s)", strings.Join(weightParts, ", "))

		for _, item := range scoredItems {
			subScores := make(map[string]float64)
			pointsLost := make(map[string]float64)
			missingMap := make(map[string]bool)
			for _, c := range item.score.Criteria {
				subScores[c.ID] = c.SubScore
				pointsLost[c.ID] = c.PointsLost
				missingMap[c.ID] = c.Missing
			}

			formatSubScoreCell := func(id string) string {
				if missingMap[id] {
					return "—"
				}
				sub := math.Round(subScores[id])
				lost := math.Round(pointsLost[id])
				scorePart := fmt.Sprintf("%.0f", sub)
				if lost >= 1 {
					scorePart = fmt.Sprintf("%.0f (-%.0f)", sub, lost)
				}

				lowID := strings.ToLower(id)
				if (lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback") && item.row.Canonical.SelectedBRP != nil {
					brp := item.row.Canonical.SelectedBRP
					sign := "+"
					if brp.RailOffsetMm < 0 {
						sign = ""
					}
					return fmt.Sprintf("%s [rail %s%.0fmm]", scorePart, sign, brp.RailOffsetMm)
				}
				if (lowID == "saddle_hb" || lowID == "saddlehb") && item.row.Canonical.SelectedHB != nil && item.row.Canonical.SelectedHB.Stem.LengthMm > 0 {
					hb := item.row.Canonical.SelectedHB
					sign := "+"
					if hb.SpacerMm < 0 {
						sign = ""
					}
					return fmt.Sprintf("%s [%.0fmm %.0f° %s%.0fmm]", scorePart, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm)
				}
				if (lowID == "saddle_hb_drop" || lowID == "hb_drop" || lowID == "saddlehbdrop") && item.row.Canonical.SelectedHB != nil {
					drop := item.row.Canonical.SelectedHB.SaddleHbDropMm
					sign := ""
					if drop > 0 {
						sign = "+"
					}
					return fmt.Sprintf("%s [%s%.0fmm]", scorePart, sign, drop)
				}

				return scorePart
			}
			formatTotalCell := func(total float64) string {
				rndTotal := math.Round(total)
				totalLost := math.Round(100.0 - total)
				if totalLost >= 1 {
					return fmt.Sprintf("%.0f (-%.0f)", rndTotal, totalLost)
				}
				return fmt.Sprintf("%.0f", rndTotal)
			}

			rowCells := []string{
				fmt.Sprintf("%d", item.score.Rank),
				fmt.Sprintf("%d", item.origIdx+1),
				item.displayName,
			}
			for _, wa := range weightedAttrs {
				rowCells = append(rowCells, formatSubScoreCell(wa.ID))
			}
			rowCells = append(rowCells, formatTotalCell(item.score.Total))
			detailRows = append(detailRows, rowCells)
		}
		detailFooters = footers
	}

	primaryFooters := footers
	if scoreDetail && scoringEnabled && a.Format != FormatCSV {
		// Delay footers until after the detail table
		primaryFooters = nil
	}

	if state.Transpose {
		headers := []string{
			"#",
			"Frame (Size)",
		}
		if scoringEnabled {
			headers = append(headers, "Score")
		}
		headers = append(headers, "Fork & Eff A2C")

		for _, attr := range prof.Attributes {
			meta, _ := ideal.LookupAttribute(attr.ID)
			headers = append(headers, meta.Label)
		}

		var tableRows [][]string

		// Row 0: Ideal
		idealRow := make([]string, len(headers))
		idealRow[1] = "Ideal"
		attrOffset := 2
		if scoringEnabled {
			attrOffset = 3
		}
		attrOffset++ // past Fork & Eff A2C

		for idx, attr := range prof.Attributes {
			meta, _ := ideal.LookupAttribute(attr.ID)
			idealRow[attrOffset+idx] = ideal.FormatRange(attr.Ideal, meta.Unit, meta.Decimals)
		}

		// Row 1: Acceptable
		accRow := make([]string, len(headers))
		accRow[1] = "Acceptable"
		for idx, attr := range prof.Attributes {
			meta, _ := ideal.LookupAttribute(attr.ID)
			accRow[attrOffset+idx] = ideal.FormatRange(attr.Acceptable, meta.Unit, meta.Decimals)
		}

		tableRows = append(tableRows, idealRow, accRow)

		for _, item := range scoredItems {
			r := item.row
			i := item.origIdx

			idxStr := fmt.Sprintf("%d", i+1)
			setupStr := canonicalizer.FormatForkEffA2c(r)

			rowCells := []string{
				idxStr,
				item.displayName,
			}
			if scoringEnabled {
				scoreStr := fmt.Sprintf("%.0f (#%d)", math.Round(item.score.Total), item.score.Rank)
				rowCells = append(rowCells, scoreStr)
			}
			rowCells = append(rowCells, setupStr)

			critMap := make(map[string]scoring.CriterionResult)
			for _, cr := range item.score.Criteria {
				critMap[cr.ID] = cr
			}

			for _, attr := range prof.Attributes {
				meta, _ := ideal.LookupAttribute(attr.ID)
				if cr, ok := critMap[attr.ID]; ok {
					rowCells = append(rowCells, cr.FormattedCell)
				} else {
					val, ok := meta.Extract(r)
					if !ok {
						rowCells = append(rowCells, "— (no data)")
					} else {
						ev := ideal.Evaluate(attr, meta, val, &r)
						rowCells = append(rowCells, ev.FormattedCell)
					}
				}
			}

			tableRows = append(tableRows, rowCells)
		}

		if err := RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, tableRows, primaryFooters); err != nil {
			return err
		}
	} else {
		// Default orientation: Metrics as rows, Bike Setups as columns
		// Headers: Metric, Ideal, Acceptable, [1] Bike 1, [2] Bike 2...
		headers := make([]string, len(scoredItems)+3)
		headers[0] = "Metric"
		headers[1] = "Ideal"
		headers[2] = "Acceptable"
		for colIdx, item := range scoredItems {
			r := item.row
			i := item.origIdx
			forkLine := r.DisplayLabel
			if forkLine == "" {
				forkLine = fmt.Sprintf("%.0fmm A2C", math.Round(r.EffectiveA2c))
			}
			headers[colIdx+3] = fmt.Sprintf("[%d] %s %s (%s)\n%s", i+1, r.Bike.Manufacturer, r.Bike.Model, r.Bike.Size, forkLine)
		}

		buildRow := func(metric string, idealStr string, accStr string, cellFn func(colIdx int, item scoredRowItem) string) []string {
			out := make([]string, len(scoredItems)+3)
			out[0] = metric
			out[1] = idealStr
			out[2] = accStr
			for colIdx, item := range scoredItems {
				out[colIdx+3] = cellFn(colIdx, item)
			}
			return out
		}

		var tableRows [][]string

		// Score row if enabled
		if scoringEnabled {
			tableRows = append(tableRows, buildRow("Score", "", "", func(colIdx int, item scoredRowItem) string {
				return fmt.Sprintf("%.0f (#%d)", math.Round(item.score.Total), item.score.Rank)
			}))
		}

		// Setup Fork & Effective A2C
		tableRows = append(tableRows, buildRow("Fork & Eff. A2C", "", "", func(colIdx int, item scoredRowItem) string {
			return canonicalizer.FormatForkFlow(item.row)
		}))

		// Attribute rows from prof.Attributes
		for _, attr := range prof.Attributes {
			meta, _ := ideal.LookupAttribute(attr.ID)
			label := meta.Label
			if strings.EqualFold(attr.ID, "aett") || strings.EqualFold(attr.ID, "setback") || strings.EqualFold(attr.ID, "saddle_hb") || strings.EqualFold(attr.ID, "brp_setback") || strings.EqualFold(attr.ID, "saddle_hb_drop") {
				label = fmt.Sprintf("%s @ %.0fmm", meta.Label, state.SaddleHeightMm)
			}
			idealStr := ideal.FormatRange(attr.Ideal, meta.Unit, meta.Decimals)
			accStr := ideal.FormatRange(attr.Acceptable, meta.Unit, meta.Decimals)

			attrID := attr.ID
			tableRows = append(tableRows, buildRow(label, idealStr, accStr, func(colIdx int, item scoredRowItem) string {
				for _, cr := range item.score.Criteria {
					if cr.ID == attrID {
						return cr.FormattedCell
					}
				}
				val, ok := meta.Extract(item.row)
				if !ok {
					return "— (no data)"
				}
				ev := ideal.Evaluate(attr, meta, val, &item.row)
				return ev.FormattedCell
			}))
		}

		if err := RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, tableRows, primaryFooters); err != nil {
			return err
		}
	}

	if scoreDetail && scoringEnabled && a.Format != FormatCSV {
		fmt.Fprintln(a.Out)
		detailTitle := "Scoring Breakdown"
		if err := RenderOutputWithFooter(a.Out, a.Format, detailTitle, detailSubtitle, detailHeaders, detailRows, detailFooters); err != nil {
			return err
		}
	}

	return nil
}

// disambiguateFrameNames returns a slice of display names corresponding to rows.
// When a (FrameID, Size) appears multiple times in the comparison, each instance
// is disambiguated with concise fork/sag modifiers (or fallback column index if identical).
func disambiguateFrameNames(rows []model.CanonicalComparisonRow) []string {
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
