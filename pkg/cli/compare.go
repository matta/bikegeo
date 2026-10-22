package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/model"
	"bikegeo/pkg/scoring"
	"bikegeo/pkg/validator"
)

type optionalBool struct {
	val   bool
	isSet bool
}

func (o *optionalBool) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	o.val = v
	o.isSet = true
	return nil
}

func (o *optionalBool) String() string {
	return strconv.FormatBool(o.val)
}

// IsBoolFlag allows the flag to be passed without "=true" (e.g., just "--save")
func (o *optionalBool) IsBoolFlag() bool { return true }

func (o *optionalBool) Resolve(defaultVal bool) bool {
	if o.isSet {
		return o.val
	}
	return defaultVal
}

func (a *App) runCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	fs.Usage = func() {
		a.printCompareUsage(a.Out)
	}

	var interactive bool
	var transposeFlag bool
	var compactFlag bool
	var noScoreFlag bool
	var scoreDetailFlag bool
	var saveFlag optionalBool

	fs.BoolVar(&interactive, "i", false, "Interactive prompt/response mode to build or edit comparison columns")
	fs.BoolVar(&interactive, "interactive", false, "Interactive prompt/response mode to build or edit comparison columns")
	fs.BoolVar(&transposeFlag, "transpose", false, "Transpose table (Columns as rows, Metrics as columns)")
	fs.BoolVar(&compactFlag, "compact", false, "Compact/transposed table view (alias for --transpose)")
	fs.BoolVar(&noScoreFlag, "no-score", false, "Disable scoring and ranking display")
	fs.BoolVar(&scoreDetailFlag, "score-detail", false, "Display detailed scoring breakdown table")
	fs.Var(&saveFlag, "save", "Persist comparison state to state file (defaults to true if -i/--interactive)")

	boolFlags := map[string]bool{
		"i":            true,
		"interactive":  true,
		"transpose":    true,
		"compact":      true,
		"no-score":     true,
		"score-detail": true,
		"save":         true,
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

	shouldSave := saveFlag.Resolve(interactive)

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
	if len(frames) == 0 {
		fmt.Fprintf(a.Err, "Error: no bike frames found in %s\n", a.DataDir)
		return 1
	}

	state := a.loadOrDefaultComparisonState(frames)

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
					cols, err := ParseComparisonSpecs(frames, specStr)
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
						targetCol, err := ParseComparisonSpec(frames, rmToken)
						if err != nil {
							fmt.Fprintf(a.Err, "Error: %v\n", err)
							return 1
						}
						var kept []model.ComparisonSetup
						for _, c := range state.Columns {
							if c.FrameID == targetCol.FrameID &&
								(targetCol.Size == "" || strings.EqualFold(c.Size, targetCol.Size)) &&
								(targetCol.Fork == "" || forksMatch(c.Fork, targetCol.Fork)) &&
								(targetCol.SagPercent == nil || (c.SagPercent != nil && math.Abs(*c.SagPercent-*targetCol.SagPercent) < 0.1)) {
								continue
							}
							kept = append(kept, c)
						}
						state.Columns = kept
					} else {
						f, sz, err := ResolveFrameByToken(frames, rmToken)
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
				cols, err := ParseComparisonSpecs(frames, specStr)
				if err != nil {
					fmt.Fprintf(a.Err, "Error: %v\n", err)
					return 1
				}
				if len(posArgs) == 1 && len(cols) > 1 {
					hasAllExpansion = true
					f, _, _ := ResolveFrameByToken(frames, strings.Split(specStr, "@")[0])
					singleExpandedFrame = f
				}
				newCols = append(newCols, cols...)
			}
			state.Columns = newCols
			state.RefIndex = 0

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

	if interactive {
		newState, shouldRun, err := a.runInteractiveCompare(frames, state)
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
		if !shouldRun {
			return 0
		}
		state = newState
	}

	if len(state.Columns) == 0 {
		fmt.Fprintf(a.Err, "Error: no columns selected for comparison\n")
		return 1
	}

	// Automatically enable compact/transpose view if more than 4 columns and not explicitly overridden
	if !explicitFlags["transpose"] && !explicitFlags["compact"] && len(state.Columns) > 4 {
		state.Transpose = true
	}

	if state.RefIndex < 0 || state.RefIndex >= len(state.Columns) {
		state.RefIndex = 0
	}

	// Resolve and validate all columns against frame constraints
	rows, normalizedCols, err := resolveAllColumns(frames, state)
	if err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}
	state.Columns = normalizedCols

	// Save the validated comparison state if opted in (or by default in interactive mode)
	if shouldSave {
		_ = a.saveComparisonState(state)
	}

	if err := a.renderComparisonMatrix(rows, state, !noScoreFlag, scoreDetailFlag); err != nil {
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
  bikegeo compare -i | --interactive

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
      Suspension travel in millimeters. Must be within the frame's supported_travel_mm.
    - Rigid fork ("@rigid" or "@0mm"):
      Configures the frame with its declared rigid fork (rigid_a2c_mm).
      Only permitted if the frame supports rigid forks. Rigid forks require 0% sag.
    - Fork preset or configuration ID (e.g. "@enve_mountain_472", "@stock"):
      Selects a known preset fork or frame configuration.
    - Sag percentage (e.g. "@20%", "@sag=25", or "@20"):
      Suspension sag percent. Pure numbers < 60 are treated as sag %; numbers >= 60
      are treated as fork travel in mm. If omitted, uses the frame's default sag.
    - Default fork (omitted fork):
      If no fork is specified, the frame's declared default_fork is used.

Comparison State & Column Management:
  State Persistence:
    - By default, non-interactive compare commands do NOT modify or write the state file.
      The state file (data/last_compare.json or --state-file) is consulted to read rider
      profile and existing columns, but remains untouched unless you opt in with --save.
    - Interactive commands (bikegeo compare -i) persist the updated state by default
      when confirmed with [Enter], unless --save=false is passed.
    - Quitting interactive mode with 'q' or 'quit' cancels and exits immediately without saving.
    - Running 'bikegeo compare' with no arguments replays the saved comparison.

  Full Replacement (Bare Specs):
    - Specifying arguments without '+' or '-' replaces all existing columns.
      Example: bikegeo compare fenrir:L@110mm hayduke:S3@120mm
    - Column 1 becomes the reference bike ([REF]), against which all delta
      offsets (Δ in parens) are calculated.
    - When replacing with a single frame's ':all' sizes, the reference column
      automatically targets the frame's declared default_size (e.g. "S 60cm"),
      centering delta values around the designer baseline.

  Incremental Addition ('+spec'):
    - Prefixing a spec with '+' appends column(s) to the existing comparison state
      without altering existing columns or the reference bike.
      Example: bikegeo compare +seven-expedition-926f:all@120mm

  Incremental Removal ('-spec' or '-col_num'):
    - By column number: '-1', '-2', etc. removes the 1-based column.
    - By frame ID / substring: '-fenrir' or '-fenrir:all' removes all columns of that frame.
    - By frame size: '-fenrir:L' removes all columns of that frame in size L.
    - By specific fork/sag: '-fenrir:L@110mm' removes only that matching setup.

  Reference Column ([REF]):
    - In table output, all numeric deltas in parentheses (e.g. "+12", "-0.8°") are
      calculated relative to the reference column.
    - Can be modified interactively via 'bikegeo compare -i' using the 'ref #' action.

Table Layout & Auto-Transposition:
  - Standard View (Columns): Each bike is a vertical column; metrics are rows.
    Ideal for 2 to 4 bikes.
  - Transposed View (Rows): Each bike is a row; metrics are columns.
    Enabled with --transpose or --compact.
  - Automatic Transposition: When a comparison has more than 4 columns (such as
    expanding ':all'), the table automatically renders in transposed mode unless
    explicitly disabled with --transpose=false or --compact=false.

Scoring & Ranking:
  - When scoring is enabled (default), setups are scored against target biomechanical
    criteria (setback, AETT, spare vertical travel, chainstay, BB drop, HTA, front wt %).
  - Displayed rows are sorted descending by total score, placing the top-match #1 at the top.
  - Explanations section highlights Biggest Losses vs. Ideal and Head-to-Head Deciding Factors.
  - Use --score-detail to view the full criteria breakdown table.
  - Use --no-score to disable scoring and view columns in their original configuration order.

Flags:
  -save[=bool], --save[=bool]  Persist comparison state to state file (defaults to true if -i/--interactive)
  -compact, --compact       Render compact/transposed table (Columns as rows, Metrics as columns)
  -transpose, --transpose   Alias for --compact
  -i, --interactive         Launch interactive prompt/response comparison builder
  -score-detail             Display detailed scoring breakdown table for each criterion
  -no-score                 Disable scoring and ranking display

Global Flags:
  --data-dir <dir>          Path to bike JSON directory (default: "data/bikes")
  --state-file <file>       Path to saved comparison state (default: "data/last_compare.json")
  -f, --format <format>     Output format: table (default), json, md, csv
  --json                    Shorthand for --format json

Examples:
  # Replay the most recently saved comparison:
  bikegeo compare

  # Perform a one-off comparison without altering your saved comparison state:
  bikegeo compare seven-expedition-926f:all@120mm

  # Compare bikes and persist the result as your new saved comparison:
  bikegeo compare --save fenrir:L@110mm@20% hayduke:S3@120mm@25%

  # Add all sizes of Seven Expedition to your existing comparison against your reference bike:
  bikegeo compare +seven-expedition-926f:all@120mm

  # Compare a single frame across multiple fork travels and rigid:
  bikegeo compare fenrir:L@100mm@20% fenrir:L@120mm@20% fenrir:L@rigid

  # Compare multiple different frames and forks:
  bikegeo compare fenrir:L@110mm@20% hayduke:S3@120mm@25% krampus:XL@120mm

  # Remove column 3 from the active comparison:
  bikegeo compare -3

  # Remove all columns of a specific frame:
  bikegeo compare -seven-expedition-926f

  # Remove a specific fork setup when multiple setups of the same frame exist:
  bikegeo compare -fenrir:L@110mm

  # View detailed scoring loss breakdown table:
  bikegeo compare --score-detail

  # Interactively edit and persist comparison (persists on [Enter], cancels on 'q'):
  bikegeo compare -i

  # Launch interactive comparison builder without persisting on exit:
  bikegeo compare -i --save=false
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

	setup := model.ComparisonSetup{
		FrameID: frame.ID,
		Size:    implicitSize,
	}

	for _, rawMod := range parts[1:] {
		mod := strings.TrimSpace(rawMod)
		if mod == "" {
			continue
		}
		low := strings.ToLower(mod)

		// 0. "all" modifier representing all sizes
		if low == "all" {
			setup.Size = "all"
			continue
		}

		// 1. Sag modifier (e.g. "20%", "sag=25")
		if strings.HasSuffix(low, "%") || strings.HasPrefix(low, "sag=") {
			clean := strings.TrimSuffix(strings.TrimPrefix(low, "sag="), "%")
			val, err := strconv.ParseFloat(clean, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid sag modifier %q in %q", mod, spec)
			}
			setup.SagPercent = &val
			continue
		}

		// 2. Rigid fork modifier
		if low == "rigid" || low == "0mm" || strings.HasPrefix(low, "rigid") {
			setup.Fork = "rigid"
			continue
		}

		// 3. Exact match on one of the frame's sizes
		matchedSize := ""
		for _, sz := range frame.GetSizes() {
			if strings.EqualFold(sz, mod) {
				matchedSize = sz
				break
			}
		}
		if matchedSize != "" {
			setup.Size = matchedSize
			continue
		}

		// 4. Fork travel modifier ending with "mm" (e.g. "120mm", "150mm")
		if strings.HasSuffix(low, "mm") {
			setup.Fork = low
			continue
		}

		// 5. Pure numeric modifier: >= 60 is fork travel (mm); < 60 is sag (%)
		if val, err := strconv.ParseFloat(low, 64); err == nil {
			if val >= 60 {
				setup.Fork = fmt.Sprintf("%.0fmm", val)
			} else {
				setup.SagPercent = &val
			}
			continue
		}

		// 6. Fork preset ID or frame configuration ID
		if _, ok := canonicalizer.GetForkPreset(mod); ok {
			setup.Fork = mod
			continue
		}
		matchedCfg := false
		for _, cfg := range frame.Configurations {
			if strings.EqualFold(cfg.ID, mod) {
				setup.Fork = cfg.ID
				matchedCfg = true
				break
			}
		}
		if matchedCfg {
			continue
		}

		// If modifier looks like a size token (e.g. XS, S, M, L, XL, XXL), let size validation report it
		if isLikelySizeToken(low) {
			setup.Size = mod
			continue
		}

		return nil, fmt.Errorf(
			"unrecognized modifier %q for frame \"%s %s\" (sizes: %s; supported forks: %s; sag: e.g. 20%%)",
			mod, frame.Manufacturer, frame.Model,
			strings.Join(frame.GetSizes(), ", "),
			strings.Join(frame.GetSupportedForkOptions(), ", "),
		)
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

func (a *App) loadOrDefaultComparisonState(frames []model.FrameModel) model.SavedComparisonState {
	if data, err := os.ReadFile(a.StatePath); err == nil {
		var saved model.SavedComparisonState
		if err := json.Unmarshal(data, &saved); err == nil && len(saved.Columns) > 0 {
			var validCols []model.ComparisonSetup
			for _, col := range saved.Columns {
				if f, _, err := ResolveFrameByToken(frames, col.FrameID); err == nil && validator.ValidateFrame(*f).IsValid {
					validCols = append(validCols, col)
				}
			}
			if len(validCols) > 0 {
				saved.Columns = validCols
				if saved.RefIndex < 0 || saved.RefIndex >= len(saved.Columns) {
					saved.RefIndex = 0
				}
				saved.NormalizeRiderProfile()
				return saved
			}
		}
	}

	// Default initial state: up to 3 valid frames with their default size and default fork
	var cols []model.ComparisonSetup
	for _, f := range frames {
		if !validator.ValidateFrame(f).IsValid {
			continue
		}
		cols = append(cols, model.ComparisonSetup{
			FrameID: f.ID,
			Size:    f.GetDefaultSize(),
			Fork:    f.GetDefaultForkSpecString(),
		})
		if len(cols) >= 3 {
			break
		}
	}

	return model.SavedComparisonState{
		RiderProfile: model.DefaultRiderProfile(),
		Transpose:    false,
		RefIndex:     0,
		Columns:      cols,
	}
}

func (a *App) saveComparisonState(state model.SavedComparisonState) error {
	if a.StatePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(a.StatePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.StatePath, data, 0644)
}

func (a *App) runInteractiveCompare(frames []model.FrameModel, state model.SavedComparisonState) (model.SavedComparisonState, bool, error) {
	scanner := bufio.NewScanner(a.In)

	for {
		fmt.Fprintf(a.Out, "\nSaved Comparison (Saddle Height: %.0fmm):\n", state.SaddleHeightMm)
		if len(state.Columns) == 0 {
			fmt.Fprintln(a.Out, "  (No columns currently selected)")
		} else {
			for i, col := range state.Columns {
				refTag := "     "
				if i == state.RefIndex {
					refTag = "[REF]"
				}
				frame, _, err := ResolveFrameByToken(frames, col.FrameID)
				if err != nil {
					fmt.Fprintf(a.Out, "  [%d] %s %s (%s)\n", i+1, refTag, col.FrameID, col.Size)
					continue
				}
				row, err := canonicalizer.ResolveFrameForkSetup(*frame, col, state.RiderProfile)
				if err != nil {
					fmt.Fprintf(a.Out, "  [%d] %s %s %s (%s) - %v\n", i+1, refTag, frame.Manufacturer, frame.Model, col.Size, err)
					continue
				}
				sagInfo := "0% sag (rigid)"
				if row.SetupFork.TravelMm > 0 {
					sagInfo = fmt.Sprintf("%.0f%% sag", row.SagPercent)
				}
				fmt.Fprintf(
					a.Out,
					"  [%d] %s %-26s | %-28s | %-14s -> %.1fmm eff A2C\n",
					i+1,
					refTag,
					fmt.Sprintf("%s %s (%s)", frame.Manufacturer, frame.Model, row.Setup.Size),
					row.SetupFork.Name,
					sagInfo,
					row.EffectiveA2c,
				)
			}
		}

		fmt.Fprint(a.Out, "\nAction ([a]dd, [e]dit #, [d]elete #, [r]ef #, [s]addle, [c]lear, [Enter] to run, or [q]uit): ")
		if !scanner.Scan() {
			return state, false, nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.EqualFold(line, "run") {
			return state, true, nil
		}
		if strings.EqualFold(line, "q") || strings.EqualFold(line, "quit") {
			return state, false, nil
		}

		fields := strings.Fields(line)
		cmd := strings.ToLower(fields[0])
		arg := ""
		if len(fields) > 1 {
			arg = strings.Join(fields[1:], " ")
		}

		switch {
		case cmd == "a" || cmd == "add" || strings.HasPrefix(cmd, "+"):
			specArg := arg
			if strings.HasPrefix(cmd, "+") {
				specArg = strings.TrimPrefix(line, "+")
			}
			if specArg != "" && (strings.Contains(specArg, "@") || strings.Contains(strings.ToLower(specArg), ":all")) {
				cols, err := ParseComparisonSpecs(frames, specArg)
				if err != nil {
					fmt.Fprintf(a.Out, "  ! %v\n", err)
					continue
				}
				for _, col := range cols {
					frame, _, _ := ResolveFrameByToken(frames, col.FrameID)
					if _, err := canonicalizer.ResolveFrameForkSetup(*frame, col, state.RiderProfile); err != nil {
						fmt.Fprintf(a.Out, "  ! %v\n", err)
						continue
					}
					state.Columns = append(state.Columns, col)
				}
			} else {
				col, err := a.promptColumnSetup(scanner, frames, specArg, nil, state)
				if err != nil {
					fmt.Fprintf(a.Out, "  ! %v\n", err)
					continue
				}
				state.Columns = append(state.Columns, col)
			}

		case cmd == "e" || cmd == "edit":
			idx, err := promptIndex(scanner, a.Out, arg, len(state.Columns), "Edit column #")
			if err != nil {
				fmt.Fprintf(a.Out, "  ! %v\n", err)
				continue
			}
			existing := state.Columns[idx]
			updated, err := a.promptColumnSetup(scanner, frames, existing.FrameID, &existing, state)
			if err != nil {
				fmt.Fprintf(a.Out, "  ! %v\n", err)
				continue
			}
			state.Columns[idx] = updated

		case cmd == "d" || cmd == "del" || cmd == "delete" || strings.HasPrefix(cmd, "-"):
			rmArg := arg
			if strings.HasPrefix(cmd, "-") {
				rmArg = strings.TrimPrefix(cmd, "-")
			}
			idx, err := promptIndex(scanner, a.Out, rmArg, len(state.Columns), "Delete column #")
			if err != nil {
				fmt.Fprintf(a.Out, "  ! %v\n", err)
				continue
			}
			state.Columns = append(state.Columns[:idx], state.Columns[idx+1:]...)
			if state.RefIndex >= len(state.Columns) {
				state.RefIndex = 0
			}

		case cmd == "r" || cmd == "ref":
			idx, err := promptIndex(scanner, a.Out, arg, len(state.Columns), "Reference column #")
			if err != nil {
				fmt.Fprintf(a.Out, "  ! %v\n", err)
				continue
			}
			state.RefIndex = idx

		case cmd == "s" || cmd == "saddle" || cmd == "seat":
			valStr := arg
			if valStr == "" {
				fmt.Fprintf(a.Out, "  Rider Saddle Height in mm [%.0f]: ", state.SaddleHeightMm)
				if scanner.Scan() {
					valStr = strings.TrimSpace(scanner.Text())
				}
			}
			if valStr != "" {
				if v, err := strconv.ParseFloat(valStr, 64); err == nil && v >= 400 && v <= 1100 {
					state.SaddleHeightMm = v
				} else {
					fmt.Fprintf(a.Out, "  ! Invalid saddle height %q\n", valStr)
				}
			}

		case cmd == "c" || cmd == "clear":
			state.Columns = nil
			state.RefIndex = 0
			col, err := a.promptColumnSetup(scanner, frames, "", nil, state)
			if err != nil {
				fmt.Fprintf(a.Out, "  ! %v\n", err)
				continue
			}
			state.Columns = append(state.Columns, col)

		default:
			fmt.Fprintf(a.Out, "  ! Unknown action %q (use a, e, d, r, s, c, or Enter)\n", line)
		}
	}
}

func promptIndex(scanner *bufio.Scanner, out interface{ Write([]byte) (int, error) }, arg string, max int, label string) (int, error) {
	if max == 0 {
		return 0, fmt.Errorf("no columns in comparison")
	}
	valStr := strings.TrimSpace(arg)
	if valStr == "" {
		fmt.Fprintf(out, "  %s (1-%d): ", label, max)
		if scanner.Scan() {
			valStr = strings.TrimSpace(scanner.Text())
		}
	}
	idx, err := strconv.Atoi(valStr)
	if err != nil || idx < 1 || idx > max {
		return 0, fmt.Errorf("invalid column number %q (expected 1..%d)", valStr, max)
	}
	return idx - 1, nil
}

func (a *App) promptColumnSetup(
	scanner *bufio.Scanner,
	frames []model.FrameModel,
	initialFrameToken string,
	existing *model.ComparisonSetup,
	state model.SavedComparisonState,
) (model.ComparisonSetup, error) {
	var selectedFrame *model.FrameModel

	if initialFrameToken != "" {
		if idx, err := strconv.Atoi(initialFrameToken); err == nil && idx >= 1 && idx <= len(frames) {
			selectedFrame = &frames[idx-1]
		} else if f, _, err := ResolveFrameByToken(frames, initialFrameToken); err == nil {
			selectedFrame = f
		}
	}

	if selectedFrame == nil {
		fmt.Fprintln(a.Out, "\n  Available Frames:")
		for i, f := range frames {
			fmt.Fprintf(
				a.Out,
				"    [%d] %-28s | Sizes: %-12s | Forks: %s\n",
				i+1,
				fmt.Sprintf("%s %s", f.Manufacturer, f.Model),
				strings.Join(f.GetSizes(), ", "),
				strings.Join(f.GetSupportedForkOptions(), ", "),
			)
		}
		defHint := ""
		if existing != nil {
			defHint = fmt.Sprintf(" [%s]", existing.FrameID)
		}
		fmt.Fprintf(a.Out, "  Select frame (1-%d or name)%s: ", len(frames), defHint)
		if !scanner.Scan() {
			return model.ComparisonSetup{}, fmt.Errorf("cancelled")
		}
		inp := strings.TrimSpace(scanner.Text())
		if inp == "" && existing != nil {
			inp = existing.FrameID
		}
		if idx, err := strconv.Atoi(inp); err == nil && idx >= 1 && idx <= len(frames) {
			selectedFrame = &frames[idx-1]
		} else {
			f, _, err := ResolveFrameByToken(frames, inp)
			if err != nil {
				return model.ComparisonSetup{}, err
			}
			selectedFrame = f
		}
	}

	// 1. Size prompt (skipped if frame only has 1 size)
	sizes := selectedFrame.GetSizes()
	chosenSize := selectedFrame.GetDefaultSize()
	if existing != nil && existing.FrameID == selectedFrame.ID && existing.Size != "" {
		chosenSize = existing.Size
	}

	if len(sizes) == 1 {
		chosenSize = sizes[0]
		fmt.Fprintf(a.Out, "  Size: %s (only size)\n", chosenSize)
	} else {
		fmt.Fprintf(a.Out, "  Size [%s] (%s): ", chosenSize, strings.Join(sizes, ", "))
		if scanner.Scan() {
			inp := strings.TrimSpace(scanner.Text())
			if inp != "" {
				chosenSize = inp
			}
		}
	}

	// 2. Fork prompt (skipped if frame only supports 1 fixed fork)
	chosenFork := selectedFrame.GetDefaultForkSpecString()
	if existing != nil && existing.FrameID == selectedFrame.ID && existing.Fork != "" {
		chosenFork = existing.Fork
	}

	if selectedFrame.IsFixedFork() {
		fmt.Fprintf(a.Out, "  Fork: %s (fixed for this frame)\n", strings.Join(selectedFrame.GetSupportedForkOptions(), ""))
	} else {
		fmt.Fprintf(
			a.Out,
			"  Fork [%s] (%s): ",
			chosenFork,
			strings.Join(selectedFrame.GetSupportedForkOptions(), ", "),
		)
		if scanner.Scan() {
			inp := strings.TrimSpace(scanner.Text())
			if inp != "" {
				chosenFork = inp
			}
		}
	}

	// 3. Sag % prompt (skipped automatically if chosen fork is rigid!)
	lowFork := strings.ToLower(strings.TrimSpace(chosenFork))
	isRigid := lowFork == "rigid" || lowFork == "0mm" || strings.HasPrefix(lowFork, "rigid") || selectedFrame.IsFixedRigidOnly()
	var sagPtr *float64
	if !isRigid {
		defSag := 20.0
		if existing != nil && existing.SagPercent != nil && *existing.SagPercent > 0 {
			defSag = *existing.SagPercent
		}
		fmt.Fprintf(a.Out, "  Sag %% [%.0f]: ", defSag)
		if scanner.Scan() {
			inp := strings.TrimSpace(strings.TrimSuffix(scanner.Text(), "%"))
			if inp != "" {
				val, err := strconv.ParseFloat(inp, 64)
				if err != nil {
					return model.ComparisonSetup{}, fmt.Errorf("invalid sag percentage %q", inp)
				}
				defSag = val
			}
		}
		sagPtr = &defSag
	}

	candidate := model.ComparisonSetup{
		FrameID:    selectedFrame.ID,
		Size:       chosenSize,
		Fork:       chosenFork,
		SagPercent: sagPtr,
	}

	// Validate against frame constraints immediately
	row, err := canonicalizer.ResolveFrameForkSetup(*selectedFrame, candidate, state.RiderProfile)
	if err != nil {
		return model.ComparisonSetup{}, err
	}

	return row.Setup, nil
}

func formatDelta(delta float64, unit string) string {
	if math.Abs(delta) < 0.05 {
		return fmt.Sprintf("0.0%s", unit)
	}
	if delta > 0 {
		return fmt.Sprintf("+%.1f%s", delta, unit)
	}
	return fmt.Sprintf("%.1f%s", delta, unit)
}

func formatDeltaMm(delta float64, unit string) string {
	rounded := math.Round(delta)
	if math.Abs(rounded) == 0 {
		return fmt.Sprintf("0%s", unit)
	}
	if rounded > 0 {
		return fmt.Sprintf("+%.0f%s", rounded, unit)
	}
	return fmt.Sprintf("%.0f%s", rounded, unit)
}

func formatDeltaAngle(delta float64, unit string) string {
	if math.Abs(delta) < 0.05 {
		return fmt.Sprintf("0.0%s", unit)
	}
	if delta > 0 {
		return fmt.Sprintf("+%.1f%s", delta, unit)
	}
	return fmt.Sprintf("%.1f%s", delta, unit)
}

type scoredRowItem struct {
	origIdx     int
	row         model.CanonicalComparisonRow
	score       scoring.Result
	displayName string
}

type ScoredColumnJSON struct {
	model.CanonicalComparisonRow
	Score *scoring.Result `json:"score,omitempty"`
}

type CompareExplanationsJSON struct {
	Losses     []string `json:"losses"`
	HeadToHead []string `json:"head_to_head"`
}

type CompareJSONPayload struct {
	SaddleHeightMm float64                  `json:"saddle_height_mm"`
	RefIndex       int                      `json:"ref_index"`
	Columns        []ScoredColumnJSON       `json:"columns"`
	Explanations   *CompareExplanationsJSON `json:"explanations,omitempty"`
}

func (a *App) renderComparisonMatrix(
	rows []model.CanonicalComparisonRow,
	state model.SavedComparisonState,
	scoringEnabled bool,
	scoreDetail bool,
) error {
	if len(rows) == 0 {
		return nil
	}

	refIdx := state.RefIndex
	if refIdx < 0 || refIdx >= len(rows) {
		refIdx = 0
	}
	refRow := rows[refIdx]

	var scoreResults []scoring.Result
	if scoringEnabled {
		analyses := make([]model.BikeFitAnalysis, len(rows))
		for i, r := range rows {
			analyses[i] = r.Canonical
		}
		scoreResults = scoring.ScoreAll(analyses, scoring.DefaultCriteria)
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
	if scoringEnabled && len(rows) > 0 {
		expl = scoring.Explain(scoredFramesForExplain, scoring.DefaultCriteria)
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

		// Sort displayed rows/columns descending by score
		sort.SliceStable(scoredItems, func(i, j int) bool {
			return scoredItems[i].score.Total > scoredItems[j].score.Total
		})
	}

	title := "Apples-to-Apples Geometry Comparison Matrix"
	subtitle := fmt.Sprintf(
		"Saddle Height: %.0fmm | Modeled Back Angle: %.1f° | Reference: [%d] %s (*max stay position; Δ in parens)",
		state.SaddleHeightMm, refRow.Canonical.BackAngleDeg, refIdx+1,
		displayNames[refIdx],
	)

	jsonCols := make([]ScoredColumnJSON, len(rows))
	for i, r := range rows {
		jsonCols[i].CanonicalComparisonRow = r
		if scoringEnabled {
			resCopy := scoreResults[i]
			jsonCols[i].Score = &resCopy
		}
	}
	var explJSON *CompareExplanationsJSON
	if scoringEnabled && len(rows) > 0 {
		explJSON = &CompareExplanationsJSON{
			Losses:     expl.Losses,
			HeadToHead: expl.HeadToHead,
		}
	}
	jsonPayload := CompareJSONPayload{
		SaddleHeightMm: state.SaddleHeightMm,
		RefIndex:       refIdx,
		Columns:        jsonCols,
		Explanations:   explJSON,
	}

	// Prepare detail table if requested
	var detailHeaders []string
	var detailRows [][]string
	if scoreDetail && scoringEnabled {
		detailHeaders = []string{"Rank", "#", "Frame", "Setback", "AETT", "Spare Vert", "Chainstay", "BB Drop", "HTA", "Front Wt", "Total"}
		for _, item := range scoredItems {
			subScores := make(map[string]float64)
			pointsLost := make(map[string]float64)
			for _, c := range item.score.Criteria {
				subScores[c.ID] = c.SubScore
				pointsLost[c.ID] = c.PointsLost
			}
			formatSubScoreCell := func(id string) string {
				sub := math.Round(subScores[id])
				lost := math.Round(pointsLost[id])
				if lost >= 1 {
					return fmt.Sprintf("%.0f (-%.0f)", sub, lost)
				}
				return fmt.Sprintf("%.0f", sub)
			}
			formatTotalCell := func(total float64) string {
				rndTotal := math.Round(total)
				totalLost := math.Round(100.0 - total)
				if totalLost >= 1 {
					return fmt.Sprintf("%.0f (-%.0f)", rndTotal, totalLost)
				}
				return fmt.Sprintf("%.0f", rndTotal)
			}
			detailRows = append(detailRows, []string{
				fmt.Sprintf("%d", item.score.Rank),
				fmt.Sprintf("%d", item.origIdx+1),
				item.displayName,
				formatSubScoreCell("setback"),
				formatSubScoreCell("aett"),
				formatSubScoreCell("sparevert"),
				formatSubScoreCell("chainstay"),
				formatSubScoreCell("bbdrop"),
				formatSubScoreCell("hta"),
				formatSubScoreCell("frontwt"),
				formatTotalCell(item.score.Total),
			})
		}
	}

	primaryFooters := footers
	if scoreDetail && scoringEnabled && a.Format != FormatJSON && a.Format != FormatCSV {
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
		headers = append(headers,
			"Fork & Eff A2C",
			"Reach",
			"Stack",
			"HTA (°)",
			"STA (°)",
			"BB Drop",
			"Chainstay",
			"AETT",
			"Setback",
			"Spare Vert",
			"Front Wt %",
		)

		var tableRows [][]string
		for _, item := range scoredItems {
			r := item.row
			i := item.origIdx

			idxStr := fmt.Sprintf("%d", i+1)
			if i == refIdx {
				idxStr += " [REF]"
			}
			setupStr := fmt.Sprintf("Rigid (%.0fmm)", math.Round(r.EffectiveA2c))
			if r.SetupFork.TravelMm > 0 {
				setupStr = fmt.Sprintf("%.0fmm @ %.0f%% (%.0fmm)", r.SetupFork.TravelMm, r.SagPercent, math.Round(r.EffectiveA2c))
			}

			reachStr := fmt.Sprintf("%.0f", math.Round(r.Canonical.ActiveReach))
			stackStr := fmt.Sprintf("%.0f", math.Round(r.Canonical.ActiveStack))
			htaStr := fmt.Sprintf("%.1f", r.Canonical.ActiveHta)
			staStr := fmt.Sprintf("%.1f", r.Canonical.ActiveSta)
			bbDropStr := fmt.Sprintf("%.0f", math.Round(r.Canonical.ActiveBbDrop))
			aettStr := fmt.Sprintf("%.0f", math.Round(r.Canonical.Aett))
			setbackStr := fmt.Sprintf("%.0f", math.Round(r.Canonical.ActualSetback))
			spareVertStr := fmt.Sprintf("%.0f", math.Round(r.Canonical.SpareVerticalStack))

			star := ""
			if r.Bike.HasAdjustableChainstay() {
				star = "*"
			}
			csStr := fmt.Sprintf("%.0f%s", math.Round(r.Canonical.ActiveChainstay), star)

			if i != refIdx {
				reachStr = fmt.Sprintf("%.0f (%s)", math.Round(r.Canonical.ActiveReach), formatDeltaMm(r.Canonical.ActiveReach-refRow.Canonical.ActiveReach, ""))
				stackStr = fmt.Sprintf("%.0f (%s)", math.Round(r.Canonical.ActiveStack), formatDeltaMm(r.Canonical.ActiveStack-refRow.Canonical.ActiveStack, ""))
				htaStr = fmt.Sprintf("%.1f (%s)", r.Canonical.ActiveHta, formatDeltaAngle(r.Canonical.ActiveHta-refRow.Canonical.ActiveHta, ""))
				staStr = fmt.Sprintf("%.1f (%s)", r.Canonical.ActiveSta, formatDeltaAngle(r.Canonical.ActiveSta-refRow.Canonical.ActiveSta, ""))
				bbDropStr = fmt.Sprintf("%.0f (%s)", math.Round(r.Canonical.ActiveBbDrop), formatDeltaMm(r.Canonical.ActiveBbDrop-refRow.Canonical.ActiveBbDrop, ""))
				deltaCS := r.Canonical.ActiveChainstay - refRow.Canonical.ActiveChainstay
				csStr = fmt.Sprintf("%.0f%s (%s)", math.Round(r.Canonical.ActiveChainstay), star, formatDeltaMm(deltaCS, ""))
				aettStr = fmt.Sprintf("%.0f (%s)", math.Round(r.Canonical.Aett), formatDeltaMm(r.Canonical.Aett-refRow.Canonical.Aett, ""))
				setbackStr = fmt.Sprintf("%.0f (%s)", math.Round(r.Canonical.ActualSetback), formatDeltaMm(r.Canonical.ActualSetback-refRow.Canonical.ActualSetback, ""))
			}

			weightStr := fmt.Sprintf("%.1f%% F", r.Canonical.WeightDistFrontPct)
			if i != refIdx {
				deltaF := r.Canonical.WeightDistFrontPct - refRow.Canonical.WeightDistFrontPct
				weightStr = fmt.Sprintf("%.1f%% F (%s)", r.Canonical.WeightDistFrontPct, formatDelta(deltaF, "%"))
			}

			rowCells := []string{
				idxStr,
				item.displayName,
			}
			if scoringEnabled {
				scoreStr := fmt.Sprintf("%.0f (#%d)", math.Round(item.score.Total), item.score.Rank)
				rowCells = append(rowCells, scoreStr)
			}
			rowCells = append(rowCells,
				setupStr,
				reachStr,
				stackStr,
				htaStr,
				staStr,
				bbDropStr,
				csStr,
				aettStr,
				setbackStr,
				spareVertStr,
				weightStr,
			)

			tableRows = append(tableRows, rowCells)
		}

		if err := RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, tableRows, jsonPayload, primaryFooters); err != nil {
			return err
		}
	} else {
		// Default orientation: Metrics as rows, Bike Setups as columns
		headers := make([]string, len(scoredItems)+1)
		headers[0] = "Metric"
		for colIdx, item := range scoredItems {
			r := item.row
			i := item.origIdx
			refBadge := ""
			if i == refIdx {
				refBadge = " [REF]"
			}
			forkLine := fmt.Sprintf("Rigid (%.0fmm A2C)", r.SetupFork.A2c)
			if r.SetupFork.TravelMm > 0 {
				forkLine = fmt.Sprintf("%.0fmm @ %.0f%% sag", r.SetupFork.TravelMm, r.SagPercent)
			}
			headers[colIdx+1] = fmt.Sprintf("[%d] %s %s (%s)%s\n%s", i+1, r.Bike.Manufacturer, r.Bike.Model, r.Bike.Size, refBadge, forkLine)
		}

		buildRow := func(metric string, cellFn func(colIdx int, item scoredRowItem) string) []string {
			out := make([]string, len(scoredItems)+1)
			out[0] = metric
			for colIdx, item := range scoredItems {
				out[colIdx+1] = cellFn(colIdx, item)
			}
			return out
		}

		var tableRows [][]string

		// Score row if enabled
		if scoringEnabled {
			tableRows = append(tableRows, buildRow("Score", func(colIdx int, item scoredRowItem) string {
				return fmt.Sprintf("%.0f (#%d)", math.Round(item.score.Total), item.score.Rank)
			}))
		}

		// 1. Setup Fork & Effective A2C
		tableRows = append(tableRows, buildRow("Fork & Eff. A2C", func(colIdx int, item scoredRowItem) string {
			r := item.row
			if r.SetupFork.TravelMm > 0 {
				return fmt.Sprintf("%.0fmm @ %.0f%% sag -> %.0fmm A2C", r.SetupFork.TravelMm, r.SagPercent, math.Round(r.EffectiveA2c))
			}
			return fmt.Sprintf("Rigid (%.0fmm A2C) -> %.0fmm A2C", r.SetupFork.A2c, math.Round(r.EffectiveA2c))
		}))

		// 2. Reach
		tableRows = append(tableRows, buildRow("Reach", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.0f mm", math.Round(r.Canonical.ActiveReach))
			if i != refIdx {
				mainLine = fmt.Sprintf("%.0f mm (%s)", math.Round(r.Canonical.ActiveReach), formatDeltaMm(r.Canonical.ActiveReach-refRow.Canonical.ActiveReach, " mm"))
			}
			return mainLine
		}))

		// 3. Stack
		tableRows = append(tableRows, buildRow("Stack", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.0f mm", math.Round(r.Canonical.ActiveStack))
			if i != refIdx {
				mainLine = fmt.Sprintf("%.0f mm (%s)", math.Round(r.Canonical.ActiveStack), formatDeltaMm(r.Canonical.ActiveStack-refRow.Canonical.ActiveStack, " mm"))
			}
			return mainLine
		}))

		// 4. HTA
		tableRows = append(tableRows, buildRow("HTA (°)", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.1f", r.Canonical.ActiveHta)
			if i != refIdx {
				mainLine = fmt.Sprintf("%.1f (%s)", r.Canonical.ActiveHta, formatDeltaAngle(r.Canonical.ActiveHta-refRow.Canonical.ActiveHta, ""))
			}
			return mainLine
		}))

		// 5. STA
		tableRows = append(tableRows, buildRow("STA (°)", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.1f", r.Canonical.ActiveSta)
			if i != refIdx {
				mainLine = fmt.Sprintf("%.1f (%s)", r.Canonical.ActiveSta, formatDeltaAngle(r.Canonical.ActiveSta-refRow.Canonical.ActiveSta, ""))
			}
			return mainLine
		}))

		// 6. BB Drop
		tableRows = append(tableRows, buildRow("BB Drop", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.0f mm", math.Round(r.Canonical.ActiveBbDrop))
			if i != refIdx {
				mainLine = fmt.Sprintf("%.0f mm (%s)", math.Round(r.Canonical.ActiveBbDrop), formatDeltaMm(r.Canonical.ActiveBbDrop-refRow.Canonical.ActiveBbDrop, " mm"))
			}
			return mainLine
		}))

		// 7. Chainstay
		tableRows = append(tableRows, buildRow("Chainstay", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			star := ""
			if r.Bike.HasAdjustableChainstay() {
				star = "*"
			}
			mainLine := fmt.Sprintf("%.0f mm%s", math.Round(r.Canonical.ActiveChainstay), star)
			if i != refIdx {
				deltaCS := r.Canonical.ActiveChainstay - refRow.Canonical.ActiveChainstay
				mainLine = fmt.Sprintf("%.0f mm%s (%s)", math.Round(r.Canonical.ActiveChainstay), star, formatDeltaMm(deltaCS, " mm"))
			}
			return mainLine
		}))

		// 8. AETT
		tableRows = append(tableRows, buildRow(fmt.Sprintf("AETT @ %.0fmm", state.SaddleHeightMm), func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.0f mm", math.Round(r.Canonical.Aett))
			if i != refIdx {
				mainLine = fmt.Sprintf("%.0f mm (%s)", math.Round(r.Canonical.Aett), formatDeltaMm(r.Canonical.Aett-refRow.Canonical.Aett, " mm"))
			}
			return mainLine
		}))

		// 9. Actual Setback
		tableRows = append(tableRows, buildRow(fmt.Sprintf("Actual Setback @ %.0fmm", state.SaddleHeightMm), func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.0f mm", math.Round(r.Canonical.ActualSetback))
			if i != refIdx {
				mainLine = fmt.Sprintf("%.0f mm (%s)", math.Round(r.Canonical.ActualSetback), formatDeltaMm(r.Canonical.ActualSetback-refRow.Canonical.ActualSetback, " mm"))
			}
			return mainLine
		}))

		// 10. Spare Steerer (Vertical)
		tableRows = append(tableRows, buildRow("Spare Steerer (Vert)", func(colIdx int, item scoredRowItem) string {
			return fmt.Sprintf("%.0f mm", math.Round(item.row.Canonical.SpareVerticalStack))
		}))

		// 11. Spare Steerer (Axial)
		tableRows = append(tableRows, buildRow("Spare Steerer (Axial)", func(colIdx int, item scoredRowItem) string {
			return fmt.Sprintf("%.0f mm", math.Round(item.row.Canonical.SpareAxialSteerer))
		}))

		// 12. Weight Distribution (Front / Rear)
		tableRows = append(tableRows, buildRow("Weight Dist (F/R)", func(colIdx int, item scoredRowItem) string {
			r := item.row
			i := item.origIdx
			mainLine := fmt.Sprintf("%.1f%% / %.1f%%", r.Canonical.WeightDistFrontPct, r.Canonical.WeightDistRearPct)
			if i != refIdx {
				deltaF := r.Canonical.WeightDistFrontPct - refRow.Canonical.WeightDistFrontPct
				mainLine = fmt.Sprintf("%.1f%% / %.1f%% (%s F)", r.Canonical.WeightDistFrontPct, r.Canonical.WeightDistRearPct, formatDelta(deltaF, "%"))
			}
			return mainLine
		}))

		if err := RenderOutputWithFooter(a.Out, a.Format, title, subtitle, headers, tableRows, jsonPayload, primaryFooters); err != nil {
			return err
		}
	}

	if scoreDetail && scoringEnabled && a.Format != FormatJSON && a.Format != FormatCSV {
		fmt.Fprintln(a.Out)
		detailTitle := "Scoring Breakdown"
		detailSubtitle := "Sub-scores (-200..+100) per criterion (points lost on total score in parens). Total = weighted average (weights: Setback 15, AETT 10, Spare Vert 5, Chainstay 4, BB Drop 3, HTA 2, Front Wt 1)"
		if err := RenderOutputWithFooter(a.Out, a.Format, detailTitle, detailSubtitle, detailHeaders, detailRows, nil, footers); err != nil {
			return err
		}
	}

	return nil
}

func forksMatch(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	normA := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(a)), "mm")
	normB := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(b)), "mm")
	return normA == normB
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
			ft := getForkTypeLabel(rows[idx])
			forkTypeCounts[ft]++
		}

		// Count fork + sag within this duplicate group in case identical setups exist
		forkSagCounts := make(map[string]int)
		for _, idx := range indices {
			fs := getForkSagLabel(rows[idx])
			forkSagCounts[fs]++
		}

		for _, idx := range indices {
			r := rows[idx]
			ft := getForkTypeLabel(r)
			fs := getForkSagLabel(r)

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

func getForkTypeLabel(r model.CanonicalComparisonRow) string {
	if r.SetupFork.TravelMm <= 0 || strings.EqualFold(r.Setup.Fork, "rigid") {
		return "rigid"
	}
	return fmt.Sprintf("%.0fmm", r.SetupFork.TravelMm)
}

func getForkSagLabel(r model.CanonicalComparisonRow) string {
	if r.SetupFork.TravelMm <= 0 || strings.EqualFold(r.Setup.Fork, "rigid") {
		return "rigid"
	}
	return fmt.Sprintf("%.0fmm @ %.0f%%", r.SetupFork.TravelMm, r.SagPercent)
}

