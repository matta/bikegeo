package geometry_test

import (
	"math"
	"testing"

	"bikegeo/internal/catalog"
	"bikegeo/internal/frames"
	"bikegeo/internal/geom2d"
	"bikegeo/internal/geometry"
	"bikegeo/internal/model"
)

func baseTestInputs() (geometry.FrameSize, geometry.BuildSpec, geometry.CockpitSpec, geometry.SeatSpec, geometry.RiderFit) {
	frame := geometry.FrameSize{
		ID:            "seven-custom-m",
		Size:          "M",
		ReachMm:       424.0,
		StackMm:       631.0,
		HtaDeg:        70.0,
		StaDeg:        73.0,
		HtlMm:         135.0,
		BbDropMm:      68.0,
		ChainstayMm:   448.0,
		CadFork:       geometry.Fork{ID: "enve_mountain_boost", StaticA2cMm: 492.0, RakeMm: 52.0},
		CadSagPct:     0,
		CadLowerCupMm: 11.0,
	}
	build := geometry.BuildSpec{
		Fork:               frame.CadFork,
		SagPct:             0,
		PhysicalLowerCupMm: 11.0,
		WheelRadiusFrontMm: model.DefaultWheelRadius29Mm,
		WheelRadiusRearMm:  model.DefaultWheelRadius29Mm,
	}
	bar, _ := catalog.GetHandlebar("persuader20_800")
	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: model.DefaultHeadsetTopStackMm,
		SpacerMm:          0.0,
		Stem:              model.StemSpec{LengthMm: 70.0, AngleDeg: -6.0},
		StemClampHeightMm: model.StemThicknessMm,
		Handlebar:         bar,
		BarRollDeg:        0.0,
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	saddle := catalog.DefaultSaddle()
	seat := geometry.SeatSpec{
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        saddle,
		RailOffsetMm:  0.0,
	}
	fit := geometry.RiderFit{
		FrameSeatAxisHeightMm: 746.0,
		BrpHeightMm:           746.0,
		TargetSitBonesXMm:     -292.0,
		Target:                geometry.RiderTarget{Point: geom2d.Point2{X: 480.9, Y: 726.1}, Ref: geometry.TargetHandGrip},
		ReferenceSaddle:       saddle,
	}
	return frame, build, cockpit, seat, fit
}

func TestBuildScene_DerivedMetricsAndCockpit(t *testing.T) {
	frame, build, cockpit, seat, fit := baseTestInputs()
	frame.SeatTubeLengthMm = model.Some(450.0)
	cockpit.SpacerMm = 15.0
	cockpit.BarRollDeg = 5.0

	sc, err := geometry.BuildScene(frame, build, cockpit, seat, fit)
	if err != nil {
		t.Fatalf("BuildScene: %v", err)
	}

	// ETT = Reach + Stack / tan(NominalSTA)
	wantEtt := sc.Reach() + sc.Stack()/sc.NominalStaDeg.Angle().Tan()
	if d := math.Abs(sc.EttMm() - wantEtt); d > 1e-9 {
		t.Errorf("EttMm = %.6f, want %.6f", sc.EttMm(), wantEtt)
	}

	// SeatAxisActualSetback = H * cos(seatAxisSTA)
	seatAxisSta := frame.EffectiveStaDegAt(fit.FrameSeatAxisHeightMm) - sc.PitchDeg
	wantSetback := fit.FrameSeatAxisHeightMm * seatAxisSta.Angle().Cos()
	if d := math.Abs(sc.SeatAxisActualSetbackMm() - wantSetback); d > 1e-9 {
		t.Errorf("SeatAxisActualSetbackMm = %.6f, want %.6f", sc.SeatAxisActualSetbackMm(), wantSetback)
	}

	// AETT = horizontal distance at SeatAxisPoint.Y from SeatAxisPoint to steerer axis.
	seatY := fit.FrameSeatAxisHeightMm * seatAxisSta.Angle().Sin()
	steererAtSeatY := sc.Reach() - (seatY-sc.Stack())/sc.HtaDeg.Angle().Tan()
	wantAett := steererAtSeatY - (-wantSetback)
	if d := math.Abs(sc.AettMm() - wantAett); d > 1e-9 {
		t.Errorf("AettMm = %.6f, want %.6f", sc.AettMm(), wantAett)
	}

	// Spare steerer stack
	wantVert := seatY - sc.Stack()
	wantAxial := wantVert / sc.HtaDeg.Angle().Sin()
	if d := math.Abs(sc.SpareVerticalStackMm() - wantVert); d > 1e-9 {
		t.Errorf("SpareVerticalStackMm = %.6f, want %.6f", sc.SpareVerticalStackMm(), wantVert)
	}
	if d := math.Abs(sc.SpareAxialSteererMm() - wantAxial); d > 1e-9 {
		t.Errorf("SpareAxialSteererMm = %.6f, want %.6f", sc.SpareAxialSteererMm(), wantAxial)
	}

	// Seatpost extension
	ext, ok := sc.SeatpostExtensionMm()
	if !ok || math.Abs(ext-(746.0-450.0)) > 1e-9 {
		t.Errorf("SeatpostExtensionMm = (%.4f, %v), want (296.0, true)", ext, ok)
	}

	// Direct cockpit point checks (StemClamp, HandGrip, GripDistFromBrpMm, GripRiseAboveSitBonesMm)
	uSteer := geometry.SteererTubeDir(sc.HtaDeg)
	nSteer := uSteer.Perpendicular(geom2d.Clockwise)
	dSteer := cockpit.HeadsetTopStackMm + cockpit.SpacerMm + cockpit.StemClampHeightMm/2.0
	wantClamp := sc.HeadTubeTop.Add(uSteer.Scale(dSteer)).Add(nSteer.Rotate(cockpit.Stem.AngleDeg.Angle()).Scale(cockpit.Stem.LengthMm))
	if d := sc.StemClamp.DistanceTo(wantClamp); d > 1e-9 {
		t.Errorf("StemClamp diff %.12f", d)
	}
	barOff := geometry.GetHandlebarOffset(cockpit.Handlebar, cockpit.GripInsetMm, cockpit.BarRollDeg)
	wantGrip := wantClamp.Add(geom2d.Vec2{X: barOff.X, Y: barOff.Y})
	if d := sc.HandGrip.DistanceTo(wantGrip); d > 1e-9 {
		t.Errorf("HandGrip diff %.12f", d)
	}
	if d := math.Abs(sc.GripDistFromBrpMm() - sc.HandGrip.DistanceTo(sc.BRP)); d > 1e-9 {
		t.Errorf("GripDistFromBrpMm diff %.12f", d)
	}
	if d := math.Abs(sc.GripRiseAboveSitBonesMm() - (sc.HandGrip.Y - sc.SitBones.Y)); d > 1e-9 {
		t.Errorf("GripRiseAboveSitBonesMm diff %.12f", d)
	}
}

