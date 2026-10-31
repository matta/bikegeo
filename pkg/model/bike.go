package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Float64Ptr returns a pointer to v, used for explicit JSON numeric fields where 0 is valid.
func Float64Ptr(v float64) *float64 {
	return &v
}

type HeadsetCupType string

const (
	CupEC34 HeadsetCupType = "EC34"
	CupEC44 HeadsetCupType = "EC44"
	CupZS44 HeadsetCupType = "ZS44"
	CupZS56 HeadsetCupType = "ZS56"
	CupIS42 HeadsetCupType = "IS42"
	CupIS52 HeadsetCupType = "IS52"
)

// CupStackMm returns the physical lower headset cup stack height in mm for a 1-1/8" straight or 1.5" tapered fork steerer.
func (c HeadsetCupType) CupStackMm() float64 {
	switch c {
	case CupEC34, CupEC44:
		return 12.0
	case CupZS44, CupZS56:
		return 2.0
	case CupIS42, CupIS52:
		return 0.0
	default:
		return 0.0
	}
}

// ExpectedSpacerRangeMm returns the typical physical range [min, max] in mm for this lower headset cup standard.
func (c HeadsetCupType) ExpectedSpacerRangeMm() (float64, float64) {
	switch c {
	case CupEC34, CupEC44:
		return 10.0, 14.0
	case CupZS44, CupZS56:
		return 0.0, 4.0
	case CupIS42, CupIS52:
		return 0.0, 2.0
	default:
		return 0.0, 0.0
	}
}

// IsValid returns true if the HeadsetCupType is a recognized non-empty SHIS standard.
func (c HeadsetCupType) IsValid() bool {
	switch c {
	case CupEC34, CupEC44, CupZS44, CupZS56, CupIS42, CupIS52:
		return true
	default:
		return false
	}
}

type BikeCategory string

const (
	CategoryHardtail       BikeCategory = "hardtail"
	CategoryFullSuspension BikeCategory = "full-suspension"
	CategoryGravel         BikeCategory = "gravel"
	CategoryRigid          BikeCategory = "rigid"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
)

// DefaultGeoSlopMm is the default maximum allowed discrepancy (in mm) for geometric consistency checks.
const DefaultGeoSlopMm = 1.0

