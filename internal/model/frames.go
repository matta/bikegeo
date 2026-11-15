package model

import (
	"fmt"
	"maps"
	"math"
)

// ForkCatalogOption specifies an available (A2C, rake) pair for a catalog fork.
type ForkCatalogOption struct {
	A2cMm  float64
	RakeMm float64
}

// ForkCatalogEntry encapsulates a fork model in pkg/catalog.
type ForkCatalogEntry struct {
	ID          string
	Name        string
	TravelMm    float64
	Standard    bool
	Options     []ForkCatalogOption
	Description string
}

// ForkPreset is an alias for ForkCatalogEntry.
type ForkPreset = ForkCatalogEntry

// ForkRef encapsulates a reference to a catalog fork with resolved A2C and rake.
type ForkRef struct {
	Fork     string
	TravelMm float64
	SagPct   float64
	A2cMm    float64
	RakeMm   float64
}

func (r ForkRef) GetSagPct() float64 {
	return r.SagPct
}

func (r ForkRef) StaticA2c() float64 {
	if r.TravelMm > 0 && r.SagPct > 0 {
		return r.A2cMm + r.TravelMm*(r.SagPct/100.0)
	}
	return r.A2cMm
}

// ForkSpec encapsulates physical and geometric properties of a fork (for compatibility).
type ForkSpec struct {
	ID                 string
	Name               string
	TravelMm           float64
	A2c                float64
	Rake               float64
	StandardSagPercent float64
	Description        string
	IsInferred         bool
}

// SizeGeometry represents primary and optional frame dimensions for one size.
type SizeGeometry struct {
	Reach           float64
	Stack           float64
	HtaDeg          Deg
	StaDeg          Deg
	Htl             float64
	BbDrop          float64
	Wheelbase       float64
	ChainstayLength float64
	FrontCenter     float64
	SeatTubeLength  float64
	StandoverHeight float64
}

// FrameModel represents a unified bike frame family in memory.
type FrameModel struct {
	ID           string
	Manufacturer string
	Model        string
	Year         int
	Category     BikeCategory
	Notes        string
	// TODO: WheelSize should be a type
	WheelSize          string
	HeadsetCupType     HeadsetCupType
	GeoHeadsetSpacerMm float64
	GeoSlopMm          *GeoSlopMm
	ChartPrecision     ChartPrecision
	ActualStaDeg       Deg
	SeatTubeOffsetXMm  Opt[float64]
	ChainstayRangeMm   []float64
	Sizes              []string
	DefaultSize        string
	CadFork            ForkRef
	Forks              []ForkRef
	Geometries         map[string]SizeGeometry

	// Cockpit & Stem/Spacer Envelope
	MaxSpacerMm         float64
	CanonicalStem       StemSpec
	Stems               []string
	ExpectedStemRangeMm []float64
}

// Clone returns a deep copy of the FrameModel.
func (fm FrameModel) Clone() FrameModel {
	out := fm
	if fm.GeoSlopMm != nil {
		slop := *fm.GeoSlopMm
		out.GeoSlopMm = &slop
	}
	if fm.ChainstayRangeMm != nil {
		out.ChainstayRangeMm = append([]float64(nil), fm.ChainstayRangeMm...)
	}
	if fm.Sizes != nil {
		out.Sizes = append([]string(nil), fm.Sizes...)
	}
	if fm.Forks != nil {
		out.Forks = append([]ForkRef(nil), fm.Forks...)
	}
	if fm.Stems != nil {
		out.Stems = append([]string(nil), fm.Stems...)
	}
	if fm.ExpectedStemRangeMm != nil {
		out.ExpectedStemRangeMm = append([]float64(nil), fm.ExpectedStemRangeMm...)
	}
	if fm.Geometries != nil {
		out.Geometries = make(map[string]SizeGeometry, len(fm.Geometries))
		maps.Copy(out.Geometries, fm.Geometries)
	}
	return out
}

