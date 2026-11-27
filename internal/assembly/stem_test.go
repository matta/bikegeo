package assembly_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/assembly"
	"github.com/matta/bikegeo/internal/pose"
)

func dist(a, b pose.Point) float64 {
	return a.DistanceTo(b)
}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.6f, want %.6f (tol %.6f)", name, got, want, tol)
	}
}

func steererTransform(x, y float64, headingDeg pose.Deg) pose.Transform {
	return pose.NewPose(pose.NewPoint(assembly.DynamicWorld, x, y), headingDeg).Anchor(assembly.SteererLocal)
}

func TestPlaceStemWorkedExample60DegHTA(t *testing.T) {
	// 60° HTA => steererUp heading is 180° - 60° = 120°.
	// Stack along steerer: 10 (cover) + 20 (spacers) + 20 (half of 40 clamp) = 50 mm.
	// At 120°: cos(120°) = -0.5, sin(120°) = sqrt(3)/2.
	// SteererClamp = (-25, 25*sqrt(3)).
	// Stem angle = 0° => turn -90° to 30°; advance 100 mm.
	// At 30°: cos(30°) = sqrt(3)/2, sin(30°) = 0.5.
	// BarClamp = (-25 + 50*sqrt(3), 25*sqrt(3) + 50).
	got, err := placeStem(steererTransform(0, 0, 120), assembly.StemSetup{
		UpperStackMm:  10,
		SpacersMm:     20,
		MaxSpacersMm:  30,
		ClampHeightMm: 40,
		LengthMm:      100,
		AngleDeg:      0,
	})
	if err != nil {
		t.Fatalf("PlaceStem() error = %v", err)
	}

	sqrt3 := math.Sqrt(3)
	assertNear(t, "SteererClamp.X", got.SteererClamp.Origin().X(), -25.0, 1e-9)
	assertNear(t, "SteererClamp.Y", got.SteererClamp.Origin().Y(), 25.0*sqrt3, 1e-9)
	assertNear(t, "BarClamp.X", got.BarClamp.Origin().X(), -25.0+50.0*sqrt3, 1e-9)
	assertNear(t, "BarClamp.Y", got.BarClamp.Origin().Y(), 25.0*sqrt3+50.0, 1e-9)
	assertNear(t, "BarClamp.HeadingDeg", float64(got.BarClamp.HeadingDeg()), 30.0, 1e-9)
}

func TestPlaceStemWorkedExampleNonzeroAngle(t *testing.T) {
	// 60° HTA => steererUp heading is 120°; SteererClamp at (-25, 25*sqrt(3)).
	// Perpendicular (-90°) gives 30°, then +30° stem angle gives 60°.
	// BarClamp = SteererClamp + 100*(cos 60°, sin 60°) = (25, 75*sqrt(3)).
	got, err := placeStem(steererTransform(0, 0, 120), assembly.StemSetup{
		UpperStackMm:  10,
		SpacersMm:     20,
		MaxSpacersMm:  30,
		ClampHeightMm: 40,
		LengthMm:      100,
		AngleDeg:      30,
	})
	if err != nil {
		t.Fatalf("PlaceStem() error = %v", err)
	}

	sqrt3 := math.Sqrt(3)
	assertNear(t, "BarClamp.X", got.BarClamp.Origin().X(), 25.0, 1e-9)
	assertNear(t, "BarClamp.Y", got.BarClamp.Origin().Y(), 75.0*sqrt3, 1e-9)
	assertNear(t, "BarClamp.HeadingDeg", float64(got.BarClamp.HeadingDeg()), 60.0, 1e-9)
}

