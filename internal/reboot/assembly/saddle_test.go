package assembly_test

import (
	"errors"
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
	// Zero post setback => RailCradle = (-50, 50*sqrt(3)) @ 120°.
	// Level rails (0°), rail offset +12 mm (advances -12 mm along rails):
	//   RailCenter = (-62, 50*sqrt(3)) @ 0°
	//   BRP        = (-30, 50*sqrt(3)) @ 0°
	//   SitBones   = (-110, 50*sqrt(3) + 10) @ 0°
	sqrt3 := math.Sqrt(3)
	seatAxis := pose.NewPose(pose.Point{}, 120.0)
	seatTubeTop := seatAxis.Advance(60.0)

	post, err := assembly.PlaceSeatpost(seatTubeTop, workedPost(0.0), workedBounds)
	if err != nil {
		t.Fatalf("PlaceSeatpost: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 12.0
	saddle, err := assembly.PlaceSaddle(post.RailCradle, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle: %v", err)
	}

	assertNear(t, "ShaftTop.X", post.ShaftTop.Origin().X, -50.0, 1e-9)
	assertNear(t, "ShaftTop.Y", post.ShaftTop.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "RailCradle.X", post.RailCradle.Origin().X, -50.0, 1e-9)
	assertNear(t, "RailCradle.Y", post.RailCradle.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "RailCradle.Heading", float64(post.RailCradle.HeadingDeg()), 120.0, 1e-9)

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
}

func TestPlaceSeatpostSetbackAndSaddleRailOffsetSigns(t *testing.T) {
	// At 60° STA (heading 120°), local +Y is 120° + 90° = 210° (cos = -sqrt(3)/2, sin = -0.5).
	// Adding +20 mm PostSetbackMm shifts RailCradle and all saddle landmarks by (-10*sqrt(3), -10) mm
	// while leaving ShaftTop unchanged.
	seatTubeTop := pose.NewPose(pose.Point{X: -30, Y: 30 * math.Sqrt(3)}, 120.0)
	basePost, err := assembly.PlaceSeatpost(seatTubeTop, workedPost(0.0), workedBounds)
	if err != nil {
		t.Fatalf("PlaceSeatpost base: %v", err)
	}
	setbackPost, err := assembly.PlaceSeatpost(seatTubeTop, workedPost(20.0), workedBounds)
	if err != nil {
		t.Fatalf("PlaceSeatpost setback: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 5.0
	baseSaddle, err := assembly.PlaceSaddle(basePost.RailCradle, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle base: %v", err)
	}
	setbackSaddle, err := assembly.PlaceSaddle(setbackPost.RailCradle, 30.0, saddleSetup)
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
		{"RailCradle", basePost.RailCradle, setbackPost.RailCradle},
		{"RailCenter", baseSaddle.RailCenter, setbackSaddle.RailCenter},
		{"BRP", baseSaddle.BRP, setbackSaddle.BRP},
		{"SitBones", baseSaddle.SitBones, setbackSaddle.SitBones},
	} {
		assertNear(t, pair.name+" setback dX", pair.b.Origin().X-pair.a.Origin().X, wantDX, 1e-9)
		assertNear(t, pair.name+" setback dY", pair.b.Origin().Y-pair.a.Origin().Y, wantDY, 1e-9)
	}

	// Increasing RailOffsetMm by +10 mm moves saddle landmarks rearward (-10 mm X) while leaving clamp fixed.
	moreOffsetSetup := saddleSetup
	moreOffsetSetup.RailOffsetMm = 15.0
	moreOffsetSaddle, err := assembly.PlaceSaddle(basePost.RailCradle, 30.0, moreOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle +15 offset: %v", err)
	}
	assertNear(t, "RailCradle fixed X", moreOffsetSaddle.RailCradle.Origin().X, baseSaddle.RailCradle.Origin().X, 1e-12)
	assertNear(t, "RailCenter shifted -10", moreOffsetSaddle.RailCenter.Origin().X-baseSaddle.RailCenter.Origin().X, -10.0, 1e-12)
	assertNear(t, "BRP shifted -10", moreOffsetSaddle.BRP.Origin().X-baseSaddle.BRP.Origin().X, -10.0, 1e-12)
	assertNear(t, "SitBones shifted -10", moreOffsetSaddle.SitBones.Origin().X-baseSaddle.SitBones.Origin().X, -10.0, 1e-12)

	// Negative RailOffsetMm (-15 mm, delta -20 mm from +5 mm) moves saddle landmarks forward (+20 mm X).
	negOffsetSetup := saddleSetup
	negOffsetSetup.RailOffsetMm = -15.0
	negOffsetSaddle, err := assembly.PlaceSaddle(basePost.RailCradle, 30.0, negOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle -15 offset: %v", err)
	}
	assertNear(t, "RailCenter shifted +20", negOffsetSaddle.RailCenter.Origin().X-baseSaddle.RailCenter.Origin().X, 20.0, 1e-12)
	assertNear(t, "BRP shifted +20", negOffsetSaddle.BRP.Origin().X-baseSaddle.BRP.Origin().X, 20.0, 1e-12)
}

func TestPlaceSaddleWorldRelativePitch(t *testing.T) {
	setup := sqlab610DefaultSaddle()
	setup.RailPitchDeg = 10.0 // Positive pitch raises the nose CCW from world horizontal.
	setup.RailOffsetMm = 5.0

	clamp60 := pose.NewPose(pose.Point{X: -180, Y: 720}, 180.0-60.0)
	clamp75 := pose.NewPose(pose.Point{X: -180, Y: 720}, 180.0-75.25)

	s60, err := assembly.PlaceSaddle(clamp60, 30.0, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle 60°: %v", err)
	}
	s75, err := assembly.PlaceSaddle(clamp75, 30.0, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle 75.25°: %v", err)
	}

	if s60.BRP.Origin().Y <= s60.RailCenter.Origin().Y {
		t.Fatalf("positive rail pitch must raise BRP above RailCenter: BRP.Y=%.4f, RailCenter.Y=%.4f",
			s60.BRP.Origin().Y, s60.RailCenter.Origin().Y)
	}

	// Because Pose.Face sets an absolute world heading, saddle landmarks depend
	// only on RailCradle.Origin() and world RailPitchDeg, never on STA heading.
	assertNear(t, "BRP.X STA-independent", s60.BRP.Origin().X, s75.BRP.Origin().X, 1e-12)
	assertNear(t, "BRP.Y STA-independent", s60.BRP.Origin().Y, s75.BRP.Origin().Y, 1e-12)
	assertNear(t, "SitBones.X STA-independent", s60.SitBones.Origin().X, s75.SitBones.Origin().X, 1e-12)
	assertNear(t, "SitBones.Y STA-independent", s60.SitBones.Origin().Y, s75.SitBones.Origin().Y, 1e-12)
	assertNear(t, "BRP.Heading", float64(s60.BRP.HeadingDeg()), 10.0, 1e-12)
	assertNear(t, "SitBones.Heading", float64(s60.SitBones.HeadingDeg()), 10.0, 1e-12)
}

func TestOtsoFenrirRearCockpit(t *testing.T) {
	frame := fenrirSizeLFrame()
	wheels := fenrirWheels()
	cadFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0, Steerer: assembly.SteererCAD},
	}
	postSetup := assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 136.5}
	saddleSetup := sqlab610DefaultSaddle()

	// Check the fixture's reference saddle height at the 75.25° STA attitude:
	// 490.0 mm seat tube + 263.5 mm extension = 753.5 mm along seat axis, and |BRP - BB| ≈ 746.0 mm.
	refSeatAxis := pose.NewPose(pose.Point{}, 180.0-frame.SeatTubeAngleDeg)
	refSeatTubeTop := refSeatAxis.Advance(frame.SeatTubeLengthMm)
	refBounds, err := assembly.SeatpostInsertionBounds(frame, postSetup.Spec)
	if err != nil {
		t.Fatalf("SeatpostInsertionBounds: %v", err)
	}
	refPost, err := assembly.PlaceSeatpost(refSeatTubeTop, postSetup, refBounds)
	if err != nil {
		t.Fatalf("PlaceSeatpost ref: %v", err)
	}
	refSaddle, err := assembly.PlaceSaddle(refPost.RailCradle, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle ref: %v", err)
	}
	assertNear(t, "BB to clamp along seat axis", refSeatAxis.DistanceTo(refPost.ShaftTop), 753.5, 1e-9)
	assertNear(t, "reference |BRP - BB|", refSeatAxis.DistanceTo(refSaddle.BRP), 746.0, 0.01)

	// Full leveled frameset + cockpits via PlaceBike.
	bikeSetup := assembly.BikeSetup{
		Frame:     frame,
		LowerCup:  assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0.0},
		Fork:      cadFork,
		Wheels:    wheels,
		Seatpost:  postSetup,
		Saddle:    saddleSetup,
		Stem:      fenrirDefaultStem(),
		Handlebar: persuader800DefaultHandlebar(),
	}
	bike, err := assembly.PlaceBike(bikeSetup)
	if err != nil {
		t.Fatalf("PlaceBike: %v", err)
	}

	assertNear(t, "leveled BB to post top along seat axis", bike.SeatAxisToPostTopMm(), 753.5, 1e-9)
	assertNear(t, "CAD leveled |BRP - BB|", bike.SaddleHeightMm(), 746.0, 0.1)
	assertNear(t, "Setup preserved on BikePlacement", bike.Setup.Seatpost.Spec.RailClampLengthMm, 30.0, 1e-12)
	// The post shaft continues the seat axis from SeatTubeTop; the saddle shares the leveled-rail heading.
	assertNear(t, "seat tube top to shaft top", bike.Frameset.SeatTubeTop.DistanceTo(bike.Seatpost.ShaftTop), postSetup.ExposedMm(), 1e-9)
	assertNear(t, "RailFrame.Heading", float64(bike.Saddle.RailFrame.HeadingDeg()), float64(saddleSetup.RailPitchDeg), 1e-12)

	badSeatpost := bikeSetup
	badSeatpost.Seatpost.InsertionMm = -1
	if _, err := assembly.PlaceBike(badSeatpost); err == nil {
		t.Fatal("PlaceBike expected error for negative seatpost extension")
	}

	badSaddle := bikeSetup
	badSaddle.Saddle.RailLengthMm = 0
	if _, err := assembly.PlaceBike(badSaddle); err == nil {
		t.Fatal("PlaceBike expected error for zero saddle rail length")
	}
}