func TestBuildScene_CADBaselineAndResiduals(t *testing.T) {
	_, _, cockpit, seat, fit := baseTestInputs()
	for _, frameModel := range frames.AllFrames() {
		for _, sz := range frameModel.GetSizes() {
			bike, err := geometry.MaterializeBikeGeometry(frameModel, sz, catalog.ExpandStemSetsFrom(catalog.CommonStemCatalog, frameModel.Stems))
			if err != nil {
				t.Fatalf("MaterializeBikeGeometry(%s, %s): %v", frameModel.ID, sz, err)
			}
			frame, err := geometry.FrameSizeFromBikeGeometry(bike)
			if err != nil {
				t.Fatalf("FrameSizeFromBikeGeometry(%s, %s): %v", frameModel.ID, sz, err)
			}
			build := geometry.BuildSpec{
				Fork:               frame.CadFork,
				SagPct:             frame.CadSagPct,
				PhysicalLowerCupMm: frame.CadLowerCupMm,
				WheelRadiusFrontMm: bike.WheelRadiusMm(),
				WheelRadiusRearMm:  bike.WheelRadiusMm(),
			}
			scene, err := geometry.BuildScene(frame, build, cockpit, seat, fit)
			if err != nil {
				t.Fatalf("BuildScene(%s, %s): %v", frameModel.ID, sz, err)
			}
			if scene.PitchDeg != 0 {
				t.Errorf("%s %s: CAD baseline PitchDeg = %v, want 0", frameModel.ID, sz, scene.PitchDeg)
			}
			if scene.Reach() != bike.Reach || scene.Stack() != bike.Stack || scene.HtaDeg != bike.HtaDeg || scene.NominalStaDeg != bike.StaDeg || scene.BbDrop() != bike.BbDrop {
				t.Errorf("%s %s: CAD baseline did not reproduce published chart exactly", frameModel.ID, sz)
			}
			if a2cBudget, ok := geometry.A2cBudget(bike); ok {
				if d := math.Abs(scene.ResidualA2cMm - a2cBudget.Residual); d > 1e-9 {
					t.Errorf("%s %s: ResidualA2cMm %.9f != A2cBudget.Residual %.9f", frameModel.ID, sz, scene.ResidualA2cMm, a2cBudget.Residual)
				}
			}
			if fcBudget, ok := geometry.FrontCenterBudget(bike); ok {
				if resFc, has := scene.ResidualFcMm.Get(); !has || math.Abs(resFc-fcBudget.Residual) > 1e-9 {
					t.Errorf("%s %s: ResidualFcMm %v != FrontCenterBudget.Residual %.9f", frameModel.ID, sz, scene.ResidualFcMm, fcBudget.Residual)
				}
			}
			if wbBudget, ok := geometry.WheelbaseBudget(bike); ok {
				if resWb, has := scene.ResidualWbMm.Get(); !has || math.Abs(resWb-wbBudget.Residual) > 1e-9 {
					t.Errorf("%s %s: ResidualWbMm %v != WheelbaseBudget.Residual %.9f", frameModel.ID, sz, scene.ResidualWbMm, wbBudget.Residual)
				}
			}
		}
	}
}

