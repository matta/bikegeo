package model

import (
	"fmt"
	"math"
	"strings"
)

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

// DefaultTireWidthIn is the baseline tire width in inches for steering trail calculations.
const DefaultTireWidthIn = 2.4

// DefaultWheelRadius29Mm is the standard loaded radius (in mm) for a 29" wheel with a 2.4" tire
// (ISO 622 bead seat radius 311.0mm + 2.4 * 25.4mm section height 60.96mm ≈ 372.0mm).
const DefaultWheelRadius29Mm = 372.0

// GeoSlopMm declares chart error that rounding cannot explain, in mm, for a frame.
// Each field is an allowance added on top of the propagated rounding budget of
// that closure check (see ErrorBudget); unset means no allowance. It exists only
// for genuine manufacturer discrepancies and requires a Reason.
type GeoSlopMm struct {
	A2c         Opt[float64]
	FrontCenter Opt[float64]
	Wheelbase   Opt[float64]
	Reason      string
}

func (s GeoSlopMm) HasCustomSlop() bool {
	return s.A2c.IsSet() || s.FrontCenter.IsSet() || s.Wheelbase.IsSet()
}

func (s GeoSlopMm) String() string {
	var parts []string
	if v, ok := s.A2c.Get(); ok {
		parts = append(parts, fmt.Sprintf("a2c: +%.1fmm", v))
	}
	if v, ok := s.FrontCenter.Get(); ok {
		parts = append(parts, fmt.Sprintf("fc: +%.1fmm", v))
	}
	if v, ok := s.Wheelbase.Get(); ok {
		parts = append(parts, fmt.Sprintf("wb: +%.1fmm", v))
	}
	if len(parts) == 0 {
		return "none (rounding budget only)"
	}
	return strings.Join(parts, ", ")
}

// ChartPrecision holds the rounding half-width of each published chart column:
// a value printed as 627 could be anything in 627 ± 0.5 before the manufacturer
// rounded it. Lengths are in mm, HtaDeg in degrees. A zero field means "not yet
// inferred"; geometry.InferChartPrecision derives it from a frame's size chart.
type ChartPrecision struct {
	ReachMm       float64
	StackMm       float64
	HtaDeg        Deg
	HtlMm         float64
	BbDropMm      float64
	ChainstayMm   float64
	WheelbaseMm   float64
	FrontCenterMm float64
}

// BudgetTier classifies how much of an ErrorBudget a residual consumes.
type BudgetTier string

const (
	// TierLengthRounding: explained by rounding of published lengths alone.
	TierLengthRounding BudgetTier = "length_rounding"
	// TierAngleRounding: explained only if the published head angle is also rounded.
	TierAngleRounding BudgetTier = "angle_rounding"
	// TierAllowance: beyond rounding, covered only by the frame's declared GeoSlopMm allowance.
	TierAllowance BudgetTier = "allowance"
	// TierExceeds: not explained by rounding plus any declared allowance.
	TierExceeds BudgetTier = "exceeds"
)

// budgetEpsilonMm absorbs floating-point noise when a residual sits exactly on a budget boundary.
const budgetEpsilonMm = 1e-9

// BudgetTerm is the worst-case contribution (in mm) that rounding of one published
// chart input can make to a residual.
type BudgetTerm struct {
	Name           string
	HalfWidth      float64 // rounding half-width of the input, in its own unit
	IsAngle        bool    // true when HalfWidth is in degrees rather than mm
	ContributionMm float64
}

// ErrorBudget is the worst-case discrepancy that rounding of the published chart
// inputs can produce in one geometric closure check, broken down per input.
// Each term is the first-order sensitivity of the residual to that input times
// the input's rounding half-width, so inputs that appear several times in an
// equation (BB drop, head angle) contribute coherently rather than being
// double-counted.
type ErrorBudget struct {
	Residual  float64      // signed, unrounded discrepancy in mm
	Terms     []BudgetTerm // largest contribution first
	LengthTol float64      // sum of the length terms
	AngleTol  float64      // sum of the angle terms
	Allowance float64      // declared GeoSlopMm allowance beyond rounding
}

