package pose_test

import (
	"math"
	"testing"

	"bikegeo/internal/reboot/pose"
)

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.6f, want %.6f (tol %.6f)", name, got, want, tol)
	}
}

func TestPoseImmutabilityAndBranching(t *testing.T) {
	tr := pose.NewTrace()
	root := pose.NewPose(pose.Point{X: 10, Y: 20}, 90, tr)

	a := root.Advance("branch a", 50)
	b := root.Turn("branch b", -90).Advance("move b", 30)

	// Root must be unchanged.
	assertNear(t, "root.X", root.Origin().X, 10, 1e-9)
	assertNear(t, "root.Y", root.Origin().Y, 20, 1e-9)
	assertNear(t, "root.Heading", float64(root.HeadingDeg()), 90, 1e-9)
	if root.StepID() != 0 {
		t.Fatalf("root.StepID() = %d, want 0", root.StepID())
	}

	// Branch A: moved +50 along +Y.
	assertNear(t, "a.X", a.Origin().X, 10, 1e-9)
	assertNear(t, "a.Y", a.Origin().Y, 70, 1e-9)
	if a.StepID() != 1 {
		t.Fatalf("a.StepID() = %d, want 1", a.StepID())
	}

	// Branch B: turned -90 to 0° (+X), moved +30 along +X.
	assertNear(t, "b.X", b.Origin().X, 40, 1e-9)
	assertNear(t, "b.Y", b.Origin().Y, 20, 1e-9)
	assertNear(t, "b.Heading", float64(b.HeadingDeg()), 0, 1e-9)

	steps := tr.Steps()
	if len(steps) != 3 {
		t.Fatalf("len(steps) = %d, want 3", len(steps))
	}
	if steps[0].ParentID != 0 || steps[0].ID != 1 {
		t.Fatalf("step 0 linkage = %d->%d, want 0->1", steps[0].ParentID, steps[0].ID)
	}
	if steps[1].ParentID != 0 || steps[1].ID != 2 {
		t.Fatalf("step 1 linkage = %d->%d, want 0->2", steps[1].ParentID, steps[1].ID)
	}
	if steps[2].ParentID != 2 || steps[2].ID != 3 {
		t.Fatalf("step 2 linkage = %d->%d, want 2->3", steps[2].ParentID, steps[2].ID)
	}
}

func TestTraceFreezeAndNilEquivalence(t *testing.T) {
	tr := pose.NewTrace()
	traced := pose.NewPose(pose.Point{X: 0, Y: 0}, 60, tr).
		Advance("step1", 10).
		Turn("step2", -30).
		Advance("step3", 20)
	tr.Freeze()

	// Subsequent operations on a pose from a frozen trace must not append steps.
	afterFreeze := traced.Advance("ignored", 100)
	assertNear(t, "afterFreeze.X", afterFreeze.Origin().X, traced.Origin().X+100*math.Cos(30.0*math.Pi/180.0), 1e-9)
	if got := len(tr.Steps()); got != 3 {
		t.Fatalf("len(steps) after freeze = %d, want 3", got)
	}

	untraced := pose.NewPose(pose.Point{X: 0, Y: 0}, 60, nil).
		Advance("step1", 10).
		Turn("step2", -30).
		Advance("step3", 20)

	assertNear(t, "X equivalence", traced.Origin().X, untraced.Origin().X, 1e-12)
	assertNear(t, "Y equivalence", traced.Origin().Y, untraced.Origin().Y, 1e-12)
	assertNear(t, "Heading equivalence", float64(traced.HeadingDeg()), float64(untraced.HeadingDeg()), 1e-12)
}

func TestTranslateAngleToAndDistanceTo(t *testing.T) {
	tr := pose.NewTrace()
	// At 90° heading (+Y is local +X, -X is local +Y).
	p0 := pose.NewPose(pose.Point{X: 10, Y: 20}, 90, tr)
	p1 := p0.Translate("local shift", 30, 40)

	// World delta = (-40, +30) -> origin = (-30, 50).
	assertNear(t, "p1.X", p1.Origin().X, -30, 1e-9)
	assertNear(t, "p1.Y", p1.Origin().Y, 50, 1e-9)
	assertNear(t, "p1.Heading", float64(p1.HeadingDeg()), 90, 1e-9)
	assertNear(t, "p0.DistanceTo(p1)", p0.DistanceTo(p1), 50, 1e-9)
	assertNear(t, "p0.VerticalOffsetTo(p1)", p0.VerticalOffsetTo(p1), 30, 1e-9)
	assertNear(t, "p1.VerticalOffsetTo(p0)", p1.VerticalOffsetTo(p0), -30, 1e-9)
	assertNear(t, "p0.VerticalOffsetTo(p0)", p0.VerticalOffsetTo(p0), 0, 1e-12)
	assertNear(t, "p0.AngleTo(p1)", float64(p0.AngleTo(p1)), 180.0-math.Atan2(30, 40)*180.0/math.Pi, 1e-9)

	untraced := pose.NewPose(pose.Point{X: 10, Y: 20}, 90, nil).
		Translate("local shift", 30, 40)
	assertNear(t, "untraced X", untraced.Origin().X, p1.Origin().X, 1e-12)
	assertNear(t, "untraced Y", untraced.Origin().Y, p1.Origin().Y, 1e-12)

	steps := tr.Steps()
	if len(steps) != 1 || steps[0].Op != "translate" {
		t.Fatalf("expected 1 translate step, got %+v", steps)
	}
}