func TestPlaceStemDimensions(t *testing.T) {
	// Default Milestone 1 front cockpit dimensions at HTA = 68.25°.
	setup := assembly.StemSetup{
		UpperStackMm:  11.0,
		SpacersMm:     20.0,
		MaxSpacersMm:  30,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
	}
	heading := pose.Deg(180.0 - 68.25)
	origin := pose.NewPoint(assembly.DynamicWorld, 428.0, 642.0)

	got, err := placeStem(pose.NewPose(origin, heading).Anchor(assembly.SteererLocal), setup)
	if err != nil {
		t.Fatalf("PlaceStem() error = %v", err)
	}

	assertNear(t, "stack distance", dist(origin, got.SteererClamp.Origin()), 11.0+20.0+20.0, 1e-9)
	assertNear(t, "stem length", dist(got.SteererClamp.Origin(), got.BarClamp.Origin()), 60.0, 1e-9)
	// Perpendicular to the steerer (heading - 90°), then the stem angle.
	assertNear(t, "BarClamp.Heading", float64(got.BarClamp.HeadingDeg()), float64(heading)-90.0+float64(setup.AngleDeg), 1e-12)
}

func TestPlaceStemValidation(t *testing.T) {
	validSteerer := steererTransform(0, 0, 111.75)
	validSetup := assembly.StemSetup{
		UpperStackMm:  11,
		SpacersMm:     20,
		MaxSpacersMm:  30,
		ClampHeightMm: 40,
		LengthMm:      60,
		AngleDeg:      -6,
	}

	cases := []struct {
		name    string
		steerer pose.Transform
		setup   assembly.StemSetup
	}{
		{"NaN origin X", steererTransform(math.NaN(), 0, 111.75), validSetup},
		{"Inf heading", steererTransform(0, 0, pose.Deg(math.Inf(1))), validSetup},
		{"negative cover", validSteerer, assembly.StemSetup{UpperStackMm: -1, SpacersMm: 20, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"negative spacers", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: -1, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"zero clamp height", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: 20, MaxSpacersMm: 30, ClampHeightMm: 0, LengthMm: 60, AngleDeg: -6}},
		{"zero length", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: 20, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 0, AngleDeg: -6}},
		{"spacers over max", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: 31, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"negative max spacers", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: 0, MaxSpacersMm: -1, ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"NaN max spacers", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: 0, MaxSpacersMm: math.NaN(), ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"NaN angle", validSteerer, assembly.StemSetup{UpperStackMm: 11, SpacersMm: 20, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60, AngleDeg: pose.Deg(math.NaN())}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := placeStem(tc.steerer, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestPlaceStemRejectsLowerHeadsetTransform(t *testing.T) {
	frameset, err := placeFrameset(
		fenrirSizeLFrame(),
		fenrirEc44Cup(),
		assembly.ForkSetup{Spec: assembly.ForkSpec{StaticA2cMm: 495, RakeMm: 54, Steerer: assembly.SteererTapered}},
		fenrirWheels(),
	)
	if err != nil {
		t.Fatalf("PlaceFrameset: %v", err)
	}
	setup := assembly.StemSetup{
		UpperStackMm:  11,
		SpacersMm:     20,
		MaxSpacersMm:  30,
		ClampHeightMm: 40,
		LengthMm:      60,
		AngleDeg:      -6,
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected PlaceStem(LowerHeadsetInWorld) to panic on CoordinateSystem mismatch")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "coordinate system mismatch") {
			t.Fatalf("panic = %v, want coordinate system mismatch", r)
		}
	}()
	_, _ = placeStem(frameset.LowerHeadsetInWorld(), setup)
}

func TestPlaceStemSpacersOverMaxIsInputError(t *testing.T) {
	_, err := placeStem(steererTransform(0, 0, 111.75), assembly.StemSetup{UpperStackMm: 11, SpacersMm: 31, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60})
	var inputErr *assembly.InputError
	if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputStemSpacers {
		t.Fatalf("err = %v, want InputStemSpacers", err)
	}
}

func TestPlaceStemSpacersExactlyAtMaxPlaces(t *testing.T) {
	if _, err := placeStem(steererTransform(0, 0, 111.75), assembly.StemSetup{UpperStackMm: 11, SpacersMm: 30, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60}); err != nil {
		t.Fatalf("PlaceStem at max spacers: %v", err)
	}
}

func TestPlaceStemRejectsMaxSpacersAboveCeiling(t *testing.T) {
	setup := assembly.StemSetup{UpperStackMm: assembly.DefaultUpperStackMm, SpacersMm: 0, MaxSpacersMm: assembly.DefaultMaxSpacersMm, ClampHeightMm: 40, LengthMm: 60}
	steerer := steererTransform(0, 0, 111.75)
	if _, err := placeStem(steerer, setup); err != nil {
		t.Fatalf("max spacers at the ceiling: %v", err)
	}
	setup.MaxSpacersMm = assembly.DefaultMaxSpacersMm + 1
	if _, err := placeStem(steerer, setup); err == nil {
		t.Fatal("configured max spacers above 30 mm was accepted")
	}
}

// Isolated SteererPort, Stem, and ClampedStem tests below construct no bicycle
// frameset or ambient world except where verifying world placement transforms.

func TestSteererPortIsolatedAndLowerHeadsetSafeguard(t *testing.T) {
	steererInFrame := pose.NewPose(pose.NewPoint(assembly.FrameLocal, 428, 642), 111.75).Anchor(assembly.SteererLocal)
	port, err := assembly.NewSteererPort(steererInFrame, assembly.DefaultUpperStackMm, assembly.DefaultMaxSpacersMm)
	if err != nil {
		t.Fatalf("NewSteererPort: %v", err)
	}
	if port.UpperStackMm() != assembly.DefaultUpperStackMm {
		t.Errorf("UpperStackMm = %v, want %v", port.UpperStackMm(), assembly.DefaultUpperStackMm)
	}
	if port.SteererInFrame().Src() != assembly.SteererLocal || port.SteererInFrame().Dst() != assembly.FrameLocal {
		t.Errorf("SteererInFrame = %s -> %s", port.SteererInFrame().Src(), port.SteererInFrame().Dst())
	}
	allowed := port.AllowedSpacers()
	if allowed.Min() != 0 || allowed.Max() != assembly.DefaultMaxSpacersMm {
		t.Errorf("AllowedSpacers = [%v, %v], want [0, %v]", allowed.Min(), allowed.Max(), assembly.DefaultMaxSpacersMm)
	}

	// Scalar validation errors.
	for _, tc := range []struct {
		name           string
		tf             pose.Transform
		upper, maxSpac float64
	}{
		{"NaN steerer origin", pose.NewTransform(assembly.SteererLocal, assembly.FrameLocal, pose.NewPoint(assembly.FrameLocal, math.NaN(), 0), 111.75), 11, 30},
		{"Inf steerer heading", pose.NewTransform(assembly.SteererLocal, assembly.FrameLocal, pose.Origin(assembly.FrameLocal), pose.Deg(math.Inf(1))), 11, 30},
		{"negative upper stack", steererInFrame, -1, 30},
		{"NaN upper stack", steererInFrame, math.NaN(), 30},
		{"negative max spacers", steererInFrame, 11, -1},
		{"NaN max spacers", steererInFrame, 11, math.NaN()},
		{"max spacers above ceiling", steererInFrame, 11, assembly.DefaultMaxSpacersMm + 0.1},
	} {
		if _, err := assembly.NewSteererPort(tc.tf, tc.upper, tc.maxSpac); err == nil {
			t.Errorf("%s: want error", tc.name)
		}
	}

	// Passing a LowerHeadsetLocal → FrameLocal transform must panic deterministically
	// with a coordinate system mismatch so lower-headset and upper-steerer mounts
	// cannot be confused in FrameLocal.
	t.Run("rejects LowerHeadsetLocal to FrameLocal", func(t *testing.T) {
		lowerInFrame := pose.NewPose(pose.NewPoint(assembly.FrameLocal, 460, 560), -68.25).Anchor(assembly.LowerHeadsetLocal)
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected NewSteererPort(LowerHeadsetLocal -> FrameLocal) to panic")
			}
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, "coordinate system mismatch") {
				t.Fatalf("panic = %v, want coordinate system mismatch", r)
			}
		}()
		_, _ = assembly.NewSteererPort(lowerInFrame, 11, 30)
	})

	assertPanics(t, "SteererLocal to DynamicWorld", func() {
		worldSteerer := pose.NewPose(pose.Origin(assembly.DynamicWorld), 111.75).Anchor(assembly.SteererLocal)
		_, _ = assembly.NewSteererPort(worldSteerer, 11, 30)
	})
	assertPanics(t, "zero Transform", func() {
		_, _ = assembly.NewSteererPort(pose.Transform{}, 11, 30)
	})
	var zeroPort assembly.SteererPort
	assertPanics(t, "zero SteererPort UpperStackMm", func() { _ = zeroPort.UpperStackMm() })
	assertPanics(t, "zero SteererPort AllowedSpacers", func() { _ = zeroPort.AllowedSpacers() })
}

