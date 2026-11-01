package geometry

import (
	"math"

	"bikegeo/pkg/model"
)

// IdealStaResult encapsulates the calculated ideal seat tube angle for a specific saddle and seatpost.
type IdealStaResult struct {
	Saddle                    model.SaddleSpec
	TargetSitBoneSetbackMm    float64
	SaddleHeightMm            float64
	SeatpostSetbackMm         float64
	SeatpostDesc              string
	TargetRailCenterSetbackMm float64
	SeatTubeSetbackMm         float64
	IdealStaDeg               float64 // STA with saddle clamped dead-center (0.0mm)
	MinStaDeg                 float64 // Slackest STA within clamp window
	MaxStaDeg                 float64 // Steepest STA within clamp window
	IdealMarginMinStaDeg      float64 // Slackest STA within ±5mm ideal rail margin
	IdealMarginMaxStaDeg      float64 // Steepest STA within ±5mm ideal rail margin
}

// SaddleClampOffset represents the required rail clamp offset on a specific saddle to achieve target fit.
type SaddleClampOffset struct {
	SaddleID   string
	SaddleName string
	OffsetMm   float64 // Fore-aft rail offset (+ forward toward nose, - rearward toward tail)
}

// UniversalCompromiseStaResult encapsulates the minimax compromise STA across all catalog saddles.
type UniversalCompromiseStaResult struct {
	SeatpostDesc          string
	SeatpostSetbackMm     float64
	CompromiseStaDeg      float64
	MaxRailOffsetMm       float64
	UniversalMarginMinSta float64
	UniversalMarginMaxSta float64
	HasUniversalMargin    bool
	SaddleOffsets         []SaddleClampOffset
}

// CalculateUniversalCompromiseSTA finds the minimax compromise effective Seat Tube Angle
// across all provided saddles that minimizes the maximum rail offset from dead-center.
func CalculateUniversalCompromiseSTA(
	saddles []model.SaddleSpec,
	targetSitBoneSetbackMm float64,
	saddleHeightMm float64,
	seatpostSetbackMm float64,
	seatpostDesc string,
	clampLengthMm float64,
) UniversalCompromiseStaResult {
	if len(saddles) == 0 || saddleHeightMm <= 0 {
		return UniversalCompromiseStaResult{
			SeatpostDesc:      seatpostDesc,
			SeatpostSetbackMm: Round(seatpostSetbackMm, 1),
		}
	}
	if clampLengthMm <= 0 {
		clampLengthMm = model.DefaultSeatpostClampLengthMm
	}

	staForSetback := func(sb float64) float64 {
		ratio := sb / saddleHeightMm
		if ratio < -1.0 {
			ratio = -1.0
		} else if ratio > 1.0 {
			ratio = 1.0
		}
		return Round(ToDeg(math.Acos(ratio)), 2)
	}

	var minST, maxST float64
	var maxSTMinus5, minSTPlus5 float64

	for i, s := range saddles {
		st := (targetSitBoneSetbackMm + s.RailCenterToSitBonesMm) - seatpostSetbackMm
		stMinus5 := st - model.SaddleRailIdealMarginMm
		stPlus5 := st + model.SaddleRailIdealMarginMm

		if i == 0 {
			minST = st
			maxST = st
			maxSTMinus5 = stMinus5
			minSTPlus5 = stPlus5
		} else {
			if st < minST {
				minST = st
			}
			if st > maxST {
				maxST = st
			}
			if stMinus5 > maxSTMinus5 {
				maxSTMinus5 = stMinus5
			}
			if stPlus5 < minSTPlus5 {
				minSTPlus5 = stPlus5
			}
		}
	}

	compST := (minST + maxST) / 2.0
	maxOffset := (maxST - minST) / 2.0
	compSTA := staForSetback(compST)

	hasUnivMargin := maxSTMinus5 <= minSTPlus5
	var uMinSTA, uMaxSTA float64
	if hasUnivMargin {
		sta1 := staForSetback(minSTPlus5)
		sta2 := staForSetback(maxSTMinus5)
		uMinSTA = math.Min(sta1, sta2)
		uMaxSTA = math.Max(sta1, sta2)
	}

	var offsets []SaddleClampOffset
	for _, s := range saddles {
		st := (targetSitBoneSetbackMm + s.RailCenterToSitBonesMm) - seatpostSetbackMm
		// ClampOffset = RailCenterSetback - (SeatTubeSetback + SeatpostSetback)
		// = (targetSitBoneSetbackMm + s.RailCenterToSitBonesMm) - (compST + seatpostSetbackMm)
		// = st - compST
		offset := st - compST
		offsets = append(offsets, SaddleClampOffset{
			SaddleID:   s.ID,
			SaddleName: s.Name,
			OffsetMm:   Round(offset, 1),
		})
	}

	return UniversalCompromiseStaResult{
		SeatpostDesc:          seatpostDesc,
		SeatpostSetbackMm:     Round(seatpostSetbackMm, 1),
		CompromiseStaDeg:      compSTA,
		MaxRailOffsetMm:       Round(maxOffset, 1),
		UniversalMarginMinSta: uMinSTA,
		UniversalMarginMaxSta: uMaxSTA,
		HasUniversalMargin:    hasUnivMargin,
		SaddleOffsets:         offsets,
	}
}

