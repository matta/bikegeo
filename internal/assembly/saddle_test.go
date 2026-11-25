package assembly_test

import (
	"errors"
	"math"
	"testing"

	"bikegeo/internal/assembly"
	"bikegeo/internal/pose"
)

func sqlab610DefaultSaddle() assembly.SaddleSetup {
	return assembly.SaddleSetup{
		RailCenterToBrpMm:      32.0,
		RailCenterToSitBonesMm: -48.0,
		RailToBrpHeightMm:      45.0,
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
	// Zero post setback => RailPivot = (-50, 50*sqrt(3)) @ 120°.
	// Level rails (0°), rail offset +12 mm (advances -12 mm along rails):
	//   RailCenter = (-62, 50*sqrt(3)) @ 0°
	//   BRP        = (-30, 50*sqrt(3) + 45) @ 0°  (45 mm RailToBrpHeightMm above the rails)
	//   SitBones   = (-110, 50*sqrt(3) + 55) @ 0°  (10 mm above BRP)
	sqrt3 := math.Sqrt(3)
	seatAxis := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{}, 120.0)
	seatTubeTop := seatAxis.Advance(60.0)

	post, err := assembly.PlaceSeatpost(seatTubeTop, workedFrame, workedPost(0.0))
	if err != nil {
		t.Fatalf("PlaceSeatpost: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 12.0
	saddle, err := assembly.PlaceSaddle(post.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle: %v", err)
	}

	assertNear(t, "ShaftTop.X", post.ShaftTop.Origin().X, -50.0, 1e-9)
	assertNear(t, "ShaftTop.Y", post.ShaftTop.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "RailPivot.X", post.RailPivot.Origin().X, -50.0, 1e-9)
	assertNear(t, "RailPivot.Y", post.RailPivot.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "RailPivot.Heading", float64(post.RailPivot.HeadingDeg()), 120.0, 1e-9)

	assertNear(t, "RailCenter.X", saddle.RailCenter.Origin().X, -62.0, 1e-9)
	assertNear(t, "RailCenter.Y", saddle.RailCenter.Origin().Y, 50.0*sqrt3, 1e-9)
	assertNear(t, "BRP.X", saddle.BRP.Origin().X, -30.0, 1e-9)
	assertNear(t, "BRP.Y", saddle.BRP.Origin().Y, 50.0*sqrt3+45.0, 1e-9)
	assertNear(t, "SitBones.X", saddle.SitBones.Origin().X, -110.0, 1e-9)
	assertNear(t, "SitBones.Y", saddle.SitBones.Origin().Y, 50.0*sqrt3+55.0, 1e-9)

	for _, lm := range []struct {
		name string
		p    pose.Pose[assembly.SaddleRailDir, assembly.DynamicWorld]
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
	// Adding +20 mm PostSetbackMm shifts RailPivot and all saddle landmarks by (-10*sqrt(3), -10) mm
	// while leaving ShaftTop unchanged.
	seatTubeTop := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: -30, Y: 30 * math.Sqrt(3)}, 120.0)
	basePost, err := assembly.PlaceSeatpost(seatTubeTop, workedFrame, workedPost(0.0))
	if err != nil {
		t.Fatalf("PlaceSeatpost base: %v", err)
	}
	setbackPost, err := assembly.PlaceSeatpost(seatTubeTop, workedFrame, workedPost(20.0))
	if err != nil {
		t.Fatalf("PlaceSeatpost setback: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 5.0
	baseSaddle, err := assembly.PlaceSaddle(basePost.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle base: %v", err)
	}
	setbackSaddle, err := assembly.PlaceSaddle(setbackPost.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle setback: %v", err)
	}

	assertNear(t, "ShaftTop unchanged X", setbackPost.ShaftTop.Origin().X, basePost.ShaftTop.Origin().X, 1e-12)
	assertNear(t, "ShaftTop unchanged Y", setbackPost.ShaftTop.Origin().Y, basePost.ShaftTop.Origin().Y, 1e-12)

	wantDX := -10.0 * math.Sqrt(3)
	wantDY := -10.0
	for _, pair := range []struct {
		name string
		a, b pose.Point[assembly.DynamicWorld]
	}{
		{"RailPivot", basePost.RailPivot.Origin(), setbackPost.RailPivot.Origin()},
		{"RailCenter", baseSaddle.RailCenter.Origin(), setbackSaddle.RailCenter.Origin()},
		{"BRP", baseSaddle.BRP.Origin(), setbackSaddle.BRP.Origin()},
		{"SitBones", baseSaddle.SitBones.Origin(), setbackSaddle.SitBones.Origin()},
	} {
		assertNear(t, pair.name+" setback dX", pair.b.X-pair.a.X, wantDX, 1e-9)
		assertNear(t, pair.name+" setback dY", pair.b.Y-pair.a.Y, wantDY, 1e-9)
	}

	// Increasing RailOffsetMm by +10 mm moves saddle landmarks rearward (-10 mm X) while leaving clamp fixed.
	moreOffsetSetup := saddleSetup
	moreOffsetSetup.RailOffsetMm = 15.0
	moreOffsetSaddle, err := assembly.PlaceSaddle(basePost.RailPivot, 30.0, moreOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle +15 offset: %v", err)
	}
	assertNear(t, "RailPivot fixed X", moreOffsetSaddle.RailPivot.Origin().X, baseSaddle.RailPivot.Origin().X, 1e-12)
	assertNear(t, "RailCenter shifted -10", moreOffsetSaddle.RailCenter.Origin().X-baseSaddle.RailCenter.Origin().X, -10.0, 1e-12)
	assertNear(t, "BRP shifted -10", moreOffsetSaddle.BRP.Origin().X-baseSaddle.BRP.Origin().X, -10.0, 1e-12)
	assertNear(t, "SitBones shifted -10", moreOffsetSaddle.SitBones.Origin().X-baseSaddle.SitBones.Origin().X, -10.0, 1e-12)

	// Negative RailOffsetMm (-15 mm, delta -20 mm from +5 mm) moves saddle landmarks forward (+20 mm X).
	negOffsetSetup := saddleSetup
	negOffsetSetup.RailOffsetMm = -15.0
	negOffsetSaddle, err := assembly.PlaceSaddle(basePost.RailPivot, 30.0, negOffsetSetup)
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

	clamp60 := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: -180, Y: 720}, 180.0-60.0)
	clamp75 := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: -180, Y: 720}, 180.0-75.25)

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
	// only on RailPivot.Origin() and world RailPitchDeg, never on STA heading.
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
	basisFork := assembly.ForkSetup{
		Spec: assembly.ForkSpec{StaticA2cMm: 495.0, RakeMm: 54.0, Steerer: assembly.SteererBasis},
	}
	postSetup := assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 180.576}
	saddleSetup := sqlab610DefaultSaddle()

	// Check the fixture's reference saddle height at the 75.25° STA attitude:
	// 490.0 mm seat tube + 219.424 mm exposed = 709.424 mm along seat axis, and the raised BRP is |BRP - BB| = 746.000 mm (Q92) in the unpitched reference attitude.
	refSeatAxis := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{}, pose.Heading[assembly.DynamicWorld](180.0-frame.SeatTubeAngleDeg))
	refSeatTubeTop := refSeatAxis.Advance(frame.SeatTubeLengthMm)
	refPost, err := assembly.PlaceSeatpost(refSeatTubeTop, frame, postSetup)
	if err != nil {
		t.Fatalf("PlaceSeatpost ref: %v", err)
	}
	refSaddle, err := assembly.PlaceSaddle(refPost.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle ref: %v", err)
	}
	assertNear(t, "BB to shaft top along seat axis", refSeatAxis.Origin().DistanceTo(refPost.ShaftTop.Origin()), 709.424, 1e-9)
	assertNear(t, "reference (unpitched attitude) |BRP - BB|", refSeatAxis.Origin().DistanceTo(refSaddle.BRP.Origin()), 746.000, 0.001)

	// Full leveled frameset + cockpits via PlaceBike.
	bikeSetup := assembly.BikeSetup{
		Frame:     frame,
		LowerCup:  assembly.LowerCupSetup{Standard: assembly.LowerCupBareHeadTube, StackMm: 0.0},
		Fork:      basisFork,
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

	assertNear(t, "leveled BB to post top along seat axis", bike.SeatAxisToPostTopMm(), 709.424, 1e-9)
	// The basis fork levels the frame at a slightly different pitch than the unpitched
	// reference attitude, so its radial BRP height differs from 746.000 mm.
	assertNear(t, "basis leveled |BRP - BB|", bike.SaddleHeightMm(), 746.008, 0.001)
	assertNear(t, "Setup preserved on BikePlacement", bike.Setup.Seatpost.Spec.RailClampLengthMm, 30.0, 1e-12)
	// The post shaft continues the seat axis from SeatTubeTop; the saddle shares the leveled-rail heading.
	assertNear(t, "seat tube top to shaft top", bike.Frameset.SeatTubeTop.Origin().DistanceTo(bike.Seatpost.ShaftTop.Origin()), postSetup.ExposedMm(), 1e-9)
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
	validPose := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: -120, Y: 470}, 104.75)
	validPost := assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 136.5}
	validFrame := fenrirSizeLFrame() // [100, 290] mm with the generic post
	with := func(f func(*assembly.SeatpostSetup)) assembly.SeatpostSetup {
		s := validPost
		f(&s)
		return s
	}

	postCases := []struct {
		name  string
		pose  pose.Pose[assembly.SeatTubeDir, assembly.DynamicWorld]
		setup assembly.SeatpostSetup
	}{
		{"NaN origin X", pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: math.NaN(), Y: 0}, 104.75), validPost},
		{"Inf heading", pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{}, pose.Heading[assembly.DynamicWorld](math.Inf(1))), validPost},
		{"negative insertion", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = -1 })},
		{"NaN insertion", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = math.NaN() })},
		{"insertion below minimum", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = 99.999 })},
		{"insertion above maximum", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = 290.001 })},
		{"negative setback", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.SetbackMm = -1 })},
		{"NaN setback", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.SetbackMm = math.NaN() })},
		{"zero clamp length", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.RailClampLengthMm = 0 })},
		{"negative clamp length", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.RailClampLengthMm = -10 })},
		{"exposure overflow", pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: -math.MaxFloat64, Y: 0}, 120), with(func(s *assembly.SeatpostSetup) { s.Spec.TotalLengthMm = math.MaxFloat64 })},
	}
	for _, tc := range postCases {
		t.Run("seatpost/"+tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceSeatpost(tc.pose, validFrame, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}

	validSaddle := sqlab610DefaultSaddle()
	saddleCases := []struct {
		name  string
		pose  pose.Pose[assembly.SeatTubeDir, assembly.DynamicWorld]
		setup assembly.SaddleSetup
	}{
		{"NaN origin Y", pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: 0, Y: math.NaN()}, 104.75), validSaddle},
		{"Inf heading", pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{}, pose.Heading[assembly.DynamicWorld](math.Inf(-1))), validSaddle},
		{"NaN RailCenterToBrpMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: math.NaN(), RailLengthMm: 60}},
		{"Inf RailCenterToSitBonesMm", validPose, assembly.SaddleSetup{RailCenterToSitBonesMm: math.Inf(1), RailLengthMm: 60}},
		{"NaN SitBonesElevationMm", validPose, assembly.SaddleSetup{SitBonesElevationMm: math.NaN(), RailLengthMm: 60}},
		{"zero RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: 0}},
		{"negative RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: -60}},
		{"NaN RailPitchDeg", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailPitchDeg: pose.Deg(math.NaN())}},
		{"Inf RailOffsetMm", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: math.Inf(1)}},
		{"rail offset overflow", pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: math.MaxFloat64, Y: 0}, 104.75), assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: -math.MaxFloat64}},
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
	clamp := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{X: -100, Y: 700}, 105.0)
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

