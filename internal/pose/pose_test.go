package pose_test

import (
	"encoding/json"
	"math"
	"testing"
	"unsafe"

	"github.com/matta/bikegeo/internal/pose"
)

type testWorld struct{}
type testDir struct{}
type testOtherDir struct{}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if !(math.Abs(got-want) <= tol) {
		t.Fatalf("%s = %.6f, want %.6f (tol %.6f)", name, got, want, tol)
	}
}

func TestPoseImmutabilityAndBranching(t *testing.T) {
	root := pose.NewPose[testDir](pose.Point[testWorld]{X: 10, Y: 20}, 90)

	a := root.Advance(50)
	b := pose.Turn(root, pose.TurnDeg[testDir, testOtherDir](-90)).Advance(30)

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
	start := pose.NewPose[testDir](pose.Point[testWorld]{}, 60).Advance(10)
	p := pose.Turn(start, pose.TurnDeg[testDir, testOtherDir](-30)).Advance(20)
	// 10 along 60°, then 20 along 30°.
	wantX := 10*math.Cos(60*math.Pi/180) + 20*math.Cos(30*math.Pi/180)
	wantY := 10*math.Sin(60*math.Pi/180) + 20*math.Sin(30*math.Pi/180)
	assertNear(t, "X", p.Origin().X, wantX, 1e-12)
	assertNear(t, "Y", p.Origin().Y, wantY, 1e-12)
	assertNear(t, "Heading", float64(p.HeadingDeg()), 30, 1e-12)
}

func TestTranslateAngleToAndDistanceTo(t *testing.T) {
	// At 90° heading (+Y is local +X, -X is local +Y).
	p0 := pose.NewPose[testDir](pose.Point[testWorld]{X: 10, Y: 20}, 90)
	p1 := p0.Translate(pose.Vec[testDir]{X: 30, Y: 40})

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
	start := pose.Point[testWorld]{X: 10, Y: 20}
	end := pose.Point[testWorld]{X: 7, Y: 24}
	move := start.OffsetTo(end)
	if want := (pose.Vec[testWorld]{X: -3, Y: 4}); move != want {
		t.Fatalf("offset = %v, want %v", move, want)
	}
	if got := start.Add(move); got != end {
		t.Errorf("translated point = %v, want %v", got, end)
	}
	if got, want := start.Sub(end), (pose.Vec[testWorld]{X: 3, Y: -4}); got != want {
		t.Errorf("reverse displacement = %v, want %v", got, want)
	}
	assertNear(t, "displacement norm", move.Norm(), 5, 1e-12)

	adjustment := pose.Vec[testWorld]{X: 2, Y: -1}
	if got, want := move.Add(adjustment), (pose.Vec[testWorld]{X: -1, Y: 3}); got != want {
		t.Errorf("vector sum = %v, want %v", got, want)
	}
	if got, want := move.Sub(adjustment), (pose.Vec[testWorld]{X: -5, Y: 5}); got != want {
		t.Errorf("vector difference = %v, want %v", got, want)
	}
	if got, want := move.Scale(2), (pose.Vec[testWorld]{X: -6, Y: 8}); got != want {
		t.Errorf("scaled vector = %v, want %v", got, want)
	}
	if got, want := move.Neg(), (pose.Vec[testWorld]{X: 3, Y: -4}); got != want {
		t.Errorf("negated vector = %v, want %v", got, want)
	}
}

func TestFaceSetsAbsoluteHeading(t *testing.T) {
	start := pose.NewPose[testDir](pose.Point[testWorld]{X: 100, Y: 72}, 382.75)
	moved := start.Advance(10)
	faced := pose.Face[testOtherDir](moved, -90.0)

	assertNear(t, "faced.X", faced.Origin().X, moved.Origin().X, 1e-12)
	assertNear(t, "faced.Y", faced.Origin().Y, moved.Origin().Y, 1e-12)
	assertNear(t, "faced.Heading", float64(faced.HeadingDeg()), -90.0, 1e-12)
	assertNear(t, "faced.Advance(20).X", faced.Advance(20).Origin().X, faced.Origin().X, 1e-9)
	assertNear(t, "faced.Advance(20).Y", faced.Advance(20).Origin().Y, faced.Origin().Y-20.0, 1e-9)
	assertNear(t, "start.Heading", float64(start.HeadingDeg()), 382.75, 1e-12)
}

