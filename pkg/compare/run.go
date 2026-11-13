package compare

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"bikegeo/pkg/catalog"
	"bikegeo/pkg/frames"
	"bikegeo/pkg/ideal"
	"bikegeo/pkg/model"
)

// Options is the transport-neutral description of a comparison request. The
// CLI fills it from flags and positional args; the web layer from a URL query.
type Options struct {
	// Specs are positional specs: bare replaces the baseline, "+spec" appends,
	// "-spec" / "-N" removes.
	Specs []string

	Saddle     string // saddle catalog ID; "" keeps the default
	Handlebar  string // handlebar catalog ID; "" keeps the default
	Transpose  *bool  // nil means automatic
	All        bool
	AllConfigs bool

	TopPerFrame *int // nil means automatic
	Top         int  // 0 means unlimited
}

// Result is a fully resolved, pruned comparison ready for scoring/rendering.
type Result struct {
	Rows    []model.CanonicalComparisonRow
	State   model.SavedComparisonState
	Profile *ideal.Profile
}

// Run resolves opts against the compiled default baseline.
func Run(opts Options) (*Result, error) {
	allFrames := frames.AllFrames()
	if len(allFrames) == 0 {
		return nil, errors.New("no bike frames found")
	}

	state := DefaultComparisonState()
	if opts.Transpose != nil {
		state.Transpose = *opts.Transpose
	}

	posArgs := opts.Specs
	if opts.All && len(posArgs) == 0 {
		posArgs = []string{"all:all@all"}
	} else if opts.AllConfigs && len(posArgs) == 0 {
		posArgs = []string{"all:all@all"}
	} else if len(posArgs) == 1 && strings.EqualFold(posArgs[0], "all") {
		posArgs = []string{"all:all@all"}
	}

	if opts.AllConfigs && len(posArgs) > 0 {
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

	isAllFrames := opts.All
	hasAllForksExpansion := opts.AllConfigs
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

	if opts.Top < 0 {
		return nil, fmt.Errorf("--top must be a non-negative integer, got %d", opts.Top)
	}
	topPerFrame := 0
	if opts.TopPerFrame != nil {
		if *opts.TopPerFrame < 0 {
			return nil, fmt.Errorf("--top-per-frame must be a non-negative integer, got %d", *opts.TopPerFrame)
		}
		topPerFrame = *opts.TopPerFrame
	} else if isAllFrames {
		topPerFrame = 1
	} else if hasAllForksExpansion {
		topPerFrame = 5
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
					cols, err := ParseComparisonSpecs(allFrames, after)
					if err != nil {
						return nil, err
					}
					state.Columns = append(state.Columns, cols...)
				} else if after, ok := strings.CutPrefix(arg, "-"); ok {
					rmToken := after
					if idx, err := strconv.Atoi(rmToken); err == nil {
						colIdx := idx - 1
						if colIdx < 0 || colIdx >= len(state.Columns) {
							return nil, fmt.Errorf("column index %d out of range (1..%d)", idx, len(state.Columns))
						}
						state.Columns = append(state.Columns[:colIdx], state.Columns[colIdx+1:]...)
					} else if strings.Contains(rmToken, "@") {
						targetCol, err := ParseComparisonSpec(allFrames, rmToken)
						if err != nil {
							return nil, err
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
							return nil, err
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
					return nil, err
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
			if opts.Transpose == nil {
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
		return nil, errors.New("no columns selected for comparison")
	}

	// Automatically enable transposed view if more than 4 columns and not explicitly overridden
	if opts.Transpose == nil && len(state.Columns) > 4 {
		state.Transpose = true
	}

	prof, err := ideal.GetProfile("idmatch")
	if err != nil {
		return nil, err
	}
	state.IdealProfile = prof.Name

	if prof.SaddleHeightMm != nil && *prof.SaddleHeightMm > 0 {
		state.SaddleHeightMm = *prof.SaddleHeightMm
	}

	if opts.Saddle != "" {
		s, ok := catalog.GetSaddle(opts.Saddle)
		if !ok {
			saddles := catalog.AllSaddles()
			ids := make([]string, len(saddles))
			for i, item := range saddles {
				ids[i] = item.ID
			}
			return nil, fmt.Errorf("unrecognized saddle %q (available: %s)", opts.Saddle, strings.Join(ids, ", "))
		}
		state.SaddleID = s.ID
		state.Saddle = s
	} else if state.Saddle.ID == "" {
		state.Saddle = catalog.DefaultSaddle()
		state.SaddleID = state.Saddle.ID
	}

	if opts.Handlebar != "" {
		b, ok := catalog.GetHandlebar(opts.Handlebar)
		if !ok {
			bars := catalog.AllHandlebars()
			ids := make([]string, len(bars))
			for i, item := range bars {
				ids[i] = item.ID
			}
			return nil, fmt.Errorf("unrecognized handlebar %q (available: %s)", opts.Handlebar, strings.Join(ids, ", "))
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
	rows, normalizedCols, err := ResolveAllColumns(allFrames, state)
	if err != nil {
		return nil, err
	}
	state.Columns = normalizedCols

	// Two-stage ranking and pruning for combinatorial sanity
	rows = PruneRows(allFrames, rows, prof, topPerFrame, opts.Top)
	if len(rows) == 0 {
		return nil, errors.New("no valid setups found for comparison")
	}
	var prunedCols []model.ComparisonSetup
	for _, r := range rows {
		prunedCols = append(prunedCols, r.Setup)
	}
	state.Columns = prunedCols
	if state.RefIndex >= len(state.Columns) {
		state.RefIndex = 0
	}
	if opts.Transpose == nil && len(state.Columns) > 4 {
		state.Transpose = true
	}

	return &Result{Rows: rows, State: state, Profile: prof}, nil
}
