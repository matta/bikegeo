package pose_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/matta/bikegeo/internal/pose"
)

var (
	testWorld   = pose.NewFrame("test:world")
	staticWorld = pose.NewFrame("test:static_world")
	postLocal   = pose.NewFrame("test:post_local")
	cradleLocal = pose.NewFrame("test:cradle_local")
	railClamp   = pose.NewFrame("test:rail_clamp")
	saddleLocal = pose.NewFrame("test:saddle_local")
)

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if !(math.Abs(got-want) <= tol) {
		t.Fatalf("%s = %.6f, want %.6f (tol %.6f)", name, got, want, tol)
	}
}

func assertPanicsWith(t *testing.T, name, wantSubstr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s: expected panic containing %q, did not panic", name, wantSubstr)
		}
		if wantSubstr != "" {
			msg, ok := r.(string)
			if !ok || !strings.Contains(msg, wantSubstr) {
				t.Fatalf("%s: panic = %v, want string containing %q", name, r, wantSubstr)
			}
		}
	}()
	fn()
}

func TestPoseImmutabilityAndBranching(t *testing.T) {
	root := pose.NewPose(pose.Pt(testWorld, 10, 20), 90)

	a := root.Advance(50)
	b := root.Turn(-90).Advance(30)

	// Root must be unchanged.
	assertNear(t, "root.X", root.Origin().X(), 10, 1e-9)
	assertNear(t, "root.Y", root.Origin().Y(), 20, 1e-9)
	assertNear(t, "root.Heading", float64(root.HeadingDeg()), 90, 1e-9)

	// Branch A: moved +50 along +Y.
	assertNear(t, "a.X", a.Origin().X(), 10, 1e-9)
	assertNear(t, "a.Y", a.Origin().Y(), 70, 1e-9)

	// Branch B: turned -90 to 0° (+X), moved +30 along +X.
	assertNear(t, "b.X", b.Origin().X(), 40, 1e-9)
	assertNear(t, "b.Y", b.Origin().Y(), 20, 1e-9)
	assertNear(t, "b.Heading", float64(b.HeadingDeg()), 0, 1e-9)
}

func TestAdvanceTurnChain(t *testing.T) {
	p := pose.OriginPose(testWorld).Turn(60).Advance(10).Turn(-30).Advance(20)
	// 10 along 60°, then 20 along 30°.
	wantX := 10*math.Cos(60*math.Pi/180) + 20*math.Cos(30*math.Pi/180)
	wantY := 10*math.Sin(60*math.Pi/180) + 20*math.Sin(30*math.Pi/180)
	assertNear(t, "X", p.Origin().X(), wantX, 1e-12)
	assertNear(t, "Y", p.Origin().Y(), wantY, 1e-12)
	assertNear(t, "Heading", float64(p.HeadingDeg()), 30, 1e-12)
}

func TestTranslateLocalAngleToAndDistanceTo(t *testing.T) {
	// At 90° heading (+Y is local +X, -X is local +Y).
	p0 := pose.NewPose(pose.Pt(testWorld, 10, 20), 90)
	p1 := p0.TranslateLocal(30, 40)

	// World delta = (-40, +30) -> origin = (-30, 50).
	assertNear(t, "p1.X", p1.Origin().X(), -30, 1e-9)
	assertNear(t, "p1.Y", p1.Origin().Y(), 50, 1e-9)
	assertNear(t, "p1.Heading", float64(p1.HeadingDeg()), 90, 1e-9)
	start, end := p0.Origin(), p1.Origin()
	assertNear(t, "distance", start.DistanceTo(end), 50, 1e-9)
	assertNear(t, "vertical offset", start.VerticalOffsetTo(end), 30, 1e-9)
	assertNear(t, "reverse vertical offset", end.VerticalOffsetTo(start), -30, 1e-9)
	assertNear(t, "vertical offset to self", start.VerticalOffsetTo(start), 0, 1e-12)
	assertNear(t, "angle", float64(start.AngleTo(end).Deg()), 180.0-math.Atan2(30, 40)*180.0/math.Pi, 1e-9)
}

