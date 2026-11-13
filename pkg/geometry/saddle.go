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
	IdealStaDeg               model.Deg // STA with saddle clamped dead-center (0.0mm)
	MinStaDeg                 model.Deg // Slackest STA within clamp window
	MaxStaDeg                 model.Deg // Steepest STA within clamp window
	IdealMarginMinStaDeg      model.Deg // Slackest STA within ±5mm ideal rail margin
	IdealMarginMaxStaDeg      model.Deg // Steepest STA within ±5mm ideal rail margin
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
	CompromiseStaDeg      model.Deg
	MaxRailOffsetMm       float64
	UniversalMarginMinSta model.Deg
	UniversalMarginMaxSta model.Deg
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

	ref := model.ReferenceIdmatchSaddle()
	refBrpSetback := ref.BrpSetbackFromSitBones(targetSitBoneSetbackMm)

	type saddleFit struct {
		spec      model.SaddleSpec
		eqSH      float64
		targetBrp float64
	}
	fits := make([]saddleFit, len(saddles))
	var minSta0, maxSta0 model.Deg
	var univMarginMin, univMarginMax model.Deg
	marginInit := false

	for i, s := range saddles {
		eqSH := saddleHeightMm
		if s.ID != ref.ID {
			eqSH = s.TranslateSaddleHeightFromRef(ref, saddleHeightMm, refBrpSetback)
		}
		targetBrp := s.BrpSetbackFromSitBones(targetSitBoneSetbackMm)
		fits[i] = saddleFit{spec: s, eqSH: eqSH, targetBrp: targetBrp}

		sta0 := s.StaForBrpAndOffset(targetBrp, eqSH, seatpostSetbackMm, 0.0)
		mMin := s.StaForBrpAndOffset(targetBrp, eqSH, seatpostSetbackMm, model.SaddleRailIdealMarginMm)
		mMax := s.StaForBrpAndOffset(targetBrp, eqSH, seatpostSetbackMm, -model.SaddleRailIdealMarginMm)
		if mMin > mMax {
			mMin, mMax = mMax, mMin
		}

		if i == 0 {
			minSta0 = sta0
			maxSta0 = sta0
		} else {
			if sta0 < minSta0 {
				minSta0 = sta0
			}
			if sta0 > maxSta0 {
				maxSta0 = sta0
			}
		}

		if !marginInit {
			univMarginMin = mMin
			univMarginMax = mMax
			marginInit = true
		} else {
			if mMin > univMarginMin {
				univMarginMin = mMin
			}
			if mMax < univMarginMax {
				univMarginMax = mMax
			}
		}
	}

	// Solve for the minimax compromise STA where max(railOffset) + min(railOffset) == 0.
	offsetBounds := func(sta model.Deg) (float64, float64) {
		var lo, hi float64
		for i, sf := range fits {
			r := sf.spec.RailOffsetForBrpAtSta(sta, sf.targetBrp, sf.eqSH, seatpostSetbackMm)
			if i == 0 {
				lo, hi = r, r
			} else {
				if r < lo {
					lo = r
				}
				if r > hi {
					hi = r
				}
			}
		}
		return lo, hi
	}

	loSTA, hiSTA := minSta0, maxSta0
	for range 64 {
		midSTA := 0.5 * (loSTA + hiSTA)
		loR, hiR := offsetBounds(midSTA)
		if loR+hiR < 0 {
			loSTA = midSTA
		} else {
			hiSTA = midSTA
		}
	}
	compSTA := 0.5 * (loSTA + hiSTA)
	loR, hiR := offsetBounds(compSTA)
	maxOffset := math.Max(math.Abs(loR), math.Abs(hiR))

	hasUnivMargin := marginInit && univMarginMin <= univMarginMax
	var uMinSTA, uMaxSTA model.Deg
	if hasUnivMargin {
		uMinSTA = univMarginMin
		uMaxSTA = univMarginMax
	}

	var offsets []SaddleClampOffset
	for _, sf := range fits {
		offset := sf.spec.RailOffsetForBrpAtSta(compSTA, sf.targetBrp, sf.eqSH, seatpostSetbackMm)
		offsets = append(offsets, SaddleClampOffset{
			SaddleID:   sf.spec.ID,
			SaddleName: sf.spec.Name,
			OffsetMm:   offset,
		})
	}

	return UniversalCompromiseStaResult{
		SeatpostDesc:          seatpostDesc,
		SeatpostSetbackMm:     Round(seatpostSetbackMm, 1),
		CompromiseStaDeg:      compSTA,
		MaxRailOffsetMm:       maxOffset,
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

	// 1. Dead-center target BRP setback & seat-tube setback projection (nominal geometric reference):
	railCenterSetback := targetSitBoneSetbackMm + saddle.RailCenterToSitBonesMm
	stSetback := railCenterSetback - seatpostSetbackMm
	targetBrpSetback := saddle.BrpSetbackFromSitBones(targetSitBoneSetbackMm)

	staForOffset := func(railOffset float64) model.Deg {
		return saddle.StaForBrpAndOffset(targetBrpSetback, saddleHeightMm, seatpostSetbackMm, railOffset)
	}

	idealSTA := staForOffset(0.0)

	// 2. Clamp window bounds:
	// Clamping forward on rails (positive rail offset) pushes saddle body rearward relative to clamp,
	// requiring steeper STA to keep BRP setback unchanged.
	minClamp, maxClamp := saddle.ClampWindowMm(clampLengthMm)
	sta1 := staForOffset(minClamp)
	sta2 := staForOffset(maxClamp)
	slackestSTA := min(sta1, sta2)
	steepestSTA := max(sta1, sta2)

	marginMinClamp := -model.SaddleRailIdealMarginMm
	marginMaxClamp := model.SaddleRailIdealMarginMm
	mSta1 := staForOffset(marginMinClamp)
	mSta2 := staForOffset(marginMaxClamp)
	marginSlackSTA := min(mSta1, mSta2)
	marginSteepSTA := max(mSta1, mSta2)

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
	staDeg model.Deg, seatHeight, seatpostSetbackMm float64,
	saddle model.SaddleSpec,
	railOffsetMm float64,
) model.BRPPoint {
	return saddle.CalculatePoint(staDeg, seatHeight, seatpostSetbackMm, railOffsetMm)
}

// MapPossibleSaddleBRPs maps candidate BRP points within the clamp window
// of the given saddle and clamp length.
func MapPossibleSaddleBRPs(
	staDeg model.Deg, seatHeight, seatpostSetbackMm float64,
	saddle model.SaddleSpec,
	clampLengthMm float64,
) []model.BRPPoint {
	return saddle.MapPossibleBRPs(staDeg, seatHeight, seatpostSetbackMm, clampLengthMm)
}