func (fm *FrameModel) GetExpectedStemRangeMm() (float64, float64, bool) {
	if len(fm.ExpectedStemRangeMm) == 2 && fm.ExpectedStemRangeMm[0] > 0 && fm.ExpectedStemRangeMm[1] > fm.ExpectedStemRangeMm[0] {
		return fm.ExpectedStemRangeMm[0], fm.ExpectedStemRangeMm[1], true
	}
	return 0, 0, false
}

func (fm *FrameModel) GetMaxSpacerMm() float64 {
	return fm.MaxSpacerMm
}

func (fm *FrameModel) GetSpacersMm() []float64 {
	max := fm.GetMaxSpacerMm()
	if max < 0 {
		return []float64{0}
	}
	var spacers []float64
	for s := 0.0; s <= max+0.001; s += SpacerStepMm {
		spacers = append(spacers, s)
	}
	return spacers
}

func (fm *FrameModel) GetCanonicalStem() StemSpec {
	return fm.CanonicalStem
}

func (fm *FrameModel) HasAdjustableChainstay() bool {
	return len(fm.ChainstayRangeMm) == 2 && fm.ChainstayRangeMm[1] > fm.ChainstayRangeMm[0]
}

func (fm *FrameModel) GetMinChainstay() float64 {
	if fm.HasAdjustableChainstay() {
		return fm.ChainstayRangeMm[0]
	}
	return 0
}

func (fm *FrameModel) GetMaxChainstay() float64 {
	if fm.HasAdjustableChainstay() {
		return fm.ChainstayRangeMm[1]
	}
	return 0
}

func (fm *FrameModel) GetGeoHeadsetSpacerMm() float64 {
	return fm.GeoHeadsetSpacerMm
}

func (fm *FrameModel) GetGeoSlopMm() GeoSlopMm {
	if fm.GeoSlopMm != nil {
		return *fm.GeoSlopMm
	}
	return GeoSlopMm{}
}

func (fm *FrameModel) GetSizes() []string {
	if len(fm.Sizes) > 0 {
		return fm.Sizes
	}
	return []string{"M"}
}

func (fm *FrameModel) GetDefaultSize() string {
	if fm.DefaultSize != "" {
		return fm.DefaultSize
	}
	if len(fm.Sizes) > 0 {
		return fm.Sizes[0]
	}
	return "M"
}

func (fm *FrameModel) GetCadA2c() float64 {
	return fm.CadFork.A2cMm
}

func (fm *FrameModel) GetCadRake() float64 {
	return fm.CadFork.RakeMm
}

func (fm *FrameModel) SupportsRigid() bool {
	for _, f := range fm.Forks {
		if f.TravelMm == 0 && f.A2cMm > 0 {
			return true
		}
	}
	return false
}

func (fm *FrameModel) GetRigidA2c() float64 {
	for _, f := range fm.Forks {
		if f.TravelMm == 0 && f.A2cMm > 0 {
			return f.A2cMm
		}
	}
	return 0
}

func (fm *FrameModel) GetRigidRake() float64 {
	for _, f := range fm.Forks {
		if f.TravelMm == 0 && f.A2cMm > 0 {
			return f.RakeMm
		}
	}
	return 0
}

func (fm *FrameModel) GetDefaultFork() ForkRef {
	if len(fm.Forks) > 0 {
		return fm.Forks[0]
	}
	return fm.CadFork
}

func (fm *FrameModel) GetSizeGeometry(size string) (SizeGeometry, bool) {
	if fm.Geometries == nil {
		return SizeGeometry{}, false
	}
	geo, ok := fm.Geometries[size]
	return geo, ok
}

