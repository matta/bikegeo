// Package compare ranks bikes across frames. It expands frame, size, fork, and
// sag rows, finds each row's winning build with assembly.FitBuilds, and ranks
// and prunes the rows (Q52, Q61, Q69). It never imports catalog: callers pass
// frames, parts, and weights as arguments.
package compare

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"bikegeo/internal/reboot/assembly"
)

// Frame is one frame model offered for comparison.
type Frame struct {
	ID    string
	Sizes []Size // declared order (Q69)
	Forks []Fork // declared order (Q69)
}

// Size is one frame size as compare installs it. Callers set sliding
// dropouts to max chainstay before passing it (Q67).
type Size struct {
	Name  string
	Frame assembly.FrameSetup
}

// Fork is one physical fork variant the frame supports (Q66, Q128).
// ModelID is the fork model ID (defaults to ID when empty) so a Selection may
// target either a concrete variant ID or a multi-variant model ID (Q131).
// LowerCup is the lower headset cup the fork installs with (Q136); the zero
// value installs Size.Frame.DefaultLowerCup.
type Fork struct {
	ID       string
	ModelID  string
	Spec     assembly.ForkSpec
	LowerCup assembly.LowerCupSetup
}

func (f Fork) modelID() string {
	if f.ModelID != "" {
		return f.ModelID
	}
	return f.ID
}

// Selection picks rows from one frame (Q87). An empty Size or ForkID selects
// every size or fork. A nil SagPct selects 0% for a rigid fork and each of
// Fork.Spec.Travel.SagPcts() for a suspension fork. A non-nil SagPct needs a ForkID.
type Selection struct {
	FrameID string
	Size    string
	ForkID  string
	SagPct  *float64
}

// Request is one comparison.
type Request struct {
	Frames      []Frame
	Selections  []Selection // empty selects every size, fork, and sag of every frame
	Wheels      assembly.WheelSetup
	Seatpost    assembly.SeatpostSpec
	Saddle      assembly.SaddleSetup    // the saddle fit sets RailOffsetMm; RailPitchDeg is kept
	Stem        assembly.StemSetup      // build fitting sets LengthMm, AngleDeg, and SpacersMm
	Handlebar   assembly.HandlebarSetup // the selected bar; its BarRollDeg applies to every bar (Q24)
	Target      assembly.FitTarget      // needs both BRP and HandGrip (Q76)
	Parts       assembly.PartOptions    // Forks must be empty: each row fixes its fork
	Weights     assembly.ScoreWeights
	TopPerFrame int // most rows kept per frame; 0 keeps all
	Top         int // most rows kept overall; 0 keeps all
}

// Row is one distinct (frame, size, fork, sag) and its winning build (Q69).
type Row struct {
	FrameID   string
	Size      string
	ForkID    string
	SagPct    float64
	Build     assembly.ScoredBuild
	Evaluated int // combinations FitBuilds placed for this row

	sizeIndex int // position of Size in Frame.Sizes (Q69 tie-break)
	forkIndex int // position of ForkID in Frame.Forks (Q69 tie-break)
}

// Ranking is the result of Rank.
type Ranking struct {
	Rows      []Row // best first, after pruning
	RowCount  int   // distinct rows ranked before pruning
	Evaluated int   // combinations placed across every row
}

// idealSagPct is the sag the Q69 cross-row tie-break prefers (the Q59a ideal).
const idealSagPct = 20.0

