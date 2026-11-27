package assembly_test

import (
	"fmt"
	"math"
	"sort"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

type testForkMounter interface {
	CheckHeadset(bore assembly.LowerBore, cup assembly.LowerCupSetup) error
	AxleInFork(state assembly.LoadState) pose.Point
}

func frameDims(f assembly.FrameSetup) assembly.FrameDimensions {
	return assembly.FrameDimensions{
		ReachMm:             f.ReachMm,
		StackMm:             f.StackMm,
		HeadTubeAngleDeg:    f.HeadTubeAngleDeg,
		HeadTubeLengthMm:    f.HeadTubeLengthMm,
		SeatTubeAngleDeg:    f.SeatTubeAngleDeg,
		SeatTubeLengthMm:    f.SeatTubeLengthMm,
		BbDropMm:            f.BbDropMm,
		ChainstayMm:         f.ChainstayMm,
		LowerBore:           f.LowerBore,
		DefaultLowerCup:     f.DefaultLowerCup,
		MaxPostInsertionMm:  f.MaxPostInsertionMm,
		MinFrameInsertionMm: f.MinFrameInsertionMm,
	}
}

func isFiniteVal(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func validateTestPoints(component string, pts ...pose.Point) error {
	for _, p := range pts {
		if !isFiniteVal(p.X()) || !isFiniteVal(p.Y()) {
			return fmt.Errorf("%s placement overflow", component)
		}
	}
	return nil
}

func validateTestPose(name string, p pose.Pose) error {
	if !isFiniteVal(p.Origin().X()) || !isFiniteVal(p.Origin().Y()) || !isFiniteVal(float64(p.HeadingDeg())) {
		return fmt.Errorf("%s pose must be finite", name)
	}
	return nil
}

func validateForkSetup(f assembly.ForkSetup) error {
	if !isFiniteVal(f.Spec.StaticA2cMm) || f.Spec.StaticA2cMm <= 0 {
		return assembly.NewInputError(assembly.InputForkA2c, "fork static A2C must be finite and positive, got %v mm", f.Spec.StaticA2cMm)
	}
	if !f.Spec.Travel.Valid() {
		return fmt.Errorf("unsupported fork travel %d mm", f.Spec.Travel)
	}
	if f.Spec.Steerer == assembly.SteererBasis && f.Spec.Travel != assembly.TravelRigid {
		return fmt.Errorf("basis fork cannot have travel, got %d mm", f.Spec.Travel)
	}
	if !isFiniteVal(f.SagPct) || f.SagPct < 0 || f.SagPct > 100 {
		return assembly.NewInputError(assembly.InputForkSag, "fork sag must be in [0%%, 100%%], got %v%%", f.SagPct)
	}
	if f.Spec.Travel == assembly.TravelRigid && f.SagPct != 0 {
		return assembly.NewInputError(assembly.InputForkSag, "rigid fork (0 mm travel) cannot have non-zero sag, got %v%%", f.SagPct)
	}
	eff := f.EffectiveA2cMm()
	if !isFiniteVal(eff) || eff <= 0 {
		return assembly.NewInputError(assembly.InputForkA2c, "fork effective A2C must be finite and positive, got %v mm", eff)
	}
	if !isFiniteVal(f.Spec.RakeMm) {
		return fmt.Errorf("fork rake must be finite, got %v mm", f.Spec.RakeMm)
	}
	return nil
}

func forkMounterFromSetup(f assembly.ForkSetup) (testForkMounter, error) {
	if err := validateForkSetup(f); err != nil {
		return nil, err
	}
	switch f.Spec.Steerer {
	case assembly.SteererBasis:
		return assembly.NewBasisFork(f.Spec.StaticA2cMm, f.Spec.RakeMm)
	case assembly.SteererStraight118:
		if f.Spec.Travel.IsSuspension() {
			return assembly.NewSuspensionFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, f.Spec.Travel, assembly.PhysicalSteererStraight118, f.SagPct)
		}
		return assembly.NewRigidFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, assembly.PhysicalSteererStraight118)
	case assembly.SteererTapered:
		if f.Spec.Travel.IsSuspension() {
			return assembly.NewSuspensionFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, f.Spec.Travel, assembly.PhysicalSteererTapered, f.SagPct)
		}
		return assembly.NewRigidFork(f.Spec.StaticA2cMm, f.Spec.RakeMm, assembly.PhysicalSteererTapered)
	default:
		return nil, assembly.NewInputError(assembly.InputLowerCup, "unknown or missing fork steerer %q", f.Spec.Steerer)
	}
}