// GeoSlopMm defines optional measure-specific validation tolerances (in mm) for a frame.
// Any nil field defaults to DefaultGeoSlopMm (1.0mm).
type GeoSlopMm struct {
	A2c         *float64 `json:"a2c,omitempty"`
	FrontCenter *float64 `json:"front_center,omitempty"`
	Wheelbase   *float64 `json:"wheelbase,omitempty"`
	CrossConfig *float64 `json:"cross_config,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

func (s GeoSlopMm) HasCustomSlop() bool {
	return s.A2c != nil || s.FrontCenter != nil || s.Wheelbase != nil || s.CrossConfig != nil
}

func (s GeoSlopMm) GetA2c() float64 {
	if s.A2c != nil {
		return *s.A2c
	}
	return DefaultGeoSlopMm
}

func (s GeoSlopMm) GetFrontCenter() float64 {
	if s.FrontCenter != nil {
		return *s.FrontCenter
	}
	return DefaultGeoSlopMm
}

func (s GeoSlopMm) GetWheelbase() float64 {
	if s.Wheelbase != nil {
		return *s.Wheelbase
	}
	return DefaultGeoSlopMm
}

func (s GeoSlopMm) GetCrossConfig() float64 {
	if s.CrossConfig != nil {
		return *s.CrossConfig
	}
	return DefaultGeoSlopMm
}

func (s GeoSlopMm) String() string {
	var parts []string
	if s.A2c != nil {
		parts = append(parts, fmt.Sprintf("a2c: ±%.1fmm", *s.A2c))
	}
	if s.FrontCenter != nil {
		parts = append(parts, fmt.Sprintf("fc: ±%.1fmm", *s.FrontCenter))
	}
	if s.Wheelbase != nil {
		parts = append(parts, fmt.Sprintf("wb: ±%.1fmm", *s.Wheelbase))
	}
	if s.CrossConfig != nil {
		parts = append(parts, fmt.Sprintf("cross: ±%.1fmm", *s.CrossConfig))
	}
	if len(parts) == 0 {
		return "defaults (a2c: ±1.0mm, fc: ±1.0mm, wb: dynamic, cross: ±1.0mm)"
	}
	return strings.Join(parts, ", ")
}

// BikeGeometry represents bicycle frame geometry data from disk JSON.
type BikeGeometry struct {
	ID           string       `json:"id"`
	Manufacturer string       `json:"manufacturer"`
	Model        string       `json:"model"`
	Size         string       `json:"size"`
	Year         int          `json:"year,omitempty"`
	Category     BikeCategory `json:"category,omitempty"`
	Notes        string       `json:"notes,omitempty"`

	// Primary Frame Dimensions (mm / deg)
	Reach     float64 `json:"reach"`
	Stack     float64 `json:"stack"`
	HtaDeg    float64 `json:"hta_deg"`
	StaDeg    float64 `json:"sta_deg"`
	Htl       float64 `json:"htl"`
	BbDrop    float64 `json:"bb_drop"`
	ForkRake  float64 `json:"fork_rake"`
	Wheelbase float64 `json:"wheelbase"`

	// Optional Frame Dimensions
	ChainstayLength  float64   `json:"chainstay_length,omitempty"`
	ChainstayRangeMm []float64 `json:"chainstay_range_mm,omitempty"`
	FrontCenter      float64   `json:"front_center,omitempty"`
	SeatTubeLength   float64   `json:"seat_tube_length,omitempty"`
	StandoverHeight  float64   `json:"standover_height,omitempty"`
	WheelSize        string    `json:"wheel_size,omitempty"`

	// Published CAD & Hardware Assumptions
	CadFork            ForkRef        `json:"cad_fork"`
	ForkA2c            float64        `json:"fork_a2c,omitempty"`
	HeadsetCupType     HeadsetCupType `json:"headset_cup_type"`
	GeoHeadsetSpacerMm *float64       `json:"geo_headset_spacer_mm"`
	GeoSlopMm          *GeoSlopMm     `json:"geo_slop_mm,omitempty"`

	// Bent / Offset Seat Tube
	ActualStaDeg      float64  `json:"actual_sta_deg,omitempty"`
	SeatTubeOffsetXMm *float64 `json:"seat_tube_offset_x_mm,omitempty"`

	// Cockpit & Stem/Spacer Envelope
	MaxSpacerMm   *float64   `json:"max_spacer_mm,omitempty"`
	CanonicalStem *StemSpec  `json:"canonical_stem,omitempty"`
	Stems         []StemSpec `json:"stems,omitempty"`
}

func (b BikeGeometry) GetCadA2c() float64 {
	if b.CadFork.A2cMm > 0 {
		return b.CadFork.A2cMm
	}
	return b.ForkA2c
}

func (b BikeGeometry) GetCadRake() float64 {
	if b.CadFork.RakeMm > 0 {
		return b.CadFork.RakeMm
	}
	return b.ForkRake
}

func (b BikeGeometry) HasBentSeatTube() bool {
	return b.ActualStaDeg > 0 && math.Abs(b.ActualStaDeg-b.StaDeg) > 0.001
}

func (b BikeGeometry) GetActualStaDeg() float64 {
	if b.ActualStaDeg > 0 {
		return b.ActualStaDeg
	}
	return b.StaDeg
}

func (b BikeGeometry) GetSeatTubeOffsetXMm(stack, nominalSta float64) float64 {
	if b.SeatTubeOffsetXMm != nil {
		return *b.SeatTubeOffsetXMm
	}
	if !b.HasBentSeatTube() {
		return 0
	}
	if stack <= 0 {
		stack = b.Stack
	}
	if nominalSta <= 0 {
		nominalSta = b.StaDeg
	}
	cotAct := 1.0 / math.Tan(b.ActualStaDeg*math.Pi/180.0)
	cotNom := 1.0 / math.Tan(nominalSta*math.Pi/180.0)
	return stack * (cotAct - cotNom)
}

func (b BikeGeometry) CalculateEffectiveSta(seatHeight, stack, nominalSta float64) float64 {
	if !b.HasBentSeatTube() || seatHeight <= 0 {
		if nominalSta > 0 {
			return nominalSta
		}
		return b.StaDeg
	}
	ox := b.GetSeatTubeOffsetXMm(stack, nominalSta)
	actRad := b.ActualStaDeg * math.Pi / 180.0
	sinVal := (ox * math.Sin(actRad)) / seatHeight
	if sinVal > 1.0 {
		sinVal = 1.0
	} else if sinVal < -1.0 {
		sinVal = -1.0
	}
	alphaRad := math.Asin(sinVal)
	return b.ActualStaDeg + (alphaRad * 180.0 / math.Pi)
}

func (b BikeGeometry) GetGeoHeadsetSpacerMm() float64 {
	if b.GeoHeadsetSpacerMm != nil {
		return *b.GeoHeadsetSpacerMm
	}
	return 0
}

func (b BikeGeometry) GetGeoSlopMm() GeoSlopMm {
	if b.GeoSlopMm != nil {
		return *b.GeoSlopMm
	}
	return GeoSlopMm{}
}

func (b BikeGeometry) HasAdjustableChainstay() bool {
	return len(b.ChainstayRangeMm) == 2 && b.ChainstayRangeMm[1] > b.ChainstayRangeMm[0]
}

func (b BikeGeometry) GetMinChainstay() float64 {
	if b.HasAdjustableChainstay() {
		return b.ChainstayRangeMm[0]
	}
	return 0
}

func (b BikeGeometry) GetMaxChainstay() float64 {
	if b.HasAdjustableChainstay() {
		return b.ChainstayRangeMm[1]
	}
	return 0
}

// StemThicknessMm is the fixed axial height (in mm) of the stem clamp on the steerer tube.
const StemThicknessMm = 40.0

// SpacerStepMm is the fixed increment (in mm) for steerer spacers from 0 up to MaxSpacerMm.
const SpacerStepMm = 5.0

// StemLengthStepMm is the fixed increment (in mm) for stem lengths in catalog stem ranges.
const StemLengthStepMm = 10.0

// StemSpec represents a stem configuration (length in mm, angle in degrees).
type StemSpec struct {
	LengthMm float64 `json:"length_mm"`
	AngleDeg float64 `json:"angle_deg"`
}

// StemCatalogEntry encapsulates a stem set in data/stems.json.
type StemCatalogEntry struct {
	ID            string    `json:"id"`
	LengthRangeMm []float64 `json:"length_range_mm"`
	AnglesDeg     []float64 `json:"angles_deg"`
}

// Expand generates the Cartesian product of stems across LengthRangeMm (in StemLengthStepMm steps) and AnglesDeg.
func (e StemCatalogEntry) Expand() []StemSpec {
	if len(e.LengthRangeMm) != 2 || e.LengthRangeMm[0] <= 0 || e.LengthRangeMm[1] < e.LengthRangeMm[0] {
		return nil
	}
	var stems []StemSpec
	for l := e.LengthRangeMm[0]; l <= e.LengthRangeMm[1]+0.001; l += StemLengthStepMm {
		for _, a := range e.AnglesDeg {
			stems = append(stems, StemSpec{
				LengthMm: math.Round(l*10.0) / 10.0,
				AngleDeg: a,
			})
		}
	}
	return stems
}

// DefaultSeatpostSetbackMm is the perpendicular rearward offset (in mm) of the seatpost clamp
// from the seat tube centerline (modeled on a Redshift ShockStop suspension seatpost: 7mm static, up to 12mm sagged).
const DefaultSeatpostSetbackMm = 7.0

// DefaultSaddleBrpOffsetMm is the perpendicular rearward offset (in mm) from the center of the
// saddle rails to the saddle's 70mm-width Biomechanical Reference Point (BRP).
// For SQlab stepped Ergowave saddles with extended rails, the effective BRP sits at the rail center (0.0mm).
const DefaultSaddleBrpOffsetMm = 0.0

// DefaultBrpOffsetMm is the total perpendicular offset (in mm) from the seat tube centerline
// to the BRP when the saddle is clamped at dead center (0mm) on the rails (7.0 + 0.0 = 7.0mm).
const DefaultBrpOffsetMm = DefaultSeatpostSetbackMm + DefaultSaddleBrpOffsetMm

// SaddleRailIdealMarginMm is the fore-aft rail offset window (±mm from dead center of rails) considered ideal.
const SaddleRailIdealMarginMm = 5.0

// SaddleRailMaxAdjustmentMm is the maximum safe fore-aft adjustment (±mm from dead center of rails; 40mm total window).
const SaddleRailMaxAdjustmentMm = 20.0

// SaddleRailStepMm is the discrete step size (in mm) when mapping candidate BRP positions along the saddle rails.
const SaddleRailStepMm = 1.0

// BRPPoint represents a saddle Biomechanical Reference Point (70mm width mark)
// for a specific fore-aft rail offset along the constant-radius SaddleHeightMm arc.
type BRPPoint struct {
	SeatpostSetbackMm float64 `json:"seatpost_setback_mm"`
	SaddleBrpOffsetMm float64 `json:"saddle_brp_offset_mm"`
	RailOffsetMm      float64 `json:"rail_offset_mm"`
	TotalPerpOffsetMm float64 `json:"total_perp_offset_mm"`
	BrpX              float64 `json:"brp_x"`
	BrpY              float64 `json:"brp_y"`
	BrpSetbackMm      float64 `json:"brp_setback_mm"`
	BrpAngleFromBbDeg float64 `json:"brp_angle_from_bb_deg"`
}

// HBPoint represents a mapped handlebar-stem intersection point (HB) and its resulting Saddle-Handlebar distance and drop.
type HBPoint struct {
	Stem           StemSpec `json:"stem"`
	SpacerMm       float64  `json:"spacer_mm"`
	HbX            float64  `json:"hb_x"`
	HbY            float64  `json:"hb_y"`
	SaddleHbMm     float64  `json:"saddle_hb_mm"`
	SaddleHbDropMm float64  `json:"saddle_hb_drop_mm"`
}

func (b BikeGeometry) GetMaxSpacerMm() float64 {
	if b.MaxSpacerMm != nil {
		return *b.MaxSpacerMm
	}
	return 0
}

func (b BikeGeometry) GetSpacersMm() []float64 {
	max := b.GetMaxSpacerMm()
	if max < 0 {
		return []float64{0}
	}
	var spacers []float64
	for s := 0.0; s <= max+0.001; s += SpacerStepMm {
		spacers = append(spacers, s)
	}
	return spacers
}

func (b BikeGeometry) GetCanonicalStem() StemSpec {
	if b.CanonicalStem != nil {
		return *b.CanonicalStem
	}
	return StemSpec{}
}

// ForkCatalogOption specifies an available (A2C, rake) pair for a catalog fork.
type ForkCatalogOption struct {
	A2cMm  float64 `json:"a2c_mm"`
	RakeMm float64 `json:"rake_mm"`
}

// ForkCatalogEntry encapsulates a fork model in data/forks.json.
type ForkCatalogEntry struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	TravelMm    float64             `json:"travel_mm"`
	Standard    bool                `json:"standard,omitempty"`
	Options     []ForkCatalogOption `json:"options"`
	Description string              `json:"description,omitempty"`
}

// ForkPreset is an alias for ForkCatalogEntry.
type ForkPreset = ForkCatalogEntry

// ForkRef encapsulates a reference to a catalog fork with resolved A2C and rake.
type ForkRef struct {
	Fork   string   `json:"fork"`
	SagPct *float64 `json:"sag_pct,omitempty"`
	A2cMm  float64  `json:"a2c_mm"`
	RakeMm float64  `json:"rake_mm"`
}

func (r ForkRef) GetSagPct() float64 {
	if r.SagPct != nil {
		return *r.SagPct
	}
	return 0.0
}

func (r ForkRef) ForkTravel() float64 {
	parts := strings.Split(r.Fork, "_")
	for _, p := range parts {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(p, "mm"), 64); err == nil && v >= 60 && v <= 200 {
			return v
		}
	}
	return 0.0
}

func (r ForkRef) StaticA2c() float64 {
	tr := r.ForkTravel()
	if tr > 0 && r.SagPct != nil && *r.SagPct > 0 {
		return r.A2cMm + tr*(*r.SagPct/100.0)
	}
	return r.A2cMm
}

// ForkSpec encapsulates physical and geometric properties of a fork (for compatibility).
type ForkSpec struct {
	ID                 string  `json:"id,omitempty"`
	Name               string  `json:"name,omitempty"`
	TravelMm           float64 `json:"travel_mm"`
	A2c                float64 `json:"a2c"`
	Rake               float64 `json:"rake"`
	StandardSagPercent float64 `json:"standard_sag_percent,omitempty"`
	Description        string  `json:"description,omitempty"`
	IsInferred         bool    `json:"is_inferred,omitempty"`
}

// SizeGeometry represents primary and optional frame dimensions for one size.
type SizeGeometry struct {
	Reach           float64 `json:"reach"`
	Stack           float64 `json:"stack"`
	HtaDeg          float64 `json:"hta_deg"`
	StaDeg          float64 `json:"sta_deg"`
	Htl             float64 `json:"htl"`
	BbDrop          float64 `json:"bb_drop"`
	Wheelbase       float64 `json:"wheelbase"`
	ChainstayLength float64 `json:"chainstay_length,omitempty"`
	FrontCenter     float64 `json:"front_center,omitempty"`
	SeatTubeLength  float64 `json:"seat_tube_length,omitempty"`
	StandoverHeight float64 `json:"standover_height,omitempty"`
}

// FrameModel represents a unified bike frame family on disk and in memory.
type FrameModel struct {
	ID                 string                  `json:"id"`
	Manufacturer       string                  `json:"manufacturer"`
	Model              string                  `json:"model"`
	Year               int                     `json:"year,omitempty"`
	Category           BikeCategory            `json:"category,omitempty"`
	Notes              string                  `json:"notes,omitempty"`
	WheelSize          string                  `json:"wheel_size,omitempty"`
	HeadsetCupType     HeadsetCupType          `json:"headset_cup_type"`
	GeoHeadsetSpacerMm *float64                `json:"geo_headset_spacer_mm"`
	GeoSlopMm          *GeoSlopMm              `json:"geo_slop_mm,omitempty"`
	ActualStaDeg       float64                 `json:"actual_sta_deg,omitempty"`
	SeatTubeOffsetXMm  *float64                `json:"seat_tube_offset_x_mm,omitempty"`
	ChainstayRangeMm   []float64               `json:"chainstay_range_mm,omitempty"`
	Sizes              []string                `json:"sizes"`
	DefaultSize        string                  `json:"default_size,omitempty"`
	CadFork            ForkRef                 `json:"cad_fork"`
	Forks              []ForkRef               `json:"forks"`
	Geometries         map[string]SizeGeometry `json:"geometries"`

	// Cockpit & Stem/Spacer Envelope
	MaxSpacerMm   *float64  `json:"max_spacer_mm"`
	CanonicalStem *StemSpec `json:"canonical_stem"`
	Stems         []string  `json:"stems"`
}

func (fm *FrameModel) GetMaxSpacerMm() float64 {
	if fm.MaxSpacerMm != nil {
		return *fm.MaxSpacerMm
	}
	return 0
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
	if fm.CanonicalStem != nil {
		return *fm.CanonicalStem
	}
	return StemSpec{}
}

func (fm *FrameModel) HasBentSeatTube() bool {
	return fm.ActualStaDeg > 0
}

func (fm *FrameModel) GetActualStaDeg() float64 {
	return fm.ActualStaDeg
}

func (fm *FrameModel) GetSeatTubeOffsetXMm(stack, nominalSta float64) float64 {
	if fm.SeatTubeOffsetXMm != nil {
		return *fm.SeatTubeOffsetXMm
	}
	if !fm.HasBentSeatTube() || stack <= 0 || nominalSta <= 0 {
		return 0
	}
	cotAct := 1.0 / math.Tan(fm.ActualStaDeg*math.Pi/180.0)
	cotNom := 1.0 / math.Tan(nominalSta*math.Pi/180.0)
	return stack * (cotAct - cotNom)
}

func (fm *FrameModel) CalculateEffectiveSta(seatHeight, stack, nominalSta float64) float64 {
	if !fm.HasBentSeatTube() || seatHeight <= 0 || nominalSta <= 0 {
		return nominalSta
	}
	ox := fm.GetSeatTubeOffsetXMm(stack, nominalSta)
	actRad := fm.ActualStaDeg * math.Pi / 180.0
	sinVal := (ox * math.Sin(actRad)) / seatHeight
	if sinVal > 1.0 {
		sinVal = 1.0
	} else if sinVal < -1.0 {
		sinVal = -1.0
	}
	alphaRad := math.Asin(sinVal)
	return fm.ActualStaDeg + (alphaRad * 180.0 / math.Pi)
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
	if fm.GeoHeadsetSpacerMm != nil {
		return *fm.GeoHeadsetSpacerMm
	}
	return 0
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
		if f.ForkTravel() == 0 && f.A2cMm > 0 {
			return true
		}
	}
	return false
}

func (fm *FrameModel) GetRigidA2c() float64 {
	for _, f := range fm.Forks {
		if f.ForkTravel() == 0 && f.A2cMm > 0 {
			return f.A2cMm
		}
	}
	return 0
}

func (fm *FrameModel) GetRigidRake() float64 {
	for _, f := range fm.Forks {
		if f.ForkTravel() == 0 && f.A2cMm > 0 {
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
// Suspension forks expand across standard sag (15% to 30%), with the upper bound extending
// to static A2C (0% sag). Rigid forks contribute a ±5.0mm tolerance window.
func (fm *FrameModel) EffectiveA2cRange() (minA2c float64, maxA2c float64, ok bool) {
	minA2c = math.Inf(1)
	maxA2c = math.Inf(-1)

	processFork := func(ref ForkRef) {
		if ref.A2cMm <= 0 {
			return
		}
		tr := ref.ForkTravel()
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
		tr := f.ForkTravel()
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
		if f.ForkTravel() > 0 {
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

type ValidationIssue struct {
	ID         string   `json:"id"`
	Field      string   `json:"field,omitempty"`
	Severity   Severity `json:"severity"`
	Title      string   `json:"title"`
	Message    string   `json:"message"`
	Details    string   `json:"details,omitempty"`
	Expected   string   `json:"expected,omitempty"`
	Actual     string   `json:"actual,omitempty"`
	Difference float64  `json:"difference,omitempty"`
}

type PublishMode string

const (
	ModeStaticCad     PublishMode = "static_cad"
	ModeSaggedCad     PublishMode = "sagged_cad_15_25"
	ModeMismatchedCad PublishMode = "mismatched_cad"
	ModeUnspecified   PublishMode = "unspecified"
)

type ValidationReport struct {
	BikeID              string            `json:"bikeId"`
	IsValid             bool              `json:"isValid"`
	Issues              []ValidationIssue `json:"issues"`
	GeometricA2c        float64           `json:"geometricA2c"`
	StatedA2c           float64           `json:"statedA2c,omitempty"`
	ExpectedForkA2c     float64           `json:"expectedForkA2c,omitempty"`
	GeoHeadsetSpacerMm  float64           `json:"geoHeadsetSpacerMm"`
	CupStack            float64           `json:"cupStack"`
	TotalStructuralA2c  float64           `json:"totalStructuralA2c"`
	A2cDifference       *float64          `json:"a2cDifference,omitempty"`
	InferredPublishMode PublishMode       `json:"inferredPublishMode"`
	FrontCenterCalc     float64           `json:"frontCenterCalc,omitempty"`
	FrontCenterDiff     *float64          `json:"frontCenterDiff,omitempty"`
}

// FrameValidationReport holds the complete validation result across all sizes of a FrameModel.
type FrameValidationReport struct {
	FrameID     string                      `json:"frame_id"`
	IsValid     bool                        `json:"is_valid"`
	Issues      []ValidationIssue           `json:"issues,omitempty"`
	SizeReports map[string]ValidationReport `json:"size_reports,omitempty"`
}

// ErrorSummary returns a human-readable summary of all validation errors on the frame.
func (r FrameValidationReport) ErrorSummary() string {
	if r.IsValid {
		return ""
	}
	var msgs []string
	for _, iss := range r.Issues {
		if iss.Severity == SeverityError {
			msgs = append(msgs, fmt.Sprintf("%s: %s", iss.Title, iss.Message))
		}
	}
	for sz, rep := range r.SizeReports {
		for _, iss := range rep.Issues {
			if iss.Severity == SeverityError {
				msgs = append(msgs, fmt.Sprintf("[Size %s] %s: %s", sz, iss.Title, iss.Message))
			}
		}
	}
	if len(msgs) == 0 {
		return fmt.Sprintf("frame %q is in an invalid state", r.FrameID)
	}
	return fmt.Sprintf("frame %q failed validation: %s", r.FrameID, msgs[0])
}

// RiderProfile encapsulates anthropometric measurements and baseline cockpit reference parameters.
type RiderProfile struct {
	HeightIn                      float64 `json:"rider_height_in"`
	InseamIn                      float64 `json:"rider_inseam_in"`
	SaddleHeightMm                float64 `json:"saddle_height_mm"`
	RiderWeightLbs                float64 `json:"rider_weight_lbs"`
	BikeWeightLbs                 float64 `json:"bike_weight_lbs"`
	BaselineStemMm                float64 `json:"baseline_stem_mm"`
	BaselineStemAngleDeg          float64 `json:"baseline_stem_angle_deg"`
	BaselineBarRiseMm             float64 `json:"baseline_bar_rise_mm"`
	BaselineBarSweepDeg           float64 `json:"baseline_bar_sweep_deg"`
	BaselineBarWidthMm            float64 `json:"baseline_bar_width_mm"`
	BaselineSaddleSetbackOffsetMm float64 `json:"baseline_saddle_setback_offset_mm"`
}

// DefaultRiderProfile returns default anthropometric and cockpit parameters
// based on a 6'2" rider (35" inseam) on the Otso Fenrir Large baseline build.
func DefaultRiderProfile() RiderProfile {
	return RiderProfile{
		HeightIn:                      74.0, // 6'2"
		InseamIn:                      35.0, // 35"
		SaddleHeightMm:                780.0,
		RiderWeightLbs:                190.0,
		BikeWeightLbs:                 28.0,
		BaselineStemMm:                110.0,
		BaselineStemAngleDeg:          -6.0,
		BaselineBarRiseMm:             50.0,
		BaselineBarSweepDeg:           20.0,
		BaselineBarWidthMm:            800.0,
		BaselineSaddleSetbackOffsetMm: 30.0,
	}
}

type CanonicalConfig struct {
	TargetA2c    float64      `json:"targetA2c"`
	SagPercent   float64      `json:"sagPercent"`
	Name         string       `json:"name,omitempty"`
	RiderProfile RiderProfile `json:"riderProfile"`
}

// ComparisonSetup represents one column in a comparison matrix.
type ComparisonSetup struct {
	FrameID      string  `json:"frame_id"`
	Size         string  `json:"size"`
	A2cMm        float64 `json:"a2c_mm"`
	RakeMm       float64 `json:"rake_mm"`
	DisplayLabel string  `json:"display_label"`
}

// SavedComparisonState persists the user's most recent comparison in data/last_compare.json.
type SavedComparisonState struct {
	RiderProfile
	Transpose    bool              `json:"transpose"`
	RefIndex     int               `json:"ref_index,omitempty"`
	IdealProfile string            `json:"ideal_profile,omitempty"`
	Columns      []ComparisonSetup `json:"columns"`
}

// NormalizeRiderProfile fills any zero values in RiderProfile with standard defaults.
func (s *SavedComparisonState) NormalizeRiderProfile() {
	def := DefaultRiderProfile()
	if s.HeightIn <= 0 {
		s.HeightIn = def.HeightIn
	}
	if s.InseamIn <= 0 {
		s.InseamIn = def.InseamIn
	}
	if s.SaddleHeightMm <= 0 {
		s.SaddleHeightMm = def.SaddleHeightMm
	}
	if s.RiderWeightLbs <= 0 {
		s.RiderWeightLbs = def.RiderWeightLbs
	}
	if s.BikeWeightLbs <= 0 {
		s.BikeWeightLbs = def.BikeWeightLbs
	}
	if s.BaselineStemMm <= 0 {
		s.BaselineStemMm = def.BaselineStemMm
	}
	if s.BaselineStemAngleDeg == 0 {
		s.BaselineStemAngleDeg = def.BaselineStemAngleDeg
	}
	if s.BaselineBarRiseMm <= 0 {
		s.BaselineBarRiseMm = def.BaselineBarRiseMm
	}
	if s.BaselineBarSweepDeg <= 0 {
		s.BaselineBarSweepDeg = def.BaselineBarSweepDeg
	}
	if s.BaselineBarWidthMm <= 0 {
		s.BaselineBarWidthMm = def.BaselineBarWidthMm
	}
	if s.BaselineSaddleSetbackOffsetMm <= 0 {
		s.BaselineSaddleSetbackOffsetMm = def.BaselineSaddleSetbackOffsetMm
	}
}

type DynamicGeometryResult struct {
	NewHta        float64 `json:"new_hta"`
	NewSta        float64 `json:"new_sta"`
	NewReach      float64 `json:"new_reach"`
	NewStack      float64 `json:"new_stack"`
	NewBbDrop     float64 `json:"new_bb_drop"`
	DeltaAngleDeg float64 `json:"delta_angle_deg"`
	DeltaReach    float64 `json:"delta_reach"`
	DeltaStack    float64 `json:"delta_stack"`
	DeltaBbDrop   float64 `json:"delta_bb_drop"`
	TotalNewA2c   float64 `json:"total_new_a2c"`
}

type BikeFitAnalysis struct {
	StructuralA2c       float64 `json:"structural_a2c"`
	ActiveReach         float64 `json:"active_reach"`
	ActiveStack         float64 `json:"active_stack"`
	ActiveHta           float64 `json:"active_hta"`
	ActiveSta           float64 `json:"active_sta"`
	ActiveBbDrop        float64 `json:"active_bb_drop"`
	Aett                float64 `json:"aett"`
	ActualSetback       float64 `json:"actual_setback"`
	SpareVerticalStack  float64 `json:"spare_vertical_stack"`
	SpareAxialSteerer   float64 `json:"spare_axial_steerer"`
	DeltaAngleDeg       float64 `json:"delta_angle_deg"`
	DeltaReach          float64 `json:"delta_reach"`
	DeltaStack          float64 `json:"delta_stack"`
	DeltaBbDrop         float64 `json:"delta_bb_drop"`
	InferredPublishMode string  `json:"inferredPublishMode"`

	// Weight Distribution & Biomechanical Posture
	WeightDistFrontPct float64 `json:"weight_dist_front_pct"`
	WeightDistRearPct  float64 `json:"weight_dist_rear_pct"`
	BackAngleDeg       float64 `json:"back_angle_deg"`
	FrontCenterCalc    float64 `json:"front_center_calc"`
	RearCenterCalc     float64 `json:"rear_center_calc"`
	SeatToRearCenter   float64 `json:"seat_to_rear_center"`
	WheelbaseCalc      float64 `json:"wheelbase_calc"`
	ActiveChainstay    float64 `json:"active_chainstay"`
	ChainstayDesc      string  `json:"chainstay_desc"`

	// Cockpit BRP & HB Points and Saddle-Handlebar Metrics
	DeadCenterBRP  BRPPoint   `json:"dead_center_brp"`
	SelectedBRP    *BRPPoint  `json:"selected_brp,omitempty"`
	PossibleBRPs   []BRPPoint `json:"possible_brps,omitempty"`
	SaddleHbMm     float64    `json:"saddle_hb_mm"`
	SaddleHbDropMm float64    `json:"saddle_hb_drop_mm"`
	SelectedHB     *HBPoint   `json:"selected_hb,omitempty"`
	PossibleHBs    []HBPoint  `json:"possible_hbs,omitempty"`
}

type BaselineMetrics struct {
	Reach              float64 `json:"reach"`
	Stack              float64 `json:"stack"`
	HtaDeg             float64 `json:"hta_deg"`
	StaDeg             float64 `json:"sta_deg"`
	BbDrop             float64 `json:"bb_drop"`
	A2c                float64 `json:"a2c"`
	Aett               float64 `json:"aett"`
	ActualSetback      float64 `json:"actual_setback"`
	SpareVerticalStack float64 `json:"spare_vertical_stack"`
	SpareAxialSteerer  float64 `json:"spare_axial_steerer"`
	Chainstay          float64 `json:"chainstay"`
	SeatToRearCenter   float64 `json:"seat_to_rear_center"`
	SaddleHbMm         float64 `json:"saddle_hb_mm"`
}

type CanonicalComparisonRow struct {
	Setup         ComparisonSetup  `json:"setup"`
	DisplayLabel  string           `json:"display_label"`
	EffectiveA2c  float64          `json:"effective_a2c"`
	EffectiveRake float64          `json:"effective_rake"`
	Bike          BikeGeometry     `json:"bike"`
	Validation    ValidationReport `json:"validation"`
	Baseline      BaselineMetrics  `json:"baseline"`
	Canonical     BikeFitAnalysis  `json:"canonical"`
}