// EffectiveA2cRange derives the frame's valid operating A2C range [min, max] in mm
// purely from the mathematical union of each fork in Forks.
// Suspension forks expand across standard sag (20% to 30%), with the upper bound extending
// to static A2C (0% sag). Rigid forks contribute a ±5.0mm tolerance window.
func (fm *FrameModel) EffectiveA2cRange() (minA2c float64, maxA2c float64, ok bool) {
	minA2c = math.Inf(1)
	maxA2c = math.Inf(-1)

	processFork := func(ref ForkRef) {
		if ref.A2cMm <= 0 {
			return
		}
		tr := ref.TravelMm
		if tr > 0 {
			st := ref.StaticA2c()
			minA := st - tr*0.30
			maxA := st
			if minA < minA2c {
				minA2c = minA
			}
			if maxA > maxA2c {
				maxA2c = maxA
			}
		} else {
			// Rigid fork
			minA := ref.A2cMm - 5.0
			maxA := ref.A2cMm + 5.0
			if minA < minA2c {
				minA2c = minA
			}
			if maxA > maxA2c {
				maxA2c = maxA
			}
		}
	}

	for _, f := range fm.Forks {
		processFork(f)
	}

	if math.IsInf(minA2c, 1) || math.IsInf(maxA2c, -1) {
		if fm.CadFork.A2cMm > 0 {
			processFork(fm.CadFork)
		}
	}

	if math.IsInf(minA2c, 1) || math.IsInf(maxA2c, -1) {
		return 0, 0, false
	}

	minA2c = math.Round(minA2c*10.0) / 10.0
	maxA2c = math.Round(maxA2c*10.0) / 10.0
	return minA2c, maxA2c, true
}

// GetSupportedForkOptions returns human-readable strings for all legal fork choices on this frame.
func (fm *FrameModel) GetSupportedForkOptions() []string {
	if fm.IsFixedRigidOnly() {
		return []string{fmt.Sprintf("rigid (%.0fmm)", fm.GetRigidA2c())}
	}

	minA2c, maxA2c, ok := fm.EffectiveA2cRange()
	if !ok {
		return []string{"none"}
	}

	var hasRigid bool
	var minTr, maxTr float64
	for _, f := range fm.Forks {
		tr := f.TravelMm
		if tr == 0 {
			hasRigid = true
		} else {
			if minTr == 0 || tr < minTr {
				minTr = tr
			}
			if tr > maxTr {
				maxTr = tr
			}
		}
	}

	var exSummary string
	if minTr > 0 && maxTr > 0 {
		if hasRigid {
			if math.Abs(minTr-maxTr) < 0.1 {
				exSummary = fmt.Sprintf("e.g. rigid, %.0fmm", maxTr)
			} else {
				exSummary = fmt.Sprintf("e.g. rigid, up to %.0fmm", maxTr)
			}
		} else {
			if math.Abs(minTr-maxTr) < 0.1 {
				exSummary = fmt.Sprintf("e.g. %.0fmm", minTr)
			} else {
				exSummary = fmt.Sprintf("e.g. %.0f–%.0fmm", minTr, maxTr)
			}
		}
	} else if hasRigid {
		exSummary = fmt.Sprintf("rigid %.0fmm", fm.GetRigidA2c())
	}

	if exSummary != "" {
		return []string{fmt.Sprintf("%.0f–%.0fmm A2C (%s)", minA2c, maxA2c, exSummary)}
	}
	return []string{fmt.Sprintf("%.0f–%.0fmm A2C", minA2c, maxA2c)}
}

// IsFixedRigidOnly returns true if the frame only supports a rigid fork.
func (fm *FrameModel) IsFixedRigidOnly() bool {
	if !fm.SupportsRigid() {
		return false
	}
	for _, f := range fm.Forks {
		if f.TravelMm > 0 {
			return false
		}
	}
	return true
}

// IsFixedFork returns true if the frame has a single fixed fork travel/A2C.
func (fm *FrameModel) IsFixedFork() bool {
	return len(fm.Forks) == 1
}

// GetDefaultForkSpecString returns a token representation of the default fork for CLI display.
func (fm *FrameModel) GetDefaultForkSpecString() string {
	def := fm.GetDefaultFork()
	if def.Fork != "" {
		return def.Fork
	}
	if def.A2cMm > 0 {
		return fmt.Sprintf("a2c=%.0f", def.A2cMm)
	}
	return "stock"
}