func placeUnpitchedFrameset(frame assembly.FrameSetup, lowerCup assembly.LowerCupSetup, fork assembly.ForkSetup, wheels assembly.WheelSetup) (assembly.FramesetPlacement, error) {
	f, err := assembly.NewFrame(frameDims(frame))
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	fm, err := forkMounterFromSetup(fork)
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	fs, err := assembly.MateFrameset(f, lowerCup, assembly.DefaultUpperStackMm, fm, wheels)
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	return fs.Unpitched(), nil
}

func placeFrameset(frame assembly.FrameSetup, lowerCup assembly.LowerCupSetup, fork assembly.ForkSetup, wheels assembly.WheelSetup) (assembly.FramesetPlacement, error) {
	f, err := assembly.NewFrame(frameDims(frame))
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	fm, err := forkMounterFromSetup(fork)
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	fs, err := assembly.MateFrameset(f, lowerCup, assembly.DefaultUpperStackMm, fm, wheels)
	if err != nil {
		return assembly.FramesetPlacement{}, err
	}
	return fs.Place(assembly.Dynamic), nil
}

type insertionBounds struct {
	EffectiveMinInsertionMm, MaxAllowedInsertionMm float64
}

func seatTubeSocketFromSetup(frame assembly.FrameSetup) (assembly.SeatTubeSocket, error) {
	if !isFiniteVal(frame.SeatTubeLengthMm) || frame.SeatTubeLengthMm <= 0 {
		return assembly.SeatTubeSocket{}, fmt.Errorf("seat tube length must be finite and positive, got %v mm", frame.SeatTubeLengthMm)
	}
	if !isFiniteVal(frame.MaxPostInsertionMm) || frame.MaxPostInsertionMm < 0 ||
		!isFiniteVal(frame.MinFrameInsertionMm) || frame.MinFrameInsertionMm < 0 {
		return assembly.SeatTubeSocket{}, fmt.Errorf("frame insertion limits must be finite and non-negative")
	}
	frameCap := frame.MaxPostInsertionMm
	maxSource := "frame max insertion"
	if frameCap == 0 {
		frameCap = frame.SeatTubeLengthMm - 20.0
		maxSource = "frame seat tube clearance"
	}
	allowed, err := assembly.NewBoundedRange(frame.MinFrameInsertionMm, frameCap, "frame min insertion", maxSource)
	if err != nil {
		return assembly.SeatTubeSocket{}, fmt.Errorf("frame allows no seatpost insertion: %w", err)
	}
	return assembly.SeatTubeSocket{
		CollarInFrame:    pose.NewTransform(assembly.SeatTubeCollar, assembly.FrameLocal, pose.Origin(assembly.FrameLocal), 0),
		AllowedInsertion: allowed,
	}, nil
}

func seatpostInsertionBounds(frame assembly.FrameSetup, post assembly.SeatpostSpec) (insertionBounds, error) {
	socket, err := seatTubeSocketFromSetup(frame)
	if err != nil {
		return insertionBounds{}, err
	}
	rigid, err := assembly.NewRigidPost(post.TotalLengthMm, post.MinInsertionMm, post.MaxInsertionMm, post.SetbackMm, post.RailClampLengthMm)
	if err != nil {
		return insertionBounds{}, err
	}
	dof, err := assembly.MateSeatpost(socket, rigid)
	if err != nil {
		return insertionBounds{}, err
	}
	allowed := dof.AllowedInsertion()
	return insertionBounds{EffectiveMinInsertionMm: allowed.Min(), MaxAllowedInsertionMm: allowed.Max()}, nil
}

