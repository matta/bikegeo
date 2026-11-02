package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Float64Ptr returns a pointer to v, used for optional numeric fields where nil means unset.
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
	A2c         *float64
	FrontCenter *float64
	Wheelbase   *float64
	CrossConfig *float64
	Reason      string
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
	HtaDeg    float64
	StaDeg    float64
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

	// Bent / Offset Seat Tube
	ActualStaDeg      float64
	SeatTubeOffsetXMm *float64

	// Cockpit & Stem/Spacer Envelope
	MaxSpacerMm   float64
	CanonicalStem StemSpec
	Stems         []StemSpec
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

// StemThicknessMm is the fixed axial height (in mm) of the stem clamp on the steerer tube.
const StemThicknessMm = 40.0

// SpacerStepMm is the fixed increment (in mm) for steerer spacers from 0 up to MaxSpacerMm.
const SpacerStepMm = 5.0

// StemLengthStepMm is the fixed increment (in mm) for stem lengths in catalog stem ranges.
const StemLengthStepMm = 10.0

// StemSpec represents a stem configuration (length in mm, angle in degrees).
type StemSpec struct {
	LengthMm float64
	AngleDeg float64
}

// StemCatalogEntry encapsulates a stem set in pkg/catalog.
type StemCatalogEntry struct {
	ID            string
	LengthRangeMm []float64
	AnglesDeg     []float64
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

// SaddleSpec defines a saddle's longitudinal geometry in the Canonical Saddle Coordinate System,
// where the origin (0.0mm) is the midpoint of the nominal straight rail clamping section,
// positive (+) is forward toward the nose, and negative (-) is rearward toward the tail.
type SaddleSpec struct {
	ID                     string
	Name                   string
	RailLengthMm           float64 // Total straight clamping length on rails (e.g. 60mm for SQlab [-30 to +30])
	RailCenterToBrpMm      float64 // Forward distance from rail center to 70mm BRP (+mm toward nose)
	RailCenterToSitBonesMm float64 // Distance from rail center to sit bones (-mm toward tail)
}

// DeltaBrpToSitBonesMm returns the longitudinal distance from the 70mm BRP back to the sit bone line:
// Delta_BRP_to_SitBones = RailCenterToBrpMm - RailCenterToSitBonesMm.
func (s SaddleSpec) DeltaBrpToSitBonesMm() float64 {
	return s.RailCenterToBrpMm - s.RailCenterToSitBonesMm
}

// SitBoneSetbackFromBrp converts a BRP setback behind the bottom bracket into the rider's sit bone setback:
// SitBoneSetback = brpSetback + (RailCenterToBrpMm - RailCenterToSitBonesMm).
func (s SaddleSpec) SitBoneSetbackFromBrp(brpSetbackMm float64) float64 {
	return brpSetbackMm + (s.RailCenterToBrpMm - s.RailCenterToSitBonesMm)
}

// BrpSetbackFromSitBones converts a desired sit bone setback into the BRP caliper reading for this specific saddle:
// BrpSetback = sitBoneSetback - (RailCenterToBrpMm - RailCenterToSitBonesMm).
func (s SaddleSpec) BrpSetbackFromSitBones(sitBoneSetbackMm float64) float64 {
	return sitBoneSetbackMm - (s.RailCenterToBrpMm - s.RailCenterToSitBonesMm)
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

// IdealSeatTubeAngle computes the exact effective Seat Tube Angle required to mount this saddle
// with its clamp dead-center (0.0mm) on the rails for a given sit bone setback and saddle height:
func (s SaddleSpec) IdealSeatTubeAngle(sitBoneSetbackMm, saddleHeightMm, seatpostSetbackMm float64) float64 {
	if saddleHeightMm <= 0 {
		return 0
	}
	railCenterSetback := sitBoneSetbackMm + s.RailCenterToSitBonesMm
	stSetback := railCenterSetback - seatpostSetbackMm
	ratio := stSetback / saddleHeightMm
	if ratio < -1.0 {
		ratio = -1.0
	} else if ratio > 1.0 {
		ratio = 1.0
	}
	return math.Acos(ratio) * 180.0 / math.Pi
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
// for a specific fore-aft rail offset along the constant-radius SaddleHeightMm arc.
type BRPPoint struct {
	Saddle                SaddleSpec
	SeatpostSetbackMm     float64
	SaddleBrpOffsetMm     float64
	RailOffsetMm          float64
	AchievedClampOffsetMm float64
	TotalPerpOffsetMm     float64
	BrpX                  float64
	BrpY                  float64
	BrpSetbackMm          float64
	SitBoneSetbackMm      float64
	BrpAngleFromBbDeg     float64
}

// CalculatePoint computes the Cartesian coordinates for BRP, Sit Bones, and Rail Center
// for a frame with the given effective STA at radial seatHeight, with perpendicular seatpost setback
// and fore-aft rail clamp offset.
func (s SaddleSpec) CalculatePoint(
	staDeg, seatHeight, seatpostSetbackMm, railOffsetMm float64,
) BRPPoint {
	sta := staDeg * math.Pi / 180.0

	// clampPerp is the perpendicular rearward offset of the seatpost clamp center from the seat tube axis.
	// Positive railOffsetMm pushes the saddle body rearward relative to the clamp (clamp moves forward on rails).
	clampPerp := seatpostSetbackMm + railOffsetMm

	// Forward offset toward nose reduces rearward perpendicular offset from seat tube axis:
	brpPerp := clampPerp - s.RailCenterToBrpMm

	// Sit bones are rearward toward the tail (increases rearward perpendicular offset):
	sitBonePerp := clampPerp - s.RailCenterToSitBonesMm

	axial := math.Sqrt(math.Max(0, seatHeight*seatHeight-brpPerp*brpPerp))

	brpX := -axial*math.Cos(sta) - brpPerp*math.Sin(sta)
	brpY := axial*math.Sin(sta) - brpPerp*math.Cos(sta)

	sitBoneX := -axial*math.Cos(sta) - sitBonePerp*math.Sin(sta)

	setback := -brpX
	sitBoneSetback := -sitBoneX

	angleFromBb := 0.0
	if setback > 0 && brpY > 0 {
		angleFromBb = math.Atan2(brpY, setback) * 180.0 / math.Pi
	}

	return BRPPoint{
		Saddle:                s,
		SeatpostSetbackMm:     seatpostSetbackMm,
		SaddleBrpOffsetMm:     s.RailCenterToBrpMm,
		RailOffsetMm:          railOffsetMm,
		AchievedClampOffsetMm: railOffsetMm,
		TotalPerpOffsetMm:     math.Round(brpPerp*10) / 10,
		BrpX:                  math.Round(brpX*10) / 10,
		BrpY:                  math.Round(brpY*10) / 10,
		BrpSetbackMm:          math.Round(setback*10) / 10,
		SitBoneSetbackMm:      math.Round(sitBoneSetback*10) / 10,
		BrpAngleFromBbDeg:     math.Round(angleFromBb*10) / 10,
	}
}

// MapPossibleBRPs generates all valid candidate BRP points within this saddle's
// straight rail clamp window in 1mm steps.
func (s SaddleSpec) MapPossibleBRPs(
	staDeg, seatHeight, seatpostSetbackMm, clampLengthMm float64,
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

// HBPoint represents a mapped handlebar-stem intersection point (HB) and its resulting Saddle-Handlebar distance and drop.
type HBPoint struct {
	Stem           StemSpec
	SpacerMm       float64
	HbX            float64
	HbY            float64
	SaddleHbMm     float64
	SaddleHbDropMm float64
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
	HtaDeg          float64
	StaDeg          float64
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
	ActualStaDeg       float64
	SeatTubeOffsetXMm  *float64
	ChainstayRangeMm   []float64
	Sizes              []string
	DefaultSize        string
	CadFork            ForkRef
	Forks              []ForkRef
	Geometries         map[string]SizeGeometry

	// Cockpit & Stem/Spacer Envelope
	MaxSpacerMm   float64
	CanonicalStem StemSpec
	Stems         []string
}

// Clone returns a deep copy of the FrameModel.
func (fm FrameModel) Clone() FrameModel {
	out := fm
	if fm.GeoSlopMm != nil {
		slop := *fm.GeoSlopMm
		if fm.GeoSlopMm.A2c != nil {
			v := *fm.GeoSlopMm.A2c
			slop.A2c = &v
		}
		if fm.GeoSlopMm.FrontCenter != nil {
			v := *fm.GeoSlopMm.FrontCenter
			slop.FrontCenter = &v
		}
		if fm.GeoSlopMm.Wheelbase != nil {
			v := *fm.GeoSlopMm.Wheelbase
			slop.Wheelbase = &v
		}
		if fm.GeoSlopMm.CrossConfig != nil {
			v := *fm.GeoSlopMm.CrossConfig
			slop.CrossConfig = &v
		}
		out.GeoSlopMm = &slop
	}
	if fm.SeatTubeOffsetXMm != nil {
		v := *fm.SeatTubeOffsetXMm
		out.SeatTubeOffsetXMm = &v
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
	if fm.Geometries != nil {
		out.Geometries = make(map[string]SizeGeometry, len(fm.Geometries))
		for k, v := range fm.Geometries {
			out.Geometries[k] = v
		}
	}
	return out
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
	ModeSaggedCad     PublishMode = "sagged_cad_15_25"
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
	A2cDifference       *float64
	InferredPublishMode PublishMode
	FrontCenterCalc     float64
	FrontCenterDiff     *float64
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
	BaselineStemAngleDeg          float64
	BaselineBarRiseMm             float64
	BaselineBarSweepDeg           float64
	BaselineBarWidthMm            float64
	BaselineSaddleSetbackOffsetMm float64
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
	TargetA2c    float64
	SagPercent   float64
	Name         string
	Saddle       SaddleSpec
	RiderProfile RiderProfile
}

// ComparisonSetup represents one column in a comparison matrix.
type ComparisonSetup struct {
	FrameID      string
	Size         string
	A2cMm        float64
	RakeMm       float64
	DisplayLabel string
}

// SavedComparisonState holds the active comparison configuration.
type SavedComparisonState struct {
	RiderProfile
	Transpose    bool
	RefIndex     int
	IdealProfile string
	SaddleID     string
	Saddle       SaddleSpec
	Columns      []ComparisonSetup
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
	NewHta        float64
	NewSta        float64
	NewReach      float64
	NewStack      float64
	NewBbDrop     float64
	DeltaAngleDeg float64
	DeltaReach    float64
	DeltaStack    float64
	DeltaBbDrop   float64
	TotalNewA2c   float64
}

type BikeFitAnalysis struct {
	StructuralA2c       float64
	ActiveReach         float64
	ActiveStack         float64
	ActiveHta           float64
	ActiveSta           float64
	ActiveBbDrop        float64
	Aett                float64
	ActualSetback       float64
	SpareVerticalStack  float64
	SpareAxialSteerer   float64
	SeatpostExtensionMm float64
	DeltaAngleDeg       float64
	DeltaReach          float64
	DeltaStack          float64
	DeltaBbDrop         float64
	InferredPublishMode string

	// Weight Distribution & Biomechanical Posture
	WeightDistFrontPct float64
	WeightDistRearPct  float64
	BackAngleDeg       float64
	FrontCenterCalc    float64
	RearCenterCalc     float64
	SeatToRearCenter   float64
	WheelbaseCalc      float64
	ActiveChainstay    float64
	ChainstayDesc      string

	// Cockpit BRP & HB Points and Saddle-Handlebar Metrics
	Saddle              SaddleSpec
	SitBoneSetbackMm    float64
	IdealStaDeg         float64
	AchievedClampOffset float64
	DeadCenterBRP       BRPPoint
	SelectedBRP         *BRPPoint
	PossibleBRPs        []BRPPoint
	SaddleHbMm          float64
	SaddleHbDropMm      float64
	SelectedHB          *HBPoint
	PossibleHBs         []HBPoint
}

type BaselineMetrics struct {
	Reach              float64
	Stack              float64
	HtaDeg             float64
	StaDeg             float64
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
