// Package compare ranks bikes across frames. It expands frame, size, fork, and
// sag rows, fits each row's winning build with assembly.FitFrontBuilds, and
// ranks and prunes the rows. It never imports catalog: callers pass frames,
// parts, and weights as arguments.
package compare

import (
	"cmp"
	"fmt"
	"math"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

// PhysicalFork is a physical rigid or suspension fork mated onto a frameset.
// Requiring Steerer rejects assembly.BasisFork at compile time because a chart
// basis fork has no physical steerer standard.
type PhysicalFork interface {
	CheckHeadset(bore assembly.LowerBore, cup assembly.LowerCupSetup) error
	AxleInFork(state assembly.LoadState) pose.Point
	Steerer() assembly.PhysicalSteerer
}

// Seatpost is a rigid or linkage seatpost mated onto a frameset's seat tube.
type Seatpost interface {
	TotalLengthMm() float64
	InsertionLimits() assembly.BoundedRange
	RailClamp(state assembly.LoadState) assembly.RailClampPort
}

// Frame is one frame model offered for comparison.
type Frame struct {
	ID    string
	Sizes []Size // declared order
	Forks []Fork // declared order
}

// Size is one frame size as compare installs it. Callers set sliding
// dropouts to max chainstay before constructing Frame.
type Size struct {
	Name  string
	Frame assembly.Frame
}

// Fork is one physical fork variant the frame supports. LowerCup is the lower
// headset cup the fork installs with; the zero value installs
// Size.Frame.DefaultLowerCup().
type Fork struct {
	ID       string
	Travel   assembly.ForkTravel
	LowerCup assembly.LowerCupSetup
	// Build constructs the validated physical fork at sagPct (0 for rigid forks).
	Build func(sagPct float64) (PhysicalFork, error)
}

// Request is one comparison. Rank ranks every size, fork, and sag of every
// frame: 0% for a rigid fork and each of Fork.Travel.SagPcts() for a
// suspension fork.
type Request struct {
	Frames            []Frame
	Wheels            assembly.WheelSetup
	Seatpost          Seatpost
	Saddle            assembly.Saddle
	DynamicRailPitch  pose.Deg
	UpperStackMm      float64
	MaxSpacersMm      float64
	StemClampHeightMm float64
	Stems             []assembly.StemOption
	SpacerStepMm      float64
	Handlebars        []assembly.Handlebar // selected bar (len 1 when HasBarOptions is false) or offered bars
	HasBarOptions     bool                 // marks ScoredBuild.HasHandlebarOption when true
	DynamicBarRoll    pose.Deg
	Target            assembly.FitTarget // requires both BRP and HandGrip
	Weights           assembly.ScoreWeights
	TopPerFrame       int // most rows kept per frame; 0 keeps all
	Top               int // most rows kept overall; 0 keeps all
}

// Row is one distinct (frame, size, fork, sag) and its winning build.
type Row struct {
	FrameID   string
	Size      string
	ForkID    string
	SagPct    float64
	Build     assembly.ScoredBuild
	Evaluated int // combinations FitFrontBuilds placed for this row

	sizeIndex int // position of Size in Frame.Sizes for deterministic tie-breaking
	forkIndex int // position of ForkID in Frame.Forks for deterministic tie-breaking
}

// Ranking is the result of Rank.
type Ranking struct {
	Rows      []Row // best first, after pruning
	RowCount  int   // distinct rows ranked before pruning
	Evaluated int   // combinations placed across every row
}

// idealSagPct is the suspension sag percentage preferred by cross-row tie-breaks.
const idealSagPct = 20.0

type expandedRow struct {
	frameID   string
	size      Size
	sizeIndex int
	fork      Fork
	forkIndex int
	sagPct    float64
}

func rankExpandedRow(req Request, frontOpts assembly.FrontBuildOptions, item expandedRow) (Row, error) {
	fk, err := item.fork.Build(item.sagPct)
	if err != nil {
		return Row{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
	}
	lowerCup := item.size.Frame.DefaultLowerCup()
	if item.fork.LowerCup != (assembly.LowerCupSetup{}) {
		lowerCup = item.fork.LowerCup
	}
	fs, err := assembly.MateFrameset(item.size.Frame, lowerCup, req.UpperStackMm, fk, req.Wheels)
	if err != nil {
		return Row{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
	}
	if req.MaxSpacersMm != assembly.DefaultMaxSpacersMm {
		fs, err = fs.WithMaxSpacers(req.MaxSpacersMm)
		if err != nil {
			return Row{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
		}
	}
	rear, err := assembly.OpenRearCockpit(fs, req.Seatpost, req.Saddle, req.DynamicRailPitch)
	if err != nil {
		return Row{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
	}
	seated, _, err := rear.FitToBRP(*req.Target.BRP)
	if err != nil {
		return Row{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
	}
	ranking, err := assembly.FitFrontBuilds(seated, req.Target, frontOpts, req.Weights, 1)
	if err != nil {
		return Row{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
	}
	return Row{
		FrameID:   item.frameID,
		Size:      item.size.Name,
		ForkID:    item.fork.ID,
		SagPct:    item.sagPct,
		Build:     ranking.Builds[0],
		Evaluated: ranking.Evaluated,
		sizeIndex: item.sizeIndex,
		forkIndex: item.forkIndex,
	}, nil
}

// Rank expands the requested rows, fits each row's winning build with
// assembly.FitFrontBuilds, orders the rows best first, and prunes them.
func Rank(req Request) (Ranking, error) {
	if err := req.validate(); err != nil {
		return Ranking{}, err
	}

	frontOpts := assembly.FrontBuildOptions{
		StemClampHeightMm: req.StemClampHeightMm,
		Stems:             req.Stems,
		SpacerStepMm:      req.SpacerStepMm,
		Handlebars:        req.Handlebars,
		HasHandlebarOpts:  req.HasBarOptions,
		DynamicBarRollDeg: req.DynamicBarRoll,
	}

	var expanded []expandedRow

	for _, f := range req.Frames {
		for si, sz := range f.Sizes {
			for fi, fk := range f.Forks {
				sags := []float64{0}
				if fk.Travel != assembly.TravelRigid {
					sags = fk.Travel.SagPcts()
				}
				for _, sag := range sags {
					expanded = append(expanded, expandedRow{
						frameID:   f.ID,
						size:      sz,
						sizeIndex: si,
						fork:      fk,
						forkIndex: fi,
						sagPct:    sag,
					})
				}
			}
		}
	}

	rows := make([]Row, len(expanded))
	workers := min(runtime.GOMAXPROCS(0), len(expanded))
	if workers <= 1 {
		for i, item := range expanded {
			row, err := rankExpandedRow(req, frontOpts, item)
			if err != nil {
				return Ranking{}, err
			}
			rows[i] = row
		}
	} else {
		errs := make([]error, len(expanded))
		var next atomic.Int64
		var failed atomic.Bool
		var wg sync.WaitGroup
		wg.Add(workers)
		for range workers {
			go func() {
				defer wg.Done()
				for !failed.Load() {
					i := int(next.Add(1) - 1)
					if i >= len(expanded) {
						return
					}
					row, err := rankExpandedRow(req, frontOpts, expanded[i])
					if err != nil {
						errs[i] = err
						failed.Store(true)
						return
					}
					rows[i] = row
				}
			}()
		}
		wg.Wait()
		for _, err := range errs {
			if err != nil {
				return Ranking{}, err
			}
		}
	}

	totalEvaluated := 0
	for _, row := range rows {
		totalEvaluated += row.Evaluated
	}

	slices.SortFunc(rows, compareRows)

	kept := make([]Row, 0, len(rows))
	perFrame := make(map[string]int, len(req.Frames))
	for _, row := range rows {
		if req.TopPerFrame > 0 && perFrame[row.FrameID] >= req.TopPerFrame {
			continue
		}
		kept = append(kept, row)
		perFrame[row.FrameID]++
		if req.Top > 0 && len(kept) >= req.Top {
			break
		}
	}

	return Ranking{
		Rows:      kept,
		RowCount:  len(rows),
		Evaluated: totalEvaluated,
	}, nil
}

func (r Request) validate() error {
	if len(r.Frames) == 0 {
		return fmt.Errorf("compare needs at least one frame")
	}

	framesByID := make(map[string]Frame, len(r.Frames))
	for i, f := range r.Frames {
		if f.ID == "" {
			return fmt.Errorf("frame %d has an empty ID", i)
		}
		if _, exists := framesByID[f.ID]; exists {
			return fmt.Errorf("duplicate frame %q", f.ID)
		}
		framesByID[f.ID] = f

		if len(f.Sizes) == 0 {
			return fmt.Errorf("frame %q has no sizes", f.ID)
		}
		if len(f.Forks) == 0 {
			return fmt.Errorf("frame %q has no forks", f.ID)
		}

		seenSizes := make(map[string]bool, len(f.Sizes))
		for j, sz := range f.Sizes {
			if sz.Name == "" {
				return fmt.Errorf("frame %q size %d has an empty name", f.ID, j)
			}
			if seenSizes[sz.Name] {
				return fmt.Errorf("frame %q has duplicate size %q", f.ID, sz.Name)
			}
			seenSizes[sz.Name] = true
		}

		seenForks := make(map[string]bool, len(f.Forks))
		for j, fk := range f.Forks {
			if fk.ID == "" {
				return fmt.Errorf("frame %q fork %d has an empty ID", f.ID, j)
			}
			if seenForks[fk.ID] {
				return fmt.Errorf("frame %q has duplicate fork %q", f.ID, fk.ID)
			}
			seenForks[fk.ID] = true
			if fk.Build == nil {
				return fmt.Errorf("frame %q fork %q has no fork builder", f.ID, fk.ID)
			}
			if !fk.Travel.Valid() {
				return fmt.Errorf("frame %q fork %q has unsupported travel %d mm", f.ID, fk.ID, fk.Travel)
			}
		}
	}

	if r.Seatpost == nil {
		return fmt.Errorf("compare needs a seatpost")
	}
	if len(r.Handlebars) == 0 {
		return fmt.Errorf("compare needs at least one handlebar")
	}
	if r.Target.BRP == nil {
		return fmt.Errorf("compare needs a BRP target")
	}
	if r.Target.HandGrip == nil {
		return fmt.Errorf("compare needs a HandGrip target")
	}
	if r.TopPerFrame < 0 {
		return fmt.Errorf("top per frame must be 0 or more, got %d", r.TopPerFrame)
	}
	if r.Top < 0 {
		return fmt.Errorf("top must be 0 or more, got %d", r.Top)
	}
	return nil
}

// compareRows orders rows best first: by CompareFitRank on the winning
// builds, then cross-row tie-breaks (frame ID, declared size order, declared
// fork order, sag closest to 20%, then lower sag).
func compareRows(a, b Row) int {
	return cmp.Or(
		assembly.CompareFitRank(a.Build.Score, b.Build.Score),
		cmp.Compare(a.FrameID, b.FrameID),
		cmp.Compare(a.sizeIndex, b.sizeIndex),
		cmp.Compare(a.forkIndex, b.forkIndex),
		cmp.Compare(math.Abs(a.SagPct-idealSagPct), math.Abs(b.SagPct-idealSagPct)),
		cmp.Compare(a.SagPct, b.SagPct),
	)
}