func railOffsetLimitMm(setup assembly.BikeSetup) (float64, error) {
	saddle, err := assembly.NewSaddle(assembly.SaddleDimensions{
		RailCenterToBrpMm:      setup.Saddle.RailCenterToBrpMm,
		RailCenterToSitBonesMm: setup.Saddle.RailCenterToSitBonesMm,
		RailToBrpHeightMm:      setup.Saddle.RailToBrpHeightMm,
		SitBonesElevationMm:    setup.Saddle.SitBonesElevationMm,
		RailLengthMm:           setup.Saddle.RailLengthMm,
	})
	if err != nil {
		return 0, err
	}
	clamp := assembly.RailClampPort{
		CradleInParent: pose.NewTransform(assembly.SeatpostCradle, assembly.SeatpostLocal, pose.Origin(assembly.SeatpostLocal), 0),
		ClampLengthMm:  setup.Seatpost.Spec.RailClampLengthMm,
	}
	dof, err := assembly.MateSaddle(clamp, saddle)
	if err != nil {
		return 0, err
	}
	return dof.AllowedRailOffset().Max(), nil
}

func placeSeatpost(seatTubeTop pose.Pose, frame assembly.FrameSetup, setup assembly.SeatpostSetup) (assembly.SeatpostPlacement, error) {
	if err := validateTestPose("seatTubeTop", seatTubeTop); err != nil {
		return assembly.SeatpostPlacement{}, err
	}
	socket, err := seatTubeSocketFromSetup(frame)
	if err != nil {
		return assembly.SeatpostPlacement{}, err
	}
	rigid, err := assembly.NewRigidPost(setup.Spec.TotalLengthMm, setup.Spec.MinInsertionMm, setup.Spec.MaxInsertionMm, setup.Spec.SetbackMm, setup.Spec.RailClampLengthMm)
	if err != nil {
		return assembly.SeatpostPlacement{}, err
	}
	dof, err := assembly.MateSeatpost(socket, rigid)
	if err != nil {
		return assembly.SeatpostPlacement{}, err
	}
	clamped, err := dof.Clamp(setup.InsertionMm)
	if err != nil {
		return assembly.SeatpostPlacement{}, err
	}
	placed := clamped.PlaceInWorld(seatTubeTop.Anchor(assembly.FrameLocal))
	if err := validateTestPoints("seatpost", seatTubeTop.Origin(), placed.ShaftTop.Origin(), placed.RailPivot.Origin()); err != nil {
		return assembly.SeatpostPlacement{}, err
	}
	return placed, nil
}

func placeSaddle(railPivot pose.Pose, clampLengthMm float64, setup assembly.SaddleSetup) (assembly.SaddlePlacement, error) {
	railTurn := assembly.RailTurnForWorldPitch(railPivot.Heading(), setup.RailPitchDeg)
	if err := validateTestPose("RailPivot", railPivot); err != nil {
		return assembly.SaddlePlacement{}, err
	}
	saddle, err := assembly.NewSaddle(assembly.SaddleDimensions{
		RailCenterToBrpMm:      setup.RailCenterToBrpMm,
		RailCenterToSitBonesMm: setup.RailCenterToSitBonesMm,
		RailToBrpHeightMm:      setup.RailToBrpHeightMm,
		SitBonesElevationMm:    setup.SitBonesElevationMm,
		RailLengthMm:           setup.RailLengthMm,
	})
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	cradleInWorld := railPivot.Anchor(assembly.SeatpostCradle)
	dof, err := assembly.MateSaddle(assembly.RailClampPort{
		CradleInParent: cradleInWorld,
		ClampLengthMm:  clampLengthMm,
	}, saddle)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	clamped, err := dof.Clamp(setup.RailOffsetMm, railTurn)
	if err != nil {
		return assembly.SaddlePlacement{}, err
	}
	placed := clamped.Place(cradleInWorld)
	if err := validateTestPoints("saddle", railPivot.Origin(), placed.RailCenter.Origin(), placed.BRP.Origin(), placed.SitBones.Origin()); err != nil {
		return assembly.SaddlePlacement{}, err
	}
	return placed, nil
}

