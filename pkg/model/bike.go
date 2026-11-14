package model

import (
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"

	"bikegeo/pkg/geom2d"
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

func (b BikeGeometry) GetActualStaDeg() Deg {
	if b.ActualStaDeg > 0 {
		return b.ActualStaDeg
	}
	return b.StaDeg
}

func (b BikeGeometry) GetSeatTubeOffsetXMm(stack float64, nominalSta Deg) float64 {
	if v, ok := b.SeatTubeOffsetXMm.Get(); ok {
		return v
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
	cotAct := 1.0 / b.ActualStaDeg.Angle().Tan()
	cotNom := 1.0 / nominalSta.Angle().Tan()
	return stack * (cotAct - cotNom)
}

func (b BikeGeometry) CalculateEffectiveSta(seatHeight, stack float64, nominalSta Deg) Deg {
	if !b.HasBentSeatTube() || seatHeight <= 0 {
		if nominalSta > 0 {
			return nominalSta
		}
		return b.StaDeg
	}
	ox := b.GetSeatTubeOffsetXMm(stack, nominalSta)
	sinVal := (ox * b.ActualStaDeg.Angle().Sin()) / seatHeight
	if sinVal > 1.0 {
		sinVal = 1.0
	} else if sinVal < -1.0 {
		sinVal = -1.0
	}
	alphaRad := math.Asin(sinVal)
	return b.ActualStaDeg + DegOf(geom2d.FromRad(alphaRad))
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

// StemThicknessMm is the fixed axial height (in mm) of the stem clamp on the steerer tube.
const StemThicknessMm = 40.0

// SpacerStepMm is the fixed increment (in mm) for steerer spacers from 0 up to MaxSpacerMm.
const SpacerStepMm = 5.0

// StemLengthStepMm is the fixed increment (in mm) for stem lengths in catalog stem ranges.
const StemLengthStepMm = 10.0

// StemSpec represents a stem configuration (length in mm, angle in degrees).
type StemSpec struct {
	LengthMm float64
	AngleDeg Deg
}

// StemCatalogEntry encapsulates a stem set in pkg/catalog.
type StemCatalogEntry struct {
	ID            string
	LengthRangeMm []float64
	AnglesDeg     []Deg
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

// SaddleSpec defines a saddle's 2D geometry in the Canonical Saddle Coordinate System,
// where the origin (0.0mm) is the midpoint of the nominal straight rail clamping section,
// positive X (+) is forward toward the nose, and negative X (-) is rearward toward the tail.
// Vertical elevation (Y) is measured upward from the 70mm BRP horizontal reference plane.
type SaddleSpec struct {
	ID                     string
	Name                   string
	RailLengthMm           float64 // Total straight clamping length on rails (e.g. 60mm for SQlab [-30 to +30])
	RailCenterToBrpMm      float64 // Forward distance from rail center to 70mm BRP (+mm toward nose)
	RailCenterToSitBonesMm float64 // Distance from rail center to sit bones (-mm toward tail)
	SitBonesElevationMm    float64 // Vertical elevation of sit bones above the 70mm BRP horizontal plane (+mm upward, e.g. 10.0mm for SQlab Ergolux step)
}

// BrpToSitBonesVec returns the 2D displacement vector from the saddle's 70mm BRP
// to the rider's sit-bone support point in the canonical sagittal plane (+X forward, +Y upward):
//
//	X = RailCenterToSitBonesMm - RailCenterToBrpMm (negative, rearward toward tail)
//	Y = SitBonesElevationMm (positive, upward step above 70mm BRP plane)
func (s SaddleSpec) BrpToSitBonesVec() geom2d.Vec2 {
	return geom2d.Vec2{
		X: s.RailCenterToSitBonesMm - s.RailCenterToBrpMm,
		Y: s.SitBonesElevationMm,
	}
}

// SitBonesToBrpVec returns the 2D displacement vector from the sit-bone support point
// to the saddle's 70mm BRP in the canonical sagittal plane (+X forward, +Y upward):
//
//	SitBonesToBrpVec = -BrpToSitBonesVec()
func (s SaddleSpec) SitBonesToBrpVec() geom2d.Vec2 {
	return s.BrpToSitBonesVec().Scale(-1)
}

// SitBonePointFromBrp translates a 2D BRP coordinate into the saddle's 2D sit-bone support coordinate:
//
//	SitBonePoint = brp + BrpToSitBonesVec()
func (s SaddleSpec) SitBonePointFromBrp(brp geom2d.Point2) geom2d.Point2 {
	return brp.Add(s.BrpToSitBonesVec())
}

// BrpPointFromSitBones translates a 2D sit-bone support coordinate into the saddle's 2D BRP coordinate:
//
//	BrpPoint = sitBones + SitBonesToBrpVec()
func (s SaddleSpec) BrpPointFromSitBones(sitBones geom2d.Point2) geom2d.Point2 {
	return sitBones.Add(s.SitBonesToBrpVec())
}

// TranslateBrpFromRef translates a 2D BRP coordinate measured on a reference saddle
// into the equivalent 2D BRP coordinate on this saddle that preserves the exact 2D sit-bone position:
//
//	TargetBrp = BrpPointFromSitBones(ref.SitBonePointFromBrp(refBrp))
func (s SaddleSpec) TranslateBrpFromRef(ref SaddleSpec, refBrp geom2d.Point2) geom2d.Point2 {
	if s.ID != "" && ref.ID != "" && s.ID == ref.ID {
		return refBrp
	}
	return s.BrpPointFromSitBones(ref.SitBonePointFromBrp(refBrp))
}

// BrpPointAtHeightAndSetback returns the 2D BRP coordinate in the sagittal plane
// for a given radial saddle height (BB to BRP) and horizontal BRP setback behind the BB.
func (s SaddleSpec) BrpPointAtHeightAndSetback(saddleHeightMm, brpSetbackMm float64) geom2d.Point2 {
	brpY := math.Sqrt(math.Max(0, saddleHeightMm*saddleHeightMm-brpSetbackMm*brpSetbackMm))
	return geom2d.Point2{X: -brpSetbackMm, Y: brpY}
}

// DeltaBrpToSitBonesMm returns the longitudinal distance from the 70mm BRP back to the sit bone line:
// Delta_BRP_to_SitBones = SitBonesToBrpVec().X.
func (s SaddleSpec) DeltaBrpToSitBonesMm() float64 {
	return s.SitBonesToBrpVec().X
}

// SitBoneSetbackFromBrp converts a BRP setback behind the bottom bracket into the rider's sit bone setback
// using the saddle's 2D BRP-to-SitBones translation vector.
func (s SaddleSpec) SitBoneSetbackFromBrp(brpSetbackMm float64) float64 {
	return -s.SitBonePointFromBrp(geom2d.Point2{X: -brpSetbackMm}).X
}

// BrpSetbackFromSitBones converts a desired sit bone setback into the BRP caliper reading for this specific saddle
// using the saddle's 2D SitBones-to-BRP translation vector.
func (s SaddleSpec) BrpSetbackFromSitBones(sitBoneSetbackMm float64) float64 {
	return -s.BrpPointFromSitBones(geom2d.Point2{X: -sitBoneSetbackMm}).X
}

// RailCenterSetbackFromBrp converts a BRP setback into the setback of the saddle's 0.0mm rail center mark:
// RailCenterSetback = brpSetback + RailCenterToBrpMm.
func (s SaddleSpec) RailCenterSetbackFromBrp(brpSetbackMm float64) float64 {
	return brpSetbackMm + s.RailCenterToBrpMm
}

// ClampOffsetFromBrp calculates the position of the seatpost clamp center on the saddle rails
// given the BRP setback, the frame's seat tube setback, and seatpost setback:
// ClampOffset = RailCenterSetback - (SeatTubeSetback + SeatpostSetback).
func (s SaddleSpec) ClampOffsetFromBrp(brpSetbackMm, seatTubeSetbackMm, seatpostSetbackMm float64) float64 {
	return s.RailCenterSetbackFromBrp(brpSetbackMm) - (seatTubeSetbackMm + seatpostSetbackMm)
}

func clampUnit(v float64) float64 {
	if v < -1.0 {
		return -1.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}

// StaForBrpAndOffset returns the exact effective Seat Tube Angle (degrees) such that
// CalculatePoint(staDeg, saddleHeightMm, seatpostSetbackMm, railOffsetMm).BrpSetbackMm equals brpSetbackMm.
func (s SaddleSpec) StaForBrpAndOffset(brpSetbackMm, saddleHeightMm, seatpostSetbackMm, railOffsetMm float64) Deg {
	if saddleHeightMm <= 0 {
		return 0
	}
	brpPerp := seatpostSetbackMm + railOffsetMm - s.RailCenterToBrpMm
	rad := math.Acos(clampUnit(brpSetbackMm/saddleHeightMm)) + math.Asin(clampUnit(brpPerp/saddleHeightMm))
	return DegOf(geom2d.FromRad(rad))
}

// RailOffsetForBrpAtSta returns the exact rail clamp offset (mm) such that
// CalculatePoint(staDeg, saddleHeightMm, seatpostSetbackMm, railOffsetMm).BrpSetbackMm equals brpSetbackMm.
func (s SaddleSpec) RailOffsetForBrpAtSta(staDeg Deg, brpSetbackMm, saddleHeightMm, seatpostSetbackMm float64) float64 {
	if saddleHeightMm <= 0 {
		return 0
	}
	staRad := staDeg.Angle().Rad()
	delta := staRad - math.Acos(clampUnit(brpSetbackMm/saddleHeightMm))
	return saddleHeightMm*math.Sin(delta) - seatpostSetbackMm + s.RailCenterToBrpMm
}

// IdealSeatTubeAngle computes the exact effective Seat Tube Angle required to mount this saddle
// with its clamp dead-center (0.0mm) on the rails for a given sit bone setback and saddle height.
func (s SaddleSpec) IdealSeatTubeAngle(sitBoneSetbackMm, saddleHeightMm, seatpostSetbackMm float64) Deg {
	if saddleHeightMm <= 0 {
		return 0
	}
	return s.StaForBrpAndOffset(s.BrpSetbackFromSitBones(sitBoneSetbackMm), saddleHeightMm, seatpostSetbackMm, 0.0)
}

// TranslateClampOffset converts a clamp position from this saddle to targetSaddle to keep sit bones in the identical position:
// TargetClampOffset = thisClampOffset + (targetSaddle.RailCenterToSitBonesMm - s.RailCenterToSitBonesMm).
func (s SaddleSpec) TranslateClampOffset(thisClampOffset float64, targetSaddle SaddleSpec) float64 {
	return thisClampOffset + (targetSaddle.RailCenterToSitBonesMm - s.RailCenterToSitBonesMm)
}

// ClampWindowMm returns the clamp-center travel bounds [minX, maxX]
// for a seatpost clamp of length clampLengthMm (L_clamp):
// X_clamp_min = -((L_rail - L_clamp) / 2), X_clamp_max = +((L_rail - L_clamp) / 2).
func (s SaddleSpec) ClampWindowMm(clampLengthMm float64) (minX float64, maxX float64) {
	if s.RailLengthMm <= clampLengthMm {
		return 0, 0
	}
	half := (s.RailLengthMm - clampLengthMm) / 2.0
	return -half, half
}

// ReferenceIdmatchSaddle returns the canonical reference saddle used during idmatch fitting (SQlab 610).
// Since the idmatch protocol was conducted on an SQlab saddle (Delta BRP-to-SitBones = 73mm),
// any idmatch BRP specification (e.g. 219mm) derives a primary pelvic Sit Bone Setback of 219 + 73 = 292mm.
func ReferenceIdmatchSaddle() SaddleSpec {
	return SaddleSpec{
		ID:                     "sqlab-610-ergolux",
		Name:                   "SQlab 610 Ergolux Active 2.1",
		RailLengthMm:           60.0,
		RailCenterToBrpMm:      23.0,
		RailCenterToSitBonesMm: -50.0,
		SitBonesElevationMm:    10.0,
	}
}

// DerivePelvicSetbackFromIdmatchBrp derives the rider's primary pelvic Sit Bone Setback
// from an idmatch BRP setback (e.g. 219mm) using the reference SQlab saddle (219 + 73 = 292mm).
func DerivePelvicSetbackFromIdmatchBrp(idmatchBrpSetbackMm float64) float64 {
	return ReferenceIdmatchSaddle().SitBoneSetbackFromBrp(idmatchBrpSetbackMm)
}

// TargetBrpForPelvicSetback derives the target BRP setback for this saddle given the primary pelvic Sit Bone Setback:
// TargetBrp = pelvicSitBoneSetback - (RailCenterToBrpMm - RailCenterToSitBonesMm).
func (s SaddleSpec) TargetBrpForPelvicSetback(pelvicSitBoneSetbackMm float64) float64 {
	return s.BrpSetbackFromSitBones(pelvicSitBoneSetbackMm)
}

// SitSurfaceHeightFromBrpHeight calculates the radial distance from BB (0,0) to the sit-bone horizontal plane
// at X = -brpSetbackMm, using the saddle's 2D BRP-to-SitBones translation vector.
func (s SaddleSpec) SitSurfaceHeightFromBrpHeight(brpSaddleHeightMm, brpSetbackMm float64) float64 {
	if brpSaddleHeightMm <= 0 {
		return 0
	}
	brp := s.BrpPointAtHeightAndSetback(brpSaddleHeightMm, brpSetbackMm)
	sitBone := s.SitBonePointFromBrp(brp)
	return geom2d.Point2{X: brp.X, Y: sitBone.Y}.DistanceToOrigin()
}

// TranslateSaddleHeightFromRef calculates the equivalent radial saddle height (BB to BRP)
// for this saddle to keep the rider's 2D pelvic position (SitBoneX, SitBoneY) identical
// to that of the reference saddle (e.g. SQlab 610 at refSaddleHeightMm and refBrpSetbackMm).
func (s SaddleSpec) TranslateSaddleHeightFromRef(ref SaddleSpec, refSaddleHeightMm, refBrpSetbackMm float64) float64 {
	if refSaddleHeightMm <= 0 {
		return 0
	}
	if s.ID != "" && ref.ID != "" && s.ID == ref.ID {
		return refSaddleHeightMm
	}
	refBrp := ref.BrpPointAtHeightAndSetback(refSaddleHeightMm, refBrpSetbackMm)
	return s.TranslateBrpFromRef(ref, refBrp).DistanceToOrigin()
}

// TranslateHbDistanceFromRef translates the target Saddle-HB Euclidean distance (BRP -> HB)
// from a reference saddle (e.g. SQlab 610 at refSaddleHeightMm and refBrpSetbackMm)
// to this saddle's BRP reference point, keeping the 2D handlebar point (HbX, HbY) and
// sit-bone-relative HB rise (hbRiseFromSitBonesMm) invariant in space.
func (s SaddleSpec) TranslateHbDistanceFromRef(
	ref SaddleSpec,
	refSaddleHeightMm, refBrpSetbackMm float64,
	refHbDistMm, hbRiseFromSitBonesMm float64,
) float64 {
	if refHbDistMm <= 0 {
		return 0
	}
	if s.ID != "" && ref.ID != "" && s.ID == ref.ID {
		return refHbDistMm
	}
	refBrp := ref.BrpPointAtHeightAndSetback(refSaddleHeightMm, refBrpSetbackMm)
	sitBone := ref.SitBonePointFromBrp(refBrp)

	hbY := sitBone.Y + hbRiseFromSitBonesMm
	dyRef := hbY - refBrp.Y
	dxRef := math.Sqrt(math.Max(0, refHbDistMm*refHbDistMm-dyRef*dyRef))
	hb := geom2d.Point2{X: refBrp.X + dxRef, Y: hbY}

	targetBrp := s.TranslateBrpFromRef(ref, refBrp)
	return hb.DistanceTo(targetBrp)
}

// DefaultSeatpostStaticSetbackMm is the perpendicular rearward offset (in mm) of the static Redshift ShockStop post.
const DefaultSeatpostStaticSetbackMm = 7.0

// DefaultSeatpostSagMm is the typical dynamic sag setback (in mm) for a suspension seatpost under rider weight.
const DefaultSeatpostSagMm = 4.0

// DefaultSeatpostSetbackMm is the active perpendicular rearward offset (in mm) of the seatpost clamp
// from the seat tube centerline, hardcoded to a Redshift ShockStop suspension seatpost sagged under rider weight (7 + 4 = 11mm).
const DefaultSeatpostSetbackMm = DefaultSeatpostStaticSetbackMm + DefaultSeatpostSagMm

// DefaultSeatpostClampLengthMm is the standard seatpost clamp length (in mm).
const DefaultSeatpostClampLengthMm = 30.0

// DropBestClampLengthMm is the clamp length (in mm) of a Fair Bicycle Drop Best UC M6 adapter.
const DropBestClampLengthMm = 45.0

// DefaultSaddleBrpOffsetMm is the legacy perpendicular rearward offset (in mm) from the center of the
// saddle rails to the saddle's 70mm-width Biomechanical Reference Point (BRP).
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
// and its associated 2D sit-bone support point for a specific fore-aft rail offset
// along the constant-radius SaddleHeightMm arc.
type BRPPoint struct {
	Saddle                SaddleSpec
	SeatpostSetbackMm     float64
	SaddleBrpOffsetMm     float64
	RailOffsetMm          float64
	AchievedClampOffsetMm float64
	TotalPerpOffsetMm     float64
	BrpX                  float64
	BrpY                  float64
	SitBoneX              float64
	SitBoneY              float64
	BrpSetbackMm          float64
	SitBoneSetbackMm      float64
	BrpAngleFromBbDeg     Deg
}

// BrpPoint returns the 2D BRP coordinate (BrpX, BrpY).
func (p BRPPoint) BrpPoint() geom2d.Point2 {
	return geom2d.Point2{X: p.BrpX, Y: p.BrpY}
}

// SitBonePoint returns the 2D sit-bone support coordinate (SitBoneX, SitBoneY).
// If SitBoneX and SitBoneY were not explicitly populated (e.g. in a struct literal),
// it translates BrpPoint() via Saddle.SitBonePointFromBrp.
func (p BRPPoint) SitBonePoint() geom2d.Point2 {
	if p.SitBoneX != 0 || p.SitBoneY != 0 {
		return geom2d.Point2{X: p.SitBoneX, Y: p.SitBoneY}
	}
	return p.Saddle.SitBonePointFromBrp(p.BrpPoint())
}

// CalculatePoint computes the Cartesian coordinates for BRP and Sit Bones
// for a frame with the given effective STA at radial seatHeight, with perpendicular seatpost setback
// and fore-aft rail clamp offset.
func (s SaddleSpec) CalculatePoint(
	staDeg Deg, seatHeight, seatpostSetbackMm, railOffsetMm float64,
) BRPPoint {
	seatAxis := geom2d.NewAxisFromPolar(geom2d.Origin, (180.0 - staDeg).Angle())

	// clampPerp is the perpendicular rearward offset of the seatpost clamp center from the seat tube axis.
	// Positive railOffsetMm pushes the saddle body rearward relative to the clamp (clamp moves forward on rails).
	clampPerp := seatpostSetbackMm + railOffsetMm

	// Forward offset toward nose reduces rearward perpendicular offset from seat tube axis:
	brpPerp := clampPerp - s.RailCenterToBrpMm

	axial := math.Sqrt(math.Max(0, seatHeight*seatHeight-brpPerp*brpPerp))

	brpPoint := seatAxis.PointWithOffset(axial, brpPerp, geom2d.CounterClockwise)
	sitBonePoint := s.SitBonePointFromBrp(brpPoint)

	setback := -brpPoint.X
	sitBoneSetback := -sitBonePoint.X

	angleFromBb := Deg(0.0)
	if setback > 0 && brpPoint.Y > 0 {
		angleFromBb = DegOf(geom2d.Point2{X: setback, Y: brpPoint.Y}.PolarAngle())
	}

	return BRPPoint{
		Saddle:                s,
		SeatpostSetbackMm:     seatpostSetbackMm,
		SaddleBrpOffsetMm:     s.RailCenterToBrpMm,
		RailOffsetMm:          railOffsetMm,
		AchievedClampOffsetMm: railOffsetMm,
		TotalPerpOffsetMm:     brpPerp,
		BrpX:                  brpPoint.X,
		BrpY:                  brpPoint.Y,
		SitBoneX:              sitBonePoint.X,
		SitBoneY:              sitBonePoint.Y,
		BrpSetbackMm:          setback,
		SitBoneSetbackMm:      sitBoneSetback,
		BrpAngleFromBbDeg:     angleFromBb,
	}
}

// MapPossibleBRPs generates all valid candidate BRP points within this saddle's
// straight rail clamp window in 1mm steps.
func (s SaddleSpec) MapPossibleBRPs(
	staDeg Deg, seatHeight, seatpostSetbackMm, clampLengthMm float64,
) []BRPPoint {
	if clampLengthMm <= 0 {
		clampLengthMm = DefaultSeatpostClampLengthMm
	}
	minClamp, maxClamp := s.ClampWindowMm(clampLengthMm)

	var points []BRPPoint
	for r := minClamp; r <= maxClamp+0.001; r += SaddleRailStepMm {
		pt := s.CalculatePoint(staDeg, seatHeight, seatpostSetbackMm, math.Round(r*10)/10)
		points = append(points, pt)
	}
	return points
}

// DefaultBarClampAndRiseSpanMm is the lateral distance (in mm) from the stem centerline across the
// 31.8mm clamp section and upward riser bends before handlebar backsweep begins.
const DefaultBarClampAndRiseSpanMm = 190.0

// DefaultBarGripInboardOffsetMm is the distance (in mm) from the outer bar tip to the center of the
// rider's palm (3rd metacarpal) on standard ~135mm grips.
const DefaultBarGripInboardOffsetMm = 70.0

// DefaultGripInsetMm is the standard inboard distance along the lateral (Z) axis from the bar end
// to the functional center of the hand grip (typically 65mm for 130mm grips).
const DefaultGripInsetMm = 65.0

// WingGripInsetMm is the inboard distance for ergonomic wing grips (e.g. SQlab 710/7OX).
const WingGripInsetMm = 70.0

// Point3D represents a 3D coordinate or vector in millimeters within a right-handed coordinate frame:
//   - X: Reach / Longitudinal axis (+ forward, horizontal)
//   - Y: Stack / Vertical axis (+ upward, vertical)
//   - Z: Lateral axis (+ right of centerline, symmetric for left)
type Point3D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Add returns the vector sum p + other.
func (p Point3D) Add(other Point3D) Point3D {
	return Point3D{X: p.X + other.X, Y: p.Y + other.Y, Z: p.Z + other.Z}
}

// Sub returns the vector difference p - other.
func (p Point3D) Sub(other Point3D) Point3D {
	return Point3D{X: p.X - other.X, Y: p.Y - other.Y, Z: p.Z - other.Z}
}

// Scale multiplies p by scalar s.
func (p Point3D) Scale(s float64) Point3D {
	return Point3D{X: p.X * s, Y: p.Y * s, Z: p.Z * s}
}

// DistanceTo returns the 3D Euclidean distance between p and other.
func (p Point3D) DistanceTo(other Point3D) float64 {
	dx := p.X - other.X
	dy := p.Y - other.Y
	dz := p.Z - other.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// Distance2D returns the 2D Euclidean distance in the sagittal (X-Y) plane.
func (p Point3D) Distance2D(other Point3D) float64 {
	dx := p.X - other.X
	dy := p.Y - other.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// RiseMeasurementMode specifies how a manufacturer measures handlebar rise.
type RiseMeasurementMode string

const (
	// RiseModeStandardKnuckle indicates rise is measured from the clamp axis to the post-rise bend knuckle.
	// Outboard upsweep adds vertical height on top of the stated rise. Used by SQlab and standard DIN/MTB.
	RiseModeStandardKnuckle RiseMeasurementMode = "standard_knuckle"

	// RiseModeBarEndCenter indicates rise is measured from the clamp center to the center of the bar end tip.
	// Outboard upsweep is already baked into the stated rise figure. Used by Tumbleweed Persuader.
	RiseModeBarEndCenter RiseMeasurementMode = "bar_end_center"
)

// HandlebarSpec defines the geometric specification and measurement standard for a handlebar.
type HandlebarSpec struct {
	ID                   string              `json:"id,omitempty"`
	Name                 string              `json:"name"`
	StockWidthMm         float64             `json:"stock_width_mm"`
	CurrentWidthMm       float64             `json:"current_width_mm"`
	StatedRiseMm         float64             `json:"stated_rise_mm"`
	BacksweepDeg         Deg                 `json:"backsweep_deg"`
	UpsweepDeg           Deg                 `json:"upsweep_deg"`
	EffectiveBendStartMm float64             `json:"effective_bend_start_mm"` // z_b (lateral bend start from centerline)
	RiseMode             RiseMeasurementMode `json:"rise_mode"`
}

// EffectiveWidthMm returns CurrentWidthMm if positive, or StockWidthMm.
func (h HandlebarSpec) EffectiveWidthMm() float64 {
	if h.CurrentWidthMm > 0 {
		return h.CurrentWidthMm
	}
	return h.StockWidthMm
}

// HandLateralMm returns the lateral distance from centerline to hand grip center:
//
//	z_hand = EffectiveWidth / 2.0 - gripInsetMm
func (h HandlebarSpec) HandLateralMm(gripInsetMm float64) float64 {
	if gripInsetMm <= 0 {
		gripInsetMm = DefaultGripInsetMm
	}
	return h.EffectiveWidthMm()/2.0 - gripInsetMm
}

// BaseRiseAtBend returns the base vertical rise y_k at the bend intersection z_b.
// For standard knuckle measurement, y_k = statedRise.
// For bar-end center measurement, y_k = statedRise - (stockWidth/2 - z_b) * tan(upsweep).
func (h HandlebarSpec) BaseRiseAtBend() float64 {
	if h.RiseMode == RiseModeBarEndCenter {
		alpha := h.UpsweepDeg.Angle()
		stockTipSpan := h.StockWidthMm/2.0 - h.EffectiveBendStartMm
		return h.StatedRiseMm - stockTipSpan*alpha.Tan()
	}
	return h.StatedRiseMm
}

// SQlabEffectiveBendStartMm computes the effective lateral bend intersection point (z_b)
// for SQlab handlebars: z_b = 55 + 0.9 * statedRiseMm.
func SQlabEffectiveBendStartMm(statedRiseMm float64) float64 {
	return 55.0 + 0.9*statedRiseMm
}

// HBPoint represents a mapped handlebar position (HB) and its resulting Saddle-Handlebar distance and drop.
// ClampX and ClampY represent the center of the physical stem clamp, while HbX and HbY represent
// the effective hand contact position after accounting for handlebar sweep and rise.
type HBPoint struct {
	Stem           StemSpec
	SpacerMm       float64
	ClampX         float64
	ClampY         float64
	HbX            float64
	HbY            float64
	SaddleHbMm     float64
	SaddleHbDropMm float64
}

// FitOval defines an elliptical area of reasonable fit centered on an ideal cockpit target.
type FitOval struct {
	CenterX    float64 // Absolute X coordinate of ideal center (mm)
	CenterY    float64 // Absolute Y coordinate of ideal center (mm)
	SemiMinorA float64 // Semi-minor horizontal radius (default: 10.0 mm)
	SemiMajorB float64 // Semi-major vertical radius (default: 20.0 mm)
	AngleDeg   Deg     // Clockwise rotation angle in degrees (default: 30.0 deg)
}

// CockpitFitResult captures the fit assessment of a bike's candidate handlebar positions against a FitOval.
type CockpitFitResult struct {
	TargetOval        FitOval
	SelectedHB        HBPoint
	BestR             float64
	CountInside       int
	BaseScore         float64
	VersatilityBonus  float64
	StemPenalty       float64 // Penalty points for using stem near/at extreme of expected range
	StemNormalizedDev float64 // Normalized deviation u = |L - L_mid| / HalfSpan
	FinalSubScore     float64
	HasFit            bool
}

func (b BikeGeometry) GetMaxSpacerMm() float64 {
	return b.MaxSpacerMm
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
	return b.CanonicalStem
}

// GetExpectedStemRangeMm returns the [min, max] expected stem length in mm for this frame,
// and a boolean indicating whether an expected range is available.
// If ExpectedStemRangeMm is explicitly set with 2 positive elements (min < max), it is returned.
// Otherwise, it computes the min and max lengths from the bike's expanded Stems slice.
func (b BikeGeometry) GetExpectedStemRangeMm() (float64, float64, bool) {
	if len(b.ExpectedStemRangeMm) == 2 && b.ExpectedStemRangeMm[0] > 0 && b.ExpectedStemRangeMm[1] > b.ExpectedStemRangeMm[0] {
		return b.ExpectedStemRangeMm[0], b.ExpectedStemRangeMm[1], true
	}
	if len(b.Stems) > 0 {
		minLen := b.Stems[0].LengthMm
		maxLen := b.Stems[0].LengthMm
		for _, s := range b.Stems[1:] {
			if s.LengthMm < minLen {
				minLen = s.LengthMm
			}
			if s.LengthMm > maxLen {
				maxLen = s.LengthMm
			}
		}
		if maxLen > minLen && minLen > 0 {
			return minLen, maxLen, true
		}
	}
	return 0, 0, false
}

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
	Fork   string
	SagPct float64
	A2cMm  float64
	RakeMm float64
}

func (r ForkRef) GetSagPct() float64 {
	return r.SagPct
}

func (r ForkRef) ForkTravel() float64 {
	parts := strings.SplitSeq(r.Fork, "_")
	for p := range parts {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(p, "mm"), 64); err == nil && v >= 60 && v <= 200 {
			return v
		}
	}
	return 0.0
}

func (r ForkRef) StaticA2c() float64 {
	tr := r.ForkTravel()
	if tr > 0 && r.SagPct > 0 {
		return r.A2cMm + tr*(r.SagPct/100.0)
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

func (fm *FrameModel) HasBentSeatTube() bool {
	return fm.ActualStaDeg > 0
}

func (fm *FrameModel) GetActualStaDeg() Deg {
	return fm.ActualStaDeg
}

func (fm *FrameModel) GetSeatTubeOffsetXMm(stack float64, nominalSta Deg) float64 {
	if v, ok := fm.SeatTubeOffsetXMm.Get(); ok {
		return v
	}
	if !fm.HasBentSeatTube() || stack <= 0 || nominalSta <= 0 {
		return 0
	}
	cotAct := 1.0 / fm.ActualStaDeg.Angle().Tan()
	cotNom := 1.0 / nominalSta.Angle().Tan()
	return stack * (cotAct - cotNom)
}

func (fm *FrameModel) CalculateEffectiveSta(seatHeight, stack float64, nominalSta Deg) Deg {
	if !fm.HasBentSeatTube() || seatHeight <= 0 || nominalSta <= 0 {
		return nominalSta
	}
	ox := fm.GetSeatTubeOffsetXMm(stack, nominalSta)
	sinVal := (ox * fm.ActualStaDeg.Angle().Sin()) / seatHeight
	if sinVal > 1.0 {
		sinVal = 1.0
	} else if sinVal < -1.0 {
		sinVal = -1.0
	}
	alphaRad := math.Asin(sinVal)
	return fm.ActualStaDeg + DegOf(geom2d.FromRad(alphaRad))
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
// Suspension forks expand across standard sag (20% to 30%), with the upper bound extending
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
	ID         string
	Field      string
	Severity   Severity
	Title      string
	Message    string
	Details    string
	Expected   string
	Actual     string
	Difference float64
}

type PublishMode string

const (
	ModeStaticCad     PublishMode = "static_cad"
	ModeSaggedCad     PublishMode = "sagged_cad"
	ModeMismatchedCad PublishMode = "mismatched_cad"
	ModeUnspecified   PublishMode = "unspecified"
)

type ValidationReport struct {
	BikeID              string
	IsValid             bool
	Issues              []ValidationIssue
	GeometricA2c        float64
	StatedA2c           float64
	ExpectedForkA2c     float64
	GeoHeadsetSpacerMm  float64
	CupStack            float64
	TotalStructuralA2c  float64
	A2cDifference       Opt[float64]
	InferredPublishMode PublishMode
	FrontCenterCalc     float64
	FrontCenterDiff     Opt[float64]
	// Rounding budgets of the three closure checks; nil when a check did not run.
	A2cBudget         *ErrorBudget
	FrontCenterBudget *ErrorBudget
	WheelbaseBudget   *ErrorBudget
}

// FrameValidationReport holds the complete validation result across all sizes of a FrameModel.
type FrameValidationReport struct {
	FrameID     string
	IsValid     bool
	Issues      []ValidationIssue
	SizeReports map[string]ValidationReport
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
	HeightIn                      float64
	InseamIn                      float64
	SaddleHeightMm                float64
	RiderWeightLbs                float64
	BikeWeightLbs                 float64
	BaselineStemMm                float64
	BaselineStemAngleDeg          Deg
	Handlebar                     HandlebarSpec
	BaselineSaddleSetbackOffsetMm float64
}

type CanonicalConfig struct {
	TargetA2c    float64
	SagPercent   float64
	Name         string
	Saddle       SaddleSpec
	Handlebar    HandlebarSpec
	RiderProfile RiderProfile
}

// ComparisonSetup represents one column in a comparison matrix.
type ComparisonSetup struct {
	FrameID      string
	Size         string
	A2cMm        float64
	RakeMm       float64
	DisplayLabel string
	SagPct       float64
	AutoExpanded bool
}

// SavedComparisonState holds the active comparison configuration.
type SavedComparisonState struct {
	RiderProfile
	Transpose    bool
	RefIndex     int
	IdealProfile string
	SaddleID     string
	Saddle       SaddleSpec
	HandlebarID  string
	Handlebar    HandlebarSpec
	Columns      []ComparisonSetup
}

type DynamicGeometryResult struct {
	NewHta        Deg
	NewSta        Deg
	NewReach      float64
	NewStack      float64
	NewBbDrop     float64
	DeltaAngleDeg Deg
	DeltaReach    float64
	DeltaStack    float64
	DeltaBbDrop   float64
	TotalNewA2c   float64
}

type BikeFitAnalysis struct {
	StructuralA2c       float64
	ActiveReach         float64
	ActiveStack         float64
	ActiveHta           Deg
	ActiveSta           Deg
	ActiveBbDrop        float64
	Aett                float64
	Ett                 float64
	ActualSetback       float64
	SpareVerticalStack  float64
	SpareAxialSteerer   float64
	SeatpostExtensionMm float64
	DeltaAngleDeg       Deg
	DeltaReach          float64
	DeltaStack          float64
	DeltaBbDrop         float64
	InferredPublishMode string

	// Weight Distribution & Biomechanical Posture
	WeightDistFrontPct float64
	WeightDistRearPct  float64
	BackAngleDeg       Deg
	FrontCenterCalc    float64
	RearCenterCalc     float64
	SeatToRearCenter   float64
	WheelbaseCalc      float64
	ActiveChainstay    float64
	ChainstayDesc      string
	TrailMm            float64

	// Cockpit BRP & HB Points and Saddle-Handlebar Metrics
	Saddle              SaddleSpec
	Handlebar           HandlebarSpec
	SitBoneSetbackMm    float64
	IdealStaDeg         Deg
	AchievedClampOffset float64
	DeadCenterBRP       BRPPoint
	SelectedBRP         *BRPPoint
	PossibleBRPs        []BRPPoint
	SaddleHbMm          float64
	SaddleHbDropMm      float64
	SelectedHB          *HBPoint
	PossibleHBs         []HBPoint
	CockpitFitResult    *CockpitFitResult
}

type BaselineMetrics struct {
	Reach              float64
	Stack              float64
	HtaDeg             Deg
	StaDeg             Deg
	BbDrop             float64
	A2c                float64
	Aett               float64
	ActualSetback      float64
	SpareVerticalStack float64
	SpareAxialSteerer  float64
	SeatpostExtension  float64
	Chainstay          float64
	SeatToRearCenter   float64
	SaddleHbMm         float64
	TrailMm            float64
}

type CanonicalComparisonRow struct {
	Setup         ComparisonSetup
	DisplayLabel  string
	EffectiveA2c  float64
	EffectiveRake float64
	Bike          BikeGeometry
	Validation    ValidationReport
	Baseline      BaselineMetrics
	Canonical     BikeFitAnalysis
}