func TestBuildScene_LevelsBothAxlesOnRakeAndCupChanges(t *testing.T) {
	_, _, cockpit, seat, fit := baseTestInputs()
	for _, frameModel := range frames.AllFrames() {
		for _, sz := range frameModel.GetSizes() {
			bike, err := geometry.MaterializeBikeGeometry(frameModel, sz, catalog.ExpandStemSetsFrom(catalog.CommonStemCatalog, frameModel.Stems))
			if err != nil {
				t.Fatalf("MaterializeBikeGeometry(%s, %s): %v", frameModel.ID, sz, err)
			}
			frame, err := geometry.FrameSizeFromBikeGeometry(bike)
			if err != nil {
				t.Fatalf("FrameSizeFromBikeGeometry(%s, %s): %v", frameModel.ID, sz, err)
			}

			wantHtBb := math.Hypot(frame.ReachMm, frame.StackMm)
			u0 := geometry.SteererTubeDir(frame.HtaDeg)
			htb0 := geom2d.Point2{X: frame.ReachMm, Y: frame.StackMm}.SubVec(u0.Scale(frame.HtlMm))
			wantHtbBb := htb0.DistanceToOrigin()
			wantNominalInc := frame.HtaDeg - frame.StaDeg

			for _, testRake := range []float64{44.0, 51.0, 55.0} {
				for _, testCup := range []float64{frame.CadLowerCupMm, bike.HeadsetCupType.CupStackMm()} {
					for _, sag := range []float64{0.0, 20.0} {
						travel := 0.0
						a2c := 495.0
						if sag > 0 {
							travel = 120.0
							a2c = 530.0
						}
						build := geometry.BuildSpec{
							Fork:               geometry.Fork{ID: "swap", StaticA2cMm: a2c, RakeMm: testRake, TravelMm: travel},
							SagPct:             sag,
							PhysicalLowerCupMm: testCup,
							WheelRadiusFrontMm: bike.WheelRadiusMm(),
							WheelRadiusRearMm:  bike.WheelRadiusMm(),
						}
						scene, err := geometry.BuildScene(frame, build, cockpit, seat, fit)
						if err != nil {
							t.Fatalf("BuildScene(%s %s rake=%.0f cup=%.0f sag=%.0f): %v", frameModel.ID, sz, testRake, testCup, sag, err)
						}
						if d := math.Abs(scene.FrontAxle.Y - scene.RearAxle.Y); d > 1e-9 {
							t.Errorf("%s %s rake=%.0f cup=%.0f: |FA.Y - RA.Y| = %.12f mm", frameModel.ID, sz, testRake, testCup, d)
						}
						if d := math.Abs(scene.HeadTubeTop.DistanceToOrigin() - wantHtBb); d > 1e-9 {
							t.Errorf("%s %s: |HT-BB| drifted %.12f mm", frameModel.ID, sz, d)
						}
						if d := math.Abs(scene.HeadTubeBottom.DistanceToOrigin() - wantHtbBb); d > 1e-9 {
							t.Errorf("%s %s: |HTB-BB| drifted %.12f mm", frameModel.ID, sz, d)
						}
						if d := math.Abs(scene.RearAxle.DistanceToOrigin() - frame.ChainstayMm); d > 1e-9 {
							t.Errorf("%s %s: |RA-BB| drifted %.12f mm", frameModel.ID, sz, d)
						}
						if d := math.Abs(scene.HeadTubeTop.DistanceTo(scene.HeadTubeBottom) - frame.HtlMm); d > 1e-9 {
							t.Errorf("%s %s: |HT-HTB| drifted %.12f mm", frameModel.ID, sz, d)
						}
						if d := ((scene.HtaDeg - scene.NominalStaDeg) - wantNominalInc).Abs(); d > 1e-9 {
							t.Errorf("%s %s: nominal (HTA-STA) drifted %.12f deg", frameModel.ID, sz, d)
						}
					}
				}
			}

			// Cup vs A2C interchangeability on equal rake.
			const extra = 10.0
			cupBuild := geometry.BuildSpec{
				Fork:               geometry.Fork{ID: "f", StaticA2cMm: 506.0, RakeMm: 44.0},
				PhysicalLowerCupMm: frame.CadLowerCupMm + extra,
				WheelRadiusFrontMm: bike.WheelRadiusMm(),
				WheelRadiusRearMm:  bike.WheelRadiusMm(),
			}
			a2cBuild := geometry.BuildSpec{
				Fork:               geometry.Fork{ID: "f", StaticA2cMm: 506.0 + extra, RakeMm: 44.0},
				PhysicalLowerCupMm: frame.CadLowerCupMm,
				WheelRadiusFrontMm: bike.WheelRadiusMm(),
				WheelRadiusRearMm:  bike.WheelRadiusMm(),
			}
			scCup, err := geometry.BuildScene(frame, cupBuild, cockpit, seat, fit)
			if err != nil {
				t.Fatalf("scCup: %v", err)
			}
			scA2c, err := geometry.BuildScene(frame, a2cBuild, cockpit, seat, fit)
			if err != nil {
				t.Fatalf("scA2c: %v", err)
			}
			if d := scCup.HeadTubeTop.DistanceTo(scA2c.HeadTubeTop); d > 1e-9 {
				t.Errorf("%s %s: cup vs A2C HT diff %.12f", frameModel.ID, sz, d)
			}
			if d := scCup.FrontAxle.DistanceTo(scA2c.FrontAxle); d > 1e-9 {
				t.Errorf("%s %s: cup vs A2C FA diff %.12f", frameModel.ID, sz, d)
			}
			if d := (scCup.HtaDeg - scA2c.HtaDeg).Abs(); d > 1e-9 {
				t.Errorf("%s %s: cup vs A2C HTA diff %.12f", frameModel.ID, sz, d)
			}
		}
	}
}

