package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bikegeo/pkg/canonicalizer"
	"bikegeo/pkg/model"
	"bikegeo/pkg/validator"
)

func (a *App) runCompare(args []string) int {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(a.Err)

	var interactive bool
	var transposeFlag bool
	var compactFlag bool

	fs.BoolVar(&interactive, "i", false, "Interactive prompt/response mode to build or edit comparison columns")
	fs.BoolVar(&interactive, "interactive", false, "Interactive prompt/response mode to build or edit comparison columns")
	fs.BoolVar(&transposeFlag, "transpose", false, "Transpose table (Columns as rows, Metrics as columns)")
	fs.BoolVar(&compactFlag, "compact", false, "Compact/transposed table view (alias for --transpose)")

	boolFlags := map[string]bool{
		"i":           true,
		"interactive": true,
		"transpose":   true,
		"compact":     true,
	}
	reordered := reorderFlagsForStdlib(fs, args, boolFlags)
	if err := fs.Parse(reordered); err != nil {
		return 2
	}

	explicitFlags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
	})

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
					col, err := ParseComparisonSpec(frames, specStr)
					if err != nil {
						fmt.Fprintf(a.Err, "Error: %v\n", err)
						return 1
					}
					state.Columns = append(state.Columns, col)
				} else if strings.HasPrefix(arg, "-") {
					rmToken := strings.TrimPrefix(arg, "-")
					if idx, err := strconv.Atoi(rmToken); err == nil {
						colIdx := idx - 1
						if colIdx < 0 || colIdx >= len(state.Columns) {
							fmt.Fprintf(a.Err, "Error: column index %d out of range (1..%d)\n", idx, len(state.Columns))
							return 1
						}
						state.Columns = append(state.Columns[:colIdx], state.Columns[colIdx+1:]...)
					} else {
						f, sz, err := ResolveFrameByToken(frames, rmToken)
						if err != nil {
							fmt.Fprintf(a.Err, "Error: %v\n", err)
							return 1
						}
						var kept []model.ComparisonSetup
						for _, c := range state.Columns {
							if c.FrameID == f.ID && (sz == "" || strings.EqualFold(c.Size, sz)) {
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
			for _, specStr := range posArgs {
				specStr = strings.TrimPrefix(specStr, "+")
				col, err := ParseComparisonSpec(frames, specStr)
				if err != nil {
					fmt.Fprintf(a.Err, "Error: %v\n", err)
					return 1
				}
				newCols = append(newCols, col)
			}
			state.Columns = newCols
			state.RefIndex = 0
		}
	}

	if interactive {
		var err error
		state, err = a.runInteractiveCompare(frames, state)
		if err != nil {
			fmt.Fprintf(a.Err, "Error: %v\n", err)
			return 1
		}
	}

	if len(state.Columns) == 0 {
		fmt.Fprintf(a.Err, "Error: no columns selected for comparison\n")
		return 1
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

	// Save the validated comparison state so bare `bikegeo compare` replays it
	_ = a.saveComparisonState(state)

	if err := a.renderComparisonMatrix(rows, state); err != nil {
		fmt.Fprintf(a.Err, "Error: %v\n", err)
		return 1
	}

	return 0
}

// ParseComparisonSpec parses a CLI token of the form "frame[:size][@mod1][@mod2]..."
// where modifiers can be a size (e.g. "L"), a fork travel/type (e.g. "120mm", "rigid"), or a sag % (e.g. "20%").
func ParseComparisonSpec(
	frames []model.FrameModel,
	spec string,
) (model.ComparisonSetup, error) {
	parts := strings.Split(spec, "@")
	frameToken := strings.TrimSpace(parts[0])

	frame, implicitSize, err := ResolveFrameByToken(frames, frameToken)
	if err != nil {
		return model.ComparisonSetup{}, err
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

		// 1. Sag modifier (e.g. "20%", "sag=25")
		if strings.HasSuffix(low, "%") || strings.HasPrefix(low, "sag=") {
			clean := strings.TrimSuffix(strings.TrimPrefix(low, "sag="), "%")
			val, err := strconv.ParseFloat(clean, 64)
			if err != nil {
				return model.ComparisonSetup{}, fmt.Errorf("invalid sag modifier %q in %q", mod, spec)
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

		return model.ComparisonSetup{}, fmt.Errorf(
			"unrecognized modifier %q for frame \"%s %s\" (sizes: %s; supported forks: %s; sag: e.g. 20%%)",
			mod, frame.Manufacturer, frame.Model,
			strings.Join(frame.GetSizes(), ", "),
			strings.Join(frame.GetSupportedForkOptions(), ", "),
		)
	}

	return setup, nil
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
		row, err := canonicalizer.ResolveFrameForkSetup(*frame, col, state.SeatHeight)
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
				if saved.SeatHeight <= 0 {
					saved.SeatHeight = 760.0
				}
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
		SeatHeight: 760.0,
		Transpose:  false,
		RefIndex:   0,
		Columns:    cols,
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

func (a *App) runInteractiveCompare(frames []model.FrameModel, state model.SavedComparisonState) (model.SavedComparisonState, error) {
	scanner := bufio.NewScanner(a.In)

	for {
		fmt.Fprintf(a.Out, "\nSaved Comparison (Saddle Height: %.0fmm):\n", state.SeatHeight)
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
				row, err := canonicalizer.ResolveFrameForkSetup(*frame, col, state.SeatHeight)
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

		fmt.Fprint(a.Out, "\nAction ([a]dd, [e]dit #, [d]elete #, [r]ef #, [s]addle, [c]lear, or [Enter] to run): ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.EqualFold(line, "run") || strings.EqualFold(line, "q") {
			break
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
			if specArg != "" && strings.Contains(specArg, "@") {
				col, err := ParseComparisonSpec(frames, specArg)
				if err != nil {
					fmt.Fprintf(a.Out, "  ! %v\n", err)
					continue
				}
				frame, _, _ := ResolveFrameByToken(frames, col.FrameID)
				if _, err := canonicalizer.ResolveFrameForkSetup(*frame, col, state.SeatHeight); err != nil {
					fmt.Fprintf(a.Out, "  ! %v\n", err)
					continue
				}
				state.Columns = append(state.Columns, col)
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
				fmt.Fprintf(a.Out, "  Rider Saddle Height in mm [%.0f]: ", state.SeatHeight)
				if scanner.Scan() {
					valStr = strings.TrimSpace(scanner.Text())
				}
			}
			if valStr != "" {
				if v, err := strconv.ParseFloat(valStr, 64); err == nil && v >= 400 && v <= 1100 {
					state.SeatHeight = v
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

	return state, nil
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
	row, err := canonicalizer.ResolveFrameForkSetup(*selectedFrame, candidate, state.SeatHeight)
	if err != nil {
		return model.ComparisonSetup{}, err
	}

	return row.Setup, nil
}

func formatDelta(delta float64, unit string) string {
	if delta > 0 {
		return fmt.Sprintf("+%.1f%s", delta, unit)
	}
	return fmt.Sprintf("%.1f%s", delta, unit)
}

func (a *App) renderComparisonMatrix(rows []model.CanonicalComparisonRow, state model.SavedComparisonState) error {
	if len(rows) == 0 {
		return nil
	}

	refIdx := state.RefIndex
	if refIdx < 0 || refIdx >= len(rows) {
		refIdx = 0
	}
	refRow := rows[refIdx]

	title := "Apples-to-Apples Geometry Comparison Matrix"
	subtitle := fmt.Sprintf(
		"Saddle Height: %.0fmm | Reference Column: [%d] %s %s (%s)",
		state.SeatHeight, refIdx+1,
		refRow.Bike.Manufacturer, refRow.Bike.Model, refRow.Bike.Size,
	)

	jsonPayload := struct {
		SeatHeight float64                        `json:"seat_height"`
		RefIndex   int                            `json:"ref_index"`
		Columns    []model.CanonicalComparisonRow `json:"columns"`
	}{
		SeatHeight: state.SeatHeight,
		RefIndex:   refIdx,
		Columns:    rows,
	}

	if state.Transpose {
		headers := []string{
			"#",
			"Frame (Size)",
			"Fork & Sag Setup",
			"Eff A2C",
			"Reach (ΔRef)",
			"Stack (ΔRef)",
			"HTA (ΔRef)",
			"STA (ΔRef)",
			"BB Drop",
			fmt.Sprintf("AETT @%.0f (ΔRef)", state.SeatHeight),
			fmt.Sprintf("Setback @%.0f (ΔRef)", state.SeatHeight),
			"Steerer (V / A)",
		}
		var tableRows [][]string
		for i, r := range rows {
			idxStr := fmt.Sprintf("%d", i+1)
			if i == refIdx {
				idxStr += " [REF]"
			}
			setupStr := "Rigid"
			if r.SetupFork.TravelMm > 0 {
				setupStr = fmt.Sprintf("%.0fmm @ %.0f%%", r.SetupFork.TravelMm, r.SagPercent)
			} else {
				setupStr = fmt.Sprintf("Rigid (%.0fmm)", r.SetupFork.A2c)
			}

			reachStr := fmt.Sprintf("%.1f mm", r.Canonical.ActiveReach)
			stackStr := fmt.Sprintf("%.1f mm", r.Canonical.ActiveStack)
			htaStr := fmt.Sprintf("%.2f°", r.Canonical.ActiveHta)
			staStr := fmt.Sprintf("%.2f°", r.Canonical.ActiveSta)
			aettStr := fmt.Sprintf("%.1f mm", r.Canonical.Aett)
			setbackStr := fmt.Sprintf("%.1f mm", r.Canonical.ActualSetback)

			if i != refIdx {
				reachStr = fmt.Sprintf("%.1f (%s)", r.Canonical.ActiveReach, formatDelta(r.Canonical.ActiveReach-refRow.Canonical.ActiveReach, ""))
				stackStr = fmt.Sprintf("%.1f (%s)", r.Canonical.ActiveStack, formatDelta(r.Canonical.ActiveStack-refRow.Canonical.ActiveStack, ""))
				htaStr = fmt.Sprintf("%.2f° (%s)", r.Canonical.ActiveHta, formatDelta(r.Canonical.ActiveHta-refRow.Canonical.ActiveHta, "°"))
				staStr = fmt.Sprintf("%.2f° (%s)", r.Canonical.ActiveSta, formatDelta(r.Canonical.ActiveSta-refRow.Canonical.ActiveSta, "°"))
				aettStr = fmt.Sprintf("%.1f (%s)", r.Canonical.Aett, formatDelta(r.Canonical.Aett-refRow.Canonical.Aett, ""))
				setbackStr = fmt.Sprintf("%.1f (%s)", r.Canonical.ActualSetback, formatDelta(r.Canonical.ActualSetback-refRow.Canonical.ActualSetback, ""))
			}

			tableRows = append(tableRows, []string{
				idxStr,
				fmt.Sprintf("%s %s (%s)", r.Bike.Manufacturer, r.Bike.Model, r.Bike.Size),
				setupStr,
				fmt.Sprintf("%.1f mm", r.EffectiveA2c),
				reachStr,
				stackStr,
				htaStr,
				staStr,
				fmt.Sprintf("%.1f mm", r.Canonical.ActiveBbDrop),
				aettStr,
				setbackStr,
				fmt.Sprintf("%.1f / %.1f mm", r.Canonical.SpareVerticalStack, r.Canonical.SpareAxialSteerer),
			})
		}

		return RenderOutput(a.Out, a.Format, title, subtitle, headers, tableRows, jsonPayload)
	}

	// Default orientation: Metrics as rows, Bike Setups as columns
	headers := make([]string, len(rows)+1)
	headers[0] = "Metric"
	for i, r := range rows {
		refBadge := ""
		if i == refIdx {
			refBadge = " [REF]"
		}
		forkLine := fmt.Sprintf("Rigid (%.0fmm A2C)", r.SetupFork.A2c)
		if r.SetupFork.TravelMm > 0 {
			forkLine = fmt.Sprintf("%.0fmm @ %.0f%% sag", r.SetupFork.TravelMm, r.SagPercent)
		}
		headers[i+1] = fmt.Sprintf("[%d] %s %s (%s)%s\n%s", i+1, r.Bike.Manufacturer, r.Bike.Model, r.Bike.Size, refBadge, forkLine)
	}

	buildRow := func(metric string, cellFn func(i int, r model.CanonicalComparisonRow) string) []string {
		out := make([]string, len(rows)+1)
		out[0] = metric
		for i, r := range rows {
			out[i+1] = cellFn(i, r)
		}
		return out
	}

	var tableRows [][]string

	// 1. Setup Fork & Effective A2C
	tableRows = append(tableRows, buildRow("Fork & Eff. A2C", func(i int, r model.CanonicalComparisonRow) string {
		if r.SetupFork.TravelMm > 0 {
			return fmt.Sprintf("%.0fmm @ %.0f%% sag -> %.1fmm A2C", r.SetupFork.TravelMm, r.SagPercent, r.EffectiveA2c)
		}
		return fmt.Sprintf("Rigid (%.0fmm A2C) -> %.1fmm A2C", r.SetupFork.A2c, r.EffectiveA2c)
	}))

	// 2. Reach
	tableRows = append(tableRows, buildRow("Reach", func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.1f mm", r.Canonical.ActiveReach)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.1f mm (%s)", r.Canonical.ActiveReach, formatDelta(r.Canonical.ActiveReach-refRow.Canonical.ActiveReach, " mm"))
		}
		return mainLine
	}))

	// 3. Stack
	tableRows = append(tableRows, buildRow("Stack", func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.1f mm", r.Canonical.ActiveStack)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.1f mm (%s)", r.Canonical.ActiveStack, formatDelta(r.Canonical.ActiveStack-refRow.Canonical.ActiveStack, " mm"))
		}
		return mainLine
	}))

	// 4. HTA
	tableRows = append(tableRows, buildRow("HTA", func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.2f°", r.Canonical.ActiveHta)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.2f° (%s)", r.Canonical.ActiveHta, formatDelta(r.Canonical.ActiveHta-refRow.Canonical.ActiveHta, "°"))
		}
		return mainLine
	}))

	// 5. STA
	tableRows = append(tableRows, buildRow("STA", func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.2f°", r.Canonical.ActiveSta)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.2f° (%s)", r.Canonical.ActiveSta, formatDelta(r.Canonical.ActiveSta-refRow.Canonical.ActiveSta, "°"))
		}
		return mainLine
	}))

	// 6. BB Drop
	tableRows = append(tableRows, buildRow("BB Drop", func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.1f mm", r.Canonical.ActiveBbDrop)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.1f mm (%s)", r.Canonical.ActiveBbDrop, formatDelta(r.Canonical.ActiveBbDrop-refRow.Canonical.ActiveBbDrop, " mm"))
		}
		return mainLine
	}))

	// 7. AETT
	tableRows = append(tableRows, buildRow(fmt.Sprintf("AETT @ %.0fmm", state.SeatHeight), func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.1f mm", r.Canonical.Aett)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.1f mm (%s)", r.Canonical.Aett, formatDelta(r.Canonical.Aett-refRow.Canonical.Aett, " mm"))
		}
		return mainLine
	}))

	// 8. Actual Setback
	tableRows = append(tableRows, buildRow(fmt.Sprintf("Actual Setback @ %.0fmm", state.SeatHeight), func(i int, r model.CanonicalComparisonRow) string {
		mainLine := fmt.Sprintf("%.1f mm", r.Canonical.ActualSetback)
		if i != refIdx {
			mainLine = fmt.Sprintf("%.1f mm (%s)", r.Canonical.ActualSetback, formatDelta(r.Canonical.ActualSetback-refRow.Canonical.ActualSetback, " mm"))
		}
		return mainLine
	}))

	// 9. Spare Steerer (Vertical)
	tableRows = append(tableRows, buildRow("Spare Steerer (Vert)", func(i int, r model.CanonicalComparisonRow) string {
		return fmt.Sprintf("%.1f mm", r.Canonical.SpareVerticalStack)
	}))

	// 10. Spare Steerer (Axial)
	tableRows = append(tableRows, buildRow("Spare Steerer (Axial)", func(i int, r model.CanonicalComparisonRow) string {
		return fmt.Sprintf("%.1f mm", r.Canonical.SpareAxialSteerer)
	}))

	return RenderOutput(a.Out, a.Format, title, subtitle, headers, tableRows, jsonPayload)
}