func TestFaceAbsoluteHeadingAndTrace(t *testing.T) {
	tr := pose.NewTrace()
	start := pose.NewPose(pose.Point{X: 100, Y: 72}, 382.75, tr)
	step1 := start.Advance("offset", 10)
	faced := step1.Face("down to ground", -90.0)

	assertNear(t, "faced.X", faced.Origin().X, step1.Origin().X, 1e-12)
	assertNear(t, "faced.Y", faced.Origin().Y, step1.Origin().Y, 1e-12)
	assertNear(t, "faced.Heading", float64(faced.HeadingDeg()), -90.0, 1e-12)

	tr.Freeze()
	afterFreeze := faced.Face("ignored", 0)
	assertNear(t, "afterFreeze.Heading", float64(afterFreeze.HeadingDeg()), 0, 1e-12)

	steps := tr.Steps()
	if len(steps) != 2 {
		t.Fatalf("len(steps) = %d, want 2", len(steps))
	}
	if steps[1].Op != "face" || steps[1].Param != "-90.00°" {
		t.Fatalf("step 1 = %+v, want Op=face Param=-90.00°", steps[1])
	}
	if steps[1].ParentID != steps[0].ID {
		t.Fatalf("step 1 ParentID = %d, want %d", steps[1].ParentID, steps[0].ID)
	}

	untraced := pose.NewPose(pose.Point{X: 100, Y: 72}, 382.75, nil).
		Advance("offset", 10).
		Face("down to ground", -90.0)
	assertNear(t, "untraced.X", untraced.Origin().X, faced.Origin().X, 1e-12)
	assertNear(t, "untraced.Y", untraced.Origin().Y, faced.Origin().Y, 1e-12)
	assertNear(t, "untraced.Heading", float64(untraced.HeadingDeg()), float64(faced.HeadingDeg()), 1e-12)

	// Deg.Normalized, Step.TurnDeg, and PoseSnapshot.Pose.
	assertNear(t, "Normalized(382.75)", float64(pose.Deg(382.75).Normalized()), 22.75, 1e-12)
	assertNear(t, "Normalized(-270)", float64(pose.Deg(-270).Normalized()), 90.0, 1e-12)
	assertNear(t, "Normalized(-180)", float64(pose.Deg(-180).Normalized()), 180.0, 1e-12)
	assertNear(t, "face TurnDeg", float64(steps[1].TurnDeg()), -112.75, 1e-12)

	turnTrace := pose.NewTrace()
	_ = pose.NewPose(pose.Point{}, 0, turnTrace).Turn("quarter", -90)
	turnSteps := turnTrace.Steps()
	assertNear(t, "turn TurnDeg", float64(turnSteps[0].TurnDeg()), -90.0, 1e-12)

	snapPt := steps[1].To.Pose().Advance("", 20).Origin()
	assertNear(t, "snapshot Advance X", snapPt.X, faced.Origin().X, 1e-12)
	assertNear(t, "snapshot Advance Y", snapPt.Y, faced.Origin().Y-20.0, 1e-12)
	if len(tr.Steps()) != 2 {
		t.Fatalf("snapshot Pose must not mutate trace, got %d steps", len(tr.Steps()))
	}
}

func TestPrimitivesDeflectionAndRightTriangleLeg(t *testing.T) {
	assertNear(t, "DeflectionOverSpan(0 deg)", pose.DeflectionOverSpan(250, 0), 0, 1e-12)
	assertNear(t, "DeflectionOverSpan(45 deg)", pose.DeflectionOverSpan(180, 45), 180, 1e-12)
	assertNear(t, "DeflectionOverSpan(-20 deg)", pose.DeflectionOverSpan(195, -20), -195*math.Tan(20*math.Pi/180), 1e-12)
	assertNear(t, "DeflectionOverSpan(0 span)", pose.DeflectionOverSpan(0, 20), 0, 1e-12)

	assertNear(t, "RightTriangleLeg(3-4-5)", pose.RightTriangleLeg(5, 3), 4, 1e-12)
	assertNear(t, "RightTriangleLeg(chainstay, bbDrop)", pose.RightTriangleLeg(450, 72), math.Sqrt(450*450-72*72), 1e-12)
}