// Rank expands the requested rows, fits each row's winning build with
// assembly.FitBuilds, orders the rows best first, and prunes them.
func Rank(req Request) (Ranking, error) {
	if err := req.validate(); err != nil {
		return Ranking{}, err
	}

	framesByID := make(map[string]Frame, len(req.Frames))
	for _, f := range req.Frames {
		framesByID[f.ID] = f
	}

	selections := req.Selections
	if len(selections) == 0 {
		selections = make([]Selection, len(req.Frames))
		for i, f := range req.Frames {
			selections[i] = Selection{FrameID: f.ID}
		}
	}

	type rowKey struct {
		frameID string
		size    string
		forkID  string
		sagPct  float64
	}
	seen := make(map[rowKey]bool)

	type expandedRow struct {
		frameID   string
		size      Size
		sizeIndex int
		fork      Fork
		forkIndex int
		sagPct    float64
	}
	var expanded []expandedRow

	for _, sel := range selections {
		f := framesByID[sel.FrameID]
		for si, sz := range f.Sizes {
			if sel.Size != "" && sz.Name != sel.Size {
				continue
			}
			exactFork := sel.ForkID != "" && slices.ContainsFunc(f.Forks, func(fk Fork) bool { return fk.ID == sel.ForkID })
			for fi, fk := range f.Forks {
				if sel.ForkID != "" {
					if exactFork && fk.ID != sel.ForkID {
						continue
					}
					if !exactFork && fk.modelID() != sel.ForkID {
						continue
					}
				}
				var sags []float64
				switch {
				case sel.SagPct != nil:
					sags = []float64{*sel.SagPct}
				case fk.Spec.Travel == assembly.TravelRigid:
					sags = []float64{0}
				default:
					sags = fk.Spec.Travel.SagPcts()
				}
				for _, sag := range sags {
					key := rowKey{frameID: f.ID, size: sz.Name, forkID: fk.ID, sagPct: sag}
					if seen[key] {
						continue
					}
					seen[key] = true
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

	rows := make([]Row, 0, len(expanded))
	totalEvaluated := 0
	for _, item := range expanded {
		bounds, err := assembly.SeatpostInsertionBounds(item.size.Frame, req.Seatpost)
		if err != nil {
			return Ranking{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
		}
		lowerCup := item.size.Frame.DefaultLowerCup
		if item.fork.LowerCup != (assembly.LowerCupSetup{}) {
			lowerCup = item.fork.LowerCup
		}
		setup := assembly.BikeSetup{
			Frame:     item.size.Frame,
			LowerCup:  lowerCup,
			Fork:      assembly.ForkSetup{Spec: item.fork.Spec, SagPct: item.sagPct},
			Wheels:    req.Wheels,
			Seatpost:  assembly.SeatpostSetup{Spec: req.Seatpost, InsertionMm: bounds.EffectiveMinInsertionMm}, // the saddle fit replaces it
			Saddle:    req.Saddle,
			Stem:      req.Stem,
			Handlebar: req.Handlebar,
		}
		ranking, err := assembly.FitBuilds(setup, req.Target, req.Parts, req.Weights, 1)
		if err != nil {
			return Ranking{}, fmt.Errorf("frame %q size %q fork %q at %v%% sag: %w", item.frameID, item.size.Name, item.fork.ID, item.sagPct, err)
		}
		rows = append(rows, Row{
			FrameID:   item.frameID,
			Size:      item.size.Name,
			ForkID:    item.fork.ID,
			SagPct:    item.sagPct,
			Build:     ranking.Builds[0],
			Evaluated: ranking.Evaluated,
			sizeIndex: item.sizeIndex,
			forkIndex: item.forkIndex,
		})
		totalEvaluated += ranking.Evaluated
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
			if fk.Spec.Steerer == assembly.SteererBasis {
				return fmt.Errorf("frame %q fork %q is the chart basis fork; compare installs physical forks only", f.ID, fk.ID)
			}
			if !fk.Spec.Travel.Valid() {
				return fmt.Errorf("frame %q fork %q has unsupported travel %d mm", f.ID, fk.ID, fk.Spec.Travel)
			}
			if fk.LowerCup != (assembly.LowerCupSetup{}) {
				if err := validateForkLowerCup(fk, f.Sizes); err != nil {
					return fmt.Errorf("frame %q fork %q: %w", f.ID, fk.ID, err)
				}
			}
		}
	}

	if r.Target.BRP == nil {
		return fmt.Errorf("compare needs a BRP target (Q76)")
	}
	if r.Target.HandGrip == nil {
		return fmt.Errorf("compare needs a HandGrip target (Q76)")
	}
	if len(r.Parts.Forks) != 0 {
		return fmt.Errorf("compare fixes each row's fork; part options must not list forks")
	}
	if r.TopPerFrame < 0 {
		return fmt.Errorf("top per frame must be 0 or more, got %d", r.TopPerFrame)
	}
	if r.Top < 0 {
		return fmt.Errorf("top must be 0 or more, got %d", r.Top)
	}

	for _, sel := range r.Selections {
		f, ok := framesByID[sel.FrameID]
		if !ok {
			return fmt.Errorf("unknown frame %q", sel.FrameID)
		}
		if sel.Size != "" && !hasSize(f.Sizes, sel.Size) {
			return fmt.Errorf("frame %q has no size %q", sel.FrameID, sel.Size)
		}
		if sel.ForkID != "" && !hasFork(f.Forks, sel.ForkID) {
			return fmt.Errorf("frame %q has no fork %q", sel.FrameID, sel.ForkID)
		}
		if sel.SagPct != nil && sel.ForkID == "" {
			return fmt.Errorf("selection on frame %q sets sag without a fork", sel.FrameID)
		}
	}

	return nil
}

// validateForkLowerCup checks a fork's own lower cup: a physical standard with
// a valid stack that fits every size's lower bore and the fork's steerer (Q136).
func validateForkLowerCup(fk Fork, sizes []Size) error {
	cup := fk.LowerCup
	if cup.Standard == assembly.LowerCupBareHeadTube {
		return fmt.Errorf("lower cup must be a physical cup, not %s", cup.Standard)
	}
	if err := cup.Validate(); err != nil {
		return err
	}
	for _, sz := range sizes {
		bore := sz.Frame.LowerBore
		var allowed []assembly.LowerCupStandard
		switch bore {
		case assembly.Bore34Pressed:
			allowed = []assembly.LowerCupStandard{assembly.LowerCupEC34}
		case assembly.Bore42Integrated:
			allowed = []assembly.LowerCupStandard{assembly.LowerCupIS42}
		case assembly.Bore44Pressed:
			allowed = []assembly.LowerCupStandard{assembly.LowerCupEC44}
			if fk.Spec.Steerer == assembly.SteererStraight118 {
				allowed = append(allowed, assembly.LowerCupZS44)
			}
		case assembly.Bore56Pressed:
			allowed = []assembly.LowerCupStandard{assembly.LowerCupZS56}
		case assembly.Bore52Integrated:
			allowed = []assembly.LowerCupStandard{assembly.LowerCupIS52}
		default:
			return fmt.Errorf("size %q has unknown lower bore %q", sz.Name, bore)
		}
		if !slices.Contains(allowed, cup.Standard) {
			return fmt.Errorf("%s lower cup does not fit %s on size %q with a %s steerer", cup.Standard, bore, sz.Name, fk.Spec.Steerer)
		}
		if fk.Spec.Steerer == assembly.SteererTapered && (bore == assembly.Bore34Pressed || bore == assembly.Bore42Integrated) {
			return fmt.Errorf("a tapered steerer does not fit %s on size %q", bore, sz.Name)
		}
	}
	return nil
}

func hasSize(sizes []Size, name string) bool {
	for _, s := range sizes {
		if s.Name == name {
			return true
		}
	}
	return false
}

func hasFork(forks []Fork, id string) bool {
	for _, f := range forks {
		if f.ID == id || f.modelID() == id {
			return true
		}
	}
	return false
}

// compareRows orders rows best first: by CompareFitRank on the winning
// builds, then the Q69 cross-row tie-breaks (frame ID, declared size order,
// declared fork order, sag closest to 20%, then lower sag).
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
