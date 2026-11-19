package assembly_test

import (
	"math"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func sqlab610DefaultSaddle() assembly.SaddleSetup {
	return assembly.SaddleSetup{
		RailCenterToBrpMm:      32.0,
		RailCenterToSitBonesMm: -48.0,
		SitBonesElevationMm:    10.0,
		RailLengthMm:           60.0,
		RailPitchDeg:           0.0,
		RailOffsetMm:           0.0,
	}
}

func TestPlaceSeatpostAndSaddleWorkedExample60DegSTA(t *testing.T) {
	// At STA 60° (seat-axis heading 180° - 60° = 120°; cos = -0.5, sin = sqrt(3)/2):
	// SeatTubeTop at 60 mm along seat axis = (-30, 30*sqrt(3)) @ 120°.
	// Seatpost extension 40 mm => ShaftTop at 100 mm along seat axis = (-50, 50*sqrt(3)) @ 120°.
	// Zero post setback => SeatpostClamp = (-50, 50*sqrt(3)) @ 120°.
	// Level rails (0°), rail offset +12 mm (advances -12 mm along rails):
	//   RailCenter = (-62, 50*sqrt(3)) @ 0°
	//   BRP        = (-30, 50*sqrt(3)) @ 0°
	//   SitBones   = (-110, 50*sqrt(3) + 10) @ 0°
	sqrt3 := math.Sqrt(3)
	tr := pose.NewTrace()
	seatAxis := pose.NewPose(pose.Point{}, 120.0, tr)
	seatTubeTop := seatAxis.Advance("seat tube length", 60.0)

	post, err := assembly.PlaceSeatpost(seatTubeTop, assembly.SeatpostSetup{
		ExtensionMm:   40.0,
		PostSetbackMm: 0.0,
		ClampLengthMm: 30.0,
	})
	if err != nil {
		t.Fatalf("PlaceSeatpost: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 12.0
	saddle, err := assembly.PlaceSaddle(post.SeatpostClamp, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle: %v", err)
	}
	tr.Freeze()

	assertNear(t, "ShaftTop.X", post.ShaftTop.Origin().X, -50.0, 1e-9)
	assertNear(t, "ShaftTop.Y", post.ShaftTop.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "SeatpostClamp.X", post.SeatpostClamp.Origin().X, -50.0, 1e-9)
	assertNear(t, "SeatpostClamp.Y", post.SeatpostClamp.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "SeatpostClamp.Heading", float64(post.SeatpostClamp.HeadingDeg()), 120.0, 1e-9)

	assertNear(t, "RailCenter.X", saddle.RailCenter.Origin().X, -62.0, 1e-9)
	assertNear(t, "RailCenter.Y", saddle.RailCenter.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "BRP.X", saddle.BRP.Origin().X, -30.0, 1e-9)
	assertNear(t, "BRP.Y", saddle.BRP.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "SitBones.X", saddle.SitBones.Origin().X, -110.0, 1e-9)
	assertNear(t, "SitBones.Y", saddle.SitBones.Origin().Y, 50.0*sqrt3+10.0, 1e-9)

	for _, lm := range []struct {
		name string
		p    pose.Pose
	}{
		{"RailFrame", saddle.RailFrame},
		{"RailCenter", saddle.RailCenter},
		{"BRP", saddle.BRP},
		{"SitBones", saddle.SitBones},
	} {
		assertNear(t, lm.name+".Heading", float64(lm.p.HeadingDeg()), 0.0, 1e-12)
	}

	// Verify BRP and SitBones both branch directly from RailCenter in the trace.
	steps := tr.Steps()
	if brpStep := steps[saddle.BRP.StepID()-1]; brpStep.ParentID != saddle.RailCenter.StepID() {
		t.Fatalf("BRP ParentID = %d, want RailCenter.StepID %d", brpStep.ParentID, saddle.RailCenter.StepID())
	}
	if sitStep := steps[saddle.SitBones.StepID()-1]; sitStep.ParentID != saddle.RailCenter.StepID() {
		t.Fatalf("SitBones ParentID = %d, want RailCenter.StepID %d", sitStep.ParentID, saddle.RailCenter.StepID())
	}
}

func TestPlaceSeatpostSetbackAndSaddleRailOffsetSigns(t *testing.T) {
	// At 60° STA (heading 120°), local +Y is 120° + 90° = 210° (cos = -sqrt(3)/2, sin = -0.5).
	// Adding +20 mm PostSetbackMm shifts SeatpostClamp and all saddle landmarks by (-10*sqrt(3), -10) mm
	// while leaving ShaftTop unchanged.
	seatTubeTop := pose.NewPose(pose.Point{X: -30, Y: 30 * math.Sqrt(3)}, 120.0, nil)
	basePost, err := assembly.PlaceSeatpost(seatTubeTop, assembly.SeatpostSetup{
		ExtensionMm:   40.0,
		PostSetbackMm: 0.0,
		ClampLengthMm: 30.0,
	})
	if err != nil {
		t.Fatalf("PlaceSeatpost base: %v", err)
	}
	setbackPost, err := assembly.PlaceSeatpost(seatTubeTop, assembly.SeatpostSetup{
		ExtensionMm:   40.0,
		PostSetbackMm: 20.0,
		ClampLengthMm: 30.0,
	})
	if err != nil {
		t.Fatalf("PlaceSeatpost setback: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 12.0
	baseSaddle, err := assembly.PlaceSaddle(basePost.SeatpostClamp, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle base: %v", err)
	}
	setbackSaddle, err := assembly.PlaceSaddle(setbackPost.SeatpostClamp, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle setback: %v", err)
	}

	assertNear(t, "ShaftTop unchanged X", setbackPost.ShaftTop.Origin().X, basePost.ShaftTop.Origin().X, 1e-12)
	assertNear(t, "ShaftTop unchanged Y", setbackPost.ShaftTop.Origin().Y, basePost.ShaftTop.Origin().Y, 1e-12)

	wantDX := -10.0 * math.Sqrt(3)
	wantDY := -10.0
	for _, pair := range []struct {
		name string
		a, b pose.Pose
	}{
		{"SeatpostClamp", basePost.SeatpostClamp, setbackPost.SeatpostClamp},
		{"RailCenter", baseSaddle.RailCenter, setbackSaddle.RailCenter},
		{"BRP", baseSaddle.BRP, setbackSaddle.BRP},
		{"SitBones", baseSaddle.SitBones, setbackSaddle.SitBones},
	} {
		assertNear(t, pair.name+" setback dX", pair.b.Origin().X-pair.a.Origin().X, wantDX, 1e-9)
		assertNear(t, pair.name+" setback dY", pair.b.Origin().Y-pair.a.Origin().Y, wantDY, 1e-9)
	}

	// Increasing RailOffsetMm by +10 mm moves saddle landmarks rearward (-10 mm X) while leaving clamp fixed.
	moreOffsetSetup := saddleSetup
	moreOffsetSetup.RailOffsetMm = 22.0
	moreOffsetSaddle, err := assembly.PlaceSaddle(basePost.SeatpostClamp, moreOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle +22 offset: %v", err)
	}
	assertNear(t, "SeatpostClamp fixed X", moreOffsetSaddle.SeatpostClamp.Origin().X, baseSaddle.SeatpostClamp.Origin().X, 1e-12)
	assertNear(t, "RailCenter shifted -10", moreOffsetSaddle.RailCenter.Origin().X-baseSaddle.RailCenter.Origin().X, -10.0, 1e-12)
	assertNear(t, "BRP shifted -10", moreOffsetSaddle.BRP.Origin().X-baseSaddle.BRP.Origin().X, -10.0, 1e-12)
	assertNear(t, "SitBones shifted -10", moreOffsetSaddle.SitBones.Origin().X-baseSaddle.SitBones.Origin().X, -10.0, 1e-12)

	// Negative RailOffsetMm (-8 mm, delta -20 mm from +12 mm) moves saddle landmarks forward (+20 mm X).
	negOffsetSetup := saddleSetup
	negOffsetSetup.RailOffsetMm = -8.0
	negOffsetSaddle, err := assembly.PlaceSaddle(basePost.SeatpostClamp, negOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle -8 offset: %v", err)
	}
	assertNear(t, "RailCenter shifted +20", negOffsetSaddle.RailCenter.Origin().X-baseSaddle.RailCenter.Origin().X, 20.0, 1e-12)
	assertNear(t, "BRP shifted +20", negOffsetSaddle.BRP.Origin().X-baseSaddle.BRP.Origin().X, 20.0, 1e-12)
}

func TestPlaceSaddleWorldRelativePitch(t *testing.T) {
	setup := sqlab610DefaultSaddle()
	setup.RailPitchDeg = 10.0 // Positive pitch raises the nose CCW from world horizontal.
	setup.RailOffsetMm = 5.0

	clamp60 := pose.NewPose(pose.Point{X: -180, Y: 720}, 180.0-60.0, nil)
	clamp75 := pose.NewPose(pose.Point{X: -180, Y: 720}, 180.0-75.25, nil)

	s60, err := assembly.PlaceSaddle(clamp60, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle 60°: %v", err)
	}
	s75, err := assembly.PlaceSaddle(clamp75, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle 75.25°: %v", err)
	}

	if s60.BRP.Origin().Y <= s60.RailCenter.Origin().Y {
		t.Fatalf("positive rail pitch must raise BRP above RailCenter: BRP.Y=%.4f, RailCenter.Y=%.4f",
			s60.BRP.Origin().Y, s60.RailCenter.Origin().Y)
	}

	// Because Pose.Face sets an absolute world heading, saddle landmarks depend
	// only on SeatpostClamp.Origin() and world RailPitchDeg, never on STA heading.
	assertNear(t, "BRP.X STA-independent", s60.BRP.Origin().X, s75.BRP.Origin().X, 1e-12)
	assertNear(t, "BRP.Y STA-independent", s60.BRP.Origin().Y, s75.BRP.Origin().Y, 1e-12)
	assertNear(t, "SitBones.X STA-independent", s60.SitBones.Origin().X, s75.SitBones.Origin().X, 1e-12)
	assertNear(t, "SitBones.Y STA-independent", s60.SitBones.Origin().Y, s75.SitBones.Origin().Y, 1e-12)
	assertNear(t, "BRP.Heading", float64(s60.BRP.HeadingDeg()), 10.0, 1e-12)
	assertNear(t, "SitBones.Heading", float64(s60.SitBones.HeadingDeg()), 10.0, 1e-12)
}

func TestOtsoFenrirRearCockpitAndTraceEquivalence(t *testing.T) {
	frame := fenrirSizeLFrame()
	wheels := fenrirWheels()
	cadFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0},
	}
	postSetup := assembly.SeatpostSetup{
		ExtensionMm:   263.5,
		PostSetbackMm: 0.0,
		ClampLengthMm: 30.0,
	}
	saddleSetup := sqlab610DefaultSaddle()

	// Verify the IMPLEMENTATION.md §2 invariant on the 75.25° STA reference attitude:
	// 490.0 mm seat tube + 263.5 mm extension = 753.5 mm along seat axis, and |BRP - BB| ≈ 746.0 mm.
	refSeatAxis := pose.NewPose(pose.Point{}, 180.0-frame.SeatTubeAngleDeg, nil)
	refSeatTubeTop := refSeatAxis.Advance("seat tube length", frame.SeatTubeLengthMm)
	refPost, err := assembly.PlaceSeatpost(refSeatTubeTop, postSetup)
	if err != nil {
		t.Fatalf("PlaceSeatpost ref: %v", err)
	}
	refSaddle, err := assembly.PlaceSaddle(refPost.SeatpostClamp, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle ref: %v", err)
	}
	assertNear(t, "BB to clamp along seat axis", refSeatAxis.DistanceTo(refPost.ShaftTop), 753.5, 1e-9)
	assertNear(t, "reference |BRP - BB|", refSeatAxis.DistanceTo(refSaddle.BRP), 746.0, 0.01)

	// Full leveled frameset + cockpits via PlaceBike, with trace-on/off equivalence.
	bikeSetup := assembly.BikeSetup{
		Frame:     frame,
		Fork:      cadFork,
		Wheels:    wheels,
		Seatpost:  postSetup,
		Saddle:    saddleSetup,
		Stem:      fenrirDefaultStem(),
		Handlebar: persuader800DefaultHandlebar(),
	}
	tr := pose.NewTrace()
	traced, err := assembly.PlaceBike(bikeSetup, tr)
	if err != nil {
		t.Fatalf("PlaceBike traced: %v", err)
	}

	untraced, err := assembly.PlaceBike(bikeSetup, nil)
	if err != nil {
		t.Fatalf("PlaceBike untraced: %v", err)
	}

	assertNear(t, "leveled BB to post top along seat axis", traced.SeatAxisToPostTopMm(), 753.5, 1e-9)
	assertNear(t, "CAD leveled |BRP - BB|", traced.SaddleHeightMm(), 746.0, 0.1)
	assertNear(t, "Setup preserved on BikePlacement", traced.Setup.Seatpost.ClampLengthMm, 30.0, 1e-12)

	for _, pair := range []struct {
		name string
		a, b pose.Pose
	}{
		{"SeatTubeTop", traced.Frameset.SeatTubeTop, untraced.Frameset.SeatTubeTop},
		{"ShaftTop", traced.Seatpost.ShaftTop, untraced.Seatpost.ShaftTop},
		{"SeatpostClamp", traced.Seatpost.SeatpostClamp, untraced.Seatpost.SeatpostClamp},
		{"RailFrame", traced.Saddle.RailFrame, untraced.Saddle.RailFrame},
		{"RailCenter", traced.Saddle.RailCenter, untraced.Saddle.RailCenter},
		{"BRP", traced.Saddle.BRP, untraced.Saddle.BRP},
		{"SitBones", traced.Saddle.SitBones, untraced.Saddle.SitBones},
	} {
		assertNear(t, pair.name+".X eq", pair.a.Origin().X, pair.b.Origin().X, 1e-12)
		assertNear(t, pair.name+".Y eq", pair.a.Origin().Y, pair.b.Origin().Y, 1e-12)
		assertNear(t, pair.name+".Heading eq", float64(pair.a.HeadingDeg()), float64(pair.b.HeadingDeg()), 1e-12)
	}

	badSeatpost := bikeSetup
	badSeatpost.Seatpost.ExtensionMm = -1
	if _, err := assembly.PlaceBike(badSeatpost, nil); err == nil {
		t.Fatal("PlaceBike expected error for negative seatpost extension")
	}

	badSaddle := bikeSetup
	badSaddle.Saddle.RailLengthMm = 0
	if _, err := assembly.PlaceBike(badSaddle, nil); err == nil {
		t.Fatal("PlaceBike expected error for zero saddle rail length")
	}
}

func TestPlaceSeatpostAndSaddleValidation(t *testing.T) {
	validPose := pose.NewPose(pose.Point{X: -120, Y: 470}, 104.75, nil)
	validPost := assembly.SeatpostSetup{
		ExtensionMm:   263.5,
		PostSetbackMm: 0.0,
		ClampLengthMm: 30.0,
	}

	postCases := []struct {
		name  string
		pose  pose.Pose
		setup assembly.SeatpostSetup
	}{
		{"NaN origin X", pose.NewPose(pose.Point{X: math.NaN(), Y: 0}, 104.75, nil), validPost},
		{"Inf heading", pose.NewPose(pose.Point{}, pose.Deg(math.Inf(1)), nil), validPost},
		{"negative extension", validPose, assembly.SeatpostSetup{ExtensionMm: -1, PostSetbackMm: 0, ClampLengthMm: 30}},
		{"NaN extension", validPose, assembly.SeatpostSetup{ExtensionMm: math.NaN(), PostSetbackMm: 0, ClampLengthMm: 30}},
		{"negative setback", validPose, assembly.SeatpostSetup{ExtensionMm: 100, PostSetbackMm: -1, ClampLengthMm: 30}},
		{"NaN setback", validPose, assembly.SeatpostSetup{ExtensionMm: 100, PostSetbackMm: math.NaN(), ClampLengthMm: 30}},
		{"zero clamp length", validPose, assembly.SeatpostSetup{ExtensionMm: 100, PostSetbackMm: 0, ClampLengthMm: 0}},
		{"negative clamp length", validPose, assembly.SeatpostSetup{ExtensionMm: 100, PostSetbackMm: 0, ClampLengthMm: -10}},
		{"extension overflow", pose.NewPose(pose.Point{X: -math.MaxFloat64, Y: 0}, 120, nil), assembly.SeatpostSetup{ExtensionMm: math.MaxFloat64, ClampLengthMm: 30}},
	}
	for _, tc := range postCases {
		t.Run("seatpost/"+tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceSeatpost(tc.pose, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}

	validSaddle := sqlab610DefaultSaddle()
	saddleCases := []struct {
		name  string
		pose  pose.Pose
		setup assembly.SaddleSetup
	}{
		{"NaN origin Y", pose.NewPose(pose.Point{X: 0, Y: math.NaN()}, 104.75, nil), validSaddle},
		{"Inf heading", pose.NewPose(pose.Point{}, pose.Deg(math.Inf(-1)), nil), validSaddle},
		{"NaN RailCenterToBrpMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: math.NaN(), RailLengthMm: 60}},
		{"Inf RailCenterToSitBonesMm", validPose, assembly.SaddleSetup{RailCenterToSitBonesMm: math.Inf(1), RailLengthMm: 60}},
		{"NaN SitBonesElevationMm", validPose, assembly.SaddleSetup{SitBonesElevationMm: math.NaN(), RailLengthMm: 60}},
		{"zero RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: 0}},
		{"negative RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: -60}},
		{"NaN RailPitchDeg", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailPitchDeg: pose.Deg(math.NaN())}},
		{"Inf RailOffsetMm", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: math.Inf(1)}},
		{"rail offset overflow", pose.NewPose(pose.Point{X: math.MaxFloat64, Y: 0}, 104.75, nil), assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: -math.MaxFloat64}},
	}
	for _, tc := range saddleCases {
		t.Run("saddle/"+tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceSaddle(tc.pose, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