func TestPlaceSeatpostAndSaddleValidation(t *testing.T) {
	validPose := pose.NewPose(pose.Point{X: -120, Y: 470}, 104.75)
	validPost := assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 136.5}
	validBounds := assembly.InsertionBounds{EffectiveMinInsertionMm: 100, MaxAllowedInsertionMm: 290}
	with := func(f func(*assembly.SeatpostSetup)) assembly.SeatpostSetup {
		s := validPost
		f(&s)
		return s
	}

	postCases := []struct {
		name  string
		pose  pose.Pose
		setup assembly.SeatpostSetup
	}{
		{"NaN origin X", pose.NewPose(pose.Point{X: math.NaN(), Y: 0}, 104.75), validPost},
		{"Inf heading", pose.NewPose(pose.Point{}, pose.Deg(math.Inf(1))), validPost},
		{"negative insertion", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = -1 })},
		{"NaN insertion", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = math.NaN() })},
		{"insertion below minimum", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = 99.999 })},
		{"insertion above maximum", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = 290.001 })},
		{"negative setback", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.SetbackMm = -1 })},
		{"NaN setback", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.SetbackMm = math.NaN() })},
		{"zero clamp length", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.RailClampLengthMm = 0 })},
		{"negative clamp length", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.RailClampLengthMm = -10 })},
		{"exposure overflow", pose.NewPose(pose.Point{X: -math.MaxFloat64, Y: 0}, 120), with(func(s *assembly.SeatpostSetup) { s.Spec.TotalLengthMm = math.MaxFloat64 })},
	}
	for _, tc := range postCases {
		t.Run("seatpost/"+tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceSeatpost(tc.pose, tc.setup, validBounds); err == nil {
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
		{"NaN origin Y", pose.NewPose(pose.Point{X: 0, Y: math.NaN()}, 104.75), validSaddle},
		{"Inf heading", pose.NewPose(pose.Point{}, pose.Deg(math.Inf(-1))), validSaddle},
		{"NaN RailCenterToBrpMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: math.NaN(), RailLengthMm: 60}},
		{"Inf RailCenterToSitBonesMm", validPose, assembly.SaddleSetup{RailCenterToSitBonesMm: math.Inf(1), RailLengthMm: 60}},
		{"NaN SitBonesElevationMm", validPose, assembly.SaddleSetup{SitBonesElevationMm: math.NaN(), RailLengthMm: 60}},
		{"zero RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: 0}},
		{"negative RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: -60}},
		{"NaN RailPitchDeg", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailPitchDeg: pose.Deg(math.NaN())}},
		{"Inf RailOffsetMm", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: math.Inf(1)}},
		{"rail offset overflow", pose.NewPose(pose.Point{X: math.MaxFloat64, Y: 0}, 104.75), assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: -math.MaxFloat64}},
	}
	for _, tc := range saddleCases {
		t.Run("saddle/"+tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceSaddle(tc.pose, 30.0, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestPlaceSaddleRailOffsetRange(t *testing.T) {
	clamp := pose.NewPose(pose.Point{X: -100, Y: 700}, 105.0)
	// 60 mm rails with a 30 mm clamp head allow ±15 mm.
	for _, offset := range []float64{0, 14.999, 15, -15} {
		setup := sqlab610DefaultSaddle()
		setup.RailOffsetMm = offset
		if _, err := assembly.PlaceSaddle(clamp, 30.0, setup); err != nil {
			t.Errorf("offset %v: %v", offset, err)
		}
	}
	for _, offset := range []float64{15.001, -15.001, 20} {
		setup := sqlab610DefaultSaddle()
		setup.RailOffsetMm = offset
		_, err := assembly.PlaceSaddle(clamp, 30.0, setup)
		var inputErr *assembly.InputError
		if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputSaddleRailOffset {
			t.Errorf("offset %v: err = %v, want InputSaddleRailOffset", offset, err)
		}
	}
	if _, err := assembly.PlaceSaddle(clamp, 70.0, sqlab610DefaultSaddle()); err == nil {
		t.Error("clamp longer than the rails: want error")
	}
}

var workedBounds = assembly.InsertionBounds{EffectiveMinInsertionMm: 20, MaxAllowedInsertionMm: 110}

// workedPost is a synthetic 150 mm post inserted 110 mm, so it exposes 40 mm.
func workedPost(setbackMm float64) assembly.SeatpostSetup {
	return assembly.SeatpostSetup{
		Spec: assembly.SeatpostSpec{
			TotalLengthMm: 150, MaxInsertionMm: 110, MinInsertionMm: 20,
			SetbackMm: setbackMm, RailClampLengthMm: 30,
		},
		InsertionMm: 110,
	}
}