func placeStem(steererInWorld pose.Transform, setup assembly.StemSetup) (assembly.StemPlacement, error) {
	headTubeTop := steererInWorld.ApplyPose(pose.OriginPose(assembly.SteererLocal))
	if err := validateTestPose("steererUp", headTubeTop); err != nil {
		return assembly.StemPlacement{}, err
	}
	port, err := assembly.NewSteererPort(
		pose.NewTransform(assembly.SteererLocal, assembly.FrameLocal, pose.Origin(assembly.FrameLocal), 0),
		setup.UpperStackMm,
		setup.MaxSpacersMm,
	)
	if err != nil {
		return assembly.StemPlacement{}, err
	}
	stem, err := assembly.NewStem(setup.ClampHeightMm, setup.LengthMm, setup.AngleDeg)
	if err != nil {
		return assembly.StemPlacement{}, err
	}
	clamped, err := assembly.MountStem(port, setup.SpacersMm, stem)
	if err != nil {
		return assembly.StemPlacement{}, err
	}
	placed := clamped.Place(headTubeTop.Anchor(assembly.FrameLocal))
	if err := validateTestPoints("stem", headTubeTop.Origin(), placed.SteererClamp.Origin(), placed.BarClamp.Origin()); err != nil {
		return assembly.StemPlacement{}, err
	}
	return placed, nil
}

func placeHandlebar(barClamp pose.Pose, setup assembly.HandlebarSetup) (assembly.HandlebarPlacement, error) {
	rollTurn := assembly.BarTurnForWorldRoll(barClamp.Heading(), setup.BarRollDeg)
	if err := validateTestPose("barClamp", barClamp); err != nil {
		return assembly.HandlebarPlacement{}, err
	}
	bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
		StockWidthMm:   setup.StockWidthMm,
		CurrentWidthMm: setup.CurrentWidthMm,
		BacksweepDeg:   setup.BacksweepDeg,
		UpsweepDeg:     setup.UpsweepDeg,
		StatedRiseMm:   setup.StatedRiseMm,
		RiseMode:       setup.RiseMode,
		BendStartMm:    setup.BendStartMm,
		GripInsetMm:    setup.GripInsetMm,
	})
	if err != nil {
		return assembly.HandlebarPlacement{}, err
	}
	barClampInWorld := barClamp.Anchor(assembly.StemLocal)
	clamped, err := assembly.MountHandlebar(assembly.BarClampPort{ClampInParent: barClampInWorld}, bar, rollTurn)
	if err != nil {
		return assembly.HandlebarPlacement{}, err
	}
	placed := clamped.Place(barClampInWorld)
	if err := validateTestPoints("handlebar", barClamp.Origin(), placed.BarCenter.Origin(), placed.HandGrip.Origin()); err != nil {
		return assembly.HandlebarPlacement{}, err
	}
	return placed, nil
}

func setupToParts(s assembly.BikeSetup) (*assembly.Frameset, assembly.RigidPost, assembly.Saddle, assembly.Stem, assembly.Handlebar, error) {
	frame, err := assembly.NewFrame(frameDims(s.Frame))
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	fm, err := forkMounterFromSetup(s.Fork)
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	fs, err := assembly.MateFrameset(frame, s.LowerCup, s.Stem.UpperStackMm, fm, s.Wheels)
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	if s.Stem.MaxSpacersMm != assembly.DefaultMaxSpacersMm {
		fs, err = fs.WithMaxSpacers(s.Stem.MaxSpacersMm)
		if err != nil {
			return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
		}
	}
	post, err := assembly.NewRigidPost(s.Seatpost.Spec.TotalLengthMm, s.Seatpost.Spec.MinInsertionMm, s.Seatpost.Spec.MaxInsertionMm, s.Seatpost.Spec.SetbackMm, s.Seatpost.Spec.RailClampLengthMm)
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	saddle, err := assembly.NewSaddle(assembly.SaddleDimensions{
		RailCenterToBrpMm:      s.Saddle.RailCenterToBrpMm,
		RailCenterToSitBonesMm: s.Saddle.RailCenterToSitBonesMm,
		RailToBrpHeightMm:      s.Saddle.RailToBrpHeightMm,
		SitBonesElevationMm:    s.Saddle.SitBonesElevationMm,
		RailLengthMm:           s.Saddle.RailLengthMm,
	})
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	stem, err := assembly.NewStem(s.Stem.ClampHeightMm, s.Stem.LengthMm, s.Stem.AngleDeg)
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
		StockWidthMm:   s.Handlebar.StockWidthMm,
		CurrentWidthMm: s.Handlebar.CurrentWidthMm,
		BacksweepDeg:   s.Handlebar.BacksweepDeg,
		UpsweepDeg:     s.Handlebar.UpsweepDeg,
		StatedRiseMm:   s.Handlebar.StatedRiseMm,
		RiseMode:       s.Handlebar.RiseMode,
		BendStartMm:    s.Handlebar.BendStartMm,
		GripInsetMm:    s.Handlebar.GripInsetMm,
	})
	if err != nil {
		return nil, assembly.RigidPost{}, assembly.Saddle{}, assembly.Stem{}, assembly.Handlebar{}, err
	}
	return fs, post, saddle, stem, bar, nil
}