func TestPointDisplacements(t *testing.T) {
	start := pose.Pt(testWorld, 10, 20)
	end := pose.Pt(testWorld, 7, 24)
	move := start.OffsetTo(end)
	if want := pose.Vc(testWorld, -3, 4); move != want {
		t.Fatalf("offset = %v, want %v", move, want)
	}
	if got := start.Add(move); got != end {
		t.Errorf("translated point = %v, want %v", got, end)
	}
	if got, want := start.Sub(end), pose.Vc(testWorld, 3, -4); got != want {
		t.Errorf("reverse displacement = %v, want %v", got, want)
	}
	assertNear(t, "displacement norm", move.Norm(), 5, 1e-12)

	adjustment := pose.Vc(testWorld, 2, -1)
	if got, want := move.Add(adjustment), pose.Vc(testWorld, -1, 3); got != want {
		t.Errorf("vector sum = %v, want %v", got, want)
	}
	if got, want := move.Sub(adjustment), pose.Vc(testWorld, -5, 5); got != want {
		t.Errorf("vector difference = %v, want %v", got, want)
	}
	if got, want := move.Scale(2), pose.Vc(testWorld, -6, 8); got != want {
		t.Errorf("scaled vector = %v, want %v", got, want)
	}
	if got, want := move.Neg(), pose.Vc(testWorld, 3, -4); got != want {
		t.Errorf("negated vector = %v, want %v", got, want)
	}
	assertNear(t, "dot product", move.Dot(adjustment), -10, 1e-12)
}

func TestFaceSetsAbsoluteHeading(t *testing.T) {
	start := pose.NewPose(pose.Pt(testWorld, 100, 72), 382.75)
	moved := start.Advance(10)
	faced := moved.Face(pose.NewHeading(testWorld, -90.0))

	assertNear(t, "faced.X", faced.Origin().X(), moved.Origin().X(), 1e-12)
	assertNear(t, "faced.Y", faced.Origin().Y(), moved.Origin().Y(), 1e-12)
	assertNear(t, "faced.Heading", float64(faced.HeadingDeg()), -90.0, 1e-12)
	assertNear(t, "faced.Advance(20).X", faced.Advance(20).Origin().X(), faced.Origin().X(), 1e-9)
	assertNear(t, "faced.Advance(20).Y", faced.Advance(20).Origin().Y(), faced.Origin().Y()-20.0, 1e-9)
	assertNear(t, "start.Heading", float64(start.HeadingDeg()), 382.75, 1e-12)
}

func TestTransformPointVecAndPose(t *testing.T) {
	p := pose.NewPose(pose.Pt(testWorld, 63, -147), 104.75)
	localPoint := pose.Pt(postLocal, 30, -14)
	localVec := pose.Vc(postLocal, 30, -14)
	localPose := pose.NewPose(localPoint, -15)
	transform := p.Anchor(postLocal)

	if transform.Src() != postLocal || transform.Dst() != testWorld {
		t.Fatalf("transform frames = %s → %s", transform.Src(), transform.Dst())
	}
	if transform.OriginInDst() != p.Origin() || transform.HeadingInDst() != p.Heading() || transform.PoseInDst() != p {
		t.Fatalf("PoseInDst mismatch: got %+v, want %+v", transform.PoseInDst(), p)
	}

	worldPoint := transform.ApplyPoint(localPoint)
	assertNear(t, "transformed point vs. translation", worldPoint.DistanceTo(p.TranslateLocal(30, -14).Origin()), 0, 1e-12)
	worldVec := transform.ApplyVec(localVec)
	assertNear(t, "transformed vector vs. origin displacement", worldVec.Sub(p.Origin().OffsetTo(worldPoint)).Norm(), 0, 1e-12)
	worldPose := transform.ApplyPose(localPose)
	assertNear(t, "transformed pose origin", worldPose.Origin().DistanceTo(worldPoint), 0, 1e-12)
	assertNear(t, "transformed pose heading", float64(worldPose.HeadingDeg()), 104.75-15, 1e-12)

	inv := transform.Inverse()
	assertNear(t, "round-trip point", inv.ApplyPoint(worldPoint).DistanceTo(localPoint), 0, 1e-12)
	assertNear(t, "round-trip vector", inv.ApplyVec(worldVec).Sub(localVec).Norm(), 0, 1e-12)
	roundPose := inv.ApplyPose(worldPose)
	assertNear(t, "round-trip pose origin", roundPose.Origin().DistanceTo(localPoint), 0, 1e-12)
	assertNear(t, "round-trip pose heading", float64(roundPose.HeadingDeg()), -15, 1e-12)

	id := pose.IdentityTransform(postLocal)
	if id.ApplyPoint(localPoint) != localPoint || id.ApplyVec(localVec) != localVec || id.ApplyPose(localPose) != localPose {
		t.Errorf("IdentityTransform modified input")
	}
}

