package assembly_test

import (
	"errors"
	"math"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/bounds"
	"github.com/matta/bikegeo/internal/pose"
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
	seatAxis := pose.NewPose(pose.Origin(bounds.DynamicWorld), 120.0)
	seatTubeTop := seatAxis.Advance(60.0)

	post, err := placeSeatpost(seatTubeTop, workedFrame, workedPost(0.0))
	if err != nil {
		t.Fatalf("PlaceSeatpost: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 12.0
	saddle, err := placeSaddle(post.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle: %v", err)
	}

	assertNear(t, "ShaftTop.X", post.ShaftTop.Origin().X(), -50.0, 1e-9)
	assertNear(t, "ShaftTop.Y", post.ShaftTop.Origin().Y(), 50.0*sqrt3, 1e-9)
	assertNear(t, "RailPivot.X", post.RailPivot.Origin().X(), -50.0, 1e-9)
	assertNear(t, "RailPivot.Y", post.RailPivot.Origin().Y(), 50.0*sqrt3, 1e-9)
	assertNear(t, "RailPivot.Heading", float64(post.RailPivot.HeadingDeg()), 120.0, 1e-9)

	assertNear(t, "RailCenter.X", saddle.RailCenter.Origin().X(), -62.0, 1e-9)
	assertNear(t, "RailCenter.Y", saddle.RailCenter.Origin().Y(), 50.0*sqrt3, 1e-9)
	assertNear(t, "BRP.X", saddle.BRP.Origin().X(), -30.0, 1e-9)
	assertNear(t, "BRP.Y", saddle.BRP.Origin().Y(), 50.0*sqrt3+45.0, 1e-9)
	assertNear(t, "SitBones.X", saddle.SitBones.Origin().X(), -110.0, 1e-9)
	assertNear(t, "SitBones.Y", saddle.SitBones.Origin().Y(), 50.0*sqrt3+55.0, 1e-9)

	for _, lm := range []struct {
		name string
		p    pose.Pose
	}{
		{"RailClamp", saddle.RailClamp},
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
	seatTubeTop := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, -30, 30*math.Sqrt(3)), 120.0)
	basePost, err := placeSeatpost(seatTubeTop, workedFrame, workedPost(0.0))
	if err != nil {
		t.Fatalf("PlaceSeatpost base: %v", err)
	}
	setbackPost, err := placeSeatpost(seatTubeTop, workedFrame, workedPost(20.0))
	if err != nil {
		t.Fatalf("PlaceSeatpost setback: %v", err)
	}

	saddleSetup := sqlab610DefaultSaddle()
	saddleSetup.RailOffsetMm = 5.0
	baseSaddle, err := placeSaddle(basePost.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle base: %v", err)
	}
	setbackSaddle, err := placeSaddle(setbackPost.RailPivot, 30.0, saddleSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle setback: %v", err)
	}

	assertNear(t, "ShaftTop unchanged X", setbackPost.ShaftTop.Origin().X(), basePost.ShaftTop.Origin().X(), 1e-12)
	assertNear(t, "ShaftTop unchanged Y", setbackPost.ShaftTop.Origin().Y(), basePost.ShaftTop.Origin().Y(), 1e-12)

	wantDX := -10.0 * math.Sqrt(3)
	wantDY := -10.0
	for _, pair := range []struct {
		name string
		a, b pose.Point
	}{
		{"RailPivot", basePost.RailPivot.Origin(), setbackPost.RailPivot.Origin()},
		{"RailCenter", baseSaddle.RailCenter.Origin(), setbackSaddle.RailCenter.Origin()},
		{"BRP", baseSaddle.BRP.Origin(), setbackSaddle.BRP.Origin()},
		{"SitBones", baseSaddle.SitBones.Origin(), setbackSaddle.SitBones.Origin()},
	} {
		assertNear(t, pair.name+" setback dX", pair.b.X()-pair.a.X(), wantDX, 1e-9)
		assertNear(t, pair.name+" setback dY", pair.b.Y()-pair.a.Y(), wantDY, 1e-9)
	}

	// Increasing RailOffsetMm by +10 mm moves saddle landmarks rearward (-10 mm X) while leaving clamp fixed.
	moreOffsetSetup := saddleSetup
	moreOffsetSetup.RailOffsetMm = 15.0
	moreOffsetSaddle, err := placeSaddle(basePost.RailPivot, 30.0, moreOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle +15 offset: %v", err)
	}
	assertNear(t, "RailPivot fixed X", moreOffsetSaddle.RailPivot.Origin().X(), baseSaddle.RailPivot.Origin().X(), 1e-12)
	assertNear(t, "RailCenter shifted -10", moreOffsetSaddle.RailCenter.Origin().X()-baseSaddle.RailCenter.Origin().X(), -10.0, 1e-12)
	assertNear(t, "BRP shifted -10", moreOffsetSaddle.BRP.Origin().X()-baseSaddle.BRP.Origin().X(), -10.0, 1e-12)
	assertNear(t, "SitBones shifted -10", moreOffsetSaddle.SitBones.Origin().X()-baseSaddle.SitBones.Origin().X(), -10.0, 1e-12)

	// Negative RailOffsetMm (-15 mm, delta -20 mm from +5 mm) moves saddle landmarks forward (+20 mm X).
	negOffsetSetup := saddleSetup
	negOffsetSetup.RailOffsetMm = -15.0
	negOffsetSaddle, err := placeSaddle(basePost.RailPivot, 30.0, negOffsetSetup)
	if err != nil {
		t.Fatalf("PlaceSaddle -15 offset: %v", err)
	}
	assertNear(t, "RailCenter shifted +20", negOffsetSaddle.RailCenter.Origin().X()-baseSaddle.RailCenter.Origin().X(), 20.0, 1e-12)
	assertNear(t, "BRP shifted +20", negOffsetSaddle.BRP.Origin().X()-baseSaddle.BRP.Origin().X(), 20.0, 1e-12)
}

func TestPlaceSaddleWorldRelativePitch(t *testing.T) {
	setup := sqlab610DefaultSaddle()
	setup.RailPitchDeg = 10.0 // Positive pitch raises the nose CCW from world horizontal.
	setup.RailOffsetMm = 5.0

	clamp60 := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, -180, 720), 180.0-60.0)
	clamp75 := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, -180, 720), 180.0-75.25)

	s60, err := placeSaddle(clamp60, 30.0, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle 60°: %v", err)
	}
	s75, err := placeSaddle(clamp75, 30.0, setup)
	if err != nil {
		t.Fatalf("PlaceSaddle 75.25°: %v", err)
	}

	if s60.BRP.Origin().Y() <= s60.RailCenter.Origin().Y() {
		t.Fatalf("positive rail pitch must raise BRP above RailCenter: BRP.Y=%.4f, RailCenter.Y=%.4f",
			s60.BRP.Origin().Y(), s60.RailCenter.Origin().Y())
	}

	// Because Pose.Face sets an absolute world heading, saddle landmarks depend
	// only on RailPivot.Origin() and world RailPitchDeg, never on STA heading.
	assertNear(t, "BRP.X STA-independent", s60.BRP.Origin().X(), s75.BRP.Origin().X(), 1e-12)
	assertNear(t, "BRP.Y STA-independent", s60.BRP.Origin().Y(), s75.BRP.Origin().Y(), 1e-12)
	assertNear(t, "SitBones.X STA-independent", s60.SitBones.Origin().X(), s75.SitBones.Origin().X(), 1e-12)
	assertNear(t, "SitBones.Y STA-independent", s60.SitBones.Origin().Y(), s75.SitBones.Origin().Y(), 1e-12)
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
	// 490.0 mm seat tube + 219.424 mm exposed = 709.424 mm along seat axis, and the raised BRP is |BRP - BB| = 746.000 mm in the unpitched reference attitude.
	refSeatAxis := pose.NewPose(pose.Origin(bounds.DynamicWorld), 180.0-frame.SeatTubeAngleDeg)
	refSeatTubeTop := refSeatAxis.Advance(frame.SeatTubeLengthMm)
	refPost, err := placeSeatpost(refSeatTubeTop, frame, postSetup)
	if err != nil {
		t.Fatalf("PlaceSeatpost ref: %v", err)
	}
	refSaddle, err := placeSaddle(refPost.RailPivot, 30.0, saddleSetup)
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
	bike, err := placeBike(bikeSetup)
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
	assertNear(t, "RailClamp.Heading", float64(bike.Saddle.RailClamp.HeadingDeg()), float64(saddleSetup.RailPitchDeg), 1e-12)

	badSeatpost := bikeSetup
	badSeatpost.Seatpost.InsertionMm = -1
	if _, err := placeBike(badSeatpost); err == nil {
		t.Fatal("PlaceBike expected error for negative seatpost extension")
	}

	badSaddle := bikeSetup
	badSaddle.Saddle.RailLengthMm = 0
	if _, err := placeBike(badSaddle); err == nil {
		t.Fatal("PlaceBike expected error for zero saddle rail length")
	}
}