func TestPoseSizes(t *testing.T) {
	type untypedPoint struct{ X, Y float64 }
	type untypedPose struct {
		origin     untypedPoint
		headingDeg pose.Deg
	}
	for _, tc := range []struct {
		name      string
		got, want uintptr
	}{
		{"Point", unsafe.Sizeof(pose.Point[testWorld]{}), unsafe.Sizeof(untypedPoint{})},
		{"Vec", unsafe.Sizeof(pose.Vec[testWorld]{}), unsafe.Sizeof(untypedPoint{})},
		{"Pose", unsafe.Sizeof(pose.Pose[testDir, testWorld]{}), unsafe.Sizeof(untypedPose{})},
	} {
		if tc.got != tc.want {
			t.Errorf("%s size = %d bytes, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestTransformPointAndVector(t *testing.T) {
	p := pose.NewPose[testDir](pose.Point[testWorld]{X: 63, Y: -147}, 104.75)
	localPoint := pose.Point[testDir]{X: 30, Y: -14}
	localVec := pose.Vec[testDir]{X: 30, Y: -14}
	transform := p.Transform()

	worldPoint := transform.ApplyPoint(localPoint)
	assertNear(t, "transformed point vs. translation", worldPoint.DistanceTo(p.Translate(localVec).Origin()), 0, 1e-12)
	worldVec := transform.ApplyVec(localVec)
	assertNear(t, "transformed vector vs. origin displacement", worldVec.Sub(p.Origin().OffsetTo(worldPoint)).Norm(), 0, 1e-12)
	assertNear(t, "round-trip point", transform.Inverse().ApplyPoint(worldPoint).DistanceTo(localPoint), 0, 1e-12)
	assertNear(t, "round-trip vector", transform.Inverse().ApplyVec(worldVec).Sub(localVec).Norm(), 0, 1e-12)
}

type postDir struct{}
type railDir struct{}
type staticWorld struct{}

func TestComposeRailAndPostTransforms(t *testing.T) {
	seatTubeTop := pose.NewPose[postDir](pose.Point[testWorld]{X: -130, Y: 450}, 104.75)
	cradleInPost := pose.Point[postDir]{X: 180, Y: 12}
	railHeading := pose.Heading[testWorld](5)
	railInPost := pose.NewRigidTransform[railDir](cradleInPost, pose.Heading[postDir](railHeading.Sub(seatTubeTop.Heading())))
	railToWorld := pose.Compose(railInPost, seatTubeTop.Transform())

	brpInRail := pose.Point[railDir]{X: 35, Y: 42}
	cradle := seatTubeTop.Advance(cradleInPost.X).Translate(pose.Vec[postDir]{Y: cradleInPost.Y})
	want := pose.Face[railDir](cradle, railHeading).
		Advance(brpInRail.X).Translate(pose.Vec[railDir]{Y: brpInRail.Y}).Origin()
	assertNear(t, "composed BRP vs. turtle placement", railToWorld.ApplyPoint(brpInRail).DistanceTo(want), 0, 1e-12)
	brpOffset := pose.Vec[railDir]{X: 35, Y: 42}
	wantOffset := seatTubeTop.Transform().ApplyVec(railInPost.ApplyVec(brpOffset))
	assertNear(t, "composed vector vs. sequential rotation", railToWorld.ApplyVec(brpOffset).Sub(wantOffset).Norm(), 0, 1e-12)
}

func TestTransferClampedSaddle(t *testing.T) {
	dynamicCradle := pose.NewPose[postDir](pose.Point[testWorld]{X: -190, Y: 675}, 104.75)
	dynamicRail := pose.Face[railDir](dynamicCradle, 5)
	clampedRailTurn := pose.MeasureTurn(dynamicCradle, dynamicRail)
	assertNear(t, "measured rail turn", float64(clampedRailTurn.Deg()), -99.75, 1e-12)
	assertNear(t, "replayed dynamic heading", float64(pose.Turn(dynamicCradle, clampedRailTurn).HeadingDeg()), 5, 1e-12)

	staticCradle := pose.NewPose[postDir](pose.Point[staticWorld]{X: -183, Y: 684}, 106.25)
	staticRail := pose.Turn(staticCradle, clampedRailTurn)
	assertNear(t, "replayed static heading", float64(staticRail.HeadingDeg()), 6.5, 1e-12)
	brpOffset := pose.Vec[railDir]{X: 35, Y: 42}
	dynamicBrp := dynamicRail.Translate(brpOffset).Origin()
	want := staticRail.Translate(brpOffset).Origin()
	transfer := pose.Compose(dynamicCradle.Transform().Inverse(), staticCradle.Transform())
	assertNear(t, "transferred BRP vs. static turtle placement", transfer.ApplyPoint(dynamicBrp).DistanceTo(want), 0, 1e-12)
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
	a, b, ok := pose.DecomposeAlong(pose.Vec[testWorld]{X: 3, Y: 4}, 0, 90)
	if !ok {
		t.Fatal("axis-aligned decomposition reported not ok")
	}
	assertNear(t, "a", a, 3, 1e-12)
	assertNear(t, "b", b, 4, 1e-12)

	v := pose.Vec[testWorld]{X: -37.5, Y: 81.25}
	a, b, ok = pose.DecomposeAlong(v, 105.29, 180)
	if !ok {
		t.Fatal("oblique decomposition reported not ok")
	}
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	gotX := a*math.Cos(rad(105.29)) + b*math.Cos(rad(180))
	gotY := a*math.Sin(rad(105.29)) + b*math.Sin(rad(180))
	assertNear(t, "round trip X", gotX, v.X, 1e-9)
	assertNear(t, "round trip Y", gotY, v.Y, 1e-9)

	for _, h := range [][2]pose.Heading[testWorld]{{30, 30}, {30, 210}} {
		if _, _, ok := pose.DecomposeAlong(v, h[0], h[1]); ok {
			t.Fatalf("headings %v and %v should be reported as parallel", h[0], h[1])
		}
	}
}

func TestAlongAndProjectOnto(t *testing.T) {
	p := pose.Along(pose.Heading[testWorld](90), 2)
	assertNear(t, "Along.X", p.X, 0, 1e-12)
	assertNear(t, "Along.Y", p.Y, 2, 1e-12)
	assertNear(t, "ProjectOnto", pose.ProjectOnto(pose.Vec[testWorld]{X: 1, Y: 1}, 45), math.Sqrt2, 1e-12)
}

func TestPoseMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		p    pose.Pose[testDir, testWorld]
		want string
	}{
		{"placed", pose.NewPose[testDir](pose.Point[testWorld]{X: -219, Y: 713.1}, 104.75), `{"Origin":{"X":-219,"Y":713.1},"HeadingDeg":104.75}`},
		{"zero", pose.Pose[testDir, testWorld]{}, `{"Origin":{"X":0,"Y":0},"HeadingDeg":0}`},
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
		p    pose.Pose[testDir, testWorld]
	}{
		{"NaN X", pose.NewPose[testDir](pose.Point[testWorld]{X: math.NaN(), Y: 0}, 0)},
		{"Inf Y", pose.NewPose[testDir](pose.Point[testWorld]{X: 0, Y: math.Inf(1)}, 0)},
		{"NaN heading", pose.NewPose[testDir](pose.Point[testWorld]{}, pose.Heading[testWorld](math.NaN()))},
		{"Inf heading", pose.NewPose[testDir](pose.Point[testWorld]{}, pose.Heading[testWorld](math.Inf(-1)))},
	}
	for _, tc := range nonFinite {
		if _, err := json.Marshal(tc.p); err == nil {
			t.Errorf("%s: json.Marshal succeeded, want an error", tc.name)
		}
	}
}