func TestComposeRailAndPostTransforms(t *testing.T) {
	seatTubeTop := pose.NewPose(pose.Pt(testWorld, -130, 450), 104.75)
	cradleInPost := pose.Pt(postLocal, 180, 12)
	railHeading := pose.NewHeading(testWorld, 5)
	railInPost := pose.NewTransform(saddleLocal, postLocal, cradleInPost, railHeading.Sub(seatTubeTop.Heading()))
	railToWorld := pose.Compose(railInPost, seatTubeTop.Anchor(postLocal))

	brpInRail := pose.Pt(saddleLocal, 35, 42)
	cradle := seatTubeTop.Advance(cradleInPost.X()).TranslateLocal(0, cradleInPost.Y())
	want := cradle.Face(railHeading).
		Advance(brpInRail.X()).TranslateLocal(0, brpInRail.Y()).Origin()
	assertNear(t, "composed BRP vs. turtle placement", railToWorld.ApplyPoint(brpInRail).DistanceTo(want), 0, 1e-12)
	brpOffset := pose.Vc(saddleLocal, 35, 42)
	wantOffset := seatTubeTop.Anchor(postLocal).ApplyVec(railInPost.ApplyVec(brpOffset))
	assertNear(t, "composed vector vs. sequential rotation", railToWorld.ApplyVec(brpOffset).Sub(wantOffset).Norm(), 0, 1e-12)
}

func TestTransferClampedSaddle(t *testing.T) {
	dynamicCradle := pose.NewPose(pose.Pt(testWorld, -190, 675), 104.75)
	railHeading := pose.NewHeading(testWorld, 5)
	dynamicRail := dynamicCradle.Face(railHeading)
	clampedRailTurn := dynamicRail.Heading().Sub(dynamicCradle.Heading())
	assertNear(t, "measured rail turn", float64(clampedRailTurn), -99.75, 1e-12)
	assertNear(t, "replayed dynamic heading", float64(dynamicCradle.Turn(clampedRailTurn).HeadingDeg()), 5, 1e-12)

	staticCradle := pose.NewPose(pose.Pt(staticWorld, -183, 684), 106.25)
	staticRail := staticCradle.Turn(clampedRailTurn)
	assertNear(t, "replayed static heading", float64(staticRail.HeadingDeg()), 6.5, 1e-12)
	dynamicBrp := dynamicRail.TranslateLocal(35, 42).Origin()
	want := staticRail.TranslateLocal(35, 42).Origin()
	transfer := pose.Compose(dynamicCradle.Anchor(cradleLocal).Inverse(), staticCradle.Anchor(cradleLocal))
	assertNear(t, "transferred BRP vs. static turtle placement", transfer.ApplyPoint(dynamicBrp).DistanceTo(want), 0, 1e-12)
}