// workedFrame declares a 110 mm insertion cap so the synthetic worked post
// below allows [20, 110] mm; PlaceSeatpost reads only the frame's insertion limits.
var workedFrame = assembly.FrameSetup{SeatTubeLengthMm: 60, MaxPostInsertionMm: 110}

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

// The raised landmarks rotate with the rails: at +10° pitch the 45 mm BRP
// height alone shifts BRP and SitBones by 45 mm turned 10° from vertical.
func TestPlaceSaddleRailToBrpHeightRotatesWithRailPitch(t *testing.T) {
	seatTubeTop := pose.NewPose[assembly.SeatTubeDir](pose.Point[assembly.DynamicWorld]{}, 120).Advance(60)
	post, err := assembly.PlaceSeatpost(seatTubeTop, workedFrame, workedPost(0))
	if err != nil {
		t.Fatal(err)
	}
	place := func(heightMm float64, pitch pose.Deg) assembly.SaddlePlacement {
		setup := sqlab610DefaultSaddle()
		setup.RailOffsetMm = 12
		setup.RailToBrpHeightMm = heightMm
		setup.RailPitchDeg = pitch
		saddle, err := assembly.PlaceSaddle(post.RailPivot, 30, setup)
		if err != nil {
			t.Fatal(err)
		}
		return saddle
	}
	raised, flat := place(45, 10), place(0, 10)
	wantShift := pose.Vec[assembly.DynamicWorld]{X: -7.814168, Y: 44.316349}
	for _, lm := range []struct {
		name         string
		raised, flat pose.Pose[assembly.SaddleRailDir, assembly.DynamicWorld]
	}{{"BRP", raised.BRP, flat.BRP}, {"SitBones", raised.SitBones, flat.SitBones}} {
		shift := lm.raised.Origin().Sub(lm.flat.Origin())
		assertNear(t, lm.name+" shift X", shift.X, wantShift.X, 1e-6)
		assertNear(t, lm.name+" shift Y", shift.Y, wantShift.Y, 1e-6)
	}
	// The rails themselves do not move with the height.
	assertNear(t, "RailCenter X", raised.RailCenter.Origin().X, flat.RailCenter.Origin().X, 1e-12)
	assertNear(t, "RailCenter Y", raised.RailCenter.Origin().Y, flat.RailCenter.Origin().Y, 1e-12)

	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		setup := sqlab610DefaultSaddle()
		setup.RailToBrpHeightMm = bad
		if _, err := assembly.PlaceSaddle(post.RailPivot, 30, setup); err == nil {
			t.Errorf("RailToBrpHeightMm %v: want error", bad)
		}
	}
}

// Q92: the fixture's 180.576 mm default insertion puts the raised BRP 746.000 mm
// from BB in the unpitched reference attitude. Installed forks level the frame
// differently, so the leveled radial height differs from the reference.
func TestQ92DefaultInsertionOnInstalledFork(t *testing.T) {
	bike, err := assembly.PlaceBike(fenrirFittingSetup())
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "shaft top distance from BB", bike.SeatAxisToPostTopMm(), 709.424, 1e-9)
	assertNear(t, "exposed length", bike.Setup.Seatpost.ExposedMm(), 219.424, 1e-9)
	assertNear(t, "installed Lithic |BRP - BB|", bike.SaddleHeightMm(), 745.619, 0.001)
	assertNear(t, "installed Lithic BRP.X", bike.Saddle.BRP.Origin().X, -155.065, 0.001)
	assertNear(t, "installed Lithic BRP.Y", bike.Saddle.BRP.Origin().Y, 729.317, 0.001)
}
