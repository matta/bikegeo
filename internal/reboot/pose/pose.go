// Package pose provides 2D planar poses, degree-valued angles, and optional
// construction tracing for reboot component assembly.
package pose

import (
	"fmt"
	"math"
)

const radPerDeg = math.Pi / 180.0

// Deg represents an angle in degrees with no implied coordinate system or plane.
// Coordinate and sign conventions belong to the Pose or component field using it.
type Deg float64

func degFromRad(rad float64) Deg {
	return Deg(rad / radPerDeg)
}

func (d Deg) rad() float64 {
	return float64(d) * radPerDeg
}

func (d Deg) sin() float64 {
	return math.Sin(d.rad())
}

func (d Deg) cos() float64 {
	return math.Cos(d.rad())
}

func (d Deg) tan() float64 {
	return math.Tan(d.rad())
}

// Normalized returns the angle wrapped into (-180°, +180°].
func (d Deg) Normalized() Deg {
	wrapped := math.Mod(float64(d), 360.0)
	if wrapped > 180.0 {
		wrapped -= 360.0
	} else if wrapped <= -180.0 {
		wrapped += 360.0
	}
	return Deg(wrapped)
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

// Pose returns an untraced Pose at this snapshot's origin and heading.
func (s PoseSnapshot) Pose() Pose {
	return NewPose(s.Origin, s.HeadingDeg, nil)
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

// TurnDeg returns the signed angular change of the step in degrees.
// For "turn" steps this is the exact relative delta; for "face" steps it is
// normalized to the shortest turn in (-180°, +180°].
func (s Step) TurnDeg() Deg {
	delta := s.To.HeadingDeg - s.From.HeadingDeg
	if s.Op == "face" {
		return delta.Normalized()
	}
	return delta
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

// Pose is an immutable 2D sagittal position and heading with an optional Trace sink.
// Headings are measured in degrees in the sagittal world frame (0° along +X forward,
// 90° along +Y up; positive angles rotate counter-clockwise).
type Pose struct {
	origin     Point
	headingDeg Deg
	trace      *Trace
	stepID     int
}

// NewPose creates a Pose at origin with sagittal world headingDeg (0° along +X,
// 90° along +Y, positive counter-clockwise), optionally attached to trace.
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

// HeadingDeg returns the Pose's sagittal world heading in degrees (0° along +X,
// 90° along +Y, positive counter-clockwise).
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
		X: p.origin.X + distMm*p.headingDeg.cos(),
		Y: p.origin.Y + distMm*p.headingDeg.sin(),
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
// (positive angles rotate counter-clockwise in the sagittal 2D plane).
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

// Face returns a new Pose at the same origin with sagittal world heading set
// directly to worldHeadingDeg (0° along +X, 90° along +Y, positive counter-clockwise).
func (p Pose) Face(label string, worldHeadingDeg Deg) Pose {
	next := Pose{
		origin:     p.origin,
		headingDeg: worldHeadingDeg,
		trace:      p.trace,
	}
	if p.trace != nil && !p.trace.frozen {
		param := fmt.Sprintf("%.2f°", worldHeadingDeg)
		next.stepID = p.trace.record(p.stepID, "face", label, param, p.snapshot(), next.snapshot())
	}
	return next
}

// Translate returns a new Pose moved by (localX, localY) in the current
// pose's local coordinate frame (+X along heading, +Y 90° counter-clockwise).
func (p Pose) Translate(label string, localX, localY float64) Pose {
	cos := p.headingDeg.cos()
	sin := p.headingDeg.sin()
	nextOrigin := Point{
		X: p.origin.X + localX*cos - localY*sin,
		Y: p.origin.Y + localX*sin + localY*cos,
	}
	next := Pose{
		origin:     nextOrigin,
		headingDeg: p.headingDeg,
		trace:      p.trace,
	}
	if p.trace != nil && !p.trace.frozen {
		param := fmt.Sprintf("(%.2f, %.2f) mm", localX, localY)
		next.stepID = p.trace.record(p.stepID, "translate", label, param, p.snapshot(), next.snapshot())
	}
	return next
}

// AngleTo returns the sagittal world angle in degrees (0° along +X, positive
// counter-clockwise) from p's origin to other's origin.
func (p Pose) AngleTo(other Pose) Deg {
	dx := other.origin.X - p.origin.X
	dy := other.origin.Y - p.origin.Y
	return degFromRad(math.Atan2(dy, dx))
}

// DistanceTo returns the Euclidean distance in millimeters between p and other.
func (p Pose) DistanceTo(other Pose) float64 {
	return math.Hypot(other.origin.X-p.origin.X, other.origin.Y-p.origin.Y)
}

// VerticalOffsetTo returns the signed vertical offset in millimeters from p to other
// (positive when other is above p along world +Y).
func (p Pose) VerticalOffsetTo(other Pose) float64 {
	return other.origin.Y - p.origin.Y
}
