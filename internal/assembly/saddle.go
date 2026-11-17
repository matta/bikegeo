package assembly

import (
	"fmt"

	"bikegeo/internal/geom2d"
	"bikegeo/internal/model"
)

// SeatAxis represents the validated effective seat-tube axis on a leveled frame,
// rooted at the bottom bracket (or reference origin).
// Its local basis has +Y pointing upward along the seat axis and +X pointing
// forward perpendicular to the seat axis (so -X points rearward/downward).
type SeatAxis struct {
	seatPose pose2
	valid    bool
}

// NewSeatAxis is the root adapter that validates a finite origin and effective
// seat-tube angle in (0°, 90°) and constructs a checked SeatAxis.
func NewSeatAxis(bb geom2d.Point2, effStaDeg model.Deg) (SeatAxis, error) {
	if !isFinite(bb.X) || !isFinite(bb.Y) {
		return SeatAxis{}, fmt.Errorf("seat axis origin must be finite, got (%v, %v)", bb.X, bb.Y)
	}
	if !isFinite(float64(effStaDeg)) || effStaDeg <= 0 || effStaDeg >= 90 {
		return SeatAxis{}, fmt.Errorf("effective STA %.2f° must be in (0°, 90°)", effStaDeg)
	}
	// Upward seat axis is at polar angle (180° - effStaDeg); its forward normal (+X)
	// is 90° clockwise from the seat axis, at polar angle (90° - effStaDeg).
	p, err := newPose2(bb, (90.0 - effStaDeg).Angle())
	if err != nil {
		return SeatAxis{}, err
	}
	return SeatAxis{seatPose: p, valid: true}, nil
}

func (s SeatAxis) mustBeValid() {
	if !s.valid {
		panic("assembly: uninitialized or invalid SeatAxis")
	}
	s.seatPose.mustBeValid()
}

// AxisDirection returns the checked unit vector pointing upward along the seat axis.
func (s SeatAxis) AxisDirection() geom2d.Vec2 {
	s.mustBeValid()
	return s.seatPose.axisY
}

// WithSeatpostClamp locates the projected seatpost clamp at axialDistanceMm along
// the seat axis and postSetbackMm perpendicular rearward/downward from it.
// Its reference attitude is the leveled bicycle world frame (+X forward horizontal,
// +Y upward vertical), so saddle rail pitch is measured relative to world horizontal
// rather than inheriting the seat-axis rotation.
func (s SeatAxis) WithSeatpostClamp(axialDistanceMm, postSetbackMm float64) (SeatpostClampMount, error) {
	s.mustBeValid()
	if !isFinite(axialDistanceMm) || axialDistanceMm < 0 {
		return SeatpostClampMount{}, fmt.Errorf("seat-axis distance must be finite and non-negative, got %v mm", axialDistanceMm)
	}
	if !isFinite(postSetbackMm) || postSetbackMm < 0 {
		return SeatpostClampMount{}, fmt.Errorf("seatpost setback must be finite and non-negative, got %v mm", postSetbackMm)
	}
	nextPose, err := s.seatPose.translateLocal(localVec{x: -postSetbackMm, y: axialDistanceMm})
	if err != nil {
		return SeatpostClampMount{}, err
	}
	clampPose, err := newPose2(nextPose.origin, 0)
	if err != nil {
		return SeatpostClampMount{}, err
	}
	return SeatpostClampMount{
		clampPose: clampPose,
		axialDist: axialDistanceMm,
		valid:     true,
	}, nil
}

// SeatpostClampMount represents the checked projected seatpost clamp mount,
// ready to accept a Saddle.
type SeatpostClampMount struct {
	// Distinct field names prevent conversion to or from HeadTubeTop, StemBase, or BarClampMount.
	clampPose pose2
	axialDist float64
	valid     bool
}

func (m SeatpostClampMount) mustBeValid() {
	if !m.valid {
		panic("assembly: uninitialized or invalid SeatpostClampMount")
	}
	m.clampPose.mustBeValid()
	if !isFinite(m.axialDist) || m.axialDist < 0 {
		panic(fmt.Sprintf("assembly: invalid SeatpostClampMount axial distance %v", m.axialDist))
	}
}

// Saddle represents a validated saddle component in its own local coordinate frame
// (origin at rail center, +X forward along rails toward nose, +Y normal toward cover).
type Saddle struct {
	railCenterToBrpMm      float64
	railCenterToSitBonesMm float64
	sitBonesElevationMm    float64
	railPitchDeg           model.Deg
	valid                  bool
}

// NewSaddle validates saddle geometry and rail pitch and constructs a Saddle component definition.
func NewSaddle(spec model.SaddleSpec) (Saddle, error) {
	if !isFinite(spec.RailCenterToBrpMm) {
		return Saddle{}, fmt.Errorf("saddle rail-center-to-BRP offset must be finite, got %v mm", spec.RailCenterToBrpMm)
	}
	if !isFinite(spec.RailCenterToSitBonesMm) {
		return Saddle{}, fmt.Errorf("saddle rail-center-to-sit-bones offset must be finite, got %v mm", spec.RailCenterToSitBonesMm)
	}
	if !isFinite(spec.SitBonesElevationMm) {
		return Saddle{}, fmt.Errorf("saddle sit-bones elevation must be finite, got %v mm", spec.SitBonesElevationMm)
	}
	if !isFinite(float64(spec.RailPitchDeg)) {
		return Saddle{}, fmt.Errorf("saddle rail pitch must be finite, got %v°", spec.RailPitchDeg)
	}
	return Saddle{
		railCenterToBrpMm:      spec.RailCenterToBrpMm,
		railCenterToSitBonesMm: spec.RailCenterToSitBonesMm,
		sitBonesElevationMm:    spec.SitBonesElevationMm,
		railPitchDeg:           spec.RailPitchDeg,
		valid:                  true,
	}, nil
}

