package assembly_test

import (
	"math"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
)

// TestBuildStaticVsDynamicRigidClamping verifies that a single clamped *Build
// with a SuspensionFork (20% sag) and a LinkagePost ((-6, +5) mm sag step)
// places saddle rails and handlebars at exact dynamic headings in DynamicWorld,
// and when unloaded to StaticWorld via Place(Static) extends the fork, unloads
// the seatpost cradle by (+6, -5) mm in shaft coordinates, and rotates all
// frame-clamped landmarks rigidly about BB by (pitch_static − pitch_dynamic)
// to < 1e-12 mm without re-leveling clamped joints.
func TestBuildStaticVsDynamicRigidClamping(t *testing.T) {
	frame, err := assembly.NewFrame(fenrirSizeLDims())
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	fork, err := assembly.NewSuspensionFork(530.0, 44.0, assembly.Travel120, assembly.PhysicalSteererTapered, 20.0)
	if err != nil {
		t.Fatalf("NewSuspensionFork: %v", err)
	}
	fs, err := assembly.MateFrameset(frame, fenrirEc44Cup(), assembly.DefaultUpperStackMm, fork, fenrirWheels())
	if err != nil {
		t.Fatalf("MateFrameset: %v", err)
	}

	sagStep := pose.NewVector(assembly.SeatpostLocal, -6.0, 5.0)
	post, err := assembly.NewLinkagePost(350.0, 100.0, 250.0, 7.0, 30.0, sagStep)
	if err != nil {
		t.Fatalf("NewLinkagePost: %v", err)
	}
	saddle, err := assembly.NewSaddle(sqlab610Dims())
	if err != nil {
		t.Fatalf("NewSaddle: %v", err)
	}
	stem, err := assembly.NewStem(40.0, 60.0, -6.0)
	if err != nil {
		t.Fatalf("NewStem: %v", err)
	}
	bar, err := assembly.NewHandlebar(persuader800Dims())
	if err != nil {
		t.Fatalf("NewHandlebar: %v", err)
	}

	const (
		dynamicRailPitch pose.Deg = 5.0
		dynamicBarRoll   pose.Deg = 7.5
	)

	rear, err := assembly.OpenRearCockpit(fs, post, saddle, dynamicRailPitch)
	if err != nil {
		t.Fatalf("OpenRearCockpit: %v", err)
	}
	seated, err := rear.Clamp(150.0, 8.0)
	if err != nil {
		t.Fatalf("rear.Clamp: %v", err)
	}
	build, err := seated.AttachFront(20.0, stem, bar, dynamicBarRoll)
	if err != nil {
		t.Fatalf("AttachFront: %v", err)
	}

	dyn := build.Place(bounds.Dynamic)
	stat := build.Place(bounds.Static)

	// 1. State tags, bounds, and exact dynamic rail pitch / bar roll headings.
	if dyn.State != bounds.Dynamic || stat.State != bounds.Static {
		t.Fatalf("States = (%q, %q), want (dynamic, static)", dyn.State, stat.State)
	}
	assertNear(t, "dynamic RailClamp heading", float64(dyn.Saddle.RailClamp.HeadingDeg()), float64(dynamicRailPitch), 1e-12)
	assertNear(t, "dynamic BRP heading", float64(dyn.Saddle.BRP.HeadingDeg()), float64(dynamicRailPitch), 1e-12)
	assertNear(t, "dynamic SitBones heading", float64(dyn.Saddle.SitBones.HeadingDeg()), float64(dynamicRailPitch), 1e-12)
	assertNear(t, "dynamic BarCenter heading", float64(dyn.Handlebar.BarCenter.HeadingDeg()), float64(-dynamicBarRoll), 1e-12)
	assertNear(t, "dynamic HandGrip heading", float64(dyn.Handlebar.HandGrip.HeadingDeg()), float64(-dynamicBarRoll), 1e-12)

	// Bounds carry provenance for all three clamped inputs.
	if dyn.Bounds.SeatpostInsertion.Field != bounds.InputSeatpostInsertion || dyn.Bounds.SeatpostInsertion.Value != 150.0 {
		t.Errorf("SeatpostInsertion bound = %+v", dyn.Bounds.SeatpostInsertion)
	}
	if dyn.Bounds.SaddleRailOffset.Field != bounds.InputSaddleRailOffset || dyn.Bounds.SaddleRailOffset.Value != 8.0 {
		t.Errorf("SaddleRailOffset bound = %+v", dyn.Bounds.SaddleRailOffset)
	}
	if dyn.Bounds.StemSpacers.Field != bounds.InputStemSpacers || dyn.Bounds.StemSpacers.Value != 20.0 {
		t.Errorf("StemSpacers bound = %+v", dyn.Bounds.StemSpacers)
	}

	// 2. Unloading extends fork A2C from 506 mm to 530 mm and pitches frame nose-up.
	assertNear(t, "dynamic fork A2C", dyn.Frameset.LowerCupBottom.Origin().DistanceTo(dyn.Frameset.ForkSteererBase.Origin()), 506.0, 1e-12)
	assertNear(t, "static fork A2C", stat.Frameset.LowerCupBottom.Origin().DistanceTo(stat.Frameset.ForkSteererBase.Origin()), 530.0, 1e-12)
	pitchDelta := stat.Frameset.PitchDeg() - dyn.Frameset.PitchDeg()
	if pitchDelta <= 0 {
		t.Fatalf("expected positive pitchDelta on unload, got %v°", pitchDelta)
	}

	// 3. Every frame-clamped landmark (frame + stem + handlebar + seatpost shaft)
	// rotates rigidly about BB by pitchDelta to < 1e-12 mm!
	dynToStat := pose.NewTransform(bounds.DynamicWorld, bounds.StaticWorld, pose.Origin(bounds.StaticWorld), pitchDelta)
	for _, lm := range []struct {
		name string
		dyn  pose.Pose
		stat pose.Pose
	}{
		{"RearAxle", dyn.Frameset.RearAxle, stat.Frameset.RearAxle},
		{"SeatTubeTop", dyn.Frameset.SeatTubeTop, stat.Frameset.SeatTubeTop},
		{"ShaftTop", dyn.Seatpost.ShaftTop, stat.Seatpost.ShaftTop},
		{"HeadTubeTop", dyn.Frameset.HeadTubeTop, stat.Frameset.HeadTubeTop},
		{"HeadTubeBottom", dyn.Frameset.HeadTubeBottom, stat.Frameset.HeadTubeBottom},
		{"LowerCupBottom", dyn.Frameset.LowerCupBottom, stat.Frameset.LowerCupBottom},
		{"CoverTop", dyn.Stem.CoverTop, stat.Stem.CoverTop},
		{"SpacerTop", dyn.Stem.SpacerTop, stat.Stem.SpacerTop},
		{"SteererClamp", dyn.Stem.SteererClamp, stat.Stem.SteererClamp},
		{"BarClamp", dyn.Stem.BarClamp, stat.Stem.BarClamp},
		{"BarCenter", dyn.Handlebar.BarCenter, stat.Handlebar.BarCenter},
		{"HandGrip", dyn.Handlebar.HandGrip, stat.Handlebar.HandGrip},
	} {
		want := dynToStat.ApplyPose(lm.dyn)
		assertNear(t, lm.name+".X", lm.stat.Origin().X(), want.Origin().X(), 1e-12)
		assertNear(t, lm.name+".Y", lm.stat.Origin().Y(), want.Origin().Y(), 1e-12)
		assertNear(t, lm.name+".Heading", float64(lm.stat.HeadingDeg()), float64(want.HeadingDeg()), 1e-12)
	}

	// 4. Seatpost cradle unloads by (+6, -5) mm in shaft coordinates (SeatpostLocal)
	// while the clamped saddle rotates rigidly by pitchDelta without re-leveling.
	unloadInShaft := pose.NewVector(assembly.SeatpostLocal, 6.0, -5.0)
	unloadInStaticWorld := stat.Seatpost.SeatTubeTop.Anchor(assembly.SeatpostLocal).ApplyVector(unloadInShaft)

	for _, lm := range []struct {
		name string
		dyn  pose.Pose
		stat pose.Pose
	}{
		{"RailPivot", dyn.Seatpost.RailPivot, stat.Seatpost.RailPivot},
		{"RailClamp", dyn.Saddle.RailClamp, stat.Saddle.RailClamp},
		{"RailCenter", dyn.Saddle.RailCenter, stat.Saddle.RailCenter},
		{"BRP", dyn.Saddle.BRP, stat.Saddle.BRP},
		{"SitBones", dyn.Saddle.SitBones, stat.Saddle.SitBones},
	} {
		rotated := dynToStat.ApplyPose(lm.dyn)
		wantOrigin := rotated.Origin().Add(unloadInStaticWorld)
		assertNear(t, lm.name+".X", lm.stat.Origin().X(), wantOrigin.X(), 1e-12)
		assertNear(t, lm.name+".Y", lm.stat.Origin().Y(), wantOrigin.Y(), 1e-12)
		assertNear(t, lm.name+".Heading", float64(lm.stat.HeadingDeg()), float64(rotated.HeadingDeg()), 1e-12)
	}

	// Confirm the saddle heading shifted by pitchDelta (no gyroscope re-leveling).
	assertNear(t, "static BRP heading = dynamic + pitchDelta",
		float64(stat.Saddle.BRP.HeadingDeg()), float64(dynamicRailPitch+pitchDelta), 1e-12)
	assertNear(t, "static HandGrip heading = -dynamicBarRoll + pitchDelta",
		float64(stat.Handlebar.HandGrip.HeadingDeg()), float64(-dynamicBarRoll+pitchDelta), 1e-12)

	// Verify Summary matches Placement methods in both states.
	assertNear(t, "dyn Summary.SaddleHeightMm", dyn.Summary.SaddleHeightMm, dyn.SaddleHeightMm(), 1e-12)
	assertNear(t, "stat Summary.BrpToGripDistMm", stat.Summary.BrpToGripDistMm, stat.BrpToGripDistMm(), 1e-12)
	assertNear(t, "stat Summary.TrailMm", stat.Summary.TrailMm, stat.Frameset.TrailMm(), 1e-12)

	// Verify FrontCockpitDoF + FitToGrip works cleanly on SeatedFrameset.
	lenRange, err := bounds.NewBoundedRange(40, 130, "min stem", "max stem")
	if err != nil {
		t.Fatalf("NewBoundedRange: %v", err)
	}
	front, err := assembly.OpenFrontCockpit(seated, -6, 40, lenRange, bar, dynamicBarRoll)
	if err != nil {
		t.Fatalf("OpenFrontCockpit: %v", err)
	}
	gripTarget := dyn.Handlebar.HandGrip.Origin()
	fittedBuild, limited, err := front.FitToGrip(gripTarget)
	if err != nil {
		t.Fatalf("front.FitToGrip: %v", err)
	}
	if len(limited) != 0 {
		t.Fatalf("expected reachable gripTarget, limited = %v", limited)
	}
	fittedDyn := fittedBuild.Place(bounds.Dynamic)
	assertNear(t, "fitted grip miss", gripTarget.DistanceTo(fittedDyn.Handlebar.HandGrip.Origin()), 0, 1e-9)
	assertNear(t, "fitted spacers", fittedDyn.Bounds.StemSpacers.Value, 20.0, 1e-6)
	assertNear(t, "fitted stem length", fittedDyn.Setup.Stem.LengthMm, 60.0, 1e-6)

	// Verify invalid LoadState panics deterministically on Build.Place.
	assertPanics(t, "Build.Place invalid LoadState", func() {
		_ = build.Place(bounds.LoadState("bad"))
	})
	assertPanics(t, "nil Build.Place", func() {
		var nilBuild *assembly.Build
		_ = nilBuild.Place(bounds.Dynamic)
	})
	if math.IsNaN(dyn.Summary.WheelbaseMm) {
		t.Error("unexpected NaN WheelbaseMm")
	}
}
