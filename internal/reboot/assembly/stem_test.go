package assembly_test

import (
	"math"
	"testing"

	"bikegeo/internal/reboot/assembly"
	"bikegeo/internal/reboot/pose"
)

func dist(a, b pose.Point) float64 {
	return math.Hypot(b.X-a.X, b.Y-a.Y)
}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.6f, want %.6f (tol %.6f)", name, got, want, tol)
	}
}

func TestPlaceStemWorkedExample60DegHTA(t *testing.T) {
	// 60° HTA => steererUp heading is 180° - 60° = 120°.
	// Stack along steerer: 10 (cover) + 20 (spacers) + 20 (half of 40 clamp) = 50 mm.
	// At 120°: cos(120°) = -0.5, sin(120°) = sqrt(3)/2.
	// SteererClamp = (-25, 25*sqrt(3)).
	// Stem angle = 0° => turn -90° to 30°; advance 100 mm.
	// At 30°: cos(30°) = sqrt(3)/2, sin(30°) = 0.5.
	// BarClamp = (-25 + 50*sqrt(3), 25*sqrt(3) + 50).
	tr := pose.NewTrace()
	steererUp := pose.NewPose(pose.Point{X: 0, Y: 0}, 120, tr)
	got, err := assembly.PlaceStem(steererUp, assembly.StemSetup{
		UpperCoverMm:  10,
		SpacersMm:     20,
		ClampHeightMm: 40,
		LengthMm:      100,
		AngleDeg:      0,
	})
	if err != nil {
		t.Fatalf("PlaceStem() error = %v", err)
	}
	tr.Freeze()

	sqrt3 := math.Sqrt(3)
	assertNear(t, "SteererClamp.X", got.SteererClamp.Origin().X, -25.0, 1e-9)
	assertNear(t, "SteererClamp.Y", got.SteererClamp.Origin().Y, 25.0*sqrt3, 1e-9)
	assertNear(t, "BarClamp.X", got.BarClamp.Origin().X, -25.0+50.0*sqrt3, 1e-9)
	assertNear(t, "BarClamp.Y", got.BarClamp.Origin().Y, 25.0*sqrt3+50.0, 1e-9)
	assertNear(t, "BarClamp.HeadingDeg", float64(got.BarClamp.HeadingDeg()), 30.0, 1e-9)

	if steps := tr.Steps(); len(steps) != 6 {
		t.Fatalf("len(trace.Steps()) = %d, want 6", len(steps))
	}
}

func TestPlaceStemWorkedExampleNonzeroAngle(t *testing.T) {
	// 60° HTA => steererUp heading is 120°; SteererClamp at (-25, 25*sqrt(3)).
	// Perpendicular (-90°) gives 30°, then +30° stem angle gives 60°.
	// BarClamp = SteererClamp + 100*(cos 60°, sin 60°) = (25, 75*sqrt(3)).
	steererUp := pose.NewPose(pose.Point{X: 0, Y: 0}, 120, nil)
	got, err := assembly.PlaceStem(steererUp, assembly.StemSetup{
		UpperCoverMm:  10,
		SpacersMm:     20,
		ClampHeightMm: 40,
		LengthMm:      100,
		AngleDeg:      30,
	})
	if err != nil {
		t.Fatalf("PlaceStem() error = %v", err)
	}

	sqrt3 := math.Sqrt(3)
	assertNear(t, "BarClamp.X", got.BarClamp.Origin().X, 25.0, 1e-9)
	assertNear(t, "BarClamp.Y", got.BarClamp.Origin().Y, 75.0*sqrt3, 1e-9)
	assertNear(t, "BarClamp.HeadingDeg", float64(got.BarClamp.HeadingDeg()), 60.0, 1e-9)
}

