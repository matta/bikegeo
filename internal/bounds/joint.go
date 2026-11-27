package bounds

import (
	"fmt"
	"math"

	"github.com/matta/bikegeo/internal/pose"
)

const slideJointUnitTol = 1e-9

// SlideJoint models a 1-DoF translational mating joint in a parent
// CoordinateSystem. Increasing the joint parameter by +1 mm shifts childSystem's
// datum origin (0, 0) along the unit-length displacement vector stepPerMm from
// originAtZero while keeping childSystem's +X orientation fixed at turnDeg in
// the parent CoordinateSystem.
//
// The zero value is invalid; construct instances with NewSlideJoint.
type SlideJoint struct {
	field        InputField
	allowed      BoundedRange
	originAtZero pose.Point
	stepPerMm    pose.Vector
	turnDeg      pose.Deg
	childSystem  pose.CoordinateSystem
	valid        bool
}

// NewSlideJoint constructs a validated 1-DoF translational SlideJoint placing
// childSystem in originAtZero's CoordinateSystem.
//
// Invariants enforced at construction (panics on violation):
//   - allowed must be valid.
//   - originAtZero and stepPerMm must share a valid parent CoordinateSystem.
//   - childSystem must be valid and distinct from the parent CoordinateSystem.
//   - stepPerMm must be unit-length (|stepPerMm.Norm() - 1| <= 1e-9), expressing
//     the parent-system displacement per +1 mm of parameter increase.
func NewSlideJoint(
	field InputField,
	allowed BoundedRange,
	originAtZero pose.Point,
	stepPerMm pose.Vector,
	turnDeg pose.Deg,
	childSystem pose.CoordinateSystem,
) SlideJoint {
	if !allowed.Valid() {
		panic("bounds: NewSlideJoint requires a valid BoundedRange")
	}
	parentSystem := originAtZero.CoordinateSystem()
	if !parentSystem.Valid() {
		panic("bounds: NewSlideJoint requires a valid parent CoordinateSystem on originAtZero")
	}
	if stepPerMm.CoordinateSystem() != parentSystem {
		panic(fmt.Sprintf("bounds: NewSlideJoint coordinate system mismatch (originAtZero %s vs stepPerMm %s)",
			parentSystem, stepPerMm.CoordinateSystem()))
	}
	if !childSystem.Valid() {
		panic("bounds: NewSlideJoint requires a valid child CoordinateSystem")
	}
	if childSystem == parentSystem {
		panic(fmt.Sprintf("bounds: NewSlideJoint requires distinct parent and child CoordinateSystems (%s)", parentSystem))
	}
	norm := stepPerMm.Norm()
	if !(math.Abs(norm-1) <= slideJointUnitTol) {
		panic(fmt.Sprintf("bounds: NewSlideJoint requires unit-length stepPerMm, got norm %v", norm))
	}
	return SlideJoint{
		field:        field,
		allowed:      allowed,
		originAtZero: originAtZero,
		stepPerMm:    stepPerMm,
		turnDeg:      turnDeg,
		childSystem:  childSystem,
		valid:        true,
	}
}

func (j SlideJoint) requireValid() {
	if !j.valid {
		panic("bounds: uninitialized zero-value SlideJoint")
	}
}

// Field returns the physical input field attributed to this joint.
func (j SlideJoint) Field() InputField {
	j.requireValid()
	return j.field
}

// Allowed returns the validated millimeter interval permitted for this joint.
func (j SlideJoint) Allowed() BoundedRange {
	j.requireValid()
	return j.allowed
}

// StepPerMm returns the unit-length displacement vector in ParentSystem() per
// +1 mm increase in the joint parameter.
func (j SlideJoint) StepPerMm() pose.Vector {
	j.requireValid()
	return j.stepPerMm
}

// ParentSystem returns the coordinate system in which the joint slides.
func (j SlideJoint) ParentSystem() pose.CoordinateSystem {
	j.requireValid()
	return j.originAtZero.CoordinateSystem()
}

// ChildSystem returns the coordinate system carried by the sliding child part.
func (j SlideJoint) ChildSystem() pose.CoordinateSystem {
	j.requireValid()
	return j.childSystem
}

// TransformAt returns the rigid Transform mapping ChildSystem() into
// ParentSystem() when the joint parameter is val mm, placing ChildSystem()'s
// datum origin at originAtZero + stepPerMm*val rotated by turnDeg.
//
// TransformAt does not enforce Allowed(); it is intended for internal solver
// probes and composition after validation.
func (j SlideJoint) TransformAt(val float64) pose.Transform {
	j.requireValid()
	origin := j.originAtZero.Add(j.stepPerMm.Scale(val))
	return pose.NewTransform(j.childSystem, j.originAtZero.CoordinateSystem(), origin, j.turnDeg)
}

// Bind validates val against Allowed() (returning an *InputError attributed to
// Field() if out of range or non-finite) and returns TransformAt(val) alongside
// the resulting BoundStatus readout.
func (j SlideJoint) Bind(val float64) (pose.Transform, BoundStatus, error) {
	j.requireValid()
	status, err := j.allowed.Bind(j.field, val)
	if err != nil {
		return pose.Transform{}, BoundStatus{}, err
	}
	return j.TransformAt(val), status, nil
}