func TestBuildScene_TargetRefModes(t *testing.T) {
	frame, build, cockpit, seat, baseFit := baseTestInputs()

	// 1. Seven Custom C1 calibration with HeadsetTopStackMm = 11:
	// StemClamp sits at ~(481.3, 677.1), HandGrip at ~(408.5, 722.6).
	baseScene, err := geometry.BuildScene(frame, build, cockpit, seat, baseFit)
	if err != nil {
		t.Fatalf("BuildScene base: %v", err)
	}
	if math.Abs(baseScene.StemClamp.X-481.3) > 0.2 || math.Abs(baseScene.StemClamp.Y-677.1) > 0.2 {
		t.Errorf("StemClamp = (%.2f, %.2f), want ~(481.3, 677.1)", baseScene.StemClamp.X, baseScene.StemClamp.Y)
	}

	// TargetHandGrip: C* = TargetPoint - BarOffset(persuader) = (480.9 - (-72.8), 726.1 - 45.5) = (553.7, 680.6)
	if math.Abs(baseScene.RequiredStemClamp.X-553.7) > 0.2 || math.Abs(baseScene.RequiredStemClamp.Y-680.6) > 0.2 {
		t.Errorf("TargetHandGrip C* = (%.2f, %.2f), want ~(553.7, 680.6)", baseScene.RequiredStemClamp.X, baseScene.RequiredStemClamp.Y)
	}

	// 2. TargetStemClamp: C* = TargetPoint directly (e.g. Seven's stem-center fit-sheet target)
	fitClamp := baseFit
	fitClamp.Target.Point = geom2d.Point2{X: 482.0, Y: 676.0}
	fitClamp.Target.Ref = geometry.TargetStemClamp
	scClamp, err := geometry.BuildScene(frame, build, cockpit, seat, fitClamp)
	if err != nil {
		t.Fatalf("BuildScene clamp: %v", err)
	}
	if scClamp.RequiredStemClamp != fitClamp.Target.Point {
		t.Errorf("TargetStemClamp C* = %+v, want %+v", scClamp.RequiredStemClamp, fitClamp.Target.Point)
	}

	// 3. TargetReferenceBarClamp:
	// With a flat zero-sweep/zero-rise test bar, ReferenceBarOffset == (0,0), matching TargetHandGrip!
	flatBar := model.HandlebarSpec{
		ID:                   "flat_zero",
		Name:                 "Flat Test Bar",
		StockWidthMm:         780.0,
		CurrentWidthMm:       780.0,
		StatedRiseMm:         0.0,
		BacksweepDeg:         0.0,
		UpsweepDeg:           0.0,
		EffectiveBendStartMm: 100.0,
		RiseMode:             model.RiseModeStandardKnuckle,
	}
	fitRefFlat := baseFit
	fitRefFlat.Target.Ref = geometry.TargetReferenceBarClamp
	fitRefFlat.ReferenceBar = model.Some(flatBar)
	scRefFlat, err := geometry.BuildScene(frame, build, cockpit, seat, fitRefFlat)
	if err != nil {
		t.Fatalf("BuildScene ref flat: %v", err)
	}
	if d := scRefFlat.RequiredStemClamp.DistanceTo(baseScene.RequiredStemClamp); d > 1e-9 {
		t.Errorf("TargetReferenceBarClamp with flat zero bar differed from TargetHandGrip by %.12f mm", d)
	}

	// With ReferenceBar == current Bar at neutral roll, C* == TargetPoint (when cockpit.BarRollDeg == 0).
	fitRefSame := baseFit
	fitRefSame.Target.Ref = geometry.TargetReferenceBarClamp
	fitRefSame.ReferenceBar = model.Some(cockpit.Handlebar)
	scRefSame, err := geometry.BuildScene(frame, build, cockpit, seat, fitRefSame)
	if err != nil {
		t.Fatalf("BuildScene ref same: %v", err)
	}
	if d := scRefSame.RequiredStemClamp.DistanceTo(fitRefSame.Target.Point); d > 1e-9 {
		t.Errorf("TargetReferenceBarClamp with identical bar differed from TargetPoint by %.12f mm", d)
	}

	// Verify invariant RequiredStemClamp - StemClamp == TargetPoint - evaluatedCockpitPoint across all TargetRef modes.
	cockpitRolled := cockpit
	cockpitRolled.BarRollDeg = 5.0
	for _, f := range []geometry.RiderFit{baseFit, fitClamp, fitRefFlat, fitRefSame} {
		sc, err := geometry.BuildScene(frame, build, cockpitRolled, seat, f)
		if err != nil {
			t.Fatalf("BuildScene(%s): %v", f.Target.Ref, err)
		}
		got := sc.TargetPointError()
		want := sc.Fit.Target.Point.Sub(evaluatedCockpitPoint(sc))
		if d := math.Abs(got.X-want.X) + math.Abs(got.Y-want.Y); d > 1e-9 {
			t.Errorf("%s: TargetPointError %+v != TargetPoint - evaluatedCockpitPoint %+v", f.Target.Ref, got, want)
		}
	}

	// Verify RequiredRailOffsetMm and RailMarginMm directly on BikeScene across saddles.
	for _, s := range catalog.AllSaddles() {
		refSitBones := model.ReferenceSitBonePoint(746.0, 292.0)
		seatS := seat
		seatS.Saddle = s
		seatS.RailOffsetMm = 0.0
		fitS := baseFit
		fitS.BrpHeightMm = s.BrpPointFromSitBones(refSitBones).DistanceToOrigin()
		fitS.TargetSitBonesXMm = -292.0
		sc0, err := geometry.BuildScene(frame, build, cockpit, seatS, fitS)
		if err != nil {
			t.Fatalf("BuildScene(%s): %v", s.ID, err)
		}
		seatSolved := seatS
		seatSolved.RailOffsetMm = sc0.RequiredRailOffsetMm
		scSolved, err := geometry.BuildScene(frame, build, cockpit, seatSolved, fitS)
		if err != nil {
			t.Fatalf("BuildScene solved(%s): %v", s.ID, err)
		}
		if d := math.Abs(scSolved.SitBones.X - (-292.0)); d > 1e-6 {
			t.Errorf("%s: solved SitBones.X = %.6f, want -292.0", s.ID, scSolved.SitBones.X)
		}
		if d := math.Abs(scSolved.SitBones.Y - refSitBones.Y); d > 1e-6 {
			t.Errorf("%s: solved SitBones.Y = %.6f, want %.6f", s.ID, scSolved.SitBones.Y, refSitBones.Y)
		}
		wantMargin := seatS.HalfRailWindowMm() - math.Abs(sc0.RequiredRailOffsetMm)
		if d := math.Abs(sc0.RailMarginMm - wantMargin); d > 1e-9 {
			t.Errorf("%s: RailMarginMm = %.9f, want %.9f", s.ID, sc0.RailMarginMm, wantMargin)
		}
	}
}