func placeBike(setup assembly.BikeSetup) (assembly.Placement, error) {
	fs, post, saddle, stem, bar, err := setupToParts(setup)
	if err != nil {
		return assembly.Placement{}, err
	}
	rear, err := assembly.OpenRearCockpit(fs, post, saddle, setup.Saddle.RailPitchDeg)
	if err != nil {
		return assembly.Placement{}, err
	}
	seated, err := rear.Clamp(setup.Seatpost.InsertionMm, setup.Saddle.RailOffsetMm)
	if err != nil {
		return assembly.Placement{}, err
	}
	build, err := seated.AttachFront(setup.Stem.SpacersMm, stem, bar, setup.Handlebar.BarRollDeg)
	if err != nil {
		return assembly.Placement{}, err
	}
	placed := build.Place(assembly.Dynamic)
	placed.Setup.Stem.MinLengthMm = setup.Stem.MinLengthMm
	placed.Setup.Stem.MaxLengthMm = setup.Stem.MaxLengthMm
	return placed, nil
}

func applyFitting(setup assembly.BikeSetup, target assembly.FitTarget) (assembly.Fitting, error) {
	if target.BRP == nil && target.HandGrip == nil {
		return assembly.Fitting{}, fmt.Errorf("fit target has no landmarks")
	}
	if target.BRP != nil && (!isFiniteVal(target.BRP.X()) || !isFiniteVal(target.BRP.Y())) {
		return assembly.Fitting{}, assembly.NewInputError(assembly.InputFitTargetBRP, "BRP fit target must be finite")
	}
	if target.HandGrip != nil && (!isFiniteVal(target.HandGrip.X()) || !isFiniteVal(target.HandGrip.Y())) {
		return assembly.Fitting{}, assembly.NewInputError(assembly.InputFitTargetGrip, "HandGrip fit target must be finite")
	}
	frame, err := assembly.NewFrame(frameDims(setup.Frame))
	if err != nil {
		return assembly.Fitting{}, err
	}
	fm, err := forkMounterFromSetup(setup.Fork)
	if err != nil {
		return assembly.Fitting{}, err
	}
	fs, err := assembly.MateFrameset(frame, setup.LowerCup, setup.Stem.UpperStackMm, fm, setup.Wheels)
	if err != nil {
		return assembly.Fitting{}, err
	}
	if setup.Stem.MaxSpacersMm != assembly.DefaultMaxSpacersMm {
		fs, err = fs.WithMaxSpacers(setup.Stem.MaxSpacersMm)
		if err != nil {
			return assembly.Fitting{}, err
		}
	}
	post, err := assembly.NewRigidPost(setup.Seatpost.Spec.TotalLengthMm, setup.Seatpost.Spec.MinInsertionMm, setup.Seatpost.Spec.MaxInsertionMm, setup.Seatpost.Spec.SetbackMm, setup.Seatpost.Spec.RailClampLengthMm)
	if err != nil {
		return assembly.Fitting{}, err
	}
	saddle, err := assembly.NewSaddle(assembly.SaddleDimensions{
		RailCenterToBrpMm:      setup.Saddle.RailCenterToBrpMm,
		RailCenterToSitBonesMm: setup.Saddle.RailCenterToSitBonesMm,
		RailToBrpHeightMm:      setup.Saddle.RailToBrpHeightMm,
		SitBonesElevationMm:    setup.Saddle.SitBonesElevationMm,
		RailLengthMm:           setup.Saddle.RailLengthMm,
	})
	if err != nil {
		return assembly.Fitting{}, err
	}
	bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
		StockWidthMm:   setup.Handlebar.StockWidthMm,
		CurrentWidthMm: setup.Handlebar.CurrentWidthMm,
		BacksweepDeg:   setup.Handlebar.BacksweepDeg,
		UpsweepDeg:     setup.Handlebar.UpsweepDeg,
		StatedRiseMm:   setup.Handlebar.StatedRiseMm,
		RiseMode:       setup.Handlebar.RiseMode,
		BendStartMm:    setup.Handlebar.BendStartMm,
		GripInsetMm:    setup.Handlebar.GripInsetMm,
	})
	if err != nil {
		return assembly.Fitting{}, err
	}
	rear, err := assembly.OpenRearCockpit(fs, post, saddle, setup.Saddle.RailPitchDeg)
	if err != nil {
		return assembly.Fitting{}, err
	}
	var seated *assembly.SeatedFrameset
	var limitedBy []assembly.InputField
	if target.BRP != nil {
		var rl []assembly.InputField
		seated, rl, err = rear.FitToBRP(*target.BRP)
		if err != nil {
			return assembly.Fitting{}, err
		}
		limitedBy = append(limitedBy, rl...)
	} else {
		seated, err = rear.Clamp(setup.Seatpost.InsertionMm, setup.Saddle.RailOffsetMm)
		if err != nil {
			return assembly.Fitting{}, err
		}
	}
	var build *assembly.Build
	if target.HandGrip != nil {
		if !isFiniteVal(setup.Stem.MinLengthMm) || !isFiniteVal(setup.Stem.MaxLengthMm) || setup.Stem.MinLengthMm <= 0 || setup.Stem.MinLengthMm > setup.Stem.MaxLengthMm {
			return assembly.Fitting{}, fmt.Errorf("invalid stem length range")
		}
		lenRange, err := assembly.NewBoundedRange(setup.Stem.MinLengthMm, setup.Stem.MaxLengthMm, "min stem length", "max stem length")
		if err != nil {
			return assembly.Fitting{}, err
		}
		front, err := assembly.OpenFrontCockpit(seated, setup.Stem.AngleDeg, setup.Stem.ClampHeightMm, lenRange, bar, setup.Handlebar.BarRollDeg)
		if err != nil {
			return assembly.Fitting{}, err
		}
		var fl []assembly.InputField
		build, fl, err = front.FitToGrip(*target.HandGrip)
		if err != nil {
			return assembly.Fitting{}, err
		}
		limitedBy = append(limitedBy, fl...)
	} else {
		stem, err := assembly.NewStem(setup.Stem.ClampHeightMm, setup.Stem.LengthMm, setup.Stem.AngleDeg)
		if err != nil {
			return assembly.Fitting{}, err
		}
		build, err = seated.AttachFront(setup.Stem.SpacersMm, stem, bar, setup.Handlebar.BarRollDeg)
		if err != nil {
			return assembly.Fitting{}, err
		}
	}
	placement := build.Place(assembly.Dynamic)
	placement.Setup.Stem.MinLengthMm = setup.Stem.MinLengthMm
	placement.Setup.Stem.MaxLengthMm = setup.Stem.MaxLengthMm
	fit := assembly.Fitting{Placement: placement, Outcome: assembly.FitReached}
	if target.BRP != nil {
		p := *target.BRP
		fit.Target.BRP = &p
	}
	if target.HandGrip != nil {
		p := *target.HandGrip
		fit.Target.HandGrip = &p
	}
	if len(limitedBy) > 0 {
		fit.Outcome = assembly.FitLimited
		fit.LimitedBy = limitedBy
	}
	return fit, nil
}