func (s Saddle) mustBeValid() {
	if !s.valid {
		panic("assembly: uninitialized or invalid Saddle")
	}
	if !isFinite(s.railCenterToBrpMm) || !isFinite(s.railCenterToSitBonesMm) ||
		!isFinite(s.sitBonesElevationMm) || !isFinite(float64(s.railPitchDeg)) {
		panic("assembly: non-finite Saddle local coordinates")
	}
}

func (s Saddle) railPoseOn(base pose2) (pose2, error) {
	s.mustBeValid()
	// Positive rail pitch raises the nose (counter-clockwise in +X/+Y).
	return base.rotateLocal(s.railPitchDeg.Angle())
}

// RailOffsetDirection returns the checked unit vector along which increasing
// railOffsetMm moves the saddle (rearward along the rails, in world coordinates).
func (s Saddle) RailOffsetDirection() geom2d.Vec2 {
	p, err := s.railPoseOn(identityPose2())
	if err != nil {
		panic(fmt.Sprintf("assembly: invalid saddle rail pose: %v", err))
	}
	return geom2d.Vec2{X: -p.axisX.X, Y: -p.axisX.Y}
}

func (s Saddle) railCenterPoseOn(base pose2, railOffsetMm float64) (pose2, geom2d.Point2, error) {
	s.mustBeValid()
	if !isFinite(railOffsetMm) {
		return pose2{}, geom2d.Point2{}, fmt.Errorf("saddle rail offset must be finite, got %v mm", railOffsetMm)
	}
	railPose, err := s.railPoseOn(base)
	if err != nil {
		return pose2{}, geom2d.Point2{}, err
	}
	railCenterPose, err := railPose.translateLocal(localVec{x: -railOffsetMm, y: 0})
	if err != nil {
		return pose2{}, geom2d.Point2{}, err
	}
	brp, err := railCenterPose.transformPoint(localVec{x: s.railCenterToBrpMm, y: 0})
	if err != nil {
		return pose2{}, geom2d.Point2{}, err
	}
	return railCenterPose, brp, nil
}

// ClampToBRP returns the world-coordinate displacement from the projected
// seatpost clamp datum to the saddle's BRP at railOffsetMm along the rails.
func (s Saddle) ClampToBRP(railOffsetMm float64) (geom2d.Vec2, error) {
	_, brp, err := s.railCenterPoseOn(identityPose2(), railOffsetMm)
	if err != nil {
		return geom2d.Vec2{}, err
	}
	return brp.VecFromOrigin(), nil
}

// AttachSaddle mounts saddle onto m at railOffsetMm (clamp station along the rails;
// positive offset moves the saddle body rearward relative to the clamp) and returns
// the immutable SaddlePlacement.
func (m SeatpostClampMount) AttachSaddle(saddle Saddle, railOffsetMm float64) (SaddlePlacement, error) {
	m.mustBeValid()
	railCenterPose, brp, err := saddle.railCenterPoseOn(m.clampPose, railOffsetMm)
	if err != nil {
		return SaddlePlacement{}, err
	}
	sitBones, err := railCenterPose.transformPoint(localVec{x: saddle.railCenterToSitBonesMm, y: saddle.sitBonesElevationMm})
	if err != nil {
		return SaddlePlacement{}, err
	}

	return SaddlePlacement{
		seatpostClamp:   m.clampPose.origin,
		railCenter:      railCenterPose.origin,
		brp:             brp,
		sitBones:        sitBones,
		axialDistanceMm: m.axialDist,
		valid:           true,
	}, nil
}

// SaddlePlacement is the immutable solved placement of the seatpost clamp and saddle.
type SaddlePlacement struct {
	seatpostClamp   geom2d.Point2
	railCenter      geom2d.Point2
	brp             geom2d.Point2
	sitBones        geom2d.Point2
	axialDistanceMm float64
	valid           bool
}

func (p SaddlePlacement) mustBeValid() {
	if !p.valid {
		panic("assembly: uninitialized or invalid SaddlePlacement")
	}
	if !isFinite(p.seatpostClamp.X) || !isFinite(p.seatpostClamp.Y) ||
		!isFinite(p.railCenter.X) || !isFinite(p.railCenter.Y) ||
		!isFinite(p.brp.X) || !isFinite(p.brp.Y) ||
		!isFinite(p.sitBones.X) || !isFinite(p.sitBones.Y) ||
		!isFinite(p.axialDistanceMm) {
		panic("assembly: non-finite SaddlePlacement landmarks")
	}
}

// SeatpostClamp returns the world coordinates of the projected seatpost clamp datum.
func (p SaddlePlacement) SeatpostClamp() geom2d.Point2 {
	p.mustBeValid()
	return p.seatpostClamp
}

// RailCenter returns the world coordinates of the saddle rail-center datum.
func (p SaddlePlacement) RailCenter() geom2d.Point2 {
	p.mustBeValid()
	return p.railCenter
}

// BRP returns the world coordinates of the saddle Biomechanical Reference Point.
func (p SaddlePlacement) BRP() geom2d.Point2 {
	p.mustBeValid()
	return p.brp
}

// SitBones returns the world coordinates of the modeled sit-bone support landmark.
func (p SaddlePlacement) SitBones() geom2d.Point2 {
	p.mustBeValid()
	return p.sitBones
}

// AxialDistanceMm returns the axial distance along the seat axis to the projected clamp datum.
func (p SaddlePlacement) AxialDistanceMm() float64 {
	p.mustBeValid()
	return p.axialDistanceMm
}