func TestPlaceSeatpostAndSaddleValidation(t *testing.T) {
	validPose := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, -120, 470), 104.75)
	validPost := assembly.SeatpostSetup{Spec: genericPostSpec(0), InsertionMm: 136.5}
	validFrame := fenrirSizeLFrame() // [100, 290] mm with the generic post
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
		{"NaN origin X", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, math.NaN(), 0), 104.75), validPost},
		{"Inf heading", pose.NewPose(pose.Origin(bounds.DynamicWorld), pose.Deg(math.Inf(1))), validPost},
		{"negative insertion", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = -1 })},
		{"NaN insertion", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = math.NaN() })},
		{"insertion below minimum", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = 99.999 })},
		{"insertion above maximum", validPose, with(func(s *assembly.SeatpostSetup) { s.InsertionMm = 290.001 })},
		{"negative setback", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.SetbackMm = -1 })},
		{"NaN setback", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.SetbackMm = math.NaN() })},
		{"zero clamp length", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.RailClampLengthMm = 0 })},
		{"negative clamp length", validPose, with(func(s *assembly.SeatpostSetup) { s.Spec.RailClampLengthMm = -10 })},
		{"exposure overflow", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, -math.MaxFloat64, 0), 120), with(func(s *assembly.SeatpostSetup) { s.Spec.TotalLengthMm = math.MaxFloat64 })},
	}
	for _, tc := range postCases {
		t.Run("seatpost/"+tc.name, func(t *testing.T) {
			if _, err := placeSeatpost(tc.pose, validFrame, tc.setup); err == nil {
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
		{"NaN origin Y", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, 0, math.NaN()), 104.75), validSaddle},
		{"Inf heading", pose.NewPose(pose.Origin(bounds.DynamicWorld), pose.Deg(math.Inf(-1))), validSaddle},
		{"NaN RailCenterToBrpMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: math.NaN(), RailLengthMm: 60}},
		{"Inf RailCenterToSitBonesMm", validPose, assembly.SaddleSetup{RailCenterToSitBonesMm: math.Inf(1), RailLengthMm: 60}},
		{"NaN SitBonesElevationMm", validPose, assembly.SaddleSetup{SitBonesElevationMm: math.NaN(), RailLengthMm: 60}},
		{"zero RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: 0}},
		{"negative RailLengthMm", validPose, assembly.SaddleSetup{RailCenterToBrpMm: 32, RailLengthMm: -60}},
		{"NaN RailPitchDeg", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailPitchDeg: pose.Deg(math.NaN())}},
		{"Inf RailOffsetMm", validPose, assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: math.Inf(1)}},
		{"rail offset overflow", pose.NewPose(pose.NewPoint(bounds.DynamicWorld, math.MaxFloat64, 0), 104.75), assembly.SaddleSetup{RailLengthMm: 60, RailOffsetMm: -math.MaxFloat64}},
	}
	for _, tc := range saddleCases {
		t.Run("saddle/"+tc.name, func(t *testing.T) {
			if _, err := placeSaddle(tc.pose, 30.0, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestPlaceSaddleRailOffsetRange(t *testing.T) {
	clamp := pose.NewPose(pose.NewPoint(bounds.DynamicWorld, -100, 700), 105.0)
	// 60 mm rails with a 30 mm clamp head allow ±15 mm.
	for _, offset := range []float64{0, 14.999, 15, -15} {
		setup := sqlab610DefaultSaddle()
		setup.RailOffsetMm = offset
		if _, err := placeSaddle(clamp, 30.0, setup); err != nil {
			t.Errorf("offset %v: %v", offset, err)
		}
	}
	for _, offset := range []float64{15.001, -15.001, 20} {
		setup := sqlab610DefaultSaddle()
		setup.RailOffsetMm = offset
		_, err := placeSaddle(clamp, 30.0, setup)
		var inputErr *bounds.InputError
		if !errors.As(err, &inputErr) || inputErr.Field != bounds.InputSaddleRailOffset {
			t.Errorf("offset %v: err = %v, want InputSaddleRailOffset", offset, err)
		}
	}
	if _, err := placeSaddle(clamp, 70.0, sqlab610DefaultSaddle()); err == nil {
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
	seatTubeTop := pose.NewPose(pose.Origin(bounds.DynamicWorld), 120).Advance(60)
	post, err := placeSeatpost(seatTubeTop, workedFrame, workedPost(0))
	if err != nil {
		t.Fatal(err)
	}
	place := func(heightMm float64, pitch pose.Deg) assembly.SaddlePlacement {
		setup := sqlab610DefaultSaddle()
		setup.RailOffsetMm = 12
		setup.RailToBrpHeightMm = heightMm
		setup.RailPitchDeg = pitch
		saddle, err := placeSaddle(post.RailPivot, 30, setup)
		if err != nil {
			t.Fatal(err)
		}
		return saddle
	}
	raised, flat := place(45, 10), place(0, 10)
	wantShift := pose.NewVector(bounds.DynamicWorld, -7.814168, 44.316349)
	for _, lm := range []struct {
		name         string
		raised, flat pose.Pose
	}{{"BRP", raised.BRP, flat.BRP}, {"SitBones", raised.SitBones, flat.SitBones}} {
		shift := lm.raised.Origin().Sub(lm.flat.Origin())
		assertNear(t, lm.name+" shift X", shift.X(), wantShift.X(), 1e-6)
		assertNear(t, lm.name+" shift Y", shift.Y(), wantShift.Y(), 1e-6)
	}
	// The rails themselves do not move with the height.
	assertNear(t, "RailCenter X", raised.RailCenter.Origin().X(), flat.RailCenter.Origin().X(), 1e-12)
	assertNear(t, "RailCenter Y", raised.RailCenter.Origin().Y(), flat.RailCenter.Origin().Y(), 1e-12)

	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		setup := sqlab610DefaultSaddle()
		setup.RailToBrpHeightMm = bad
		if _, err := placeSaddle(post.RailPivot, 30, setup); err == nil {
			t.Errorf("RailToBrpHeightMm %v: want error", bad)
		}
	}
}

// the fixture's 180.576 mm default insertion puts the raised BRP 746.000 mm
// from BB in the unpitched reference attitude. Installed forks level the frame
// differently, so the leveled radial height differs from the reference.
func TestQ92DefaultInsertionOnInstalledFork(t *testing.T) {
	bike, err := placeBike(fenrirFittingSetup())
	if err != nil {
		t.Fatal(err)
	}
	assertNear(t, "shaft top distance from BB", bike.SeatAxisToPostTopMm(), 709.424, 1e-9)
	assertNear(t, "exposed length", bike.Setup.Seatpost.ExposedMm(), 219.424, 1e-9)
	assertNear(t, "installed Lithic |BRP - BB|", bike.SaddleHeightMm(), 745.619, 0.001)
	assertNear(t, "installed Lithic BRP.X", bike.Saddle.BRP.Origin().X(), -155.065, 0.001)
	assertNear(t, "installed Lithic BRP.Y", bike.Saddle.BRP.Origin().Y(), 729.317, 0.001)
}

func TestPlaceSaddleRejectsLocalPose(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("PlaceSaddle accepted a SteererLocal pose; expected panic")
		}
	}()
	_, _ = placeSaddle(pose.OriginPose(assembly.SteererLocal), 30, sqlab610DefaultSaddle())
}

// Isolated Saddle tests below construct no bicycle frame or ambient world
// except where explicitly verifying static-vs-dynamic rigid clamping.

func sqlab610Dims() assembly.SaddleDimensions {
	return assembly.SaddleDimensions{
		RailCenterToBrpMm:      32.0,
		RailCenterToSitBonesMm: -48.0,
		RailToBrpHeightMm:      45.0,
		SitBonesElevationMm:    10.0,
		RailLengthMm:           60.0,
	}
}

func TestSaddleIsolated(t *testing.T) {
	dims := sqlab610Dims()
	saddle, err := assembly.NewSaddle(dims)
	if err != nil {
		t.Fatalf("NewSaddle: %v", err)
	}
	if saddle.RailLengthMm() != 60.0 || saddle.Dimensions() != dims {
		t.Errorf("unexpected saddle metadata: railLen=%v dims=%+v", saddle.RailLengthMm(), saddle.Dimensions())
	}
	brp := saddle.BRP()
	sit := saddle.SIT()
	if brp.CoordinateSystem() != assembly.SaddleLocal || sit.CoordinateSystem() != assembly.SaddleLocal {
		t.Errorf("landmark systems = %s, %s; want %s", brp.CoordinateSystem(), sit.CoordinateSystem(), assembly.SaddleLocal)
	}
	assertNear(t, "BRP.X", brp.X(), 32.0, 1e-12)
	assertNear(t, "BRP.Y", brp.Y(), 45.0, 1e-12)
	assertNear(t, "SIT.X", sit.X(), -48.0, 1e-12)
	assertNear(t, "SIT.Y", sit.Y(), 55.0, 1e-12)

	for _, tc := range []struct {
		name string
		mut  func(*assembly.SaddleDimensions)
	}{
		{"NaN BRP X", func(d *assembly.SaddleDimensions) { d.RailCenterToBrpMm = math.NaN() }},
		{"Inf SIT X", func(d *assembly.SaddleDimensions) { d.RailCenterToSitBonesMm = math.Inf(1) }},
		{"negative BRP height", func(d *assembly.SaddleDimensions) { d.RailToBrpHeightMm = -0.1 }},
		{"NaN BRP height", func(d *assembly.SaddleDimensions) { d.RailToBrpHeightMm = math.NaN() }},
		{"Inf SIT elevation", func(d *assembly.SaddleDimensions) { d.SitBonesElevationMm = math.Inf(-1) }},
		{"SIT height overflow", func(d *assembly.SaddleDimensions) {
			d.RailToBrpHeightMm = math.MaxFloat64
			d.SitBonesElevationMm = math.MaxFloat64
		}},
		{"zero rail length", func(d *assembly.SaddleDimensions) { d.RailLengthMm = 0 }},
		{"negative rail length", func(d *assembly.SaddleDimensions) { d.RailLengthMm = -10 }},
	} {
		d := dims
		tc.mut(&d)
		if _, err := assembly.NewSaddle(d); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	var zero assembly.Saddle
	assertPanics(t, "zero Saddle BRP", func() { _ = zero.BRP() })
	assertPanics(t, "zero Saddle SIT", func() { _ = zero.SIT() })
	assertPanics(t, "zero Saddle RailLengthMm", func() { _ = zero.RailLengthMm() })
	assertPanics(t, "zero Saddle Dimensions", func() { _ = zero.Dimensions() })
}

func TestMateSaddleAndClampedSaddleStaticVsDynamic(t *testing.T) {
	saddle, err := assembly.NewSaddle(sqlab610Dims())
	if err != nil {
		t.Fatalf("NewSaddle: %v", err)
	}

	// Mate onto a standalone LinkagePost cradle in SeatpostLocal (no frame/world).
	sag := pose.NewVector(assembly.SeatpostLocal, -6, 5)
	post, err := assembly.NewLinkagePost(350, 100, 250, 7, 30, sag)
	if err != nil {
		t.Fatalf("NewLinkagePost: %v", err)
	}
	dof, err := assembly.MateSaddle(post.RailClamp(bounds.Dynamic), saddle)
	if err != nil {
		t.Fatalf("MateSaddle: %v", err)
	}
	allowed := dof.AllowedRailOffset()
	assertNear(t, "allowed min", allowed.Min(), -15.0, 1e-12)
	assertNear(t, "allowed max", allowed.Max(), 15.0, 1e-12)
	if allowed.MinSource() != "saddle rail forward limit" || allowed.MaxSource() != "saddle rail rearward limit" {
		t.Errorf("unexpected rail bound sources: %q, %q", allowed.MinSource(), allowed.MaxSource())
	}

	// Clamp wider than rails or non-positive is rejected.
	wideClamp := post.RailClamp(bounds.Static)
	wideClamp.ClampLengthMm = 65
	if _, err := assembly.MateSaddle(wideClamp, saddle); err == nil {
		t.Error("clamp wider than rails: want error")
	}
	zeroClamp := post.RailClamp(bounds.Static)
	zeroClamp.ClampLengthMm = 0
	if _, err := assembly.MateSaddle(zeroClamp, saddle); err == nil {
		t.Error("zero clamp length: want error")
	}
	nanClamp := assembly.RailClampPort{
		CradleInParent: pose.NewTransform(assembly.SeatpostCradle, assembly.SeatpostLocal, pose.NewPoint(assembly.SeatpostLocal, math.NaN(), 0), 0),
		ClampLengthMm:  30,
	}
	if _, err := assembly.MateSaddle(nanClamp, saddle); err == nil {
		t.Error("non-finite clamp transform: want error")
	}

	// Out-of-bounds rail offset and non-finite rail turn return field-attributed InputErrors.
	var inputErr *bounds.InputError
	if _, err := dof.Clamp(15.1, -100); !errors.As(err, &inputErr) || inputErr.Field != bounds.InputSaddleRailOffset {
		t.Errorf("rail offset > max = %v, want InputSaddleRailOffset", err)
	}
	if _, err := dof.Clamp(0, pose.Deg(math.NaN())); !errors.As(err, &inputErr) || inputErr.Field != bounds.InputSaddleRailPitch {
		t.Errorf("NaN rail turn = %v, want InputSaddleRailPitch", err)
	}

	// Assemble a Seatpost + ClampedSaddle in FrameLocal, clamp the rails to +5°
	// in DynamicWorld (frame pitch -2.0°), then unload that exact ClampedSaddle
	// to StaticWorld (frame pitch -0.5°, +1.5° nose-up pitch change).
	collarInFrame := pose.NewPose(pose.NewPoint(assembly.FrameLocal, -120, 470), 105).Anchor(assembly.SeatTubeCollar)
	frameRange, _ := bounds.NewBoundedRange(100, 290, "frame min", "frame max")
	postDoF, err := assembly.MateSeatpost(assembly.SeatTubeSocket{CollarInFrame: collarInFrame, AllowedInsertion: frameRange}, post)
	if err != nil {
		t.Fatalf("MateSeatpost: %v", err)
	}
	clampedPost, err := postDoF.Clamp(150)
	if err != nil {
		t.Fatalf("postDoF.Clamp: %v", err)
	}

	frameToDynamic := pose.NewTransform(assembly.FrameLocal, bounds.DynamicWorld, pose.Origin(bounds.DynamicWorld), -2.0)
	frameToStatic := pose.NewTransform(assembly.FrameLocal, bounds.StaticWorld, pose.Origin(bounds.StaticWorld), -0.5)
	dynPost := clampedPost.PlaceInWorld(frameToDynamic)
	statPost := clampedPost.PlaceInWorld(frameToStatic)

	railTurn := assembly.RailTurnForWorldPitch(dynPost.CradleInWorld.HeadingInDst(), 5.0)
	clampedSaddle, err := dof.Clamp(10.0, railTurn)
	if err != nil {
		t.Fatalf("dof.Clamp: %v", err)
	}
	if clampedSaddle.RailOffset().Value != 10.0 || clampedSaddle.RailOffset().MarginMm() != 5.0 {
		t.Errorf("RailOffset = %+v, want Value=10 Margin=5", clampedSaddle.RailOffset())
	}

	dynSaddle := clampedSaddle.Place(dynPost.CradleInWorld)
	statSaddle := clampedSaddle.Place(statPost.CradleInWorld)

	// In DynamicWorld, every saddle landmark sits at exact +5.0° world pitch.
	assertNear(t, "dynamic RailClamp heading", float64(dynSaddle.RailClamp.HeadingDeg()), 5.0, 1e-12)
	assertNear(t, "dynamic BRP heading", float64(dynSaddle.BRP.HeadingDeg()), 5.0, 1e-12)
	assertNear(t, "dynamic SitBones heading", float64(dynSaddle.SitBones.HeadingDeg()), 5.0, 1e-12)

	// In StaticWorld (+1.5° pitch delta), the clamped saddle rotates rigidly to +6.5° (no gyroscope).
	assertNear(t, "static RailClamp heading", float64(statSaddle.RailClamp.HeadingDeg()), 6.5, 1e-12)
	assertNear(t, "static BRP heading", float64(statSaddle.BRP.HeadingDeg()), 6.5, 1e-12)
	assertNear(t, "static SitBones heading", float64(statSaddle.SitBones.HeadingDeg()), 6.5, 1e-12)

	// Rigid distances inside the saddle are preserved across Static and Dynamic states.
	assertNear(t, "RailPivot→RailCenter static", statSaddle.RailPivot.Origin().DistanceTo(statSaddle.RailCenter.Origin()), 10.0, 1e-12)
	assertNear(t, "RailCenter→BRP static", statSaddle.RailCenter.Origin().DistanceTo(statSaddle.BRP.Origin()), math.Hypot(32, 45), 1e-12)
	assertNear(t, "BRP→SitBones static vs dynamic",
		statSaddle.BRP.Origin().DistanceTo(statSaddle.SitBones.Origin()),
		dynSaddle.BRP.Origin().DistanceTo(dynSaddle.SitBones.Origin()), 1e-12)

	// Deterministic panics on coordinate-system mismatches and zero values.
	assertPanics(t, "MateSaddle zero Saddle", func() {
		_, _ = assembly.MateSaddle(post.RailClamp(bounds.Static), assembly.Saddle{})
	})
	assertPanics(t, "MateSaddle zero Transform", func() {
		_, _ = assembly.MateSaddle(assembly.RailClampPort{ClampLengthMm: 30}, saddle)
	})
	assertPanics(t, "MateSaddle wrong Src", func() {
		bad := pose.NewTransform(assembly.SteererLocal, assembly.FrameLocal, pose.Origin(assembly.FrameLocal), 0)
		_, _ = assembly.MateSaddle(assembly.RailClampPort{CradleInParent: bad, ClampLengthMm: 30}, saddle)
	})
	assertPanics(t, "RailTurnForWorldPitch FrameLocal", func() {
		_ = assembly.RailTurnForWorldPitch(pose.NewHeading(assembly.FrameLocal, 105), 0)
	})
	assertPanics(t, "RailTurnForWorldPitch UnpitchedChart", func() {
		_ = assembly.RailTurnForWorldPitch(pose.NewHeading(bounds.UnpitchedChart, 105), 0)
	})
	assertPanics(t, "RailTurnForWorldPitch zero Heading", func() {
		_ = assembly.RailTurnForWorldPitch(pose.Heading{}, 0)
	})
	assertPanics(t, "ClampedSaddle.Place wrong Src", func() {
		wrong := pose.NewTransform(assembly.SteererLocal, bounds.DynamicWorld, pose.Origin(bounds.DynamicWorld), 0)
		_ = clampedSaddle.Place(wrong)
	})
	assertPanics(t, "ClampedSaddle.Place zero transform", func() {
		_ = clampedSaddle.Place(pose.Transform{})
	})
	assertPanics(t, "nil ClampedSaddle Place", func() {
		var nilSaddle *assembly.ClampedSaddle
		_ = nilSaddle.Place(dynPost.CradleInWorld)
	})
}
