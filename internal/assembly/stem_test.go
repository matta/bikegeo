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
	return pose.NewPose(pose.Pt(assembly.DynamicWorld, x, y), headingDeg).Anchor(assembly.SteererLocal)
}

func TestPlaceStemWorkedExample60DegHTA(t *testing.T) {
	// 60° HTA => steererUp heading is 180° - 60° = 120°.
	// Stack along steerer: 10 (cover) + 20 (spacers) + 20 (half of 40 clamp) = 50 mm.
	// At 120°: cos(120°) = -0.5, sin(120°) = sqrt(3)/2.
	// SteererClamp = (-25, 25*sqrt(3)).
	// Stem angle = 0° => turn -90° to 30°; advance 100 mm.
	// At 30°: cos(30°) = sqrt(3)/2, sin(30°) = 0.5.
	// BarClamp = (-25 + 50*sqrt(3), 25*sqrt(3) + 50).
	got, err := assembly.PlaceStem(steererTransform(0, 0, 120), assembly.StemSetup{
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
	got, err := assembly.PlaceStem(steererTransform(0, 0, 120), assembly.StemSetup{
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
	origin := pose.Pt(assembly.DynamicWorld, 428.0, 642.0)

	got, err := assembly.PlaceStem(pose.NewPose(origin, heading).Anchor(assembly.SteererLocal), setup)
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
			if _, err := assembly.PlaceStem(tc.steerer, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestPlaceStemRejectsLowerHeadsetTransform(t *testing.T) {
	frameset, err := assembly.PlaceFrameset(
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
			t.Fatal("expected PlaceStem(LowerHeadsetInWorld) to panic on FrameID mismatch")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "coordinate frame mismatch") {
			t.Fatalf("panic = %v, want coordinate frame mismatch", r)
		}
	}()
	_, _ = assembly.PlaceStem(frameset.LowerHeadsetInWorld(), setup)
}

func TestPlaceStemSpacersOverMaxIsInputError(t *testing.T) {
	_, err := assembly.PlaceStem(steererTransform(0, 0, 111.75), assembly.StemSetup{UpperStackMm: 11, SpacersMm: 31, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60})
	var inputErr *assembly.InputError
	if !errors.As(err, &inputErr) || inputErr.Field != assembly.InputStemSpacers {
		t.Fatalf("err = %v, want InputStemSpacers", err)
	}
}

func TestPlaceStemSpacersExactlyAtMaxPlaces(t *testing.T) {
	if _, err := assembly.PlaceStem(steererTransform(0, 0, 111.75), assembly.StemSetup{UpperStackMm: 11, SpacersMm: 30, MaxSpacersMm: 30, ClampHeightMm: 40, LengthMm: 60}); err != nil {
		t.Fatalf("PlaceStem at max spacers: %v", err)
	}
}

func TestPlaceStemRejectsMaxSpacersAboveCeiling(t *testing.T) {
	setup := assembly.StemSetup{UpperStackMm: assembly.DefaultUpperStackMm, SpacersMm: 0, MaxSpacersMm: assembly.DefaultMaxSpacersMm, ClampHeightMm: 40, LengthMm: 60}
	steerer := steererTransform(0, 0, 111.75)
	if _, err := assembly.PlaceStem(steerer, setup); err != nil {
		t.Fatalf("max spacers at the ceiling: %v", err)
	}
	setup.MaxSpacersMm = assembly.DefaultMaxSpacersMm + 1
	if _, err := assembly.PlaceStem(steerer, setup); err == nil {
		t.Fatal("configured max spacers above 30 mm was accepted")
	}
}