func fitPostSetback(setup assembly.BikeSetup, brp pose.Point, railOffsetMm float64) (assembly.Fitting, error) {
	if !isFiniteVal(brp.X()) || !isFiniteVal(brp.Y()) {
		return assembly.Fitting{}, assembly.NewInputError(assembly.InputFitTargetBRP, "BRP fit target must be finite")
	}
	probe := setup
	probe.Seatpost.Spec.SetbackMm = 0
	bounds, err := seatpostInsertionBounds(probe.Frame, probe.Seatpost.Spec)
	if err != nil {
		return assembly.Fitting{}, err
	}
	probe.Seatpost.InsertionMm = bounds.MaxAllowedInsertionMm
	probe.Saddle.RailOffsetMm = railOffsetMm
	fs, post, saddle, stem, bar, err := setupToParts(probe)
	if err != nil {
		return assembly.Fitting{}, err
	}
	rear, err := assembly.OpenRearCockpit(fs, post, saddle, probe.Saddle.RailPitchDeg)
	if err != nil {
		return assembly.Fitting{}, err
	}
	seated, err := rear.Clamp(probe.Seatpost.InsertionMm, probe.Saddle.RailOffsetMm)
	if err != nil {
		return assembly.Fitting{}, err
	}
	build, err := seated.AttachFront(probe.Stem.SpacersMm, stem, bar, probe.Handlebar.BarRollDeg)
	if err != nil {
		return assembly.Fitting{}, err
	}
	fit, err := build.FitPostSetback(brp, railOffsetMm)
	if err != nil {
		return assembly.Fitting{}, err
	}
	fit.Placement.Setup.Stem.MinLengthMm = setup.Stem.MinLengthMm
	fit.Placement.Setup.Stem.MaxLengthMm = setup.Stem.MaxLengthMm
	return fit, nil
}

