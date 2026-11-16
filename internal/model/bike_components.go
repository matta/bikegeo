package model

import (
	"math"

	"bikegeo/internal/geom2d"
)

// StemThicknessMm is the fixed axial height (in mm) of the stem clamp on the steerer tube.
const StemThicknessMm = 40.0

// DefaultHeadsetTopStackMm is the default axial height (in mm) of the upper headset bearing and top cover
// between the top of the head tube and the bottom of the steerer spacers / stem clamp.
const DefaultHeadsetTopStackMm = 11.0

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

// DefaultIdmatchBrpSetbackMm is the baseline BRP setback (in mm) from the idmatch fit report.
const DefaultIdmatchBrpSetbackMm = 219.0

// DefaultIdmatchSaddleHeightMm is the baseline radial saddle height (in mm) from BB to BRP in the idmatch fit report.
const DefaultIdmatchSaddleHeightMm = 746.0

// DefaultTargetSitBoneSetbackMm derives the modeled support-landmark setback behind BB
// from the recorded idmatch BRP and reference saddle, assuming level rails.
func DefaultTargetSitBoneSetbackMm() float64 {
	return ReferenceIdmatchSaddle().SitBoneSetbackFromBrp(DefaultIdmatchBrpSetbackMm)
}

// ReferenceIdmatchSaddle returns the canonical reference saddle used during idmatch fitting (SQlab 610).
// Owner measurements and derivations are recorded in docs/SQLAB_SADDLE.md.
// Longitudinal offsets are positive forward along the rails; SIT is a physical
// support landmark, not an independently measured pelvic coordinate.
func ReferenceIdmatchSaddle() SaddleSpec {
	return SaddleSpec{
		ID:                     "sqlab-610-ergolux",
		Name:                   "SQlab 610 Ergolux Active 2.1",
		RailLengthMm:           60.0,
		RailCenterToBrpMm:      32.0, // Derived W70 station; direct width verification remains pending.
		RailCenterToSitBonesMm: -48.0,
		SitBonesElevationMm:    10.0, // Physically measured by Matt; confirmed 2026-11-16.
	}
}

// ReferenceSitBonePoint returns the 2D sit-bone coordinate in the sagittal plane
// for the reference idmatch saddle at the given radial saddle height and sit-bone setback.
func ReferenceSitBonePoint(refSaddleHeightMm, sitBoneSetbackMm float64) geom2d.Point2 {
	ref := ReferenceIdmatchSaddle()
	refBrp := ref.BrpPointAtHeightAndSetback(refSaddleHeightMm, ref.BrpSetbackFromSitBones(sitBoneSetbackMm))
	return ref.SitBonePointFromBrp(refBrp)
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

// DefaultSeatpostSetbackMm is the perpendicular rearward offset (in mm) of the seatpost clamp from the
// seat tube centerline. The baseline is a zero-setback dropper post, so the saddle rails clamp on the seat tube axis.
const DefaultSeatpostSetbackMm = 0.0

// DefaultSeatpostClampLengthMm is the standard seatpost clamp length (in mm).
const DefaultSeatpostClampLengthMm = 30.0

// DropBestClampLengthMm is the clamp length (in mm) of a Fair Bicycle Drop Best UC M6 adapter.
const DropBestClampLengthMm = 45.0

// DefaultSaddleBrpOffsetMm is the legacy perpendicular rearward offset (in mm) from the center of the
// saddle rails to the saddle's 70mm-width Biomechanical Reference Point (BRP).
const DefaultSaddleBrpOffsetMm = 0.0

// DefaultBrpOffsetMm is the total perpendicular offset (in mm) from the seat tube centerline
// to the BRP when the saddle is clamped at dead center (0mm) on the rails (0.0 + 0.0 = 0.0mm).
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

// Sub returns the vector difference p - other.
func (p Point3D) Sub(other Point3D) Point3D {
	return Point3D{X: p.X - other.X, Y: p.Y - other.Y, Z: p.Z - other.Z}
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

// TargetRef names the physical cockpit point a fit target refers to.
// Every fit target must declare one explicitly; bare or implicit "HB" targets
// are not permitted.
type TargetRef string

const (
	// TargetHandGrip indicates the fit target is the rider's physical hand contact center (HandGrip).
	TargetHandGrip TargetRef = "hand_grip"
	// TargetStemClamp indicates the fit target is the handlebar/stem clamp center directly (StemClamp).
	TargetStemClamp TargetRef = "stem_clamp"
	// TargetReferenceBarClamp indicates the fit target is the stem clamp center of ReferenceBar
	// on the fit bike, reproducing hand position through ReferenceBar's sweep/rise offset.
	// TODO: this this strictly necessary? It seems like a YAGNI thing.
	TargetReferenceBarClamp TargetRef = "reference_bar_clamp"
)

// IsValid reports whether r is a recognized TargetRef mode.
func (r TargetRef) IsValid() bool {
	switch r {
	case TargetHandGrip, TargetStemClamp, TargetReferenceBarClamp:
		return true
	default:
		return false
	}
}

// Label returns a concise human-readable description of the TargetRef mode.
func (r TargetRef) Label() string {
	switch r {
	case TargetHandGrip:
		return "HandGrip"
	case TargetStemClamp:
		return "StemClamp"
	case TargetReferenceBarClamp:
		return "ReferenceBar StemClamp"
	default:
		return "Unknown"
	}
}

// CockpitFitResult captures the fit assessment of a bike's candidate handlebar positions against a FitOval.
type CockpitFitResult struct {
	TargetRef    TargetRef
	ReferenceBar Opt[HandlebarSpec]
	TargetOval   FitOval
	SelectedHB   HBPoint
	BestR        float64
	CountInside  int
	HasFit       bool
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