func TestFrameIDNegativeSafeguards(t *testing.T) {
	// Zero-value FrameID and uninitialized geometric structs fail immediately on first use.
	var zeroFrame pose.FrameID
	if zeroFrame.Valid() {
		t.Fatal("zero FrameID reported Valid() == true")
	}
	if zeroFrame.String() != "<uninitialized>" {
		t.Fatalf("zero FrameID.String() = %q", zeroFrame.String())
	}

	var zeroPt pose.Point
	var zeroVec pose.Vec
	var zeroHeading pose.Heading
	var zeroPose pose.Pose
	var zeroTransform pose.Transform

	assertPanicsWith(t, "zero Point.X", "uninitialized zero-value FrameID", func() { _ = zeroPt.X() })
	assertPanicsWith(t, "zero Point.Sub", "uninitialized zero-value FrameID", func() { _ = zeroPt.Sub(pose.Origin(testWorld)) })
	assertPanicsWith(t, "zero Point.MarshalJSON", "uninitialized zero-value FrameID", func() { _, _ = json.Marshal(zeroPt) })
	assertPanicsWith(t, "zero Vec.Norm", "uninitialized zero-value FrameID", func() { _ = zeroVec.Norm() })
	assertPanicsWith(t, "zero Heading.Deg", "uninitialized zero-value FrameID", func() { _ = zeroHeading.Deg() })
	assertPanicsWith(t, "zero Pose.Advance", "uninitialized zero-value FrameID", func() { _ = zeroPose.Advance(10) })
	assertPanicsWith(t, "zero Pose.MarshalJSON", "uninitialized zero-value FrameID", func() { _, _ = json.Marshal(zeroPose) })
	assertPanicsWith(t, "zero Transform.ApplyPoint", "uninitialized zero-value FrameID", func() { _ = zeroTransform.ApplyPoint(pose.Origin(testWorld)) })
	assertPanicsWith(t, "zero Transform.Inverse", "uninitialized zero-value FrameID", func() { _ = zeroTransform.Inverse() })
	assertPanicsWith(t, "NewTransform zero src", "uninitialized zero-value FrameID", func() { _ = pose.NewTransform(zeroFrame, testWorld, pose.Origin(testWorld), 0) })

	// Cross-frame Point/Vec arithmetic panics.
	ptWorld := pose.Pt(testWorld, 1, 2)
	ptStatic := pose.Pt(staticWorld, 1, 2)
	vecStatic := pose.Vc(staticWorld, 3, 4)
	assertPanicsWith(t, "cross-frame Point.Sub", "coordinate frame mismatch", func() { _ = ptWorld.Sub(ptStatic) })
	assertPanicsWith(t, "cross-frame Point.Add", "coordinate frame mismatch", func() { _ = ptWorld.Add(vecStatic) })
	assertPanicsWith(t, "cross-frame Vec.Dot", "coordinate frame mismatch", func() { _ = pose.Vc(testWorld, 1, 0).Dot(vecStatic) })
	assertPanicsWith(t, "cross-frame Pose.Face", "coordinate frame mismatch", func() { _ = pose.OriginPose(testWorld).Face(pose.NewHeading(staticWorld, 0)) })
	assertPanicsWith(t, "cross-frame DecomposeAlong", "coordinate frame mismatch", func() {
		_, _, _ = pose.DecomposeAlong(pose.Vc(testWorld, 1, 1), pose.NewHeading(testWorld, 0), pose.NewHeading(staticWorld, 90))
	})

	// Transform Compose mismatch panics.
	tPostToWorld := pose.OriginPose(testWorld).Anchor(postLocal)
	tCradleToStatic := pose.OriginPose(staticWorld).Anchor(cradleLocal)
	assertPanicsWith(t, "mismatched Compose", "transform compose mismatch", func() {
		_ = pose.Compose(tPostToWorld, tCradleToStatic)
	})

	// Origin-distinct local frames (RailClamp vs. SaddleLocal) cannot be mixed.
	tClampToWorld := pose.OriginPose(testWorld).Anchor(railClamp)
	assertPanicsWith(t, "origin-distinct RailClamp vs SaddleLocal", "coordinate frame mismatch", func() {
		_ = tClampToWorld.ApplyPoint(pose.Pt(saddleLocal, 32, 45))
	})
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
	h0 := pose.NewHeading(testWorld, 0)
	h90 := pose.NewHeading(testWorld, 90)
	a, b, ok := pose.DecomposeAlong(pose.Vc(testWorld, 3, 4), h0, h90)
	if !ok {
		t.Fatal("axis-aligned decomposition reported not ok")
	}
	assertNear(t, "a", a, 3, 1e-12)
	assertNear(t, "b", b, 4, 1e-12)

	v := pose.Vc(testWorld, -37.5, 81.25)
	a, b, ok = pose.DecomposeAlong(v, pose.NewHeading(testWorld, 105.29), pose.NewHeading(testWorld, 180))
	if !ok {
		t.Fatal("oblique decomposition reported not ok")
	}
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	gotX := a*math.Cos(rad(105.29)) + b*math.Cos(rad(180))
	gotY := a*math.Sin(rad(105.29)) + b*math.Sin(rad(180))
	assertNear(t, "round trip X", gotX, v.X(), 1e-9)
	assertNear(t, "round trip Y", gotY, v.Y(), 1e-9)

	for _, h := range [][2]pose.Heading{
		{pose.NewHeading(testWorld, 30), pose.NewHeading(testWorld, 30)},
		{pose.NewHeading(testWorld, 30), pose.NewHeading(testWorld, 210)},
	} {
		if _, _, ok := pose.DecomposeAlong(v, h[0], h[1]); ok {
			t.Fatalf("headings %v and %v should be reported as parallel", h[0], h[1])
		}
	}
}

func TestAlongAndProjectOnto(t *testing.T) {
	p := pose.Along(pose.NewHeading(testWorld, 90), 2)
	assertNear(t, "Along.X", p.X(), 0, 1e-12)
	assertNear(t, "Along.Y", p.Y(), 2, 1e-12)
	assertNear(t, "ProjectOnto", pose.ProjectOnto(pose.Vc(testWorld, 1, 1), pose.NewHeading(testWorld, 45)), math.Sqrt2, 1e-12)
}

func TestPoseMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		p    pose.Pose
		want string
	}{
		{"placed", pose.NewPose(pose.Pt(testWorld, -219, 713.1), 104.75), `{"Origin":{"X":-219,"Y":713.1},"HeadingDeg":104.75}`},
		{"origin", pose.OriginPose(testWorld), `{"Origin":{"X":0,"Y":0},"HeadingDeg":0}`},
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
		{"NaN X", pose.NewPose(pose.Pt(testWorld, math.NaN(), 0), 0)},
		{"Inf Y", pose.NewPose(pose.Pt(testWorld, 0, math.Inf(1)), 0)},
		{"NaN heading", pose.NewPose(pose.Origin(testWorld), pose.Deg(math.NaN()))},
		{"Inf heading", pose.NewPose(pose.Origin(testWorld), pose.Deg(math.Inf(-1)))},
	}
	for _, tc := range nonFinite {
		if _, err := json.Marshal(tc.p); err == nil {
			t.Errorf("%s: json.Marshal succeeded, want an error", tc.name)
		}
	}
}