// CalculateIdealSeatTubeAngle calculates the ideal effective Seat Tube Angle for a custom frame
// to mount the given saddle dead-center on the rails (0.0mm clamp offset) at the specified
// saddle height and seatpost hardware setback to achieve targetSitBoneSetbackMm.
func CalculateIdealSeatTubeAngle(
	saddle model.SaddleSpec,
	targetSitBoneSetbackMm float64,
	saddleHeightMm float64,
	seatpostSetbackMm float64,
	seatpostDesc string,
	clampLengthMm float64,
) IdealStaResult {
	if saddleHeightMm <= 0 {
		return IdealStaResult{Saddle: saddle}
	}
	if clampLengthMm <= 0 {
		clampLengthMm = model.DefaultSeatpostClampLengthMm
	}

	// 1. Dead-center rail center setback:
	railCenterSetback := targetSitBoneSetbackMm + saddle.RailCenterToSitBonesMm
	stSetback := railCenterSetback - seatpostSetbackMm

	staForSetback := func(sb float64) float64 {
		ratio := sb / saddleHeightMm
		if ratio < -1.0 {
			ratio = -1.0
		} else if ratio > 1.0 {
			ratio = 1.0
		}
		return Round(ToDeg(math.Acos(ratio)), 2)
	}

	idealSTA := staForSetback(stSetback)

	// 2. Clamp window bounds:
	// Clamping forward on rails (positive X_clamp) moves saddle rearward relative to clamp,
	// meaning for the same target setback, the seat tube must sit further forward (smaller ST setback -> steeper STA).
	// Clamping rearward on rails (negative X_clamp) moves saddle forward, requiring seat tube further back (larger ST setback -> slacker STA).
	minClamp, maxClamp := saddle.ClampWindowMm(clampLengthMm)
	stSetbackForwardClamp := stSetback - maxClamp // steeper STA
	stSetbackRearClamp := stSetback - minClamp    // slacker STA

	sta1 := staForSetback(stSetbackRearClamp)
	sta2 := staForSetback(stSetbackForwardClamp)
	slackestSTA := math.Min(sta1, sta2)
	steepestSTA := math.Max(sta1, sta2)

	marginMinClamp := -model.SaddleRailIdealMarginMm
	marginMaxClamp := model.SaddleRailIdealMarginMm
	mSta1 := staForSetback(stSetback - marginMinClamp)
	mSta2 := staForSetback(stSetback - marginMaxClamp)
	marginSlackSTA := math.Min(mSta1, mSta2)
	marginSteepSTA := math.Max(mSta1, mSta2)

	return IdealStaResult{
		Saddle:                    saddle,
		TargetSitBoneSetbackMm:    Round(targetSitBoneSetbackMm, 1),
		SaddleHeightMm:            Round(saddleHeightMm, 1),
		SeatpostSetbackMm:         Round(seatpostSetbackMm, 1),
		SeatpostDesc:              seatpostDesc,
		TargetRailCenterSetbackMm: Round(railCenterSetback, 1),
		SeatTubeSetbackMm:         Round(stSetback, 1),
		IdealStaDeg:               idealSTA,
		MinStaDeg:                 slackestSTA,
		MaxStaDeg:                 steepestSTA,
		IdealMarginMinStaDeg:      marginSlackSTA,
		IdealMarginMaxStaDeg:      marginSteepSTA,
	}
}

