package catalog

import (
	"fmt"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

// ChartPrecision holds the rounding half-width of each published chart column
// that the chart-consistency test reads: a length printed as 642 could be
// anything in 642 ± 0.5 before the manufacturer rounded it. Zero means the
// column is exact and gets no term. The basis fork and basis lower cup are
// hypotheses, not rounded chart numbers, so they have no half-width.
type ChartPrecision struct {
	ReachMm          float64
	StackMm          float64
	HeadTubeAngleDeg pose.Deg
	HeadTubeLengthMm float64
	BbDropMm         float64
	ChainstayMm      float64
	WheelbaseMm      float64
}

// ChartAllowance widens one size's consistency checks beyond what rounding of
// its published columns explains. The chart-consistency test reads these and
// rejects an allowance on a check that passes on rounding alone, or one more
// than 0.5 mm beyond that check's excess. Reason documents why the chart is
// inconsistent and is required whenever a field is non-zero.
type ChartAllowance struct {
	AxleHeightMm float64
	WheelbaseMm  float64
	Reason       string
}

// Frame is one frame model with its sizes and the forks it supports.
type Frame struct {
	ID           string
	Manufacturer string
	Model        string
	Year         int
	DefaultSize  string
	Sizes        []FrameSize // declared order
	Forks        []Fork      // declared order
	// MaxChainstayMm is the rearmost sliding-dropout position, where / and
	// /bike evaluate the frame; 0 means fixed dropouts.
	MaxChainstayMm float64
	// BasisLowerCup and BasisFork are the chart's basis: the lower cup and
	// fork its numbers are computed on, whether stated by the manufacturer or
	// inferred from the chart; not physical parts.
	BasisLowerCup assembly.LowerCupSetup
	BasisFork     assembly.BasisFork
	// ChartPrecision is the rounding half-width of each published column.
	ChartPrecision ChartPrecision
}

// FrameSize is one size's published chart.
type FrameSize struct {
	Name string
	// ChartDims holds the bare frame's published chart dimensions (published
	// chainstay); Frame.InstalledDims / Frame.InstalledFrame gives the frame /
	// and /bike build.
	ChartDims            assembly.FrameDimensions
	PublishedWheelbaseMm float64        // 0 = unpublished; skips the wheelbase check
	Allowance            ChartAllowance // documented widening beyond rounding
}

func mustBasisFork(a2cMm, rakeMm float64) assembly.BasisFork {
	f, err := assembly.NewBasisFork(a2cMm, rakeMm)
	if err != nil {
		panic(fmt.Sprintf("catalog: invalid basis fork (%v, %v): %v", a2cMm, rakeMm, err))
	}
	return f
}

// Size returns the named size.
func (f Frame) Size(name string) (FrameSize, bool) {
	for _, s := range f.Sizes {
		if s.Name == name {
			return s, true
		}
	}
	return FrameSize{}, false
}

// ChartFrame validates and returns the bare frame at its published chart dimensions.
func (f Frame) ChartFrame(size FrameSize) (assembly.Frame, error) {
	return assembly.NewFrame(size.ChartDims)
}

// InstalledDims returns the size's dimensions as / and /bike build it: sliding
// dropouts at MaxChainstayMm, otherwise the published chart dimensions.
func (f Frame) InstalledDims(size FrameSize) assembly.FrameDimensions {
	dims := size.ChartDims
	if f.MaxChainstayMm != 0 {
		dims.ChainstayMm = f.MaxChainstayMm
	}
	return dims
}

// InstalledFrame validates and returns the bare frame as / and /bike build it.
func (f Frame) InstalledFrame(size FrameSize) (assembly.Frame, error) {
	return assembly.NewFrame(f.InstalledDims(size))
}

// headset returns the lower bore and default cup standard every size shares
// (frameProblems checks they agree); a frame with no sizes has neither.
func (f Frame) headset() (assembly.LowerBore, assembly.LowerCupStandard) {
	if len(f.Sizes) == 0 {
		return "", ""
	}
	fr := f.Sizes[0].ChartDims
	return fr.LowerBore, fr.DefaultLowerCup.Standard
}

// IsListedFork reports whether modelID is one of f's listed forks.
func (f Frame) IsListedFork(modelID string) bool {
	for _, fk := range f.Forks {
		if fk.ID == modelID {
			return true
		}
	}
	return false
}

// CompatibleForks returns the forks the frame accepts: its listed forks first
// in declared order, then every other catalog fork, in catalog order, whose
// steerer fits the head tube and whose class a listed fork unlocks. A listed
// rigid fork unlocks only its own rigid class; a listed suspension fork
// unlocks its own travel and the rigid classes that stand in for it.
func (f Frame) CompatibleForks() []Fork {
	bore, std := f.headset()
	seen := make(map[string]bool, len(f.Forks))
	allowedRigid := make(map[RigidClass]bool)
	allowedTravel := make(map[assembly.ForkTravel]bool)
	out := make([]Fork, 0, len(f.Forks))
	for _, fk := range f.Forks {
		out = append(out, fk)
		seen[fk.ID] = true
		if fk.Travel == assembly.TravelRigid {
			allowedRigid[fk.RigidClass] = true
			continue
		}
		allowedTravel[fk.Travel] = true
		for _, rc := range standInRigidClasses(fk.Travel) {
			allowedRigid[rc] = true
		}
	}
	for _, fk := range Forks() {
		if seen[fk.ID] {
			continue
		}
		if _, ok := LowerCupFor(bore, std, fk.Steerer); !ok {
			continue
		}
		if fk.Travel == assembly.TravelRigid {
			if !allowedRigid[fk.RigidClass] {
				continue
			}
		} else if !allowedTravel[fk.Travel] {
			continue
		}
		out = append(out, fk)
		seen[fk.ID] = true
	}
	return out
}

// ForkOptions expands CompatibleForks into concrete ForkOptions in fork and
// variant order, each with the typical lower cup it installs with.
func (f Frame) ForkOptions() []ForkOption {
	bore, std := f.headset()
	var out []ForkOption
	for _, fk := range f.CompatibleForks() {
		cup, ok := LowerCupFor(bore, std, fk.Steerer)
		if !ok {
			continue // frameProblems forbids this for listed forks
		}
		for _, v := range fk.Variants {
			out = append(out, ForkOption{Fork: fk, Variant: v, LowerCup: cup})
		}
	}
	return out
}

// LookupFork resolves id against f's compatible forks:
//   - a concrete variant ID (bare ID on a single-variant fork, or
//     "<model>-<a2c>x<rake>" on a multi-variant fork) returns that single ForkOption;
//   - a bare multi-variant fork model ID returns all variants of that model.
func (f Frame) LookupFork(id string) ([]ForkOption, bool) {
	options := f.ForkOptions()
	for _, opt := range options {
		if opt.ID() == id {
			return []ForkOption{opt}, true
		}
	}
	var out []ForkOption
	for _, opt := range options {
		if opt.Fork.IsMultiVariant() && opt.Fork.ID == id {
			out = append(out, opt)
		}
	}
	return out, len(out) > 0
}

// Frames returns every catalog frame in ascending ID order (TestFramesRegistry pins the list).
func Frames() []Frame {
	return []Frame{
		binaryHavok(),
		binaryKinetik(),
		eskerHayduke(),
		fairlightHolt2(),
		mootsMountaineer(),
		otsoFenrirStainless(),
		revelElJefe(),
		salsaCutthroat(),
		salsaFargo(),
		sevenCustom(),
		sevenExpedition926d(),
		sevenExpedition926f(),
		sklarPbj(),
		sourPastaParty(),
		surlyEcr(),
		surlyKrampus(),
		tumbleweedSunliner(),
		turnerVennTi(),
	}
}

// DefaultFrameID is the default frame for /bike.
const DefaultFrameID = "otso-fenrir-stainless"

// FrameByID returns the frame with the exact ID.
func FrameByID(id string) (Frame, bool) {
	for _, f := range Frames() {
		if f.ID == id {
			return f, true
		}
	}
	return Frame{}, false
}

// frameFamily holds headset values every size shares.
type frameFamily struct {
	LowerBore       assembly.LowerBore
	DefaultLowerCup assembly.LowerCupSetup
}

// publishedSize is one size's published chart columns.
type publishedSize struct {
	Name                               string
	ReachMm, StackMm                   float64
	HeadTubeAngleDeg, SeatTubeAngleDeg pose.Deg
	HeadTubeLengthMm, BbDropMm         float64
	ChainstayMm, SeatTubeLengthMm      float64
	WheelbaseMm                        float64 // 0 = unpublished
	MaxPostInsertionMm                 float64 // 0 = unpublished
	Allowance                          ChartAllowance
}

// sizes builds each published size's chart on the family's shared values.
func (f frameFamily) sizes(published ...publishedSize) []FrameSize {
	out := make([]FrameSize, len(published))
	for i, p := range published {
		out[i] = FrameSize{
			Name: p.Name,
			ChartDims: assembly.FrameDimensions{
				ReachMm:             p.ReachMm,
				StackMm:             p.StackMm,
				HeadTubeAngleDeg:    p.HeadTubeAngleDeg,
				HeadTubeLengthMm:    p.HeadTubeLengthMm,
				SeatTubeAngleDeg:    p.SeatTubeAngleDeg,
				SeatTubeLengthMm:    p.SeatTubeLengthMm,
				BbDropMm:            p.BbDropMm,
				ChainstayMm:         p.ChainstayMm,
				LowerBore:           f.LowerBore,
				DefaultLowerCup:     f.DefaultLowerCup,
				MaxPostInsertionMm:  p.MaxPostInsertionMm,
				MinFrameInsertionMm: 0,
			},
			PublishedWheelbaseMm: p.WheelbaseMm,
			Allowance:            p.Allowance,
		}
	}
	return out
}