// RoundingTol is the largest discrepancy that rounding alone can explain.
func (b ErrorBudget) RoundingTol() float64 {
	return b.LengthTol + b.AngleTol
}

// Tol is the largest discrepancy that passes: rounding plus any declared allowance.
func (b ErrorBudget) Tol() float64 {
	return b.RoundingTol() + b.Allowance
}

// Saturation is |Residual| as a fraction of RoundingTol: at most 1.0 means the
// discrepancy can be explained entirely by rounding of the published inputs.
func (b ErrorBudget) Saturation() float64 {
	abs := math.Abs(b.Residual)
	tol := b.RoundingTol()
	if tol <= 0 {
		if abs <= budgetEpsilonMm {
			return 0
		}
		return math.Inf(1)
	}
	return abs / tol
}

// Tier reports which part of the budget the residual needs.
func (b ErrorBudget) Tier() BudgetTier {
	abs := math.Abs(b.Residual)
	switch {
	case abs <= b.LengthTol+budgetEpsilonMm:
		return TierLengthRounding
	case abs <= b.RoundingTol()+budgetEpsilonMm:
		return TierAngleRounding
	case abs <= b.Tol()+budgetEpsilonMm:
		return TierAllowance
	default:
		return TierExceeds
	}
}

// Within reports whether the residual passes (any tier but TierExceeds).
func (b ErrorBudget) Within() bool {
	return b.Tier() != TierExceeds
}

// NeededAllowance is how far the residual lies beyond rounding (0 when rounding suffices).
func (b ErrorBudget) NeededAllowance() float64 {
	return math.Max(0, math.Abs(b.Residual)-b.RoundingTol())
}

// BikeGeometry represents materialized bicycle frame geometry for a single size.
type BikeGeometry struct {
	ID           string
	Manufacturer string
	Model        string
	Size         string
	Year         int
	Category     BikeCategory
	Notes        string

	// Primary Frame Dimensions (mm / deg)
	Reach     float64
	Stack     float64
	HtaDeg    Deg
	StaDeg    Deg
	Htl       float64
	BbDrop    float64
	ForkRake  float64
	Wheelbase float64

	// Optional Frame Dimensions
	ChainstayLength  float64
	ChainstayRangeMm []float64
	FrontCenter      float64
	SeatTubeLength   float64
	StandoverHeight  float64
	WheelSize        string

	// Published CAD & Hardware Assumptions
	CadFork            ForkRef
	ForkA2c            float64
	HeadsetCupType     HeadsetCupType
	GeoHeadsetSpacerMm float64
	GeoSlopMm          *GeoSlopMm
	// ChartPrecision is the rounding half-width of each published column, inferred
	// from the whole size chart when materialized from a FrameModel. Zero fields
	// are inferred from this size's own values.
	ChartPrecision ChartPrecision

	// Bent / Offset Seat Tube
	ActualStaDeg      Deg
	SeatTubeOffsetXMm Opt[float64]

	// Cockpit & Stem/Spacer Envelope
	MaxSpacerMm         float64
	CanonicalStem       StemSpec
	Stems               []StemSpec
	ExpectedStemRangeMm []float64
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
	return b.ActualStaDeg > 0 && (b.ActualStaDeg-b.StaDeg).Abs() > 0.001
}

func (b BikeGeometry) GetGeoHeadsetSpacerMm() float64 {
	return b.GeoHeadsetSpacerMm
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

// WheelRadiusMm returns the wheel radius in mm based on frame WheelSize and baseline 2.4" tires.
func (b BikeGeometry) WheelRadiusMm() float64 {
	switch strings.TrimSpace(b.WheelSize) {
	case "27.5", "650b", "650B":
		return 353.0
	case "26":
		return 340.0
	default:
		return DefaultWheelRadius29Mm
	}
}