type testForkOption struct {
	Fork     assembly.ForkSetup
	LowerCup assembly.LowerCupSetup
}

type testPartOptions struct {
	Stems        []assembly.StemOption
	SpacerStepMm float64
	Handlebars   []assembly.HandlebarSetup
	Forks        []testForkOption
}

func (p testPartOptions) Validate() error {
	if !isFiniteVal(p.SpacerStepMm) || p.SpacerStepMm <= 0 {
		return fmt.Errorf("spacer step must be positive and finite")
	}
	if len(p.Stems) == 0 {
		return fmt.Errorf("build fitting needs at least one stem option")
	}
	type stemKey struct {
		length float64
		angle  pose.Deg
	}
	seen := make(map[stemKey]bool, len(p.Stems))
	for i, stem := range p.Stems {
		if err := stem.Validate(); err != nil {
			return fmt.Errorf("stem option %d: %w", i, err)
		}
		key := stemKey{stem.LengthMm, stem.AngleDeg}
		if seen[key] {
			return fmt.Errorf("stem option %d duplicates %v mm at %v°", i, stem.LengthMm, stem.AngleDeg)
		}
		seen[key] = true
	}
	for i, bar := range p.Handlebars {
		if bar.BarRollDeg != 0 {
			return fmt.Errorf("handlebar option %d must not set BarRollDeg", i)
		}
		if _, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
			StockWidthMm: bar.StockWidthMm, CurrentWidthMm: bar.CurrentWidthMm,
			BacksweepDeg: bar.BacksweepDeg, UpsweepDeg: bar.UpsweepDeg,
			StatedRiseMm: bar.StatedRiseMm, RiseMode: bar.RiseMode,
			BendStartMm: bar.BendStartMm, GripInsetMm: bar.GripInsetMm,
		}); err != nil {
			return fmt.Errorf("handlebar option %d: %w", i, err)
		}
	}
	for i, f := range p.Forks {
		if err := validateForkSetup(f.Fork); err != nil {
			return fmt.Errorf("fork option %d: %w", i, err)
		}
		if err := f.LowerCup.Validate(); err != nil {
			return fmt.Errorf("fork option %d lower cup: %w", i, err)
		}
	}
	return nil
}

