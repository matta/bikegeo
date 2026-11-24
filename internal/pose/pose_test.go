package pose_test

import (
	"encoding/json"
	"math"
	"testing"

	"bikegeo/internal/pose"
)

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.6f, want %.6f (tol %.6f)", name, got, want, tol)
	}
}

func TestPoseImmutabilityAndBranching(t *testing.T) {
	root := pose.NewPose(pose.Point{X: 10, Y: 20}, 90)

	a := root.Advance(50)
	b := root.Turn(-90).Advance(30)

	// Root must be unchanged.
	assertNear(t, "root.X", root.Origin().X, 10, 1e-9)
	assertNear(t, "root.Y", root.Origin().Y, 20, 1e-9)
	assertNear(t, "root.Heading", float64(root.HeadingDeg()), 90, 1e-9)

	// Branch A: moved +50 along +Y.
	assertNear(t, "a.X", a.Origin().X, 10, 1e-9)
	assertNear(t, "a.Y", a.Origin().Y, 70, 1e-9)

	// Branch B: turned -90 to 0° (+X), moved +30 along +X.
	assertNear(t, "b.X", b.Origin().X, 40, 1e-9)
	assertNear(t, "b.Y", b.Origin().Y, 20, 1e-9)
	assertNear(t, "b.Heading", float64(b.HeadingDeg()), 0, 1e-9)
}

func TestAdvanceTurnChain(t *testing.T) {
	p := pose.NewPose(pose.Point{}, 60).Advance(10).Turn(-30).Advance(20)
	// 10 along 60°, then 20 along 30°.
	wantX := 10*math.Cos(60*math.Pi/180) + 20*math.Cos(30*math.Pi/180)
	wantY := 10*math.Sin(60*math.Pi/180) + 20*math.Sin(30*math.Pi/180)
	assertNear(t, "X", p.Origin().X, wantX, 1e-12)
	assertNear(t, "Y", p.Origin().Y, wantY, 1e-12)
	assertNear(t, "Heading", float64(p.HeadingDeg()), 30, 1e-12)
}

func TestTranslateAngleToAndDistanceTo(t *testing.T) {
	// At 90° heading (+Y is local +X, -X is local +Y).
	p0 := pose.NewPose(pose.Point{X: 10, Y: 20}, 90)
	p1 := p0.Translate(30, 40)

	// World delta = (-40, +30) -> origin = (-30, 50).
	assertNear(t, "p1.X", p1.Origin().X, -30, 1e-9)
	assertNear(t, "p1.Y", p1.Origin().Y, 50, 1e-9)
	assertNear(t, "p1.Heading", float64(p1.HeadingDeg()), 90, 1e-9)
	start, end := p0.Origin(), p1.Origin()
	assertNear(t, "distance", start.DistanceTo(end), 50, 1e-9)
	assertNear(t, "vertical offset", start.VerticalOffsetTo(end), 30, 1e-9)
	assertNear(t, "reverse vertical offset", end.VerticalOffsetTo(start), -30, 1e-9)
	assertNear(t, "vertical offset to self", start.VerticalOffsetTo(start), 0, 1e-12)
	assertNear(t, "angle", float64(start.AngleTo(end)), 180.0-math.Atan2(30, 40)*180.0/math.Pi, 1e-9)
}

func TestPointDisplacements(t *testing.T) {
	start := pose.Point{X: 10, Y: 20}
	end := pose.Point{X: 7, Y: 24}
	move := start.OffsetTo(end)
	if want := (pose.Vec{X: -3, Y: 4}); move != want {
		t.Fatalf("offset = %v, want %v", move, want)
	}
	if got := start.Add(move); got != end {
		t.Errorf("translated point = %v, want %v", got, end)
	}
	if got, want := start.Sub(end), (pose.Vec{X: 3, Y: -4}); got != want {
		t.Errorf("reverse displacement = %v, want %v", got, want)
	}
	assertNear(t, "displacement norm", move.Norm(), 5, 1e-12)

	adjustment := pose.Vec{X: 2, Y: -1}
	if got, want := move.Add(adjustment), (pose.Vec{X: -1, Y: 3}); got != want {
		t.Errorf("vector sum = %v, want %v", got, want)
	}
	if got, want := move.Sub(adjustment), (pose.Vec{X: -5, Y: 5}); got != want {
		t.Errorf("vector difference = %v, want %v", got, want)
	}
	if got, want := move.Scale(2), (pose.Vec{X: -6, Y: 8}); got != want {
		t.Errorf("scaled vector = %v, want %v", got, want)
	}
	if got, want := move.Neg(), (pose.Vec{X: 3, Y: -4}); got != want {
		t.Errorf("negated vector = %v, want %v", got, want)
	}
}

