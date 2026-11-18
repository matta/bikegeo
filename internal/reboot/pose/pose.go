// Package pose provides 2D planar poses, degree-valued angles, and optional
// construction tracing for reboot component assembly.
package pose

import (
	"fmt"
	"math"
)

// Deg represents an angle in degrees in the sagittal 2D plane (0° along +X,
// 90° along +Y; positive angles rotate counter-clockwise).
type Deg float64

// Rad converts the angle from degrees to radians.
func (d Deg) Rad() float64 {
	return float64(d) * math.Pi / 180.0
}

// Sin returns the sine of the angle.
func (d Deg) Sin() float64 {
	return math.Sin(d.Rad())
}

// Cos returns the cosine of the angle.
func (d Deg) Cos() float64 {
	return math.Cos(d.Rad())
}

// Point is a 2D position in millimeters in the sagittal plane (+X forward, +Y up).
type Point struct {
	X float64
	Y float64
}

// PoseSnapshot is a trace-free value snapshot of a Pose's position and heading.
type PoseSnapshot struct {
	Origin     Point
	HeadingDeg Deg
}

// Step records one Pose movement operation and its parent step ID.
type Step struct {
	ID       int
	ParentID int
	Op       string
	Label    string
	Param    string
	From     PoseSnapshot
	To       PoseSnapshot
}

// Trace collects Pose construction steps during a traced assembly evaluation.
type Trace struct {
	steps  []Step
	frozen bool
}

// NewTrace returns an empty, active Trace ready to record Pose movements.
func NewTrace() *Trace {
	return &Trace{}
}

// Freeze stops further step recording on this Trace. Callers should freeze
// the trace once assembly finishes so downstream rendering never mutates it.
func (t *Trace) Freeze() {
	if t != nil {
		t.frozen = true
	}
}

// Steps returns a copy of the recorded construction steps.
func (t *Trace) Steps() []Step {
	if t == nil || len(t.steps) == 0 {
		return nil
	}
	out := make([]Step, len(t.steps))
	copy(out, t.steps)
	return out
}

func (t *Trace) record(parentID int, op, label, param string, from, to PoseSnapshot) int {
	if t == nil || t.frozen {
		return 0
	}
	id := len(t.steps) + 1
	t.steps = append(t.steps, Step{
		ID:       id,
		ParentID: parentID,
		Op:       op,
		Label:    label,
		Param:    param,
		From:     from,
		To:       to,
	})
	return id
}

// Pose is an immutable 2D position and heading with an optional Trace sink.
type Pose struct {
	origin     Point
	headingDeg Deg
	trace      *Trace
	stepID     int
}

// NewPose creates a Pose at origin with headingDeg, optionally attached to trace.
func NewPose(origin Point, headingDeg Deg, trace *Trace) Pose {
	return Pose{
		origin:     origin,
		headingDeg: headingDeg,
		trace:      trace,
	}
}

// Origin returns the Pose's world position in millimeters.
func (p Pose) Origin() Point {
	return p.origin
}

// HeadingDeg returns the Pose's world heading in degrees.
func (p Pose) HeadingDeg() Deg {
	return p.headingDeg
}

// StepID returns the trace step ID that produced this Pose (0 if untraced or root).
func (p Pose) StepID() int {
	return p.stepID
}

func (p Pose) snapshot() PoseSnapshot {
	return PoseSnapshot{
		Origin:     p.origin,
		HeadingDeg: p.headingDeg,
	}
}

// Advance returns a new Pose moved distMm along the current heading.
func (p Pose) Advance(label string, distMm float64) Pose {
	nextOrigin := Point{
		X: p.origin.X + distMm*p.headingDeg.Cos(),
		Y: p.origin.Y + distMm*p.headingDeg.Sin(),
	}
	next := Pose{
		origin:     nextOrigin,
		headingDeg: p.headingDeg,
		trace:      p.trace,
	}
	if p.trace != nil && !p.trace.frozen {
		param := fmt.Sprintf("%.2f mm", distMm)
		next.stepID = p.trace.record(p.stepID, "advance", label, param, p.snapshot(), next.snapshot())
	}
	return next
}

// Turn returns a new Pose at the same origin with heading rotated by deltaDeg
// (positive angles rotate counter-clockwise).
func (p Pose) Turn(label string, deltaDeg Deg) Pose {
	next := Pose{
		origin:     p.origin,
		headingDeg: p.headingDeg + deltaDeg,
		trace:      p.trace,
	}
	if p.trace != nil && !p.trace.frozen {
		param := fmt.Sprintf("%+.2f°", deltaDeg)
		next.stepID = p.trace.record(p.stepID, "turn", label, param, p.snapshot(), next.snapshot())
	}
	return next
}
