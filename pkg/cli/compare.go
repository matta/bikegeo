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
	"bikegeo/pkg/catalog"
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
	var noScoreFlag bool
	var scoreDetailFlag bool
	var saddleFlag string
	var handlebarFlag string
	var allFlag bool
	var allConfigsFlag bool
	var topPerFrameFlag int
	var topLimitFlag int

	fs.BoolVar(&transposeFlag, "transpose", false, "Transpose table (Columns as rows, Metrics as columns)")
	fs.BoolVar(&noScoreFlag, "no-score", false, "Disable scoring and ranking display")
	fs.BoolVar(&scoreDetailFlag, "score-detail", false, "Display detailed scoring breakdown table")
	fs.StringVar(&saddleFlag, "saddle", "", "Saddle model from catalog (default: 'sqlab-610-ergolux')")
	fs.StringVar(&handlebarFlag, "handlebar", "", "Handlebar model from catalog (default: 'persuader20_800')")
	fs.BoolVar(&allFlag, "all", false, "Compare all registered frames across all sizes and plausible configurations")
	fs.BoolVar(&allConfigsFlag, "all-configs", false, "Expand all sizes and plausible fork/sag configurations for target frame(s)")
	fs.IntVar(&topPerFrameFlag, "top-per-frame", -1, "Maximum manifestations kept per frame (default: 1 for --all, 5 for specific frame @all, 0 for unlimited)")
	fs.IntVar(&topLimitFlag, "top", 0, "Maximum total comparison setups to display after ranking (0 for unlimited)")

	boolFlags := map[string]bool{
		"transpose":    true,
		"no-score":     true,
		"score-detail": true,
		"all":          true,
		"all-configs":  true,
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

	if explicitFlags["transpose"] {
		state.Transpose = transposeFlag
	}

	posArgs := fs.Args()
	if len(posArgs) == 1 && (posArgs[0] == "help" || posArgs[0] == "-h" || posArgs[0] == "--help") {
		a.printCompareUsage(a.Out)
		return 0
	}

	if allFlag && len(posArgs) == 0 {
		posArgs = []string{"all:all@all"}
	} else if allConfigsFlag && len(posArgs) == 0 {
		posArgs = []string{"all:all@all"}
	} else if len(posArgs) == 1 && strings.EqualFold(posArgs[0], "all") {
		posArgs = []string{"all:all@all"}
	}

	if allConfigsFlag && len(posArgs) > 0 {
		var rewritten []string
		for _, arg := range posArgs {
			if !strings.Contains(arg, "@all") && !strings.Contains(arg, "@forks") && !strings.Contains(arg, "@all-forks") {
				if strings.Contains(arg, ":") {
					arg = arg + "@all"
				} else {
					arg = arg + ":all@all"
				}
			}
			rewritten = append(rewritten, arg)
		}
		posArgs = rewritten
	}

	isAllFrames := allFlag
	hasAllForksExpansion := allConfigsFlag
	for _, arg := range posArgs {
		clean := strings.TrimPrefix(strings.TrimPrefix(arg, "+"), "-")
		parts := strings.Split(clean, "@")
		framePart, _, _ := strings.Cut(parts[0], ":")
		if strings.EqualFold(framePart, "all") {
			isAllFrames = true
		}
		for _, p := range parts[1:] {
			low := strings.ToLower(strings.TrimSpace(p))
			if low == "all" || low == "all-forks" || low == "forks" || low == "all-configs" {
				hasAllForksExpansion = true
			}
		}
	}

	topPerFrame := topPerFrameFlag
	if !explicitFlags["top-per-frame"] && !explicitFlags["per-frame"] {
		if isAllFrames {
			topPerFrame = 1
		} else if hasAllForksExpansion {
			topPerFrame = 5
		} else {
			topPerFrame = 0
		}
	}
	if topPerFrame < 0 {
		topPerFrame = 0
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
				if after, ok := strings.CutPrefix(arg, "+"); ok {
					specStr := after
					cols, err := ParseComparisonSpecs(allFrames, specStr)
					if err != nil {
						fmt.Fprintf(a.Err, "Error: %v\n", err)
						return 1
					}
					state.Columns = append(state.Columns, cols...)
				} else if after, ok := strings.CutPrefix(arg, "-"); ok {
					rmToken := after
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
					} else if strings.EqualFold(rmToken, "all") {
						state.Columns = nil
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
			if !explicitFlags["transpose"] {
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

	// Automatically enable transposed view if more than 4 columns and not explicitly overridden
	if !explicitFlags["transpose"] && len(state.Columns) > 4 {
		state.Transpose = true
	}

	prof, err := ideal.GetProfile("idmatch")
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	state.IdealProfile = prof.Name

	if prof.SaddleHeightMm != nil && *prof.SaddleHeightMm > 0 {
		state.SaddleHeightMm = *prof.SaddleHeightMm
	}

	if saddleFlag != "" {
		s, ok := catalog.GetSaddle(saddleFlag)
		if !ok {
			fmt.Fprintf(a.Err, "Error: unrecognized saddle %q (see 'bikegeo saddles' for catalog)\n", saddleFlag)
			return 1
		}
		state.SaddleID = s.ID
		state.Saddle = s
	} else if state.Saddle.ID == "" {
		state.Saddle = catalog.DefaultSaddle()
		state.SaddleID = state.Saddle.ID
	}

	if handlebarFlag != "" {
		b, ok := catalog.GetHandlebar(handlebarFlag)
		if !ok {
			fmt.Fprintf(a.Err, "Error: unrecognized handlebar %q (see 'bikegeo handlebars' for catalog)\n", handlebarFlag)
			return 1
		}
		state.HandlebarID = b.ID
		state.Handlebar = b
		state.RiderProfile.Handlebar = b
	} else if state.Handlebar.ID == "" {
		state.Handlebar = catalog.DefaultHandlebar()
		state.HandlebarID = state.Handlebar.ID
		state.RiderProfile.Handlebar = state.Handlebar
	}

	// Resolve and validate all columns against frame constraints
	rows, normalizedCols, err := resolveAllColumns(allFrames, state)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	state.Columns = normalizedCols

	// Two-stage ranking and pruning for combinatorial sanity
	rows = pruneComparisonRows(allFrames, rows, prof, topPerFrame, topLimitFlag)
	if len(rows) == 0 {
		fmt.Fprintf(a.Err, "Error: no valid setups found for comparison\n")
		return 1
	}
	var prunedCols []model.ComparisonSetup
	for _, r := range rows {
		prunedCols = append(prunedCols, r.Setup)
	}
	state.Columns = prunedCols
	if state.RefIndex >= len(state.Columns) {
		state.RefIndex = 0
	}
	if !explicitFlags["transpose"] && !explicitFlags["compact"] && len(state.Columns) > 4 {
		state.Transpose = true
	}

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
  bikegeo compare --all [flags]
  bikegeo compare [flags] +<spec> ...
  bikegeo compare [flags] -<spec|col_num> ...

Overview:
  The compare command performs an apples-to-apples kinematic comparison of
  bicycle frame geometries, normalizing dynamic fork travel, axle-to-crown (A2C),
  rake, headset cup spacers, and suspension sag. Each setup is evaluated against
  the rider's biomechanical profile (saddle height, back angle, arm reach, and
  center of mass), computing key fit metrics (effective reach/stack, actual setback,
  AETT, saddle-to-headtube drop, front weight distribution) and scoring each setup
  against ideal fit criteria.

Comparison Specs:
  Comparison columns are defined using flexible spec tokens:
    [+|-]frame[:size][@mod1][@mod2]...
    (or using '@' throughout: frame[@size][@fork][@sag])

  Frame Identifier:
    - Exact frame ID (e.g. "seven-expedition-926f", "binary-havok")
    - Model name or unique substring match (e.g. "expedition", "fenrir", "krampus")
    - All frames ("all"): Expands comparison across all registered frames in memory.
      Frames that do not support a requested size or fork modifier are gracefully skipped.

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
    - All plausible forks ("@all", "@all-forks", or "@forks"):
      Expands the frame across all supported rigid forks and all 10mm suspension travel
      steps in its supported range, across plausible sags (20%, 25%, 30%).
    - Sag percentage (e.g. "@20%", "@sag=25", or "@20"):
      Suspension sag percent. Pure numbers < 60 are treated as sag %; numbers >= 60
      are treated as fork travel in mm. If omitted, uses the frame's default sag.
    - Default fork (omitted fork):
      If no fork is specified, the frame's default fork (first entry in forks) is used.

Comparison State & Column Management:
  Default Comparison Baseline:
    - Running 'bikegeo compare' with no arguments evaluates the compiled default comparison setups.

  All-Frames & All-Configurations Expansion:
    - 'bikegeo compare --all' (or 'bikegeo compare all'):
      Compares all registered frames across all sizes and plausible fork/sag configurations,
      automatically selecting the #1 highest-scoring manifestation per frame (18 rows).
    - 'bikegeo compare <frame>:all@all' (or 'bikegeo compare --all-configs <frame>'):
      Compares all sizes and plausible fork/sag configurations for that frame (default: top 5).
    - 'bikegeo compare all@rigid':
      Compares all registered frames supporting rigid forks (default size, rigid fork).

  Two-Stage Ranking & Pruning Funnel:
    - Stage 1 (Optimal Sag): When expanding plausible forks and sags, each (Frame, Size, Fork)
      selects its single highest-scoring sag point (breaking score ties toward 20% sag).
    - Stage 2 (Top N per Frame): Keeps at most N manifestations per frame (--top-per-frame).
      Defaults to 1 for all-frames comparison and 5 for specific frame @all sweeps.
    - Stage 3 (Global Top Limit): Optionally restricts output to top M setups overall (--top).

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
    - All columns: '-all' removes all columns.

Table Layout & Auto-Transposition:
  - Standard View (Columns): Each bike is a vertical column; metrics are rows.
    Ideal and Acceptable ranges appear as reference columns.
  - Transposed View (Rows): Each bike is a row; metrics are columns.
    Ideal and Acceptable ranges appear as top header rows.
    Enabled with --transpose.
  - Automatic Transposition: When a comparison has more than 4 columns (such as
    expanding ':all' or '--all'), the table automatically renders in transposed mode unless
    explicitly disabled with --transpose=false.

Scoring & Ranking:
  - Setups are scored against sweet-spot curves defined in an Ideal Profile (default: 'default').
  - Metrics display their evaluated zone (✓ ideal, ~ acceptable, ✗ outside) and distance to ideal.
  - Displayed rows are sorted descending by total score, placing the top-match #1 at the top.
  - Explanations section highlights Biggest Losses vs. Ideal and Head-to-Head Deciding Factors.
  - Use --score-detail to view the full criteria breakdown table.
  - Use --no-score to disable scoring and view columns in their original configuration order.

Flags:
  --all                        Compare all registered frames across all plausible configurations
  --all-configs                Expand all sizes and plausible fork/sag configurations for target frame(s)
  --top-per-frame <N>          Maximum manifestations kept per frame (default: 1 for --all, 5 for @all, 0=all)
  --top <M>                    Maximum total comparison setups to display after ranking (0 for unlimited)
  --saddle <name>              Select saddle model from catalog ('sqlab-610-ergolux' or 'berthoud-aspin-open')
  --handlebar <name>           Select handlebar model from catalog (default: 'persuader20_800')
  --transpose                  Render transposed table (Columns as rows, Metrics as columns)
  --score-detail               Display detailed scoring breakdown table for each criterion
  --no-score                   Disable scoring and ranking display

Global Flags:
  -f, --format <format>     Output format: table (default), md, csv
`)
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

func pruneComparisonRows(
	allFrames []model.FrameModel,
	rows []model.CanonicalComparisonRow,
	prof *ideal.Profile,
	topPerFrame int,
	topLimit int,
) []model.CanonicalComparisonRow {
	if len(rows) <= 1 {
		return rows
	}

	frameMap := make(map[string]model.FrameModel, len(allFrames))
	for _, f := range allFrames {
		frameMap[strings.ToLower(f.ID)] = f
	}

	canonicalizer.ApplyIdealCockpitTargets(rows, prof)

	type scoredCandidate struct {
		row   model.CanonicalComparisonRow
		score scoring.Result
		index int
	}

	candidates := make([]scoredCandidate, len(rows))
	for i, r := range rows {
		candidates[i] = scoredCandidate{
			row:   r,
			score: scoring.ScoreRow(r, prof),
			index: i,
		}
	}

	// Stage 1: For AutoExpanded rows, keep the optimal sag per (FrameID, Size, ForkType)
	var stage1 []scoredCandidate
	type sagGroupKey struct {
		frameID  string
		size     string
		forkType string
	}
	sagGroups := make(map[sagGroupKey][]scoredCandidate)
	var sagGroupOrder []sagGroupKey

	for _, c := range candidates {
		if !c.row.Setup.AutoExpanded {
			stage1 = append(stage1, c)
			continue
		}
		ft := strings.ToLower(canonicalizer.FormatForkType(c.row))
		key := sagGroupKey{
			frameID:  strings.ToLower(c.row.Setup.FrameID),
			size:     strings.ToLower(c.row.Setup.Size),
			forkType: ft,
		}
		if _, exists := sagGroups[key]; !exists {
			sagGroupOrder = append(sagGroupOrder, key)
		}
		sagGroups[key] = append(sagGroups[key], c)
	}

	for _, k := range sagGroupOrder {
		group := sagGroups[k]
		if len(group) == 1 {
			stage1 = append(stage1, group[0])
			continue
		}
		best := group[0]
		for _, cand := range group[1:] {
			if cand.score.Total > best.score.Total+1e-4 {
				best = cand
			} else if math.Abs(cand.score.Total-best.score.Total) <= 1e-4 {
				// Score tie-break:
				// 1. Closest to 20% sag
				distCand := math.Abs(cand.row.Setup.SagPct - 20.0)
				distBest := math.Abs(best.row.Setup.SagPct - 20.0)
				if distCand < distBest-1e-4 {
					best = cand
				} else if math.Abs(distCand-distBest) <= 1e-4 {
					// 2. Lower sag (e.g. 20% before 25%)
					if cand.row.Setup.SagPct < best.row.Setup.SagPct {
						best = cand
					}
				}
			}
		}
		stage1 = append(stage1, best)
	}

	// Stage 2: Top N manifestations per frame
	stage2 := stage1
	if topPerFrame > 0 {
		frameGroups := make(map[string][]scoredCandidate)
		var frameOrder []string
		for _, c := range stage1 {
			fid := strings.ToLower(c.row.Setup.FrameID)
			if _, exists := frameGroups[fid]; !exists {
				frameOrder = append(frameOrder, fid)
			}
			frameGroups[fid] = append(frameGroups[fid], c)
		}

		var filtered []scoredCandidate
		for _, fid := range frameOrder {
			group := frameGroups[fid]
			f := frameMap[fid]
			defSize := f.GetDefaultSize()
			defFork := f.GetDefaultFork()

			sort.SliceStable(group, func(i, j int) bool {
				a, b := group[i], group[j]
				if math.Abs(a.score.Total-b.score.Total) > 1e-4 {
					return a.score.Total > b.score.Total
				}
				// Tie-breaking:
				// 1. Frame's default size
				aIsDefSize := strings.EqualFold(a.row.Setup.Size, defSize)
				bIsDefSize := strings.EqualFold(b.row.Setup.Size, defSize)
				if aIsDefSize != bIsDefSize {
					return aIsDefSize
				}
				// 2. Frame's default fork A2C (within 1mm)
				if defFork.A2cMm > 0 {
					aIsDefFork := math.Abs(a.row.EffectiveA2c-defFork.A2cMm) < 1.0
					bIsDefFork := math.Abs(b.row.EffectiveA2c-defFork.A2cMm) < 1.0
					if aIsDefFork != bIsDefFork {
						return aIsDefFork
					}
				}
				// 3. Cockpit fit: smaller BestR
				if a.row.Canonical.CockpitFitResult != nil && b.row.Canonical.CockpitFitResult != nil {
					diffR := a.row.Canonical.CockpitFitResult.BestR - b.row.Canonical.CockpitFitResult.BestR
					if math.Abs(diffR) > 1e-4 {
						return diffR < 0
					}
				}
				// 4. Rail offset: smaller absolute rail offset
				if a.row.Canonical.SelectedBRP != nil && b.row.Canonical.SelectedBRP != nil {
					diffRail := math.Abs(a.row.Canonical.SelectedBRP.RailOffsetMm) - math.Abs(b.row.Canonical.SelectedBRP.RailOffsetMm)
					if math.Abs(diffRail) > 1e-4 {
						return diffRail < 0
					}
				}
				return a.index < b.index
			})

			limit := min(topPerFrame, len(group))
			filtered = append(filtered, group[:limit]...)
		}
		stage2 = filtered
	}

	// Always sort surviving candidates globally descending by total score
	sort.SliceStable(stage2, func(i, j int) bool {
		if math.Abs(stage2[i].score.Total-stage2[j].score.Total) > 1e-4 {
			return stage2[i].score.Total > stage2[j].score.Total
		}
		return stage2[i].index < stage2[j].index
	})

	// Stage 3: Global Top Limit
	if topLimit > 0 && len(stage2) > topLimit {
		stage2 = stage2[:topLimit]
	}

	result := make([]model.CanonicalComparisonRow, len(stage2))
	for i, c := range stage2 {
		result[i] = c.row
	}
	return result
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

type scoredRowItem struct {
	origIdx     int
	row         model.CanonicalComparisonRow
	score       scoring.Result
	displayName string
}

func adaptProfileAttributeForSaddle(attr ideal.AttributeSpec, saddle model.SaddleSpec) ideal.AttributeSpec {
	if saddle.ID == "" || saddle.ID == "sqlab-610-ergolux" {
		return attr
	}
	refSQ := model.ReferenceIdmatchSaddle()
	lowID := strings.ToLower(attr.ID)
	if lowID == "brp_setback" || lowID == "brp" || lowID == "brpsetback" {
		copied := attr
		if attr.Ideal != nil {
			idCopy := *attr.Ideal
			if idCopy.Min != nil {
				val := saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(*idCopy.Min))
				idCopy.Min = &val
			}
			if idCopy.Max != nil {
				val := saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(*idCopy.Max))
				idCopy.Max = &val
			}
			copied.Ideal = &idCopy
		}
		if attr.Acceptable != nil {
			accCopy := *attr.Acceptable
			if accCopy.Min != nil {
				val := saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(*accCopy.Min))
				accCopy.Min = &val
			}
			if accCopy.Max != nil {
				val := saddle.TargetBrpForPelvicSetback(refSQ.SitBoneSetbackFromBrp(*accCopy.Max))
				accCopy.Max = &val
			}
			copied.Acceptable = &accCopy
		}
		return copied
	}
	if lowID == "saddle_hb" || lowID == "saddlehb" {
		copied := attr
		if attr.Ideal != nil {
			idCopy := *attr.Ideal
			if idCopy.Min != nil {
				tDist := saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, *idCopy.Min, 3.0)
				tDist = math.Round(tDist*10) / 10
				idCopy.Min = &tDist
			}
			if idCopy.Max != nil {
				tDist := saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, *idCopy.Max, 3.0)
				tDist = math.Round(tDist*10) / 10
				idCopy.Max = &tDist
			}
			copied.Ideal = &idCopy
		}
		if attr.Acceptable != nil {
			accCopy := *attr.Acceptable
			if accCopy.Min != nil {
				tDist := saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, *accCopy.Min, 3.0)
				tDist = math.Round(tDist*10) / 10
				accCopy.Min = &tDist
			}
			if accCopy.Max != nil {
				tDist := saddle.TranslateHbDistanceFromRef(refSQ, 746.0, 219.0, *accCopy.Max, 3.0)
				tDist = math.Round(tDist*10) / 10
				accCopy.Max = &tDist
			}
			copied.Acceptable = &accCopy
		}
		return copied
	}
	return attr
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
		def, err := ideal.GetProfile("idmatch")
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

	refDefLines := []string{
		"Chainstay (*): Asterisk indicates adjustable/sliding dropouts; displayed at maximum stay position (longest wheelbase, most rearward axle). Front Wt %, Wheelbase, and Saddle-Rear Axle are calculated at this exact position.",
		"Saddle-Rear Axle: Horizontal distance from saddle to rear axle (Rear Center minus Saddle Setback).",
		"Saddle-HT Drop: Vertical drop from saddle to head tube top (Saddle Height * sin(STA) - Stack; steerer spacer/stem rise needed for level bars).",
		"Seatpost Ext: Exposed seatpost length from collar to saddle rails (available vertical height for dropper post fitment).",
	}

	var adjustableStayLines []string
	seenAdjustable := make(map[string]bool)
	for _, item := range scoredItems {
		b := item.row.Bike
		if b.HasAdjustableChainstay() {
			key := fmt.Sprintf("%s %s (%.0f-%.0f)", b.Manufacturer, b.Model, b.GetMinChainstay(), b.GetMaxChainstay())
			if !seenAdjustable[key] {
				seenAdjustable[key] = true
				adjustableStayLines = append(adjustableStayLines, fmt.Sprintf("%s %s: %.0f–%.0fmm (max %.0fmm)", b.Manufacturer, b.Model, b.GetMinChainstay(), b.GetMaxChainstay(), b.GetMaxChainstay()))
			}
		}
	}
	if len(adjustableStayLines) > 0 {
		refDefLines = append(refDefLines, fmt.Sprintf("Sliding Dropout Ranges: %s.", strings.Join(adjustableStayLines, "; ")))
	}

	refDefFooter := FooterSection{
		Title: "Reference Metric Definitions",
		Lines: refDefLines,
	}

	if scoringEnabled && len(rows) > 0 {
		expl = scoring.ExplainProfile(scoredFramesForExplain, prof)

		if strings.EqualFold(prof.Name, "idmatch") {
			saddle := state.Saddle
			if saddle.ID == "" {
				saddle = catalog.DefaultSaddle()
			}
			targetSitBones := model.DerivePelvicSetbackFromIdmatchBrp(219.0)
			targetBRP := saddle.TargetBrpForPelvicSetback(targetSitBones)
			footers = append(footers, FooterSection{
				Title: "Biomechanical Pelvic Fit & Seatpost Baseline",
				Lines: []string{
					fmt.Sprintf("Primary Pelvic Invariant: Sit Bone Setback is %.0fmm behind BB (derived from idmatch BRP 219mm + SQlab delta 73mm).", targetSitBones),
					"Seatpost Hardware Baseline: Hardcoded to Redshift ShockStop suspension post with 11mm sagged setback (7mm static + 4mm sag).",
					fmt.Sprintf("Active Saddle Target: %s has BRP %.0fmm behind BB (Delta BRP-to-SitBones = %.0fmm).", saddle.Name, targetBRP, saddle.DeltaBrpToSitBonesMm()),
				},
			})
		}

		footers = append(footers, refDefFooter)

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
	} else {
		footers = append(footers, refDefFooter)
		if len(missingNotes) > 0 {
			footers = append(footers, FooterSection{
				Title: "Data Notes",
				Lines: missingNotes,
			})
		}
	}

	backAngle := 0.0
	if len(rows) > 0 {
		backAngle = rows[0].Canonical.BackAngleDeg
	}
	title := "Apples-to-Apples Geometry Comparison Matrix"
	saddleName := state.Saddle.Name
	if saddleName == "" {
		saddleName = catalog.DefaultSaddle().Name
	}
	bar := state.Handlebar
	if bar.ID == "" {
		bar = catalog.DefaultHandlebar()
	}
	baseBarName := bar.Name
	if idx := strings.Index(baseBarName, " ("); idx != -1 {
		baseBarName = baseBarName[:idx]
	}

	line1 := fmt.Sprintf(
		"Saddle Height: %.0fmm | Saddle: %s | Seatpost: Redshift ShockStop (sagged 11mm) | Modeled Back Angle: %.1f°",
		state.SaddleHeightMm, saddleName, backAngle,
	)
	line2 := fmt.Sprintf(
		"Handlebar: %s (%.0fmm, %.0fmm rise, %.0f° sweep, %.0f° upsweep) | Ideal Profile: %s (*max stay position)",
		baseBarName, bar.EffectiveWidthMm(), bar.StatedRiseMm, bar.BacksweepDeg, bar.UpsweepDeg, prof.Name,
	)
	subtitle := fmt.Sprintf("%s\n%s", line1, line2)

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
				if (lowID == "handlebar_clamp_fit" || lowID == "clamp_fit") && item.row.Canonical.CockpitFitResult != nil {
					cfr := item.row.Canonical.CockpitFitResult
					hb := cfr.SelectedHB
					sign := "+"
					if hb.SpacerMm < 0 {
						sign = ""
					}
					stemPenStr := ""
					if cfr.StemPenalty > 0 {
						stemPenStr = fmt.Sprintf(" [-%.0f stem]", cfr.StemPenalty)
					}
					return fmt.Sprintf("%s [%.0fmm %.0f° %s%.0fmm]%s (%d in oval)", scorePart, hb.Stem.LengthMm, hb.Stem.AngleDeg, sign, hb.SpacerMm, stemPenStr, cfr.CountInside)
				}
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
		if a.Format == FormatCSV {
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

			// Row 1: Acceptable
			accRow := make([]string, len(headers))
			accRow[1] = "Acceptable"

			attrOffset := 2
			if scoringEnabled {
				attrOffset = 3
			}
			attrOffset++ // past Fork & Eff A2C

			for idx, attr := range prof.Attributes {
				evalAttr := adaptProfileAttributeForSaddle(attr, state.Saddle)
				meta, _ := ideal.LookupAttribute(evalAttr.ID)
				idealRow[attrOffset+idx], accRow[attrOffset+idx] = formatCompareAttrRanges(evalAttr, meta)
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
			// Split into Table 1 (Fit & Cockpit) and Table 2 (Frame & Chassis Specifications)
			var fitAttrs []ideal.AttributeSpec
			for _, attr := range prof.Attributes {
				if attr.Ideal != nil || strings.EqualFold(attr.ID, "best_stem") {
					fitAttrs = append(fitAttrs, attr)
				}
			}
			if len(fitAttrs) == 0 {
				fitAttrs = prof.Attributes
			}

			table1Title := title + " - Fit & Cockpit"
			headers1 := []string{
				"#",
				"Frame (Size)",
			}
			if scoringEnabled {
				headers1 = append(headers1, "Score")
			}
			headers1 = append(headers1, wrapHeader("Fork & Eff A2C"))

			for _, attr := range fitAttrs {
				meta, _ := ideal.LookupAttribute(attr.ID)
				headers1 = append(headers1, wrapHeader(meta.Label))
			}

			var table1Rows [][]string

			// Row 0: Ideal
			idealRow := make([]string, len(headers1))
			idealRow[1] = "Ideal"

			// Row 1: Acceptable
			accRow := make([]string, len(headers1))
			accRow[1] = "Acceptable"

			attrOffset := 2
			if scoringEnabled {
				attrOffset = 3
			}
			attrOffset++ // past Fork & Eff A2C

			for idx, attr := range fitAttrs {
				evalAttr := adaptProfileAttributeForSaddle(attr, state.Saddle)
				meta, _ := ideal.LookupAttribute(evalAttr.ID)
				idealRow[attrOffset+idx], accRow[attrOffset+idx] = formatCompareAttrRanges(evalAttr, meta)
			}

			table1Rows = append(table1Rows, idealRow, accRow)

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

				for _, attr := range fitAttrs {
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

				table1Rows = append(table1Rows, rowCells)
			}

			if err := RenderOutputWithFooter(a.Out, a.Format, table1Title, subtitle, headers1, table1Rows, nil); err != nil {
				return err
			}

			fmt.Fprintln(a.Out)

			geoTitle := "Frame & Chassis Specifications"
			geoHeaders := []string{
				"#",
				"Frame (Size)",
				"Reach",
				"Stack",
				"HTA",
				"Trail",
				"STA",
				wrapHeader("BB Drop"),
				wrapHeader("Chainstay"),
				wrapHeader("Saddle-Rear Axle"),
				wrapHeader("Saddle-HT Drop"),
			}

			var geoRows [][]string
			csMeta, _ := ideal.LookupAttribute("chainstay")

			for _, item := range scoredItems {
				r := item.row
				i := item.origIdx

				reachStr := "—"
				if r.Canonical.ActiveReach > 0 {
					reachStr = fmt.Sprintf("%.0f", r.Canonical.ActiveReach)
				}
				stackStr := "—"
				if r.Canonical.ActiveStack > 0 {
					stackStr = fmt.Sprintf("%.0f", r.Canonical.ActiveStack)
				}
				htaStr := "—"
				if r.Canonical.ActiveHta > 0 {
					htaStr = fmt.Sprintf("%.1f°", r.Canonical.ActiveHta)
				}
				trailStr := "—"
				if r.Canonical.TrailMm > 0 {
					trailStr = fmt.Sprintf("%.0f", r.Canonical.TrailMm)
				}
				staStr := "—"
				if r.Canonical.ActiveSta > 0 {
					staStr = fmt.Sprintf("%.1f°", r.Canonical.ActiveSta)
				}
				bbStr := "—"
				if r.Canonical.ActiveBbDrop != 0 {
					bbStr = fmt.Sprintf("%.0f", r.Canonical.ActiveBbDrop)
				}
				csStr := "—"
				if val, ok := csMeta.Extract(r); ok {
					csStr = csMeta.FormatVal(val, &r)
				}
				seatRcStr := "—"
				if r.Canonical.SeatToRearCenter > 0 {
					seatRcStr = fmt.Sprintf("%.0f", r.Canonical.SeatToRearCenter)
				}
				spareStr := "—"
				if r.Canonical.SpareVerticalStack != 0 {
					spareStr = fmt.Sprintf("%.0f", r.Canonical.SpareVerticalStack)
				}

				geoRow := []string{
					fmt.Sprintf("%d", i+1),
					item.displayName,
					reachStr,
					stackStr,
					htaStr,
					trailStr,
					staStr,
					bbStr,
					csStr,
					seatRcStr,
					spareStr,
				}
				geoRows = append(geoRows, geoRow)
			}

			if err := RenderOutputWithFooter(a.Out, a.Format, geoTitle, "", geoHeaders, geoRows, primaryFooters); err != nil {
				return err
			}
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
			evalAttr := adaptProfileAttributeForSaddle(attr, state.Saddle)
			meta, _ := ideal.LookupAttribute(evalAttr.ID)
			label := meta.Label
			if strings.EqualFold(evalAttr.ID, "aett") || strings.EqualFold(evalAttr.ID, "setback") || strings.EqualFold(evalAttr.ID, "saddle_hb") || strings.EqualFold(evalAttr.ID, "brp_setback") || strings.EqualFold(evalAttr.ID, "saddle_hb_drop") || strings.EqualFold(evalAttr.ID, "handlebar_clamp_fit") || strings.EqualFold(evalAttr.ID, "seat_to_rc") || strings.EqualFold(evalAttr.ID, "sparevert") {
				label = fmt.Sprintf("%s @ %.0fmm", meta.Label, state.SaddleHeightMm)
			}
			idealStr, accStr := formatCompareAttrRanges(evalAttr, meta)

			attrID := evalAttr.ID
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
				ev := ideal.Evaluate(evalAttr, meta, val, &item.row)
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

func formatCompareAttrRanges(evalAttr ideal.AttributeSpec, meta ideal.AttributeMeta) (string, string) {
	if strings.EqualFold(evalAttr.ID, "handlebar_clamp_fit") {
		return "≥4", "1–3"
	}
	return ideal.FormatRange(evalAttr.Ideal, meta.Unit, meta.Decimals), ideal.FormatRange(evalAttr.Acceptable, meta.Unit, meta.Decimals)
}

func wrapHeader(h string) string {
	switch h {
	case "Fork & Eff A2C":
		return "Fork &\nEff A2C"
	case "BRP Setback":
		return "BRP\nSetback"
	case "Saddle-HB":
		return "Saddle-\nHB"
	case "HB Rise", "Saddle-HB Drop":
		return "HB\nRise"
	case "Best Stem":
		return "Best\nStem"
	case "Oval Fits", "Handlebar Clamp Fit":
		return "Oval\nFits"
	case "Seatpost Ext":
		return "Seatpost\nExt"
	case "BB Drop":
		return "BB\nDrop"
	case "Front Wt":
		return "Front\nWt"
	case "Chainstay":
		return "Chain\nstay"
	case "Saddle-Rear Axle", "Saddle-to-Axle", "Saddle-RC", "Seat-RC":
		return "Saddle-\nRear Axle"
	case "Saddle-HT Drop", "Spare Vert":
		return "Saddle-HT\nDrop"
	default:
		return h
	}
}
