package geometry

import (
	"fmt"
	"math"

	"bikegeo/internal/model"
)

// AnalyzeBikeFitWithSaddle runs the master analysis pipeline using an explicit SaddleSpec.
func AnalyzeBikeFitWithSaddle(
	bike model.BikeGeometry,
	targetA2c model.Opt[float64],
	targetRake model.Opt[float64],
	profile model.RiderProfile,
	saddle model.SaddleSpec,
) (model.BikeFitAnalysis, error) {
	if saddle.ID == "" {
		return model.BikeFitAnalysis{}, fmt.Errorf("saddle.ID must be non-empty")
	}
	if profile.SaddleHeightMm <= 0 {
		return model.BikeFitAnalysis{}, fmt.Errorf("profile.SaddleHeightMm must be positive, got %.1f", profile.SaddleHeightMm)
	}

	seatHeight := profile.SaddleHeightMm
	refSQ := model.ReferenceIdmatchSaddle()
	refSitBones := model.ReferenceSitBonePoint(seatHeight, model.DefaultTargetSitBoneSetbackMm())
	brpSeatHeight := saddle.BrpPointFromSitBones(refSitBones).DistanceToOrigin()

	// 1. Validate rake and reverse-engineer baseline CAD geometric A2C for publish-mode inference.
	rake := targetRake.Or(bike.ForkRake)
	if rake <= 0 {
		return model.BikeFitAnalysis{}, fmt.Errorf("fork rake must be positive, got %.1fmm", rake)
	}
	baseStructuralA2c, err := CalculateGeometricA2c(
		bike.Stack,
		bike.BbDrop,
		bike.Htl,
		bike.HtaDeg,
		bike.ForkRake,
	)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}
	geoSpacer := bike.GeoHeadsetSpacerMm
	baseForkA2c := baseStructuralA2c - geoSpacer

	// 2. Construct BikeScene from frame, build, cockpit, seat, and fit inputs.
	frame, err := FrameSizeFromBikeGeometry(bike)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}

	buildFork := frame.CadFork
	buildFork.RakeMm = rake
	buildSag := frame.CadSagPct
	physicalLowerCup := frame.CadLowerCupMm
	if tA2c, ok := targetA2c.Get(); ok {
		if !(bike.CadFork.A2cMm > 0 && tA2c == bike.CadFork.A2cMm) {
			buildFork = Fork{StaticA2cMm: tA2c, RakeMm: rake}
			buildSag = 0
		}
		if bike.HeadsetCupType.IsValid() {
			physicalLowerCup = bike.HeadsetCupType.CupStackMm()
		}
	}
	build := BuildSpec{
		Fork:               buildFork,
		SagPct:             buildSag,
		PhysicalLowerCupMm: physicalLowerCup,
		WheelRadiusFrontMm: bike.WheelRadiusMm(),
		WheelRadiusRearMm:  bike.WheelRadiusMm(),
	}

	bar := profile.Handlebar
	if bar.ID == "" {
		return model.BikeFitAnalysis{}, fmt.Errorf("profile.Handlebar.ID must be non-empty")
	}
	stems := bike.Stems
	if len(stems) == 0 && bike.CanonicalStem.LengthMm > 0 {
		stems = []model.StemSpec{bike.CanonicalStem}
	}
	baseStem := bike.CanonicalStem
	if baseStem.LengthMm <= 0 {
		if len(stems) > 0 && stems[0].LengthMm > 0 {
			baseStem = stems[0]
		} else {
			baseStem = model.StemSpec{LengthMm: 70.0}
		}
	}
	cockpit := CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		SpacerMm:          0.0,
		Stem:              baseStem,
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         bar,
		BarRollDeg:        0.0,
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := SeatSpec{
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        saddle,
		RailOffsetMm:  0.0,
	}
	targetRef := profile.TargetRef
	if targetRef == "" {
		targetRef = TargetHandGrip
	} else if !targetRef.IsValid() {
		return model.BikeFitAnalysis{}, fmt.Errorf("invalid RiderProfile TargetRef %q", profile.TargetRef)
	}
	fitTarget := RiderFit{
		FrameSeatAxisHeightMm: seatHeight,
		BrpHeightMm:           brpSeatHeight,
		TargetSitBonesXMm:     -model.DefaultTargetSitBoneSetbackMm(),
		Target:                DefaultRiderTarget(targetRef),
		ReferenceSaddle:       refSQ,
		ReferenceBar:          profile.ReferenceBar,
	}

	scene, err := BuildScene(frame, build, cockpit, seat, fitTarget)
	if err != nil {
		return model.BikeFitAnalysis{}, err
	}

	activeReach := scene.Reach()
	activeStack := scene.Stack()
	activeHta := scene.HtaDeg
	activeSta := scene.EffectiveStaDeg
	activeBbDrop := scene.BbDrop()
	activeStructuralA2c := scene.StructuralA2cMm
	if !targetA2c.IsSet() {
		activeStructuralA2c = baseStructuralA2c
	}
	deltaAngle := scene.PitchDeg
	deltaReach := activeReach - bike.Reach
	deltaStack := activeStack - bike.Stack
	deltaBbDrop := activeBbDrop - bike.BbDrop

	// 3. Derived frame seat-axis & cockpit metrics from BikeScene
	aett := scene.AettMm()
	ett := scene.EttMm()
	actualSetback := scene.SeatAxisActualSetbackMm()
	verticalStack := scene.SpareVerticalStackMm()
	axialLength := scene.SpareAxialSteererMm()
	seatpostExtension, _ := scene.SeatpostExtensionMm()

	// 4. Infer CAD publish mode
	inferredPublishMode := "Unspecified"
	cadA2c := bike.GetCadA2c()
	if cadA2c > 0 {
		diff := cadA2c - baseForkA2c
		if budget, ok := A2cBudget(bike); ok && budget.Within() {
			inferredPublishMode = "Static Fork (0% Sag CAD)"
		} else if diff >= 15.0 && diff <= 35.0 {
			inferredPublishMode = fmt.Sprintf("Sagged (~20%% Sag CAD, ~%dmm sag)", int(math.Round(diff)))
		} else {
			inferredPublishMode = fmt.Sprintf("Discrepancy (%dmm deviation)", int(math.Round(diff)))
		}
	}

	// 5. Chainstay Description
	activeCS := scene.Frame.ChainstayMm
	csDesc := fmt.Sprintf("%.1f mm [fixed]", activeCS)
	if bike.HasAdjustableChainstay() {
		activeCS = bike.GetMaxChainstay()
		csDesc = fmt.Sprintf("%.1f mm [max of %.0f-%.0f]", activeCS, bike.GetMinChainstay(), bike.GetMaxChainstay())
	}

	// 6. Cockpit BRP & HB Points and Saddle-Handlebar Distance from BikeScene
	deadCenterBRP := model.BRPPoint{
		Saddle:                saddle,
		SeatpostSetbackMm:     seat.PostSetbackMm,
		SaddleBrpOffsetMm:     saddle.RailCenterToBrpMm,
		RailOffsetMm:          seat.RailOffsetMm,
		AchievedClampOffsetMm: seat.RailOffsetMm,
		TotalPerpOffsetMm:     seat.PostSetbackMm + seat.RailOffsetMm - saddle.RailCenterToBrpMm,
		BrpX:                  scene.BRP.X,
		BrpY:                  scene.BRP.Y,
		SitBoneX:              scene.SitBones.X,
		SitBoneY:              scene.SitBones.Y,
		BrpSetbackMm:          scene.BrpSetbackMm(),
		SitBoneSetbackMm:      scene.SitBoneSetbackMm(),
		BrpAngleFromBbDeg:     scene.BrpAngleFromBbDeg(),
	}
	possibleBRPs := MapPossibleSaddleBRPs(
		activeSta,
		brpSeatHeight,
		model.DefaultSeatpostSetbackMm,
		saddle,
		model.DefaultSeatpostClampLengthMm,
	)
	selectedBRP := deadCenterBRP

	spacers := bike.GetSpacersMm()
	var possibleHBs []model.HBPoint
	for _, st := range stems {
		for _, sp := range spacers {
			cand := cockpit
			cand.Stem = st
			cand.SpacerMm = sp
			sc, err := scene.WithCockpit(cand)
			if err != nil {
				return model.BikeFitAnalysis{}, err
			}
			possibleHBs = append(possibleHBs, model.HBPoint{
				Stem:           st,
				SpacerMm:       sp,
				ClampX:         sc.StemClamp.X,
				ClampY:         sc.StemClamp.Y,
				HbX:            sc.HandGrip.X,
				HbY:            sc.HandGrip.Y,
				SaddleHbMm:     sc.GripDistFromBrpMm(),
				SaddleHbDropMm: sc.GripRiseAboveSitBonesMm(),
			})
		}
	}
	var selectedHB *model.HBPoint
	var saddleHbMm float64
	var saddleHbDropMm float64
	if best, ok := canonicalHB(possibleHBs, bike.GetCanonicalStem()); ok {
		selectedHB = &best
		saddleHbMm = best.SaddleHbMm
		saddleHbDropMm = best.SaddleHbDropMm
	}

	solveAngle := baseStem.AngleDeg
	if selectedHB != nil {
		solveAngle = selectedHB.Stem.AngleDeg
	}
	var stemSpacerFit *model.StemSpacerFit
	if solved, ok := scene.SolveStemSpacerFit(solveAngle); ok {
		stemSpacerFit = &model.StemSpacerFit{
			StemAngleDeg: solved.StemAngleDeg,
			StemLengthMm: solved.StemLengthMm,
			SpacerMm:     solved.SpacerMm,
		}
	}

	idealSTA := saddle.IdealSeatTubeAngle(model.DefaultTargetSitBoneSetbackMm(), brpSeatHeight, model.DefaultSeatpostSetbackMm)

	return model.BikeFitAnalysis{
		StructuralA2c:       activeStructuralA2c,
		ActiveReach:         activeReach,
		ActiveStack:         activeStack,
		ActiveHta:           activeHta,
		ActiveSta:           activeSta,
		ActiveBbDrop:        activeBbDrop,
		Aett:                aett,
		Ett:                 ett,
		ActualSetback:       actualSetback,
		SpareVerticalStack:  verticalStack,
		SpareAxialSteerer:   axialLength,
		SeatpostExtensionMm: seatpostExtension,
		DeltaAngleDeg:       deltaAngle,
		DeltaReach:          deltaReach,
		DeltaStack:          deltaStack,
		DeltaBbDrop:         deltaBbDrop,
		InferredPublishMode: inferredPublishMode,

		RearCenterWheelbaseRatio: scene.RearCenterWheelbaseRatio(),
		SeatToRearCenter:         scene.RearCenter() - actualSetback,
		WheelbaseCalc:            scene.Wheelbase(),
		ActiveChainstay:          activeCS,
		ChainstayDesc:            csDesc,
		TrailMm:                  scene.TrailMm(),

		Saddle:              saddle,
		Handlebar:           bar,
		SitBoneSetbackMm:    deadCenterBRP.SitBoneSetbackMm,
		IdealStaDeg:         Round(idealSTA, 1),
		AchievedClampOffset: deadCenterBRP.AchievedClampOffsetMm,
		DeadCenterBRP:       deadCenterBRP,
		SelectedBRP:         &selectedBRP,
		PossibleBRPs:        possibleBRPs,
		SaddleHbMm:          saddleHbMm,
		SaddleHbDropMm:      saddleHbDropMm,
		SelectedHB:          selectedHB,
		StemSpacerFit:       stemSpacerFit,
		PossibleHBs:         possibleHBs,
	}, nil
}