// SaddleTranslationResult encapsulates the cross-saddle translation for pelvic equivalence.
type SaddleTranslationResult struct {
	ReferenceSaddle      model.SaddleSpec
	TargetSaddle         model.SaddleSpec
	ReferenceClampOffset float64
	TargetClampOffset    float64
	SitBoneDeltaMm       float64 // Difference in rail-to-sit-bone distance (target - ref)
	BrpDeltaMm           float64 // Difference in BRP-to-sit-bone span (target - ref)
	ClampLengthMm        float64
	TargetClampMin       float64
	TargetClampMax       float64
	IsFeasible           bool
}

// TranslateSaddleFit translates a clamp position on refSaddle to targetSaddle to keep
// the rider's sit bones in the exact same spatial coordinate.
func TranslateSaddleFit(
	refSaddle model.SaddleSpec,
	refClampOffset float64,
	targetSaddle model.SaddleSpec,
	clampLengthMm float64,
) SaddleTranslationResult {
	if clampLengthMm <= 0 {
		clampLengthMm = model.DefaultSeatpostClampLengthMm
	}

	targetClamp := refSaddle.TranslateClampOffset(refClampOffset, targetSaddle)
	sbDelta := targetSaddle.RailCenterToSitBonesMm - refSaddle.RailCenterToSitBonesMm
	brpDelta := targetSaddle.DeltaBrpToSitBonesMm() - refSaddle.DeltaBrpToSitBonesMm()

	minClamp, maxClamp := targetSaddle.ClampWindowMm(clampLengthMm)

	isFeasible := targetClamp >= minClamp-0.001 && targetClamp <= maxClamp+0.001

	return SaddleTranslationResult{
		ReferenceSaddle:      refSaddle,
		TargetSaddle:         targetSaddle,
		ReferenceClampOffset: Round(refClampOffset, 1),
		TargetClampOffset:    Round(targetClamp, 1),
		SitBoneDeltaMm:       Round(sbDelta, 1),
		BrpDeltaMm:           Round(brpDelta, 1),
		ClampLengthMm:        clampLengthMm,
		TargetClampMin:       Round(minClamp, 1),
		TargetClampMax:       Round(maxClamp, 1),
		IsFeasible:           isFeasible,
	}
}

// CalculateSaddleBRPPoint computes saddle coordinates for a specific SaddleSpec.
func CalculateSaddleBRPPoint(
	staDeg, seatHeight, seatpostSetbackMm float64,
	saddle model.SaddleSpec,
	railOffsetMm float64,
) model.BRPPoint {
	return saddle.CalculatePoint(staDeg, seatHeight, seatpostSetbackMm, railOffsetMm)
}

// MapPossibleSaddleBRPs maps candidate BRP points within the clamp window
// of the given saddle and clamp length.
func MapPossibleSaddleBRPs(
	staDeg, seatHeight, seatpostSetbackMm float64,
	saddle model.SaddleSpec,
	clampLengthMm float64,
) []model.BRPPoint {
	return saddle.MapPossibleBRPs(staDeg, seatHeight, seatpostSetbackMm, clampLengthMm)
}