func fitBuilds(setup assembly.BikeSetup, target assembly.FitTarget, parts testPartOptions, weights assembly.ScoreWeights, limit int) (assembly.BuildRanking, error) {
	if err := parts.Validate(); err != nil {
		return assembly.BuildRanking{}, err
	}
	frame, err := assembly.NewFrame(frameDims(setup.Frame))
	if err != nil {
		return assembly.BuildRanking{}, err
	}
	post, err := assembly.NewRigidPost(setup.Seatpost.Spec.TotalLengthMm, setup.Seatpost.Spec.MinInsertionMm, setup.Seatpost.Spec.MaxInsertionMm, setup.Seatpost.Spec.SetbackMm, setup.Seatpost.Spec.RailClampLengthMm)
	if err != nil {
		return assembly.BuildRanking{}, err
	}
	saddle, err := assembly.NewSaddle(assembly.SaddleDimensions{
		RailCenterToBrpMm: setup.Saddle.RailCenterToBrpMm, RailCenterToSitBonesMm: setup.Saddle.RailCenterToSitBonesMm,
		RailToBrpHeightMm: setup.Saddle.RailToBrpHeightMm, SitBonesElevationMm: setup.Saddle.SitBonesElevationMm,
		RailLengthMm: setup.Saddle.RailLengthMm,
	})
	if err != nil {
		return assembly.BuildRanking{}, err
	}
	var bars []assembly.Handlebar
	hasBarOpts := len(parts.Handlebars) > 0
	if hasBarOpts {
		for _, bs := range parts.Handlebars {
			bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
				StockWidthMm: bs.StockWidthMm, CurrentWidthMm: bs.CurrentWidthMm,
				BacksweepDeg: bs.BacksweepDeg, UpsweepDeg: bs.UpsweepDeg,
				StatedRiseMm: bs.StatedRiseMm, RiseMode: bs.RiseMode,
				BendStartMm: bs.BendStartMm, GripInsetMm: bs.GripInsetMm,
			})
			if err != nil {
				return assembly.BuildRanking{}, err
			}
			bars = append(bars, bar)
		}
	} else {
		bar, err := assembly.NewHandlebar(assembly.HandlebarDimensions{
			StockWidthMm: setup.Handlebar.StockWidthMm, CurrentWidthMm: setup.Handlebar.CurrentWidthMm,
			BacksweepDeg: setup.Handlebar.BacksweepDeg, UpsweepDeg: setup.Handlebar.UpsweepDeg,
			StatedRiseMm: setup.Handlebar.StatedRiseMm, RiseMode: setup.Handlebar.RiseMode,
			BendStartMm: setup.Handlebar.BendStartMm, GripInsetMm: setup.Handlebar.GripInsetMm,
		})
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		bars = []assembly.Handlebar{bar}
	}
	frontOpts := assembly.FrontBuildOptions{
		StemClampHeightMm: setup.Stem.ClampHeightMm,
		Stems:             parts.Stems,
		SpacerStepMm:      parts.SpacerStepMm,
		Handlebars:        bars,
		HasHandlebarOpts:  hasBarOpts,
		DynamicBarRollDeg: setup.Handlebar.BarRollDeg,
	}
	type forkChoice struct {
		fromOpts bool
		idx      int
		fork     assembly.ForkSetup
		cup      assembly.LowerCupSetup
	}
	forks := []forkChoice{{fork: setup.Fork, cup: setup.LowerCup}}
	if len(parts.Forks) > 0 {
		forks = make([]forkChoice, len(parts.Forks))
		for i, f := range parts.Forks {
			forks[i] = forkChoice{fromOpts: true, idx: i, fork: f.Fork, cup: f.LowerCup}
		}
	}
	var best []assembly.ScoredBuild
	evaluated := 0
	for _, fc := range forks {
		fm, err := forkMounterFromSetup(fc.fork)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		fs, err := assembly.MateFrameset(frame, fc.cup, setup.Stem.UpperStackMm, fm, setup.Wheels)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		if setup.Stem.MaxSpacersMm != assembly.DefaultMaxSpacersMm {
			fs, err = fs.WithMaxSpacers(setup.Stem.MaxSpacersMm)
			if err != nil {
				return assembly.BuildRanking{}, err
			}
		}
		rear, err := assembly.OpenRearCockpit(fs, post, saddle, setup.Saddle.RailPitchDeg)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		var seated *assembly.SeatedFrameset
		if target.BRP != nil {
			seated, _, err = rear.FitToBRP(*target.BRP)
		} else {
			seated, err = rear.Clamp(setup.Seatpost.InsertionMm, setup.Saddle.RailOffsetMm)
		}
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		sub, err := assembly.FitFrontBuilds(seated, target, frontOpts, weights, limit)
		if err != nil {
			return assembly.BuildRanking{}, err
		}
		evaluated += sub.Evaluated
		for _, cand := range sub.Builds {
			cand.HasForkOption = fc.fromOpts
			cand.ForkIndex = fc.idx
			cand.Placement.Setup.Stem.MinLengthMm = setup.Stem.MinLengthMm
			cand.Placement.Setup.Stem.MaxLengthMm = setup.Stem.MaxLengthMm
			if len(best) == limit && assembly.CompareScoredBuilds(cand, best[len(best)-1]) >= 0 {
				continue
			}
			i := sort.Search(len(best), func(i int) bool { return assembly.CompareScoredBuilds(cand, best[i]) < 0 })
			if len(best) < limit {
				best = append(best, assembly.ScoredBuild{})
			}
			copy(best[i+1:], best[i:len(best)-1])
			best[i] = cand
		}
	}
	return assembly.BuildRanking{Builds: best, Evaluated: evaluated}, nil
}