func TestStemAndMountStemIsolated(t *testing.T) {
	stem, err := assembly.NewStem(40, 100, 30)
	if err != nil {
		t.Fatalf("NewStem: %v", err)
	}
	if stem.ClampHeightMm() != 40 || stem.LengthMm() != 100 || stem.AngleDeg() != 30 {
		t.Errorf("unexpected stem dimensions: clamp=%v len=%v angle=%v",
			stem.ClampHeightMm(), stem.LengthMm(), stem.AngleDeg())
	}

	var inputErr *assembly.InputError
	if _, err := assembly.NewStem(40, 0, 0); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputStemLength {
		t.Errorf("zero length err = %v, want InputStemLength", err)
	}
	if _, err := assembly.NewStem(40, 60, pose.Deg(math.NaN())); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputStemAngle {
		t.Errorf("NaN angle err = %v, want InputStemAngle", err)
	}
	if _, err := assembly.NewStem(0, 60, 0); err == nil {
		t.Error("zero clamp height: want error")
	}

	// 60° HTA (steerer heading 120° in FrameLocal at origin):
	// Stack along steerer = 10 + 20 + 20 = 50 mm => SteererClamp = (-25, 25*sqrt(3)).
	// Turn -90° + 30° => 60° heading, advance 100 mm => BarClamp = (25, 75*sqrt(3)).
	steererInFrame := pose.NewPose(pose.Origin(assembly.FrameLocal), 120).Anchor(assembly.SteererLocal)
	port, err := assembly.NewSteererPort(steererInFrame, 10, 30)
	if err != nil {
		t.Fatalf("NewSteererPort: %v", err)
	}

	if _, err := assembly.MountStem(port, 30.1, stem); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputStemSpacers {
		t.Errorf("spacers over max = %v, want InputStemSpacers", err)
	}
	if _, err := assembly.MountStem(port, -1, stem); !errors.As(err, &inputErr) || inputErr.Field != assembly.InputStemSpacers {
		t.Errorf("negative spacers = %v, want InputStemSpacers", err)
	}

	clamped, err := assembly.MountStem(port, 20, stem)
	if err != nil {
		t.Fatalf("MountStem: %v", err)
	}
	if clamped.Spacers().Field != assembly.InputStemSpacers || clamped.Spacers().Value != 20 || clamped.Spacers().MarginMm() != 10 {
		t.Errorf("Spacers BoundStatus = %+v", clamped.Spacers())
	}
	if clamped.UpperStackMm() != 10 || clamped.Stem() != stem {
		t.Errorf("unexpected ClampedStem metadata")
	}

	sqrt3 := math.Sqrt(3)
	inFrame := clamped.BarClampInFrame()
	if inFrame.ClampInParent.Src() != assembly.StemLocal || inFrame.ClampInParent.Dst() != assembly.FrameLocal {
		t.Errorf("BarClampInFrame systems = %s -> %s", inFrame.ClampInParent.Src(), inFrame.ClampInParent.Dst())
	}
	assertNear(t, "FrameLocal BarClamp.X", inFrame.ClampInParent.OriginInDst().X(), 25.0, 1e-9)
	assertNear(t, "FrameLocal BarClamp.Y", inFrame.ClampInParent.OriginInDst().Y(), 75.0*sqrt3, 1e-9)
	assertNear(t, "FrameLocal BarClamp.Heading", float64(inFrame.ClampInParent.HeadingInDst().Deg()), 60.0, 1e-9)

	// Placing into DynamicWorld (0° pitch) vs StaticWorld (+2.5° pitch) rotates rigidly.
	dynPlaced := clamped.Place(pose.NewTransform(assembly.FrameLocal, assembly.DynamicWorld, pose.Origin(assembly.DynamicWorld), 0))
	statPlaced := clamped.Place(pose.NewTransform(assembly.FrameLocal, assembly.StaticWorld, pose.Origin(assembly.StaticWorld), 2.5))

	assertNear(t, "dyn SteererClamp.X", dynPlaced.SteererClamp.Origin().X(), -25.0, 1e-9)
	assertNear(t, "dyn SteererClamp.Y", dynPlaced.SteererClamp.Origin().Y(), 25.0*sqrt3, 1e-9)
	assertNear(t, "dyn BarClamp.X", dynPlaced.BarClamp.Origin().X(), 25.0, 1e-9)
	assertNear(t, "dyn BarClamp.Y", dynPlaced.BarClamp.Origin().Y(), 75.0*sqrt3, 1e-9)
	assertNear(t, "dyn BarClampInWorld matches BarClamp X", dynPlaced.BarClampInWorld.OriginInDst().X(), dynPlaced.BarClamp.Origin().X(), 1e-12)
	assertNear(t, "dyn BarClampInWorld matches BarClamp Y", dynPlaced.BarClampInWorld.OriginInDst().Y(), dynPlaced.BarClamp.Origin().Y(), 1e-12)
	if dynPlaced.BarClampInWorld.Src() != assembly.StemLocal || dynPlaced.BarClampInWorld.Dst() != assembly.DynamicWorld {
		t.Errorf("dyn BarClampInWorld systems = %s -> %s", dynPlaced.BarClampInWorld.Src(), dynPlaced.BarClampInWorld.Dst())
	}
	assertNear(t, "stat BarClamp heading", float64(statPlaced.BarClamp.HeadingDeg()), 62.5, 1e-12)
	assertNear(t, "stat stem length preserved", dist(statPlaced.SteererClamp.Origin(), statPlaced.BarClamp.Origin()), 100.0, 1e-9)
	assertNear(t, "stat stack distance preserved", dist(statPlaced.HeadTubeTop.Origin(), statPlaced.SteererClamp.Origin()), 50.0, 1e-9)

	// Zero-value and coordinate-system mismatch panics.
	var zeroStem assembly.Stem
	assertPanics(t, "zero Stem LengthMm", func() { _ = zeroStem.LengthMm() })
	assertPanics(t, "MountStem zero Stem", func() { _, _ = assembly.MountStem(port, 10, zeroStem) })
	assertPanics(t, "MountStem zero SteererPort", func() { _, _ = assembly.MountStem(assembly.SteererPort{}, 10, stem) })
	assertPanics(t, "ClampedStem.Place wrong Src", func() {
		wrong := pose.NewTransform(assembly.SteererLocal, assembly.DynamicWorld, pose.Origin(assembly.DynamicWorld), 0)
		_ = clamped.Place(wrong)
	})
	assertPanics(t, "nil ClampedStem Place", func() {
		var nilStem *assembly.ClampedStem
		_ = nilStem.Place(pose.NewTransform(assembly.FrameLocal, assembly.DynamicWorld, pose.Origin(assembly.DynamicWorld), 0))
	})
}