func TestFaceSetsAbsoluteHeading(t *testing.T) {
	start := pose.NewPose(pose.Point{X: 100, Y: 72}, 382.75)
	moved := start.Advance(10)
	faced := moved.Face(-90.0)

	assertNear(t, "faced.X", faced.Origin().X, moved.Origin().X, 1e-12)
	assertNear(t, "faced.Y", faced.Origin().Y, moved.Origin().Y, 1e-12)
	assertNear(t, "faced.Heading", float64(faced.HeadingDeg()), -90.0, 1e-12)
	assertNear(t, "faced.Advance(20).X", faced.Advance(20).Origin().X, faced.Origin().X, 1e-9)
	assertNear(t, "faced.Advance(20).Y", faced.Advance(20).Origin().Y, faced.Origin().Y-20.0, 1e-9)
	assertNear(t, "start.Heading", float64(start.HeadingDeg()), 382.75, 1e-12)
}

func TestPrimitivesDeflectionAndRightTriangleLeg(t *testing.T) {
	assertNear(t, "DeflectionOverSpan(0 deg)", pose.DeflectionOverSpan(250, 0), 0, 1e-12)
	assertNear(t, "DeflectionOverSpan(45 deg)", pose.DeflectionOverSpan(180, 45), 180, 1e-12)
	assertNear(t, "DeflectionOverSpan(-20 deg)", pose.DeflectionOverSpan(195, -20), -195*math.Tan(20*math.Pi/180), 1e-12)
	assertNear(t, "DeflectionOverSpan(0 span)", pose.DeflectionOverSpan(0, 20), 0, 1e-12)

	assertNear(t, "RightTriangleLeg(3-4-5)", pose.RightTriangleLeg(5, 3), 4, 1e-12)
	assertNear(t, "RightTriangleLeg(chainstay, bbDrop)", pose.RightTriangleLeg(450, 72), math.Sqrt(450*450-72*72), 1e-12)
}

func TestDecomposeAlong(t *testing.T) {
	a, b, ok := pose.DecomposeAlong(pose.Vec{X: 3, Y: 4}, 0, 90)
	if !ok {
		t.Fatal("axis-aligned decomposition reported not ok")
	}
	assertNear(t, "a", a, 3, 1e-12)
	assertNear(t, "b", b, 4, 1e-12)

	v := pose.Vec{X: -37.5, Y: 81.25}
	a, b, ok = pose.DecomposeAlong(v, 105.29, 180)
	if !ok {
		t.Fatal("oblique decomposition reported not ok")
	}
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	gotX := a*math.Cos(rad(105.29)) + b*math.Cos(rad(180))
	gotY := a*math.Sin(rad(105.29)) + b*math.Sin(rad(180))
	assertNear(t, "round trip X", gotX, v.X, 1e-9)
	assertNear(t, "round trip Y", gotY, v.Y, 1e-9)

	for _, h := range [][2]pose.Deg{{30, 30}, {30, 210}} {
		if _, _, ok := pose.DecomposeAlong(v, h[0], h[1]); ok {
			t.Fatalf("headings %v and %v should be reported as parallel", h[0], h[1])
		}
	}
}

func TestAlongAndProjectOnto(t *testing.T) {
	p := pose.Along(90, 2)
	assertNear(t, "Along.X", p.X, 0, 1e-12)
	assertNear(t, "Along.Y", p.Y, 2, 1e-12)
	assertNear(t, "ProjectOnto", pose.ProjectOnto(pose.Vec{X: 1, Y: 1}, 45), math.Sqrt2, 1e-12)
}

func TestPoseMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		p    pose.Pose
		want string
	}{
		{"placed", pose.NewPose(pose.Point{X: -219, Y: 713.1}, 104.75), `{"Origin":{"X":-219,"Y":713.1},"HeadingDeg":104.75}`},
		{"zero", pose.Pose{}, `{"Origin":{"X":0,"Y":0},"HeadingDeg":0}`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.p)
		if err != nil {
			t.Fatalf("%s: json.Marshal: %v", tc.name, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}

	nonFinite := []struct {
		name string
		p    pose.Pose
	}{
		{"NaN X", pose.NewPose(pose.Point{X: math.NaN(), Y: 0}, 0)},
		{"Inf Y", pose.NewPose(pose.Point{X: 0, Y: math.Inf(1)}, 0)},
		{"NaN heading", pose.NewPose(pose.Point{}, pose.Deg(math.NaN()))},
	}
	for _, tc := range nonFinite {
		if _, err := json.Marshal(tc.p); err == nil {
			t.Errorf("%s: json.Marshal succeeded, want an error", tc.name)
		}
	}
}
