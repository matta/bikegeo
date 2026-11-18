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
	assertNear(t, "afterFreeze.X", afterFreeze.Origin().X, traced.Origin().X+100*pose.Deg(30).Cos(), 1e-9)
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
