package geom2d

import (
	"math"
	"testing"
)

const eps = 1e-9

func assertAlmostEqual(t *testing.T, name string, actual, expected, tol float64) {
	t.Helper()
	if math.Abs(actual-expected) > tol {
		t.Errorf("%s = %v, expected %v (diff %v > tol %v)", name, actual, expected, math.Abs(actual-expected), tol)
	}
}

func TestAngle(t *testing.T) {
	a := FromDeg(90.0)
	assertAlmostEqual(t, "Deg()", a.Deg(), 90.0, eps)
	assertAlmostEqual(t, "Rad()", a.Rad(), math.Pi/2.0, eps)
	assertAlmostEqual(t, "Sin()", a.Sin(), 1.0, eps)
	assertAlmostEqual(t, "Cos()", a.Cos(), 0.0, eps)

	b := FromDeg(30.0)
	assertAlmostEqual(t, "Add()", a.Add(b).Deg(), 120.0, eps)
	assertAlmostEqual(t, "Sub()", a.Sub(b).Deg(), 60.0, eps)
	assertAlmostEqual(t, "Neg()", b.Neg().Deg(), -30.0, eps)
}

func TestAngleFromLift(t *testing.T) {
	// 500mm arm with 250mm lift = sin(theta) = 0.5 => 30 degrees
	ang, err := AngleFromLift(500.0, 250.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAlmostEqual(t, "AngleFromLift(500, 250)", ang.Deg(), 30.0, 1e-6)

	// Clamp when dy exceeds arm
	angClamp, err := AngleFromLift(100.0, 200.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAlmostEqual(t, "AngleFromLift clamped", angClamp.Deg(), 90.0, 1e-6)

	// Error on invalid arm
	_, err = AngleFromLift(0, 10)
	if err == nil {
		t.Errorf("expected error on arm <= 0")
	}
}

func TestSolveTriangleSSS(t *testing.T) {
	// 3-4-5 right triangle: angle opposite to hypotenuse 5 should be 90 degrees
	ang, err := SolveTriangleSSS(3.0, 4.0, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAlmostEqual(t, "SolveTriangleSSS 3-4-5", ang.Deg(), 90.0, 1e-6)

	// Equilateral triangle: all angles 60 degrees
	angEq, err := SolveTriangleSSS(10.0, 10.0, 10.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAlmostEqual(t, "SolveTriangleSSS 10-10-10", angEq.Deg(), 60.0, 1e-6)
}

func TestUnitFromPolar(t *testing.T) {
	// 60 deg polar: X = cos(60) = 0.5, Y = sin(60) = 0.866025
	v60 := UnitFromPolar(FromDeg(60.0))
	assertAlmostEqual(t, "v60.X", v60.X, 0.5, eps)
	assertAlmostEqual(t, "v60.Y", v60.Y, math.Sqrt(3)/2.0, eps)

	// 120 deg polar (60 deg from rear ground): X = -0.5, Y = 0.866025
	v120 := UnitFromPolar(FromDeg(120.0))
	assertAlmostEqual(t, "v120.X", v120.X, -0.5, eps)
	assertAlmostEqual(t, "v120.Y", v120.Y, math.Sqrt(3)/2.0, eps)
}

func TestVec2Operations(t *testing.T) {
	v := Vec2{X: 3, Y: 4}
	assertAlmostEqual(t, "Norm", v.Norm(), 5.0, eps)
	assertAlmostEqual(t, "NormSq", v.NormSq(), 25.0, eps)

	u := v.Unit()
	assertAlmostEqual(t, "Unit.X", u.X, 0.6, eps)
	assertAlmostEqual(t, "Unit.Y", u.Y, 0.8, eps)

	w := Vec2{X: 1, Y: 2}
	add := v.Add(w)
	assertAlmostEqual(t, "Add.X", add.X, 4.0, eps)
	assertAlmostEqual(t, "Add.Y", add.Y, 6.0, eps)

	sub := v.Sub(w)
	assertAlmostEqual(t, "Sub.X", sub.X, 2.0, eps)
	assertAlmostEqual(t, "Sub.Y", sub.Y, 2.0, eps)

	dot := v.Dot(w)
	assertAlmostEqual(t, "Dot", dot, 3*1+4*2, eps)

	// Rotate 90 degrees CCW: (3, 4) -> (-4, 3)
	rot := v.Rotate(FromDeg(90.0))
	assertAlmostEqual(t, "Rotate90.X", rot.X, -4.0, eps)
	assertAlmostEqual(t, "Rotate90.Y", rot.Y, 3.0, eps)

	// Perpendicular CCW: (-4, 3)
	perpCCW := v.Perpendicular(CounterClockwise)
	assertAlmostEqual(t, "PerpCCW.X", perpCCW.X, -4.0, eps)
	assertAlmostEqual(t, "PerpCCW.Y", perpCCW.Y, 3.0, eps)

	// Perpendicular CW: (4, -3)
	perpCW := v.Perpendicular(Clockwise)
	assertAlmostEqual(t, "PerpCW.X", perpCW.X, 4.0, eps)
	assertAlmostEqual(t, "PerpCW.Y", perpCW.Y, -3.0, eps)
}

func TestPoint2(t *testing.T) {
	p1 := Point2{X: 1, Y: 2}
	p2 := Point2{X: 4, Y: 6}

	dist := p1.DistanceTo(p2)
	assertAlmostEqual(t, "DistanceTo", dist, 5.0, eps)

	v := p2.Sub(p1)
	assertAlmostEqual(t, "p2 - p1 .X", v.X, 3.0, eps)
	assertAlmostEqual(t, "p2 - p1 .Y", v.Y, 4.0, eps)

	pTrans := p1.Add(v)
	assertAlmostEqual(t, "p1 + v .X", pTrans.X, p2.X, eps)
	assertAlmostEqual(t, "p1 + v .Y", pTrans.Y, p2.Y, eps)
}

func TestAxis2(t *testing.T) {
	// Head tube top at (400, 600) with HTA 68 degrees rearward (polar 180 - 68 = 112)
	headTubeTop := Point2{X: 400, Y: 600}
	steererAxis := NewAxisFromPolar(headTubeTop, FromDeg(180.0-68.0))

	// Step 50mm along steerer
	clamp := steererAxis.PointAt(50.0)
	expectedX := 400.0 - 50.0*math.Cos(68.0*math.Pi/180.0)
	expectedY := 600.0 + 50.0*math.Sin(68.0*math.Pi/180.0)
	assertAlmostEqual(t, "clamp.X", clamp.X, expectedX, eps)
	assertAlmostEqual(t, "clamp.Y", clamp.Y, expectedY, eps)

	// Intersect horizontal line Y = 700
	pt, ok := steererAxis.IntersectHorizontalLine(700.0)
	if !ok {
		t.Fatalf("expected intersection")
	}
	assertAlmostEqual(t, "intersect.Y", pt.Y, 700.0, eps)
	expectedIntersectX := 400.0 - (700.0-600.0)/math.Tan(68.0*math.Pi/180.0)
	assertAlmostEqual(t, "intersect.X", pt.X, expectedIntersectX, eps)
}

func TestTransform2AndRotationForLift(t *testing.T) {
	rearAxle := Point2{X: -450, Y: 70}
	frontAxle := Point2{X: 700, Y: 70}
	wheelbase := frontAxle.DistanceTo(rearAxle) // 1150 mm

	deltaY := 20.0
	transform, err := RotationForLift(rearAxle, frontAxle, deltaY)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newRearAxle := transform.Apply(rearAxle)
	assertAlmostEqual(t, "pivot unchanged X", newRearAxle.X, rearAxle.X, eps)
	assertAlmostEqual(t, "pivot unchanged Y", newRearAxle.Y, rearAxle.Y, eps)

	newFrontAxle := transform.Apply(frontAxle)
	// New front axle should be lifted by approximately deltaY
	assertAlmostEqual(t, "lifted front axle Y", newFrontAxle.Y, frontAxle.Y+deltaY, 0.2)
	// Wheelbase distance preserved
	assertAlmostEqual(t, "wheelbase preserved", newRearAxle.DistanceTo(newFrontAxle), wheelbase, eps)
}

func TestEllipse2(t *testing.T) {
	ellipse := Ellipse2{
		Center:     Point2{X: 100, Y: 200},
		SemiMinorA: 10.0,
		SemiMajorB: 20.0,
		Angle:      FromDeg(30.0),
	}

	// Center point should have r = 0 and be inside
	rCenter, inside := ellipse.Evaluate(ellipse.Center)
	assertAlmostEqual(t, "center r", rCenter, 0.0, eps)
	if !inside {
		t.Errorf("center should be inside")
	}

	// Distant point should be outside
	rFar, insideFar := ellipse.Evaluate(Point2{X: 500, Y: 500})
	if insideFar || rFar <= 1.0 {
		t.Errorf("distant point should be outside, r=%v", rFar)
	}
}