func TestBuildScene_ValidationAndZeroDefaults(t *testing.T) {
	frame, build, cockpit, seat, fit := baseTestInputs()

	badCases := []struct {
		name   string
		mutate func(*geometry.FrameSize, *geometry.BuildSpec, *geometry.CockpitSpec, *geometry.SeatSpec, *geometry.RiderFit)
	}{
		{"missing TargetRef", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, fi *geometry.RiderFit) {
			fi.Target.Ref = ""
		}},
		{"invalid TargetRef", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, fi *geometry.RiderFit) {
			fi.Target.Ref = "hb"
		}},
		{"TargetReferenceBarClamp missing ReferenceBar", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, fi *geometry.RiderFit) {
			fi.Target.Ref = geometry.TargetReferenceBarClamp
			fi.ReferenceBar = model.Opt[model.HandlebarSpec]{}
		}},
		{"zero fork rake", func(_ *geometry.FrameSize, b *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			b.Fork.RakeMm = 0
		}},
		{"zero CAD fork rake", func(fr *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			fr.CadFork.RakeMm = 0
		}},
		{"sag on rigid fork", func(_ *geometry.FrameSize, b *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			b.Fork.TravelMm = 0
			b.SagPct = 20
		}},
		{"zero wheel radius", func(_ *geometry.FrameSize, b *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			b.WheelRadiusFrontMm = 0
		}},
		{"zero grip inset", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, c *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			c.GripInsetMm = 0
		}},
		{"zero stem clamp height", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, c *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			c.StemClampHeightMm = 0
		}},
		{"zero frame seat axis height", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, fi *geometry.RiderFit) {
			fi.FrameSeatAxisHeightMm = 0
		}},
		{"chainstay <= bbdrop", func(fr *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, _ *geometry.RiderFit) {
			fr.ChainstayMm = fr.BbDropMm
		}},
		{"NaN target point", func(_ *geometry.FrameSize, _ *geometry.BuildSpec, _ *geometry.CockpitSpec, _ *geometry.SeatSpec, fi *geometry.RiderFit) {
			fi.Target.Point.X = math.NaN()
		}},
	}

	for _, tc := range badCases {
		t.Run(tc.name, func(t *testing.T) {
			fr, b, c, s, fi := frame, build, cockpit, seat, fit
			tc.mutate(&fr, &b, &c, &s, &fi)
			if _, err := geometry.BuildScene(fr, b, c, s, fi); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}

	// Explicit 0.0 values must be honored cleanly (never replaced by silent fallbacks).
	cockpitZero := cockpit
	cockpitZero.HeadsetTopStackMm = 0.0
	cockpitZero.SpacerMm = 0.0
	cockpitZero.Handlebar.StatedRiseMm = 0.0
	cockpitZero.Handlebar.BacksweepDeg = 0.0
	cockpitZero.Handlebar.UpsweepDeg = 0.0
	seatZero := seat
	seatZero.PostSetbackMm = 0.0
	fitZero := fit
	fitZero.Target.Ref = geometry.TargetHandGrip
	scZero, err := geometry.BuildScene(frame, build, cockpitZero, seatZero, fitZero)
	if err != nil {
		t.Fatalf("explicit zero inputs failed: %v", err)
	}
	if scZero.HandGrip.DistanceTo(scZero.StemClamp) > 1e-9 {
		t.Errorf("zero-rise/zero-sweep bar should put HandGrip == StemClamp, got dist %.9f", scZero.HandGrip.DistanceTo(scZero.StemClamp))
	}
}

func TestSolveStemSpacerFit(t *testing.T) {
	// Seven Custom cockpit reference setup:
	// HT = (424, 631), HTA = 70°, HeadsetTopStackMm = 11, StemClampHeight = 40.
	persuader := catalog.DefaultHandlebar()
	saddle := catalog.DefaultSaddle()
	frame := geometry.FrameSize{
		ReachMm:     424,
		StackMm:     631,
		HtaDeg:      70,
		StaDeg:      74,
		HtlMm:       120,
		BbDropMm:    70,
		ChainstayMm: 450,
		CadFork:     geometry.Fork{StaticA2cMm: 492, RakeMm: 51},
		CadSagPct:   0,
		WheelbaseMm: model.Some(1140.0),
	}
	build := geometry.BuildSpec{
		Fork:               frame.CadFork,
		SagPct:             0,
		WheelRadiusFrontMm: 372,
		WheelRadiusRearMm:  372,
	}
	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: 11,
		SpacerMm:          0,
		Stem:              model.StemSpec{LengthMm: 70, AngleDeg: -6},
		StemClampHeightMm: 40,
		Handlebar:         persuader,
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := geometry.SeatSpec{
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        saddle,
	}

	// 1. TargetStemClamp @ (480.9, 676.1) with -6° stem -> ~69mm stem, ~-1mm spacers.
	fitClamp := geometry.RiderFit{
		FrameSeatAxisHeightMm: 746,
		BrpHeightMm:           746,
		TargetSitBonesXMm:     -292,
		Target:                geometry.RiderTarget{Point: geom2d.Point2{X: 480.9, Y: 676.1}, Ref: geometry.TargetStemClamp},
		ReferenceSaddle:       saddle,
	}
	scClamp, err := geometry.BuildScene(frame, build, cockpit, seat, fitClamp)
	if err != nil {
		t.Fatalf("BuildScene(TargetStemClamp): %v", err)
	}
	sClamp, ok := scClamp.SolveStemSpacerFit(-6)
	if !ok {
		t.Fatal("SolveStemSpacerFit(-6) returned false")
	}
	if math.Abs(sClamp.StemLengthMm-69.2) > 0.5 || math.Abs(sClamp.SpacerMm-(-1.3)) > 0.5 {
		t.Errorf("TargetStemClamp -6° = (L=%.2f, sp=%.2f), want ~(69.2, -1.3)", sClamp.StemLengthMm, sClamp.SpacerMm)
	}

	// 2. TargetHandGrip @ (480.9, 726.1) with Persuader:
	//    -6° -> ~140mm stem, ~-14mm spacers
	//   -17° -> ~145mm stem, ~+14mm spacers
	fitGrip := fitClamp
	fitGrip.Target.Point = geom2d.Point2{X: 480.9, Y: 726.1}
	fitGrip.Target.Ref = geometry.TargetHandGrip
	scGrip, err := geometry.BuildScene(frame, build, cockpit, seat, fitGrip)
	if err != nil {
		t.Fatalf("BuildScene(TargetHandGrip): %v", err)
	}
	sGrip6, ok := scGrip.SolveStemSpacerFit(-6)
	if !ok {
		t.Fatal("SolveStemSpacerFit(-6) returned false")
	}
	if math.Abs(sGrip6.StemLengthMm-140.0) > 1.0 || math.Abs(sGrip6.SpacerMm-(-14.0)) > 1.0 {
		t.Errorf("TargetHandGrip -6° = (L=%.2f, sp=%.2f), want ~(140, -14)", sGrip6.StemLengthMm, sGrip6.SpacerMm)
	}
	sGrip17, ok := scGrip.SolveStemSpacerFit(-17)
	if !ok {
		t.Fatal("SolveStemSpacerFit(-17) returned false")
	}
	if math.Abs(sGrip17.StemLengthMm-145.0) > 1.0 || math.Abs(sGrip17.SpacerMm-14.0) > 1.0 {
		t.Errorf("TargetHandGrip -17° = (L=%.2f, sp=%.2f), want ~(145, +14)", sGrip17.StemLengthMm, sGrip17.SpacerMm)
	}

	// 3. Round-trip: vector identity for all solved angles, plus WithCockpit for buildable +spacer solve (-17°).
	uSteer := geometry.SteererTubeDir(scGrip.HtaDeg)
	nSteer := uSteer.Perpendicular(geom2d.Clockwise)
	for _, sol := range []geometry.StemSpacerFit{sGrip6, sGrip17} {
		tSteer := cockpit.HeadsetTopStackMm + sol.SpacerMm + cockpit.StemClampHeightMm/2.0
		uStem := nSteer.Rotate(sol.StemAngleDeg.Angle())
		reconstructed := scGrip.HeadTubeTop.Add(uSteer.Scale(tSteer)).Add(uStem.Scale(sol.StemLengthMm))
		if d := reconstructed.DistanceTo(scGrip.RequiredStemClamp); d > 1e-9 {
			t.Errorf("2x2 reconstruction error at %.0f° = %.12f mm, want < 1e-9", sol.StemAngleDeg, d)
		}
	}
	c17 := cockpit
	c17.Stem = model.StemSpec{LengthMm: sGrip17.StemLengthMm, AngleDeg: sGrip17.StemAngleDeg}
	c17.SpacerMm = sGrip17.SpacerMm
	roundTrip, err := scGrip.WithCockpit(c17)
	if err != nil {
		t.Fatalf("WithCockpit(-17°): %v", err)
	}
	if d := roundTrip.TargetPointError().Norm(); d > 1e-9 {
		t.Errorf("round-trip error at -17° = %.12f mm, want < 1e-9", d)
	}
}

func TestSeatTubeTopAndBend(t *testing.T) {
	saddle := catalog.DefaultSaddle()
	frame := geometry.FrameSize{
		ReachMm:          428,
		StackMm:          642,
		HtaDeg:           68.25,
		StaDeg:           75.25,
		HtlMm:            140,
		BbDropMm:         72,
		ChainstayMm:      450,
		SeatTubeLengthMm: model.Some(490.0),
		CadFork:          geometry.Fork{StaticA2cMm: 495, RakeMm: 54},
	}
	build := geometry.BuildSpec{
		Fork:               frame.CadFork,
		WheelRadiusFrontMm: 372,
		WheelRadiusRearMm:  372,
	}
	cockpit := geometry.CockpitSpec{
		HeadsetTopStackMm: 11,
		SpacerMm:          20,
		Stem:              model.StemSpec{LengthMm: 60, AngleDeg: -6},
		StemClampHeightMm: 40,
		Handlebar:         catalog.DefaultHandlebar(),
		GripInsetMm:       model.DefaultGripInsetMm,
	}
	seat := geometry.SeatSpec{
		PostSetbackMm: model.DefaultSeatpostSetbackMm,
		ClampLengthMm: model.DefaultSeatpostClampLengthMm,
		Saddle:        saddle,
	}
	fit := geometry.RiderFit{
		FrameSeatAxisHeightMm: 746,
		BrpHeightMm:           746,
		TargetSitBonesXMm:     -292,
		Target:                geometry.RiderTarget{Point: geom2d.Point2{X: 480, Y: 700}, Ref: geometry.TargetHandGrip},
		ReferenceSaddle:       saddle,
	}

	// 1. Straight seat tube terminates at exact published SeatTubeLengthMm below SeatpostClamp.
	scStraight, err := geometry.BuildScene(frame, build, cockpit, seat, fit)
	if err != nil {
		t.Fatalf("BuildScene(straight): %v", err)
	}
	if _, hasBend := scStraight.SeatTubeBend(); hasBend {
		t.Error("straight seat tube should not report SeatTubeBend")
	}
	stTop := scStraight.SeatTubeTop()
	if got := stTop.DistanceTo(scStraight.BB); math.Abs(got-490.0) > 1e-6 {
		t.Errorf("SeatTubeTop distance from BB = %.4f, want 490.0", got)
	}
	if stTop.Y >= scStraight.SeatpostClamp.Y-150 {
		t.Errorf("SeatTubeTop.Y = %.1f should be well below SeatpostClamp.Y = %.1f", stTop.Y, scStraight.SeatpostClamp.Y)
	}

	// 2. Bent seat tube reports a bend point between BB and SeatTubeTop.
	bent := frame
	bent.StaDeg = 75.0
	bent.ActualStaDeg = 73.3
	bent.SeatTubeLengthMm = model.Some(455.0)
	scBent, err := geometry.BuildScene(bent, build, cockpit, seat, fit)
	if err != nil {
		t.Fatalf("BuildScene(bent): %v", err)
	}
	bendPt, hasBend := scBent.SeatTubeBend()
	if !hasBend {
		t.Fatal("bent seat tube should report SeatTubeBend")
	}
	bentTop := scBent.SeatTubeTop()
	if !(bendPt.Y > 0 && bendPt.Y < bentTop.Y && bentTop.Y < scBent.SeatpostClamp.Y) {
		t.Errorf("expected 0 < bendPt.Y (%.1f) < bentTop.Y (%.1f) < SeatpostClamp.Y (%.1f)", bendPt.Y, bentTop.Y, scBent.SeatpostClamp.Y)
	}

	// 3. Unset SeatTubeLengthMm still places SeatTubeTop well below SeatpostClamp.
	noLen := frame
	noLen.SeatTubeLengthMm = model.Opt[float64]{}
	scNoLen, err := geometry.BuildScene(noLen, build, cockpit, seat, fit)
	if err != nil {
		t.Fatalf("BuildScene(noLen): %v", err)
	}
	if fallbackTop := scNoLen.SeatTubeTop(); fallbackTop.Y >= scNoLen.SeatpostClamp.Y-20 {
		t.Errorf("fallback SeatTubeTop.Y = %.1f should stay below SeatpostClamp.Y = %.1f", fallbackTop.Y, scNoLen.SeatpostClamp.Y)
	}
}

func evaluatedCockpitPoint(s geometry.BikeScene) geom2d.Point2 {
	switch s.Fit.Target.Ref {
	case geometry.TargetHandGrip:
		return s.HandGrip
	case geometry.TargetStemClamp:
		return s.StemClamp
	case geometry.TargetReferenceBarClamp:
		if refBar, ok := s.Fit.ReferenceBar.Get(); ok {
			refOff := geometry.GetHandlebarOffset(refBar, s.Cockpit.GripInsetMm, 0.0)
			return s.HandGrip.SubVec(geom2d.Vec2{X: refOff.X, Y: refOff.Y})
		}
		return s.StemClamp
	default:
		return s.HandGrip
	}
}