func TestPlaceStemDimensionsAndTraceEquivalence(t *testing.T) {
	// Default Milestone 1 front cockpit dimensions at HTA = 68.25°.
	setup := assembly.StemSetup{
		UpperCoverMm:  11.0,
		SpacersMm:     20.0,
		ClampHeightMm: 40.0,
		LengthMm:      60.0,
		AngleDeg:      -6.0,
	}
	heading := pose.Deg(180.0 - 68.25)
	origin := pose.Point{X: 428.0, Y: 642.0}

	tr := pose.NewTrace()
	traced, err := assembly.PlaceStem(pose.NewPose(origin, heading, tr), setup)
	if err != nil {
		t.Fatalf("traced PlaceStem() error = %v", err)
	}
	tr.Freeze()

	untraced, err := assembly.PlaceStem(pose.NewPose(origin, heading, nil), setup)
	if err != nil {
		t.Fatalf("untraced PlaceStem() error = %v", err)
	}

	assertNear(t, "stack distance", dist(origin, traced.SteererClamp.Origin()), 11.0+20.0+20.0, 1e-9)
	assertNear(t, "stem length", dist(traced.SteererClamp.Origin(), traced.BarClamp.Origin()), 60.0, 1e-9)

	assertNear(t, "CoverTop.X eq", traced.CoverTop.Origin().X, untraced.CoverTop.Origin().X, 1e-12)
	assertNear(t, "CoverTop.Y eq", traced.CoverTop.Origin().Y, untraced.CoverTop.Origin().Y, 1e-12)
	assertNear(t, "SpacerTop.X eq", traced.SpacerTop.Origin().X, untraced.SpacerTop.Origin().X, 1e-12)
	assertNear(t, "SpacerTop.Y eq", traced.SpacerTop.Origin().Y, untraced.SpacerTop.Origin().Y, 1e-12)
	assertNear(t, "SteererClamp.X eq", traced.SteererClamp.Origin().X, untraced.SteererClamp.Origin().X, 1e-12)
	assertNear(t, "SteererClamp.Y eq", traced.SteererClamp.Origin().Y, untraced.SteererClamp.Origin().Y, 1e-12)
	assertNear(t, "BarClamp.X eq", traced.BarClamp.Origin().X, untraced.BarClamp.Origin().X, 1e-12)
	assertNear(t, "BarClamp.Y eq", traced.BarClamp.Origin().Y, untraced.BarClamp.Origin().Y, 1e-12)
	assertNear(t, "BarClamp.Heading eq", float64(traced.BarClamp.HeadingDeg()), float64(untraced.BarClamp.HeadingDeg()), 1e-12)
}

func TestPlaceStemValidation(t *testing.T) {
	validPose := pose.NewPose(pose.Point{X: 0, Y: 0}, 111.75, nil)
	validSetup := assembly.StemSetup{
		UpperCoverMm:  11,
		SpacersMm:     20,
		ClampHeightMm: 40,
		LengthMm:      60,
		AngleDeg:      -6,
	}

	cases := []struct {
		name  string
		pose  pose.Pose
		setup assembly.StemSetup
	}{
		{"NaN origin X", pose.NewPose(pose.Point{X: math.NaN(), Y: 0}, 111.75, nil), validSetup},
		{"Inf heading", pose.NewPose(pose.Point{X: 0, Y: 0}, pose.Deg(math.Inf(1)), nil), validSetup},
		{"negative cover", validPose, assembly.StemSetup{UpperCoverMm: -1, SpacersMm: 20, ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"negative spacers", validPose, assembly.StemSetup{UpperCoverMm: 11, SpacersMm: -1, ClampHeightMm: 40, LengthMm: 60, AngleDeg: -6}},
		{"zero clamp height", validPose, assembly.StemSetup{UpperCoverMm: 11, SpacersMm: 20, ClampHeightMm: 0, LengthMm: 60, AngleDeg: -6}},
		{"zero length", validPose, assembly.StemSetup{UpperCoverMm: 11, SpacersMm: 20, ClampHeightMm: 40, LengthMm: 0, AngleDeg: -6}},
		{"NaN angle", validPose, assembly.StemSetup{UpperCoverMm: 11, SpacersMm: 20, ClampHeightMm: 40, LengthMm: 60, AngleDeg: pose.Deg(math.NaN())}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := assembly.PlaceStem(tc.pose, tc.setup); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}
