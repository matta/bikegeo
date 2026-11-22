package catalog

import (
	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

// ChartPrecision holds the rounding half-width of each published chart column
// that the chart-consistency test reads: a length printed as 642 could be
// anything in 642 ± 0.5 before the manufacturer rounded it. Zero means the
// column is exact and gets no term. The basis fork and basis lower cup are
// hypotheses, not rounded chart numbers, so they have no half-width (Q44).
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
// its published columns explains (Q65, Q71). The chart-consistency test reads
// these and rejects an allowance on a check that passes on rounding alone, or
// one more than 0.5 mm beyond that check's excess (Q93). Reason documents why
// the chart is inconsistent and is required whenever a field is non-zero.
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
	Sizes        []FrameSize // declared order (Q69)
	Forks        []Fork      // declared order (Q69, Q128)
	// MaxChainstayMm is the rearmost sliding-dropout position, where / and
	// /bike evaluate the frame (Q67); 0 means fixed dropouts.
	MaxChainstayMm float64
	// BasisLowerCup and BasisFork are the chart's basis: the lower cup and
	// fork its numbers are computed on (Q44, Q81, Q121), whether stated by
	// the manufacturer or inferred from the chart; not physical parts.
	// BasisFork uses assembly.SteererBasis and has no travel; its StaticA2cMm
	// is the basis axle-to-crown, sag included.
	BasisLowerCup assembly.LowerCupSetup
	BasisFork     assembly.ForkSpec
	// ChartPrecision is the rounding half-width of each published column (Q43, Q80).
	ChartPrecision ChartPrecision
}

// FrameSize is one size's published chart.
type FrameSize struct {
	Name string
	// ChartFrame is the bare frame at its published dimensions (published
	// chainstay); Frame.InstalledFrame gives the frame / and /bike build (Q67).
	ChartFrame           assembly.FrameSetup
	PublishedWheelbaseMm float64        // 0 = unpublished; skips the wheelbase check (Q57)
	Allowance            ChartAllowance // documented widening beyond rounding (Q65, Q71, Q93)
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

// InstalledFrame returns the size's frame as / and /bike build it: sliding
// dropouts at MaxChainstayMm (Q67), otherwise the published chart frame.
func (f Frame) InstalledFrame(size FrameSize) assembly.FrameSetup {
	frame := size.ChartFrame
	if f.MaxChainstayMm != 0 {
		frame.ChainstayMm = f.MaxChainstayMm
	}
	return frame
}

// headset returns the lower bore and default cup standard every size shares
// (frameProblems checks they agree); a frame with no sizes has neither.
func (f Frame) headset() (assembly.LowerBore, assembly.LowerCupStandard) {
	if len(f.Sizes) == 0 {
		return "", ""
	}
	fr := f.Sizes[0].ChartFrame
	return fr.LowerBore, fr.DefaultLowerCup.Standard
}

// IsListedFork reports whether modelID is one of f's listed forks (Q128).
func (f Frame) IsListedFork(modelID string) bool {
	for _, fk := range f.Forks {
		if fk.ID == modelID {
			return true
		}
	}
	return false
}

// CompatibleForks returns the forks the frame accepts (Q128, Q129, Q130,
// Q135): its listed forks first in declared order, then every other catalog
// fork, in catalog order, whose steerer fits the head tube and whose class a
// listed fork unlocks. A listed rigid fork unlocks only its own rigid class; a
// listed suspension fork unlocks its own travel and the rigid classes that
// stand in for it.
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
// variant order, each with the typical lower cup it installs with (Q128, Q136).
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

// LookupFork resolves id against f's compatible forks (Q131):
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
		ritcheyUltra(),
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

// DefaultFrameID is the default frame for /bike (Q109).
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
	WheelbaseMm                        float64 // 0 = unpublished (Q57)
	MaxPostInsertionMm                 float64 // 0 = unpublished (Q95)
	Allowance                          ChartAllowance
}

// sizes builds each published size's chart on the family's shared values.
func (f frameFamily) sizes(published ...publishedSize) []FrameSize {
	out := make([]FrameSize, len(published))
	for i, p := range published {
		out[i] = FrameSize{
			Name: p.Name,
			ChartFrame: assembly.FrameSetup{
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
